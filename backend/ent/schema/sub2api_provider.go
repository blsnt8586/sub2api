package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// Sub2APIProvider 定义第三方 Sub2API Provider 实体
type Sub2APIProvider struct {
	ent.Schema
}

func (Sub2APIProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sub2api_providers"},
	}
}

func (Sub2APIProvider) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (Sub2APIProvider) Fields() []ent.Field {
	return []ent.Field{
		// 基本信息
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("Provider 显示名称"),

		field.String("base_url").
			MaxLen(500).
			NotEmpty().
			Comment("Provider 基础 URL，如 https://api.example.com"),

		// 上游类型：标识上游协议/接口实现（当前仅 sub2api），决定同步/登录/分组等逻辑走哪套。
		// 创建时指定、后续只读；与账号平台(anthropic/openai/gemini)语义不同。
		field.String("provider_type").
			MaxLen(50).
			Default(domain.ProviderTypeDefault).
			Comment("上游类型：sub2api（后续可扩展其他上游协议）"),

		field.String("status").
			MaxLen(20).
			Default(domain.StatusActive).
			Comment("状态：active, inactive"),

		field.String("notes").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("备注信息"),

		// 认证信息
		field.String("email").
			MaxLen(200).
			NotEmpty().
			Comment("登录邮箱"),

		field.String("password_encrypted").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			NotEmpty().
			Sensitive(). // Ent 会在日志中隐藏此字段
			Comment("登录密码（阶段1明文，阶段7加密）"),

		// API 路径配置
		field.String("api_path_keys").
			MaxLen(100).
			Optional().
			Nillable().
			Comment("APIKey 列表路径，如 /api/v1/keys"),

		field.String("api_path_groups").
			MaxLen(100).
			Optional().
			Nillable().
			Comment("分组列表路径，如 /api/v1/groups/available"),

		// 同步状态
		field.Time("last_sync_at").
			Optional().
			Nillable().
			Comment("最后同步时间"),

		field.String("last_sync_status").
			MaxLen(20).
			Optional().
			Nillable().
			Comment("最后同步状态：success, failed, pending"),

		field.String("last_sync_error").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("同步错误信息"),
	}
}

func (Sub2APIProvider) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个 Provider 可以关联多个 Account
		edge.To("accounts", Account.Type),
		// 一个 Provider 最多一条定时优化配置（一对一）
		edge.To("optimize_schedule", Sub2APIOptimizeSchedule.Type).
			Unique(),
	}
}

func (Sub2APIProvider) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("deleted_at"),
		// 唯一索引：base_url + email（软删除时排除）
		index.Fields("base_url", "email").
			Unique().
			Annotations(
				entsql.IndexWhere("deleted_at IS NULL"),
			),
	}
}
