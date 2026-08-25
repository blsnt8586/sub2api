package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// RunOptimize 执行一次完整的分组优化任务（供定时调度和手动触发调用）。
// 流程：取上游关联账号→登录一次→拉 groups+keys→逐账号切换+测试→写运行日志→更新下次运行时间。
func (s *Sub2APIOptimizeScheduleService) RunOptimize(ctx context.Context, scheduleID, providerID int64) error {
	// 并发去重：同一上游已有优化在执行时直接跳过（定时调度撞车）
	startedAt := time.Now()
	release, ok := s.tryAcquire(ctx, providerID)
	if !ok {
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := s.persistOptimizeDeferredLog(
			logCtx,
			providerID,
			&scheduleID,
			OptimizeLogTriggerCron,
			startedAt,
			"同一上游已有优化任务正在执行，本次定时触发已让位；计划保持到期并在下一轮重试",
			nil,
			nil,
		); err != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist deferred cron log failed: %v", providerID, err)
		}
		return nil
	}
	defer release()
	return s.runOptimizeLocked(ctx, scheduleID, providerID, OptimizeLogTriggerCron)
}

// runOptimizeLocked 是优化任务的核心逻辑，调用方须已持有 providerID 的执行锁。
// 拆分出来是为了让 RunNow 能在起后台 goroutine 之前同步抢锁并即时反馈「已有任务在跑」。
func (s *Sub2APIOptimizeScheduleService) runOptimizeLocked(ctx context.Context, scheduleID, providerID int64, trigger string) error {
	startedAt := time.Now()
	scheduleRef := &scheduleID

	// 取 Provider（含关联账号）
	provider, err := s.providerSvc.repo.GetByID(ctx, providerID)
	if err != nil {
		if logErr := s.persistOptimizeFailureLog(ctx, providerID, scheduleRef, trigger, startedAt, fmt.Sprintf("get provider failed: %v", err)); logErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist failure log failed: %v", providerID, logErr)
		}
		// 推进 next_run_at，避免临时 DB 故障导致每分钟无限重跑
		s.advanceNextRun(ctx, scheduleID, providerID, startedAt)
		return err
	}
	accounts, err := s.providerSvc.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		if logErr := s.persistOptimizeFailureLog(ctx, providerID, scheduleRef, trigger, startedAt, fmt.Sprintf("list accounts failed: %v", err)); logErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist failure log failed: %v", providerID, logErr)
		}
		s.advanceNextRun(ctx, scheduleID, providerID, startedAt)
		return err
	}

	var details []OptimizeAccountDetail
	var extraByAccount map[int64]map[string]any
	if trigger == OptimizeLogTriggerCron {
		covered, coverageErr := s.recentCoveredAccounts(ctx, providerID, startedAt)
		if coverageErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d load recent account coverage failed: %v", providerID, coverageErr)
		}
		details, extraByAccount = s.doRunCronOptimize(ctx, provider, accounts, covered)
	} else {
		details = s.doRunOptimize(ctx, provider, accounts)
	}
	if logErr := s.persistOptimizeLog(ctx, providerID, scheduleRef, trigger, startedAt, details, extraByAccount); logErr != nil {
		logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist run log failed: %v", providerID, logErr)
	}

	// 更新下次运行时间
	s.advanceNextRun(ctx, scheduleID, providerID, startedAt)

	return nil
}

func optimizeRunStatus(total, optimized, failed int) string {
	if total == 0 {
		return "skipped"
	}
	if failed > 0 && optimized == 0 {
		return "failed"
	}
	if failed > 0 {
		return "partial"
	}
	return "success"
}

// advanceNextRun 依据 schedule 的 cron 表达式推进 last_run_at/next_run_at。
// 无论任务成功还是失败都应调用，避免 next_run_at 停滞导致 ListDue 每分钟重复命中。
// scheduleID<=0（手动 RunNow 且未配置定时）时直接跳过。
func (s *Sub2APIOptimizeScheduleService) advanceNextRun(ctx context.Context, scheduleID, providerID int64, ranAt time.Time) {
	if scheduleID <= 0 {
		return
	}
	schedule, err := s.scheduleRepo.GetByProviderID(ctx, providerID)
	if err != nil || schedule == nil {
		return
	}
	sched, perr := sub2apiOptimizeCronParser.Parse(schedule.CronExpr)
	if perr != nil {
		return
	}
	_ = s.scheduleRepo.UpdateRunTimes(ctx, scheduleID, ranAt, sched.Next(time.Now()))
}

// sub2apiOptimizeTestAttempts 是连接测试的尝试次数。
// 便宜分组常被上游超卖，偶发 5xx/超时；重试后任一次成功即认为分组可用，
// 避免一次瞬时抖动就误判可用分组不可用、白白切到更贵的分组。
const sub2apiOptimizeTestAttempts = 2

// sub2apiOptimizeTestRetryDelay 是相邻两次测试之间的等待时间。
const sub2apiOptimizeTestRetryDelay = 2 * time.Second

// testAccountModel 对账号执行连接测试（复用账号测试服务），失败会重试若干次。
func (s *Sub2APIOptimizeScheduleService) testAccountModel(ctx context.Context, acc *Account) error {
	_, err := s.testAccountModelResult(ctx, acc)
	return err
}

func (s *Sub2APIOptimizeScheduleService) testAccountModelResult(ctx context.Context, acc *Account) (*ScheduledTestResult, error) {
	if acc == nil || acc.Sub2APITestModel == nil || strings.TrimSpace(*acc.Sub2APITestModel) == "" {
		return nil, fmt.Errorf("账号未设置测试模型")
	}
	testModel := strings.TrimSpace(*acc.Sub2APITestModel)

	var lastErr error
	var lastResult *ScheduledTestResult
	for attempt := 0; attempt < sub2apiOptimizeTestAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sub2apiOptimizeTestRetryDelay):
			}
		}
		var result *ScheduledTestResult
		var err error
		if accountTestStatusMutationAllowed(ctx) {
			result, err = s.accountTestSvc.RunTestBackground(ctx, acc.ID, testModel)
		} else {
			result, err = s.accountTestSvc.RunProbeTestBackground(ctx, acc.ID, testModel)
		}
		if err != nil {
			lastErr = err
			continue
		}
		lastResult = result
		if result.Status == "success" {
			return result, nil
		}
		lastErr = fmt.Errorf("%s", result.ErrorMessage)
	}
	return lastResult, lastErr
}
