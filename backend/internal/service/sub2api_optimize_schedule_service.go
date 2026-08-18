package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
	"github.com/google/uuid"
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
	ListLogs(ctx context.Context, providerID int64, filter OptimizeLogFilter) ([]*ent.Sub2APIOptimizeLog, int64, error)
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
	ProviderID int64
	ScheduleID *int64
	Trigger    string
	Status     string // success / partial / failed / skipped
	Total      int
	Optimized  int
	Skipped    int
	Failed     int
	Detail     []map[string]any
	StartedAt  *time.Time
	FinishedAt *time.Time
}

const (
	OptimizeLogTriggerCron           = "cron"
	OptimizeLogTriggerScheduleNow    = "schedule_now"
	OptimizeLogTriggerProbeUnhealthy = "probe_unhealthy"
	OptimizeLogTriggerManualAccount  = "manual_account"
	OptimizeLogTriggerManualAll      = "manual_all"
	OptimizeLogTriggerLegacy         = "legacy"
)

var validOptimizeLogTriggers = map[string]struct{}{
	OptimizeLogTriggerCron:           {},
	OptimizeLogTriggerScheduleNow:    {},
	OptimizeLogTriggerProbeUnhealthy: {},
	OptimizeLogTriggerManualAccount:  {},
	OptimizeLogTriggerManualAll:      {},
	OptimizeLogTriggerLegacy:         {},
}

var validOptimizeLogStatuses = map[string]struct{}{
	"success": {},
	"partial": {},
	"failed":  {},
	"skipped": {},
}

// OptimizeLogFilter scopes Provider audit history without exposing logs from
// another Provider. Time bounds apply to the immutable log creation time.
type OptimizeLogFilter struct {
	Trigger   string
	Status    string
	AccountID *int64
	Keyword   string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
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
	ProviderID int64            `json:"provider_id"`
	ScheduleID *int64           `json:"schedule_id,omitempty"`
	Trigger    string           `json:"trigger"`
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

	// defaultTestModels 保留旧配置兼容；参与优化仍要求账号显式选择测试模型。
	defaultTestModels map[string]string

	// running 记录正在执行优化的 providerID，防止同一上游并发重跑
	// （用户狂点「立即执行」或与定时调度撞车）
	runningMu sync.Mutex
	running   map[int64]bool

	// 所有触发入口共享 provider 级分布式锁，避免多实例同时切换同一个远端 Key。
	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	// Redis 不可用或未配置时，单实例仍用本地到期时间执行探针联动冷却。
	// 正常多实例部署由 Redis TTL claim 提供跨实例冷却，本 map 仅作明确的单实例兜底。
	probeCooldownMu    sync.Mutex
	probeCooldownUntil map[int64]time.Time
	probeBindingSyncer Sub2APIProbeTargetBindingSyncer
}

// NewSub2APIOptimizeScheduleService 创建实例
func NewSub2APIOptimizeScheduleService(
	scheduleRepo Sub2APIOptimizeScheduleRepository,
	providerSvc *Sub2APIProviderService,
	accountTestSvc *AccountTestService,
	defaultTestModels map[string]string,
) *Sub2APIOptimizeScheduleService {
	return &Sub2APIOptimizeScheduleService{
		scheduleRepo:       scheduleRepo,
		providerSvc:        providerSvc,
		accountTestSvc:     accountTestSvc,
		tokenCache:         providerSvc.tokenCache,
		defaultTestModels:  defaultTestModels,
		running:            make(map[int64]bool),
		instanceID:         uuid.NewString(),
		probeCooldownUntil: make(map[int64]time.Time),
	}
}

