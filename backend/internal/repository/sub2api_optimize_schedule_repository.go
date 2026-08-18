package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
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
		SetProviderID(input.ProviderID).
		SetTrigger(input.Trigger).
		SetStatus(input.Status).
		SetTotal(input.Total).
		SetOptimized(input.Optimized).
		SetSkipped(input.Skipped).
		SetFailed(input.Failed)
	if input.ScheduleID != nil {
		b = b.SetScheduleID(*input.ScheduleID)
	}
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

var optimizeLogLikeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// ListLogs returns only one Provider's audit records and applies every user
// value as a bound argument. PostgreSQL JSONB containment keeps account
// filtering exact instead of relying on ambiguous text matching.
func (r *Sub2APIOptimizeScheduleRepository) ListLogs(ctx context.Context, providerID int64, filter service.OptimizeLogFilter) ([]*ent.Sub2APIOptimizeLog, int64, error) {
	q := r.client.Sub2APIOptimizeLog.Query().
		Where(sub2apioptimizelog.ProviderID(providerID))
	if filter.Trigger != "" {
		q = q.Where(sub2apioptimizelog.TriggerEQ(filter.Trigger))
	}
	if filter.Status != "" {
		q = q.Where(sub2apioptimizelog.StatusEQ(filter.Status))
	}
	if filter.AccountID != nil {
		value, err := json.Marshal([]map[string]int64{{"account_id": *filter.AccountID}})
		if err != nil {
			return nil, 0, err
		}
		q = q.Where(func(selector *entsql.Selector) {
			selector.Where(entsql.P(func(builder *entsql.Builder) {
				builder.Ident(selector.C(sub2apioptimizelog.FieldDetail)).
					WriteString(" @> ").
					Arg(string(value)).
					WriteString("::jsonb")
			}))
		})
	}
	if filter.Keyword != "" {
		pattern := "%" + strings.ToLower(optimizeLogLikeEscaper.Replace(filter.Keyword)) + "%"
		q = q.Where(func(selector *entsql.Selector) {
			selector.Where(entsql.P(func(builder *entsql.Builder) {
				builder.WriteString("LOWER(CAST(").
					Ident(selector.C(sub2apioptimizelog.FieldDetail)).
					WriteString(" AS TEXT)) LIKE ").
					Arg(pattern).
					WriteString(` ESCAPE '\'`)
			}))
		})
	}
	if filter.From != nil {
		q = q.Where(sub2apioptimizelog.CreatedAtGTE(*filter.From))
	}
	if filter.To != nil {
		q = q.Where(sub2apioptimizelog.CreatedAtLTE(*filter.To))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	logs, err := q.
		Order(ent.Desc(sub2apioptimizelog.FieldCreatedAt), ent.Desc(sub2apioptimizelog.FieldID)).
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		All(ctx)
	return logs, int64(total), err
}
