package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// Sub2APIOptimizeLog records every Provider group optimization attempt,
// regardless of whether it was triggered by cron, probe linkage, or an admin.
type Sub2APIOptimizeLog struct {
	ent.Schema
}

func (Sub2APIOptimizeLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (Sub2APIOptimizeLog) Fields() []ent.Field {
	return []ent.Field{
		// provider_id is the stable audit owner. Logs must remain queryable even
		// when a schedule is disabled or deleted.
		field.Int64("provider_id").
			Comment("所属上游 Provider ID"),

		// schedule_id is present only for cron/schedule-now runs.
		field.Int64("schedule_id").
			Optional().
			Nillable().
			Comment("关联的定时配置 ID，可空"),

		field.String("trigger").
			MaxLen(32).
			Default("legacy").
			Comment("触发方式：cron / schedule_now / probe_unhealthy / manual_account / manual_all / legacy"),

		// status: 整体运行状态
		field.String("status").
			MaxLen(16).
			Default("success").
			Comment("整体状态：success / partial / failed / skipped"),

		// total: 处理的账号总数
		field.Int("total").
			Default(0).
			Comment("处理账号总数"),

		// optimized: 成功切换分组的账号数
		field.Int("optimized").
			Default(0).
			Comment("成功切换账号数"),

		// skipped: 无需切换（已是最优）的账号数
		field.Int("skipped").
			Default(0).
			Comment("跳过账号数（已最优）"),

		// failed: 失败账号数
		field.Int("failed").
			Default(0).
			Comment("失败账号数"),

		// detail: 每个账号的详细结果（JSONB）
		// 结构: [{account_id, account_name, result, old_group, new_group, error}]
		field.JSON("detail", []map[string]any{}).
			Optional().
			Comment("每账号详细结果"),

		// started_at / finished_at: 执行时间段
		field.Time("started_at").
			Optional().
			Nillable().
			Comment("开始时间"),

		field.Time("finished_at").
			Optional().
			Nillable().
			Comment("结束时间"),
	}
}

func (Sub2APIOptimizeLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("provider", Sub2APIProvider.Type).
			Ref("optimize_logs").
			Field("provider_id").
			Required().
			Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),

		// Schedule deletion must not remove audit history.
		edge.From("schedule", Sub2APIOptimizeSchedule.Type).
			Ref("logs").
			Field("schedule_id").
			Unique(),
	}
}

func (Sub2APIOptimizeLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider_id", "created_at").
			StorageKey("idx_sub2api_optimize_logs_provider_created"),
		index.Fields("provider_id", "trigger", "created_at").
			StorageKey("idx_sub2api_optimize_logs_provider_trigger_created"),
		index.Fields("provider_id", "status", "created_at").
			StorageKey("idx_sub2api_optimize_logs_provider_status_created"),
		index.Fields("schedule_id", "created_at"),
	}
}
