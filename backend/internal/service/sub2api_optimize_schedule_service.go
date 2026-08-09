package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

// ============================================================
// Repository 接口（由 repository 层实现）
// ============================================================

// Sub2APIOptimizeScheduleRepository 定义数据访问接口
type Sub2APIOptimizeScheduleRepository interface {
	GetByProviderID(ctx context.Context, providerID int64) (*ent.Sub2APIOptimizeSchedule, error)
	Upsert(ctx context.Context, input *UpsertOptimizeScheduleInput) (*ent.Sub2APIOptimizeSchedule, error)
	UpdateRunTimes(ctx context.Context, id int64, lastRun time.Time, nextRun time.Time) error
	Delete(ctx context.Context, providerID int64) error
	ListEnabled(ctx context.Context) ([]*ent.Sub2APIOptimizeSchedule, error)
	ListDue(ctx context.Context, now time.Time) ([]*ent.Sub2APIOptimizeSchedule, error)
	CreateLog(ctx context.Context, input *CreateOptimizeLogInput) (*ent.Sub2APIOptimizeLog, error)
	ListRecentLogs(ctx context.Context, scheduleID int64, limit int) ([]*ent.Sub2APIOptimizeLog, error)
}

// ============================================================
// Input / Output 类型
// ============================================================

// UpsertOptimizeScheduleInput 创建或更新定时配置
type UpsertOptimizeScheduleInput struct {
	ProviderID int64
	CronExpr   string
	Enabled    bool
	NextRunAt  *time.Time
}

