package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/sub2apioptimizelog"
	"github.com/Wei-Shaw/sub2api/ent/sub2apioptimizeschedule"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Sub2APIOptimizeScheduleRepository 定时优化配置数据访问层
type Sub2APIOptimizeScheduleRepository struct {
	client *ent.Client
}

// NewSub2APIOptimizeScheduleRepository 创建实例
func NewSub2APIOptimizeScheduleRepository(client *ent.Client) service.Sub2APIOptimizeScheduleRepository {
	return &Sub2APIOptimizeScheduleRepository{client: client}
}

// GetByProviderID 获取指定 Provider 的定时配置（不存在返回 nil, nil）
func (r *Sub2APIOptimizeScheduleRepository) GetByProviderID(ctx context.Context, providerID int64) (*ent.Sub2APIOptimizeSchedule, error) {
	s, err := r.client.Sub2APIOptimizeSchedule.Query().
		Where(sub2apioptimizeschedule.ProviderID(providerID)).
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return s, err
}

// Upsert 创建或更新定时配置
func (r *Sub2APIOptimizeScheduleRepository) Upsert(ctx context.Context, input *service.UpsertOptimizeScheduleInput) (*ent.Sub2APIOptimizeSchedule, error) {
	existing, err := r.GetByProviderID(ctx, input.ProviderID)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		// 创建
		b := r.client.Sub2APIOptimizeSchedule.Create().
			SetProviderID(input.ProviderID).
			SetCronExpr(input.CronExpr).
			SetEnabled(input.Enabled)
		if input.NextRunAt != nil {
			b = b.SetNextRunAt(*input.NextRunAt)
		}
		return b.Save(ctx)
	}

	// 更新
	u := existing.Update().
		SetCronExpr(input.CronExpr).
		SetEnabled(input.Enabled).
		SetUpdatedAt(time.Now())
	if input.NextRunAt != nil {
		u = u.SetNextRunAt(*input.NextRunAt)
	}
	return u.Save(ctx)
}

// UpdateRunTimes 更新上次/下次运行时间（任务执行后调用）
func (r *Sub2APIOptimizeScheduleRepository) UpdateRunTimes(ctx context.Context, id int64, lastRun time.Time, nextRun time.Time) error {
	return r.client.Sub2APIOptimizeSchedule.UpdateOneID(id).
		SetLastRunAt(lastRun).
		SetNextRunAt(nextRun).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
}

// Delete 删除定时配置
func (r *Sub2APIOptimizeScheduleRepository) Delete(ctx context.Context, providerID int64) error {
	_, err := r.client.Sub2APIOptimizeSchedule.Delete().
		Where(sub2apioptimizeschedule.ProviderID(providerID)).
		Exec(ctx)
	return err
}

// ListEnabled 列出所有已启用的定时配置（定时运行器扫描用）
func (r *Sub2APIOptimizeScheduleRepository) ListEnabled(ctx context.Context) ([]*ent.Sub2APIOptimizeSchedule, error) {
	return r.client.Sub2APIOptimizeSchedule.Query().
		Where(sub2apioptimizeschedule.Enabled(true)).
		All(ctx)
}

// ListDue 列出所有已启用且到期（next_run_at <= now 或为空）的定时配置
func (r *Sub2APIOptimizeScheduleRepository) ListDue(ctx context.Context, now time.Time) ([]*ent.Sub2APIOptimizeSchedule, error) {
	return r.client.Sub2APIOptimizeSchedule.Query().
		Where(
			sub2apioptimizeschedule.Enabled(true),
			sub2apioptimizeschedule.Or(
				sub2apioptimizeschedule.NextRunAtIsNil(),
				sub2apioptimizeschedule.NextRunAtLTE(now),
			),
		).
		All(ctx)
}

// CreateLog 写入一条运行日志
func (r *Sub2APIOptimizeScheduleRepository) CreateLog(ctx context.Context, input *service.CreateOptimizeLogInput) (*ent.Sub2APIOptimizeLog, error) {
	b := r.client.Sub2APIOptimizeLog.Create().
		SetScheduleID(input.ScheduleID).
		SetStatus(input.Status).
		SetTotal(input.Total).
		SetOptimized(input.Optimized).
		SetSkipped(input.Skipped).
		SetFailed(input.Failed)
	if input.Detail != nil {
		b = b.SetDetail(input.Detail)
	}
	if input.StartedAt != nil {
		b = b.SetStartedAt(*input.StartedAt)
	}
	if input.FinishedAt != nil {
		b = b.SetFinishedAt(*input.FinishedAt)
	}
	return b.Save(ctx)
}

// ListRecentLogs 取最近 N 条日志
func (r *Sub2APIOptimizeScheduleRepository) ListRecentLogs(ctx context.Context, scheduleID int64, limit int) ([]*ent.Sub2APIOptimizeLog, error) {
	return r.client.Sub2APIOptimizeLog.Query().
		Where(sub2apioptimizelog.ScheduleID(scheduleID)).
		Order(ent.Desc(sub2apioptimizelog.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
}