// SetExecutionLock 注入跨实例互斥所需的 Redis/数据库锁后端。
func (s *Sub2APIOptimizeScheduleService) SetExecutionLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *Sub2APIOptimizeScheduleService) SetProbeTargetBindingSyncer(syncer Sub2APIProbeTargetBindingSyncer) {
	if s != nil {
		s.probeBindingSyncer = syncer
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

// ListLogs returns Provider-owned optimization audit history. Schedule is only
// an optional origin reference, so deleting a schedule never hides the logs.
func (s *Sub2APIOptimizeScheduleService) ListLogs(ctx context.Context, providerID int64, filter OptimizeLogFilter) ([]OptimizeLogInfo, int64, error) {
	filter.Trigger = strings.TrimSpace(filter.Trigger)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	if filter.Trigger != "" {
		if _, ok := validOptimizeLogTriggers[filter.Trigger]; !ok {
			return nil, 0, infraerrors.BadRequest("INVALID_OPTIMIZE_LOG_TRIGGER", "invalid optimize log trigger")
		}
	}
	if filter.Status != "" {
		if _, ok := validOptimizeLogStatuses[filter.Status]; !ok {
			return nil, 0, infraerrors.BadRequest("INVALID_OPTIMIZE_LOG_STATUS", "invalid optimize log status")
		}
	}
	if filter.AccountID != nil && *filter.AccountID <= 0 {
		return nil, 0, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "account_id must be greater than 0")
	}
	if len(filter.Keyword) > 200 {
		return nil, 0, infraerrors.BadRequest("OPTIMIZE_LOG_KEYWORD_TOO_LONG", "keyword must not exceed 200 characters")
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return nil, 0, infraerrors.BadRequest("INVALID_OPTIMIZE_LOG_TIME_RANGE", "from must not be after to")
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	logs, total, err := s.scheduleRepo.ListLogs(ctx, providerID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list optimize logs failed: %w", err)
	}
	items := make([]OptimizeLogInfo, 0, len(logs))
	for _, log := range logs {
		items = append(items, logToInfo(log))
	}
	return items, total, nil
}

// UpdateAccountOptimizeSettings 更新单个账号的定时优化设置（是否参与 + 倍率下限 + 倍率上限 + 测试模型）。
// enabled 独立控制是否参与；minMultiplier/maxMultiplier/testModel 即使 enabled=false 也会持久化保留。
// 开启参与时倍率下限、倍率上限、测试模型三项都必须明确填写。
// 关闭参与时仍保留这些配置，便于用户补齐后重新开启。
func (s *Sub2APIOptimizeScheduleService) UpdateAccountOptimizeSettings(ctx context.Context, providerID, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string) error {
	if testModel != nil {
		trimmed := strings.TrimSpace(*testModel)
		if trimmed == "" {
			testModel = nil
		} else {
			testModel = &trimmed
		}
	}
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
	err := s.providerSvc.accountRepo.UpdateSub2APIOptimizeSettings(ctx, providerID, accountID, enabled, minMultiplier, maxMultiplier, testModel)
	if errors.Is(err, sql.ErrNoRows) {
		return infraerrors.BadRequest("ACCOUNT_NOT_LINKED", "该账号未关联到此上游，请重新打开账号面板后再操作")
	}
	return err
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
	startedAt := time.Now()
	release, ok := s.tryAcquire(ctx, providerID)
	if !ok {
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if logErr := s.persistOptimizeDeferredLog(
			logCtx,
			providerID,
			&scheduleID,
			OptimizeLogTriggerScheduleNow,
			startedAt,
			"同一上游已有优化任务正在执行，本次立即执行已让位",
			nil,
			nil,
		); logErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist deferred schedule-now log failed: %v", providerID, logErr)
		}
		cancel()
		return nil, infraerrors.BadRequest("OPTIMIZE_RUNNING", "该上游已有优化任务正在执行，请稍后再试")
	}

	// 后台执行：用独立 context（不随 HTTP 请求结束而取消），给足超时。
	// 锁已在上面抢到，由 goroutine 负责释放。
	go func() {
		defer release()
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = s.runOptimizeLocked(bgCtx, scheduleID, providerID, OptimizeLogTriggerScheduleNow)
	}()

	// 立即返回当前配置（日志尚未产生，前端轮询获取）
	return s.toInfo(ctx, schedule)
}

// ============================================================
// 辅助
// ============================================================

// tryAcquire 同时获取进程内锁和 provider 级分布式锁。
// 返回的 release 可安全调用一次；任何一步失败都会释放已持有的本地状态。
func (s *Sub2APIOptimizeScheduleService) tryAcquire(ctx context.Context, providerID int64) (func(), bool) {
	s.runningMu.Lock()
	if s.running[providerID] {
		s.runningMu.Unlock()
		return nil, false
	}
	s.running[providerID] = true
	s.runningMu.Unlock()

	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	key := fmt.Sprintf("sub2api:optimize:provider:%d", providerID)
	distributedRelease, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, key, s.instanceID, sub2apiOptimizeLeaderLockTTL)
	cancel()
	if !ok {
		s.releaseLocal(providerID)
		return nil, false
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			distributedRelease()
			s.releaseLocal(providerID)
		})
	}, true
}