// CreateOptimizeLogInput 写入运行日志
type CreateOptimizeLogInput struct {
	ScheduleID int64
	Status     string // success / partial / failed
	Total      int
	Optimized  int
	Skipped    int
	Failed     int
	Detail     []map[string]any
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// OptimizeScheduleInfo 返回给 handler/前端 的配置信息
type OptimizeScheduleInfo struct {
	ID         int64             `json:"id"`
	ProviderID int64             `json:"provider_id"`
	CronExpr   string            `json:"cron_expr"`
	Enabled    bool              `json:"enabled"`
	LastRunAt  *string           `json:"last_run_at,omitempty"`
	NextRunAt  *string           `json:"next_run_at,omitempty"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
	RecentLogs []OptimizeLogInfo `json:"recent_logs"`
}

// OptimizeLogInfo 单条运行日志
type OptimizeLogInfo struct {
	ID         int64            `json:"id"`
	Status     string           `json:"status"`
	Total      int              `json:"total"`
	Optimized  int              `json:"optimized"`
	Skipped    int              `json:"skipped"`
	Failed     int              `json:"failed"`
	Detail     []map[string]any `json:"detail,omitempty"`
	StartedAt  *string          `json:"started_at,omitempty"`
	FinishedAt *string          `json:"finished_at,omitempty"`
	CreatedAt  string           `json:"created_at"`
}

// ============================================================
// Service 结构体
// ============================================================

// Sub2APIOptimizeScheduleService 处理定时分组优化业务逻辑
type Sub2APIOptimizeScheduleService struct {
	scheduleRepo   Sub2APIOptimizeScheduleRepository
	providerSvc    *Sub2APIProviderService
	accountTestSvc *AccountTestService
	tokenCache     *sub2api.TokenCache

	// defaultTestModels 平台→默认测试模型（来自配置 sub2api.default_test_models）。
	// 为空时回退到内置兜底表 builtinDefaultTestModelByPlatform。
	defaultTestModels map[string]string

	// running 记录正在执行优化的 providerID，防止同一上游并发重跑
	// （用户狂点「立即执行」或与定时调度撞车）
	runningMu sync.Mutex
	running   map[int64]bool
}

// NewSub2APIOptimizeScheduleService 创建实例
func NewSub2APIOptimizeScheduleService(
	scheduleRepo Sub2APIOptimizeScheduleRepository,
	providerSvc *Sub2APIProviderService,
	accountTestSvc *AccountTestService,
	defaultTestModels map[string]string,
) *Sub2APIOptimizeScheduleService {
	return &Sub2APIOptimizeScheduleService{
		scheduleRepo:      scheduleRepo,
		providerSvc:       providerSvc,
		accountTestSvc:    accountTestSvc,
		tokenCache:        providerSvc.tokenCache,
		defaultTestModels: defaultTestModels,
		running:           make(map[int64]bool),
	}
}

// ============================================================
// CRUD
// ============================================================

// GetByProviderID 获取某上游的定时配置（含最近5条日志）
func (s *Sub2APIOptimizeScheduleService) GetByProviderID(ctx context.Context, providerID int64) (*OptimizeScheduleInfo, error) {
	schedule, err := s.scheduleRepo.GetByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("get schedule failed: %w", err)
	}
	if schedule == nil {
		return nil, nil
	}
	return s.toInfo(ctx, schedule)
}

// Upsert 创建或更新定时配置
func (s *Sub2APIOptimizeScheduleService) Upsert(ctx context.Context, providerID int64, cronExpr string, enabled bool) (*OptimizeScheduleInfo, error) {
	// 验证 cron 表达式
	sched, err := sub2apiOptimizeCronParser.Parse(cronExpr)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_CRON", fmt.Sprintf("invalid cron expression: %s", err.Error()))
	}

	nextRun := sched.Next(time.Now())
	result, err := s.scheduleRepo.Upsert(ctx, &UpsertOptimizeScheduleInput{
		ProviderID: providerID,
		CronExpr:   cronExpr,
		Enabled:    enabled,
		NextRunAt:  &nextRun,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert schedule failed: %w", err)
	}
	return s.toInfo(ctx, result)
}

// Delete 删除定时配置
func (s *Sub2APIOptimizeScheduleService) Delete(ctx context.Context, providerID int64) error {
	return s.scheduleRepo.Delete(ctx, providerID)
}

// UpdateAccountOptimizeSettings 更新单个账号的定时优化设置（是否参与 + 倍率下限 + 倍率上限 + 测试模型）。
// enabled 独立控制是否参与；minMultiplier/maxMultiplier/testModel 即使 enabled=false 也会持久化保留。
// minMultiplier 为 nil 表示无下限（从最便宜候选开始）；maxMultiplier 为 nil 表示未设上限；
// testModel 为 nil/空表示按平台默认模型测试。要求 min ≤ max，否则候选区间为空。
func (s *Sub2APIOptimizeScheduleService) UpdateAccountOptimizeSettings(ctx context.Context, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string) error {
	if maxMultiplier != nil && *maxMultiplier <= 0 {
		return infraerrors.BadRequest("INVALID_MAX_MULTIPLIER", "max_multiplier must be greater than 0")
	}
	if minMultiplier != nil && *minMultiplier < 0 {
		return infraerrors.BadRequest("INVALID_MIN_MULTIPLIER", "min_multiplier must not be negative")
	}
	// 下限 ≤ 上限，否则候选区间为空，配置无意义
	if minMultiplier != nil && maxMultiplier != nil && *minMultiplier > *maxMultiplier {
		return infraerrors.BadRequest("INVALID_MULTIPLIER_RANGE", "min_multiplier must not exceed max_multiplier")
	}
	// 参与定时优化时必须设置倍率上限、下限、测试模型
	if enabled {
		if maxMultiplier == nil {
			return infraerrors.BadRequest("MISSING_MAX_MULTIPLIER", "max_multiplier is required when optimize is enabled")
		}
		if minMultiplier == nil {
			return infraerrors.BadRequest("MISSING_MIN_MULTIPLIER", "min_multiplier is required when optimize is enabled")
		}
		if testModel == nil || *testModel == "" {
			return infraerrors.BadRequest("MISSING_TEST_MODEL", "test_model is required when optimize is enabled")
		}
	}
	return s.providerSvc.accountRepo.UpdateSub2APIOptimizeSettings(ctx, accountID, enabled, minMultiplier, maxMultiplier, testModel)
}

// RunNow 立即手动触发一次优化。优化涉及登录上游、逐账号切换分组并做模型连接测试，
// 耗时可能达数分钟，因此在后台 goroutine 中异步执行并立即返回当前配置；
// 前端应轮询 GetByProviderID 获取最新运行日志。
func (s *Sub2APIOptimizeScheduleService) RunNow(ctx context.Context, providerID int64) (*OptimizeScheduleInfo, error) {
	schedule, err := s.scheduleRepo.GetByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("get schedule failed: %w", err)
	}
	if schedule == nil {
		return nil, infraerrors.BadRequest("SCHEDULE_NOT_CONFIGURED", "请先保存定时优化配置后再执行")
	}

	scheduleID := schedule.ID

	// 同步抢锁：若该上游已有优化在跑（定时调度或上一次「立即执行」未结束），
	// 立即返回错误，避免前端轮询空等到超时。
	if !s.tryAcquire(providerID) {
		return nil, infraerrors.BadRequest("OPTIMIZE_RUNNING", "该上游已有优化任务正在执行，请稍后再试")
	}

	// 后台执行：用独立 context（不随 HTTP 请求结束而取消），给足超时。
	// 锁已在上面抢到，由 goroutine 负责释放。
	go func() {
		defer s.release(providerID)
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = s.runOptimizeLocked(bgCtx, scheduleID, providerID)
	}()

	// 立即返回当前配置（日志尚未产生，前端轮询获取）
	return s.toInfo(ctx, schedule)
}

// ============================================================
// 辅助
// ============================================================

// tryAcquire 尝试为 providerID 获取执行锁。若该上游已有优化在跑，返回 false。
func (s *Sub2APIOptimizeScheduleService) tryAcquire(providerID int64) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	if s.running[providerID] {
		return false
	}
	s.running[providerID] = true
	return true
}

// release 释放 providerID 的执行锁。
func (s *Sub2APIOptimizeScheduleService) release(providerID int64) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	delete(s.running, providerID)
}

// ============================================================
// 手动优化（单个 / 批量）
// ============================================================
// 手动优化与定时任务共用同一套智能引擎（optimizeAccounts）：
// 按倍率上限筛候选分组→逐个切换并实测连通→失败回滚，保证「倍率不超上限 + 模型联通」。
// 前置校验对所有触发方式一致：账号必须已开启参与、设置倍率上限、能解析出测试模型。

// checkOptimizeReady 校验单个账号是否满足优化前置条件（参与开关 + 倍率上限 + 可解析测试模型）。
// 任一不满足返回带具体原因的 BadRequest，供 handler 直接透传给前端提示用户先去配置。
func (s *Sub2APIOptimizeScheduleService) checkOptimizeReady(acc *Account) error {
	if acc.ProviderAPIKeyID == nil {
		return infraerrors.BadRequest("ACCOUNT_NO_REMOTE_KEY_ID", "账号未正确关联远端 Key，请重新关联后再优化")
	}
	if !acc.Sub2APIOptimizeEnabled {
		return infraerrors.BadRequest("OPTIMIZE_NOT_ENABLED", "请先开启该账号的「参与定时」开关后再优化")
	}
	if acc.Sub2APIMaxMultiplier == nil {
		return infraerrors.BadRequest("MISSING_MAX_MULTIPLIER", "请先设置该账号的倍率上限后再优化")
	}
	if s.resolveTestModel(acc) == "" {
		return infraerrors.BadRequest("MISSING_TEST_MODEL", fmt.Sprintf("平台 %s 无默认测试模型，请先为该账号设置测试模型后再优化", acc.Platform))
	}
	return nil
}

// OptimizeAccountManually 手动优化单个账号，走与定时任务一致的智能引擎并同步返回结果。
// 与 RunNow 不同，这里同步等待结果（单账号耗时可控），便于前端即时展示切换/测试结论。
func (s *Sub2APIOptimizeScheduleService) OptimizeAccountManually(ctx context.Context, providerID, accountID int64) (*OptimizeAccountDetail, error) {
	provider, err := s.providerSvc.repo.GetByID(ctx, providerID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider failed: %w", err)
	}

	// 用 ListByProviderID 取账号（而非 GetByID）：仅此查询路径会映射
	// sub2api_optimize_enabled / max_multiplier / test_model 三个优化字段，
	// GetByID 走的 accountsToService 不含这些列，会误判「未开启参与」。
	accounts, err := s.providerSvc.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("list accounts failed: %w", err)
	}
	var account *Account
	for i := range accounts {
		if accounts[i].ID == accountID {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		return nil, infraerrors.BadRequest("ACCOUNT_NOT_LINKED", "该账号未关联到此上游，请先关联")
	}
	if err := s.checkOptimizeReady(account); err != nil {
		return nil, err
	}

	// 抢锁，避免与定时调度或批量优化撞车导致对同一上游并发切换分组。
	if !s.tryAcquire(providerID) {
		return nil, infraerrors.BadRequest("OPTIMIZE_RUNNING", "该上游已有优化任务正在执行，请稍后再试")
	}
	defer s.release(providerID)

	details := s.optimizeAccounts(ctx, provider, []Account{*account})
	if len(details) == 0 {
		return nil, fmt.Errorf("optimize produced no result")
	}
	return &details[0], nil
}

// OptimizeAllManually 手动批量优化某上游下所有「已满足前置条件」的账号。
// 未开启参与 / 未设倍率上限 / 无法解析测试模型的账号会被跳过并计入 skipped 明细，
// 不阻断其余账号（与前端「批量优化」的期望一致）。
func (s *Sub2APIOptimizeScheduleService) OptimizeAllManually(ctx context.Context, providerID int64) ([]OptimizeAccountDetail, error) {
	provider, err := s.providerSvc.repo.GetByID(ctx, providerID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider failed: %w", err)
	}

	accounts, err := s.providerSvc.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("list accounts failed: %w", err)
	}

	// 分流：满足前置条件的进引擎优化，不满足的直接记 skipped 并给出原因。
	var ready []Account
	var skipped []OptimizeAccountDetail
	for i := range accounts {
		acc := accounts[i]
		if err := s.checkOptimizeReady(&acc); err != nil {
			skipped = append(skipped, OptimizeAccountDetail{
				AccountID:   acc.ID,
				AccountName: acc.Name,
				Status:      "skipped",
				Reason:      infraerrors.Message(err),
			})
			continue
		}
		ready = append(ready, acc)
	}

	if len(ready) == 0 {
		// 没有可优化账号：仅返回跳过明细（可能为空），不必抢锁。
		return skipped, nil
	}

	if !s.tryAcquire(providerID) {
		return nil, infraerrors.BadRequest("OPTIMIZE_RUNNING", "该上游已有优化任务正在执行，请稍后再试")
	}
	defer s.release(providerID)

	details := s.optimizeAccounts(ctx, provider, ready)
	return append(details, skipped...), nil
}

func (s *Sub2APIOptimizeScheduleService) toInfo(ctx context.Context, e *ent.Sub2APIOptimizeSchedule) (*OptimizeScheduleInfo, error) {
	info := &OptimizeScheduleInfo{
		ID:         e.ID,
		ProviderID: e.ProviderID,
		CronExpr:   e.CronExpr,
		Enabled:    e.Enabled,
		CreatedAt:  e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if e.LastRunAt != nil {
		t := e.LastRunAt.Format("2006-01-02T15:04:05Z07:00")
		info.LastRunAt = &t
	}
	if e.NextRunAt != nil {
		t := e.NextRunAt.Format("2006-01-02T15:04:05Z07:00")
		info.NextRunAt = &t
	}

	// 取最近5条日志
	logs, err := s.scheduleRepo.ListRecentLogs(ctx, e.ID, 5)
	if err == nil {
		for _, l := range logs {
			info.RecentLogs = append(info.RecentLogs, logToInfo(l))
		}
	}
	if info.RecentLogs == nil {
		info.RecentLogs = []OptimizeLogInfo{}
	}
	return info, nil
}

func logToInfo(l *ent.Sub2APIOptimizeLog) OptimizeLogInfo {
	li := OptimizeLogInfo{
		ID:        l.ID,
		Status:    l.Status,
		Total:     l.Total,
		Optimized: l.Optimized,
		Skipped:   l.Skipped,
		Failed:    l.Failed,
		Detail:    l.Detail,
		CreatedAt: l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if l.StartedAt != nil {
		t := l.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		li.StartedAt = &t
	}
	if l.FinishedAt != nil {
		t := l.FinishedAt.Format("2006-01-02T15:04:05Z07:00")
		li.FinishedAt = &t
	}
	return li
}
