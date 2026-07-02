package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// Sub2APIOptimizeLog 每次定时优化任务的执行记录
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
		// schedule_id: 所属定时配置
		field.Int64("schedule_id").
			Comment("所属定时配置 ID"),

		// status: 整体运行状态
		field.String("status").
			MaxLen(16).
			Default("success").
			Comment("整体状态：success / partial / failed"),

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
		// schedule: 所属定时配置（反向）
		edge.From("schedule", Sub2APIOptimizeSchedule.Type).
			Ref("logs").
			Field("schedule_id").
			Required().
			Unique(),
	}
}

func (Sub2APIOptimizeLog) Indexes() []ent.Index {
	return []ent.Index{
		// 按 schedule_id + 创建时间排序，方便取最近N条日志
		index.Fields("schedule_id", "created_at"),
	}
}
