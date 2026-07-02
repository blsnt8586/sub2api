package service

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const sub2apiOptimizeMaxWorkers = 3

const (
	// sub2apiOptimizeLeaderLockKey 多实例互斥：每分钟扫描只让一个实例执行，
	// 避免 N 个实例同时登录上游、并发切换同一账号分组。
	sub2apiOptimizeLeaderLockKey = "sub2api:optimize:runner:leader"
	// sub2apiOptimizeLeaderLockTTL 必须大于单轮扫描最坏耗时（内含 10 分钟优化超时），
	// 否则锁会在执行中途过期导致其他实例插队。
	sub2apiOptimizeLeaderLockTTL = 12 * time.Minute
)

// sub2apiOptimizeCronParser 与标准 5 字段 cron 对齐
var sub2apiOptimizeCronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// Sub2APIOptimizeRunnerService 每分钟扫描到期的定时优化配置并执行。
type Sub2APIOptimizeRunnerService struct {
	scheduleSvc *Sub2APIOptimizeScheduleService
	cfg         *config.Config

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once

	// 多实例 leader 选举：优先 Redis，回退 Postgres advisory lock；两者皆无则不设限。
	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

// NewSub2APIOptimizeRunnerService 创建实例
func NewSub2APIOptimizeRunnerService(
	scheduleSvc *Sub2APIOptimizeScheduleService,
	cfg *config.Config,
) *Sub2APIOptimizeRunnerService {
	return &Sub2APIOptimizeRunnerService{
		scheduleSvc: scheduleSvc,
		cfg:         cfg,
		instanceID:  uuid.NewString(),
	}
}

// SetLeaderLock 注入多实例 leader 选举所需的锁缓存与 DB。两者皆 nil 时任务不设限运行
// （单实例 / 测试场景）。
func (s *Sub2APIOptimizeRunnerService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// Start 启动 cron（每分钟触发一次扫描）
func (s *Sub2APIOptimizeRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(sub2apiOptimizeCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runDue() })
		if err != nil {
			logger.LegacyPrintf("service.sub2api_optimize_runner", "[Sub2APIOptimizeRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.sub2api_optimize_runner", "[Sub2APIOptimizeRunner] started (tick=every minute)")
	})
}

// Stop 优雅关闭 cron
func (s *Sub2APIOptimizeRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.sub2api_optimize_runner", "[Sub2APIOptimizeRunner] cron stop timed out")
			}
		}
	})
}

func (s *Sub2APIOptimizeRunnerService) runDue() {
	// 延迟 15s，让执行落在每分钟的 ~:15，避开账号定时测试的 :10
	time.Sleep(15 * time.Second)

	// 多实例选主：每个周期只有 leader 实例执行扫描，避免并发登录上游/重复切换分组。
	// 锁在本周期结束后立即释放，下一周期重新竞争，不会固定绑死在某个实例。
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, sub2apiOptimizeLeaderLockKey, s.instanceID, sub2apiOptimizeLeaderLockTTL)
	lockCancel()
	if !ok {
		return
	}
	defer release()

	// 优化任务涉及远端切换+测试，给足超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	now := time.Now()
	schedules, err := s.scheduleSvc.scheduleRepo.ListDue(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.sub2api_optimize_runner", "[Sub2APIOptimizeRunner] ListDue error: %v", err)
		return
	}
	if len(schedules) == 0 {
		return
	}

	logger.LegacyPrintf("service.sub2api_optimize_runner", "[Sub2APIOptimizeRunner] found %d due schedules", len(schedules))

	sem := make(chan struct{}, sub2apiOptimizeMaxWorkers)
	var wg sync.WaitGroup

	for _, sc := range schedules {
		sem <- struct{}{}
		wg.Add(1)
		go func(scheduleID, providerID int64) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := s.scheduleSvc.RunOptimize(ctx, scheduleID, providerID); err != nil {
				logger.LegacyPrintf("service.sub2api_optimize_runner", "[Sub2APIOptimizeRunner] schedule=%d RunOptimize error: %v", scheduleID, err)
			}
		}(sc.ID, sc.ProviderID)
	}

	wg.Wait()
}