func (s *Sub2APIOptimizeScheduleService) releaseLocal(providerID int64) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	delete(s.running, providerID)
}

// ============================================================
// 手动优化（单个 / 批量）
// ============================================================
// 手动优化与定时任务共用同一套智能引擎（optimizeAccounts）：
// 按倍率上限筛候选分组→逐个切换并实测连通→失败回滚，保证「倍率不超上限 + 模型联通」。
// 前置校验对所有触发方式一致：账号必须已开启参与并填写上下限与测试模型。

// checkOptimizeReady 校验单个账号是否满足优化前置条件。
// 任一不满足返回带具体原因的 BadRequest，供 handler 直接透传给前端提示用户先去配置。
func (s *Sub2APIOptimizeScheduleService) checkOptimizeReady(acc *Account) error {
	if !acc.Sub2APIOptimizeEnabled {
		return infraerrors.BadRequest("OPTIMIZE_NOT_ENABLED", "请先开启该账号的「参与定时优化」开关后再优化")
	}
	if acc.ProviderAPIKeyID == nil {
		return infraerrors.BadRequest("ACCOUNT_NO_REMOTE_KEY_ID", "账号未正确关联远端 Key，请重新关联后再优化")
	}
	if acc.Sub2APIMaxMultiplier == nil {
		return infraerrors.BadRequest("MISSING_MAX_MULTIPLIER", "请先设置该账号的倍率上限后再优化")
	}
	if acc.Sub2APIMinMultiplier == nil {
		return infraerrors.BadRequest("MISSING_MIN_MULTIPLIER", "请先设置该账号的倍率下限后再优化")
	}
	if acc.Sub2APITestModel == nil || strings.TrimSpace(*acc.Sub2APITestModel) == "" {
		return infraerrors.BadRequest("MISSING_TEST_MODEL", "请先为该账号设置测试模型后再优化")
	}
	return nil
}

// optimizeAccountConfigError 是定时任务和手动优化共享的资格规则。
// 只有开启参与且四项前置条件全部满足的账号才能触发远端分组切换。
func optimizeAccountConfigError(acc *Account) string {
	if acc == nil {
		return "账号不存在"
	}
	if acc.ProviderAPIKeyID == nil {
		return "账号未正确关联远端 Key，请重新关联后再优化"
	}
	if acc.Sub2APIMaxMultiplier == nil {
		return "缺少倍率上限，请补充配置后重新开启参与定时优化"
	}
	if acc.Sub2APIMinMultiplier == nil {
		return "缺少倍率下限，请补充配置后重新开启参与定时优化"
	}
	if acc.Sub2APITestModel == nil || strings.TrimSpace(*acc.Sub2APITestModel) == "" {
		return "缺少测试模型，请补充配置后重新开启参与定时优化"
	}
	return ""
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
	startedAt := time.Now()

	// 抢锁，避免与定时调度或批量优化撞车导致对同一上游并发切换分组。
	release, ok := s.tryAcquire(ctx, providerID)
	if !ok {
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if logErr := s.persistOptimizeDeferredLog(
			logCtx,
			providerID,
			nil,
			OptimizeLogTriggerManualAccount,
			startedAt,
			"同一上游已有优化任务正在执行，本次单账号手动优化已让位",
			[]OptimizeAccountDetail{{AccountID: account.ID, AccountName: account.Name, Status: "skipped"}},
			nil,
		); logErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d account=%d persist deferred manual log failed: %v", providerID, accountID, logErr)
		}
		cancel()
		return nil, infraerrors.BadRequest("OPTIMIZE_RUNNING", "该上游已有优化任务正在执行，请稍后再试")
	}
	defer release()

	details := s.optimizeAccounts(ctx, provider, []Account{*account})
	if len(details) == 0 {
		return nil, fmt.Errorf("optimize produced no result")
	}
	logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	if err := s.persistOptimizeLog(logCtx, providerID, nil, OptimizeLogTriggerManualAccount, startedAt, details, nil); err != nil {
		logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d account=%d persist manual log failed: %v", providerID, accountID, err)
	}
	cancel()
	return &details[0], nil
}

