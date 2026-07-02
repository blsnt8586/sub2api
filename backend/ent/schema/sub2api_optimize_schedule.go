package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// Sub2APIOptimizeSchedule 每个上游 Provider 的定时分组优化配置（一对一）
type Sub2APIOptimizeSchedule struct {
	ent.Schema
}

func (Sub2APIOptimizeSchedule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (Sub2APIOptimizeSchedule) Fields() []ent.Field {
	return []ent.Field{
		// provider_id: 关联的上游 Provider（唯一约束，每个 Provider 最多一条）
		field.Int64("provider_id").
			Comment("关联的上游 Provider ID"),

		// cron_expr: 标准 5 字段 cron 表达式，如 */30 * * * *
		field.String("cron_expr").
			MaxLen(64).
			NotEmpty().
			Comment("cron 表达式，如 */30 * * * *"),

		// enabled: 是否启用该定时任务
		field.Bool("enabled").
			Default(true).
			Comment("是否启用"),

		// last_run_at: 上次执行时间
		field.Time("last_run_at").
			Optional().
			Nillable().
			Comment("上次执行时间"),

		// next_run_at: 下次执行时间（由 cron 解析后计算）
		field.Time("next_run_at").
			Optional().
			Nillable().
			Comment("下次执行时间"),
	}
}

func (Sub2APIOptimizeSchedule) Edges() []ent.Edge {
	return []ent.Edge{
		// provider: 所属上游
		edge.From("provider", Sub2APIProvider.Type).
			Ref("optimize_schedule").
			Field("provider_id").
			Required().
			Unique(),

		// logs: 执行日志（一对多）
		edge.To("logs", Sub2APIOptimizeLog.Type),
	}
}

func (Sub2APIOptimizeSchedule) Indexes() []ent.Index {
	return []ent.Index{
		// 每个 Provider 最多一条定时配置
		index.Fields("provider_id").Unique(),
	}
}
