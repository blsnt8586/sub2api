package service

import (
	"context"
	"fmt"
	"time"
)

// builtinDefaultTestModelByPlatform 是按平台的默认测试模型兜底表。
// 优先级：账号 sub2api_test_model > 配置 sub2api.default_test_models > 此内置表。
// 内置值仅作最终兜底，模型下线后可通过配置覆盖而无需改代码。
var builtinDefaultTestModelByPlatform = map[string]string{
	"anthropic": "claude-haiku-4-5-20251001",
	"openai":    "gpt-4o-mini",
	"gemini":    "gemini-1.5-flash",
}

// RunOptimize 执行一次完整的分组优化任务（供定时调度和手动触发调用）。
// 流程：取上游关联账号→登录一次→拉 groups+keys→逐账号切换+测试→写运行日志→更新下次运行时间。
func (s *Sub2APIOptimizeScheduleService) RunOptimize(ctx context.Context, scheduleID, providerID int64) error {
	// 并发去重：同一上游已有优化在执行时直接跳过（定时调度撞车）
	if !s.tryAcquire(providerID) {
		return nil
	}
	defer s.release(providerID)
	return s.runOptimizeLocked(ctx, scheduleID, providerID)
}

// runOptimizeLocked 是优化任务的核心逻辑，调用方须已持有 providerID 的执行锁。
// 拆分出来是为了让 RunNow 能在起后台 goroutine 之前同步抢锁并即时反馈「已有任务在跑」。
func (s *Sub2APIOptimizeScheduleService) runOptimizeLocked(ctx context.Context, scheduleID, providerID int64) error {
	startedAt := time.Now()

	// 取 Provider（含关联账号）
	provider, err := s.providerSvc.repo.GetByID(ctx, providerID)
	if err != nil {
		s.writeFailedLog(ctx, scheduleID, startedAt, fmt.Sprintf("get provider failed: %v", err))
		// 推进 next_run_at，避免临时 DB 故障导致每分钟无限重跑
		s.advanceNextRun(ctx, scheduleID, providerID, startedAt)
		return err
	}
	accounts, err := s.providerSvc.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		s.writeFailedLog(ctx, scheduleID, startedAt, fmt.Sprintf("list accounts failed: %v", err))
		s.advanceNextRun(ctx, scheduleID, providerID, startedAt)
		return err
	}

	details := s.doRunOptimize(ctx, provider, accounts)

	// 统计
	total := len(details)
	optimized, skipped, failed := 0, 0, 0
	for _, d := range details {
		switch d.Status {
		case "optimized":
			optimized++
		case "skipped":
			skipped++
		default:
			failed++
		}
	}

	status := "success"
	if failed > 0 && optimized == 0 {
		status = "failed"
	} else if failed > 0 {
		status = "partial"
	}

	// details -> []map[string]any
	detailMaps := make([]map[string]any, len(details))
	for i, d := range details {
		detailMaps[i] = map[string]any{
			"account_id":     d.AccountID,
			"account_name":   d.AccountName,
			"status":         d.Status,
			"old_group":      d.OldGroup,
			"new_group":      d.NewGroup,
			"old_multiplier": d.OldMult,
			"new_multiplier": d.NewMult,
			"reason":         d.Reason,
		}
	}

	finishedAt := time.Now()
	_, _ = s.scheduleRepo.CreateLog(ctx, &CreateOptimizeLogInput{
		ScheduleID: scheduleID,
		Status:     status,
		Total:      total,
		Optimized:  optimized,
		Skipped:    skipped,
		Failed:     failed,
		Detail:     detailMaps,
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	})

	// 更新下次运行时间
	s.advanceNextRun(ctx, scheduleID, providerID, startedAt)

	return nil
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

// writeFailedLog 在任务无法启动（登录/拉取失败）时写一条失败日志。
func (s *Sub2APIOptimizeScheduleService) writeFailedLog(ctx context.Context, scheduleID int64, startedAt time.Time, reason string) {
	finishedAt := time.Now()
	_, _ = s.scheduleRepo.CreateLog(ctx, &CreateOptimizeLogInput{
		ScheduleID: scheduleID,
		Status:     "failed",
		Detail:     []map[string]any{{"reason": reason}},
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	})
}

// sub2apiOptimizeTestAttempts 是连接测试的尝试次数。
// 便宜分组常被上游超卖，偶发 5xx/超时；重试后任一次成功即认为分组可用，
// 避免一次瞬时抖动就误判可用分组不可用、白白切到更贵的分组。
const sub2apiOptimizeTestAttempts = 2

// sub2apiOptimizeTestRetryDelay 是相邻两次测试之间的等待时间。
const sub2apiOptimizeTestRetryDelay = 2 * time.Second

// resolveTestModel 解析账号最终使用的测试模型。
// 优先级：账号 sub2api_test_model > 配置 sub2api.default_test_models > 内置兜底表。
// 返回空字符串表示无法解析（该平台既未配默认模型、账号也未单独设置）。
func (s *Sub2APIOptimizeScheduleService) resolveTestModel(acc *Account) string {
	if acc.Sub2APITestModel != nil && *acc.Sub2APITestModel != "" {
		return *acc.Sub2APITestModel
	}
	return s.defaultTestModelForPlatform(acc.Platform)
}

// testAccountModel 对账号执行连接测试（复用账号测试服务），失败会重试若干次。
// 测试模型优先级见 resolveTestModel。
func (s *Sub2APIOptimizeScheduleService) testAccountModel(ctx context.Context, acc *Account) error {
	testModel := s.resolveTestModel(acc)
	if testModel == "" {
		return fmt.Errorf("平台 %s 无默认测试模型，请在账号上设置测试模型或配置 sub2api.default_test_models", acc.Platform)
	}

	var lastErr error
	for attempt := 0; attempt < sub2apiOptimizeTestAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sub2apiOptimizeTestRetryDelay):
			}
		}
		result, err := s.accountTestSvc.RunTestBackground(ctx, acc.ID, testModel)
		if err != nil {
			lastErr = err
			continue
		}
		if result.Status == "success" {
			return nil
		}
		lastErr = fmt.Errorf("%s", result.ErrorMessage)
	}
	return lastErr
}

// defaultTestModelForPlatform 返回某平台的默认测试模型：配置优先，内置表兜底。
func (s *Sub2APIOptimizeScheduleService) defaultTestModelForPlatform(platform string) string {
	if m, ok := s.defaultTestModels[platform]; ok && m != "" {
		return m
	}
	return builtinDefaultTestModelByPlatform[platform]
}