// OptimizeAllManually 手动批量优化某上游下所有「已满足前置条件」的账号。
// 只处理已经开启参与的账号；未参与账号不属于本次批量任务。
func (s *Sub2APIOptimizeScheduleService) OptimizeAllManually(ctx context.Context, providerID int64) ([]OptimizeAccountDetail, error) {
	startedAt := time.Now()
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

	// 分流：满足前置条件的进引擎优化；已开启但配置非法的明确记失败。
	var ready []Account
	var invalid []OptimizeAccountDetail
	for i := range accounts {
		acc := accounts[i]
		if !acc.Sub2APIOptimizeEnabled {
			continue
		}
		if err := s.checkOptimizeReady(&acc); err != nil {
			invalid = append(invalid, OptimizeAccountDetail{
				AccountID:   acc.ID,
				AccountName: acc.Name,
				Status:      "failed",
				Reason:      infraerrors.Message(err),
			})
			continue
		}
		ready = append(ready, acc)
	}

	if len(ready) == 0 {
		// Even an empty or invalid batch is an operator-visible audit attempt.
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if logErr := s.persistOptimizeLog(logCtx, providerID, nil, OptimizeLogTriggerManualAll, startedAt, invalid, nil); logErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist empty manual-all log failed: %v", providerID, logErr)
		}
		cancel()
		return invalid, nil
	}

	release, ok := s.tryAcquire(ctx, providerID)
	if !ok {
		deferred := make([]OptimizeAccountDetail, 0, len(ready))
		for _, account := range ready {
			deferred = append(deferred, OptimizeAccountDetail{AccountID: account.ID, AccountName: account.Name, Status: "skipped"})
		}
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if logErr := s.persistOptimizeDeferredLog(
			logCtx,
			providerID,
			nil,
			OptimizeLogTriggerManualAll,
			startedAt,
			"同一上游已有优化任务正在执行，本次批量手动优化已让位",
			deferred,
			nil,
		); logErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist deferred manual-all log failed: %v", providerID, logErr)
		}
		cancel()
		return nil, infraerrors.BadRequest("OPTIMIZE_RUNNING", "该上游已有优化任务正在执行，请稍后再试")
	}
	defer release()

	details := append(s.optimizeAccounts(ctx, provider, ready), invalid...)
	logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	if err := s.persistOptimizeLog(logCtx, providerID, nil, OptimizeLogTriggerManualAll, startedAt, details, nil); err != nil {
		logger.LegacyPrintf("service.sub2api_optimize", "[Sub2APIOptimize] provider=%d persist manual-all log failed: %v", providerID, err)
	}
	cancel()
	return details, nil
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

	// Keep the legacy embedded preview for older clients. New clients use the
	// Provider-level paginated endpoint so history survives schedule deletion.
	logs, _, err := s.scheduleRepo.ListLogs(ctx, e.ProviderID, OptimizeLogFilter{Page: 1, PageSize: 5})
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
		ID:         l.ID,
		ProviderID: l.ProviderID,
		ScheduleID: l.ScheduleID,
		Trigger:    l.Trigger,
		Status:     l.Status,
		Total:      l.Total,
		Optimized:  l.Optimized,
		Skipped:    l.Skipped,
		Failed:     l.Failed,
		Detail:     l.Detail,
		CreatedAt:  l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
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
