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

		field.Int64("proxy_id").
			Optional().
			Nillable().
			Comment("Provider 及其关联账号使用的统一出站代理；NULL 表示直连"),

		// 认证信息
		field.String("email").
			MaxLen(200).
			NotEmpty().
			Comment("登录邮箱"),

		field.String("password_encrypted").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default("").
			Sensitive(). // Ent 会在日志中隐藏此字段
			Comment("登录密码（阶段1明文，阶段7加密）"),

		field.String("auth_mode").
			MaxLen(32).
			Default(domain.Sub2APIProviderAuthModePassword).
			Comment("认证方式：password, token_pair"),

		field.String("access_token_encrypted").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Sensitive().
			Comment("AES-GCM 加密的上游 Access Token"),

		field.String("refresh_token_encrypted").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Sensitive().
			Comment("AES-GCM 加密的上游 Refresh Token"),

		field.Time("access_token_expires_at").
			Optional().
			Nillable().
			Comment("上游 Access Token 的保守失效时间"),

		field.Time("last_token_refresh_at").
			Optional().
			Nillable().
			Comment("最近一次持久化 Token 对的时间"),

		field.String("last_auth_error").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("最近一次认证错误（不含凭据正文）"),

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
		// Provider 的控制面请求和关联账号统一使用该代理。
		edge.To("proxy", Proxy.Type).
			Field("proxy_id").
			Unique().
			Annotations(entsql.OnDelete(entsql.SetNull)),
		// 一个 Provider 可以关联多个 Account
		edge.To("accounts", Account.Type),
		// 一个 Provider 最多一条定时优化配置（一对一）
		edge.To("optimize_schedule", Sub2APIOptimizeSchedule.Type).
			Unique(),
		// 分组优化日志属于 Provider；删除定时配置不影响该审计历史。
		edge.To("optimize_logs", Sub2APIOptimizeLog.Type),
		// Provider 健康探针配置与运行历史独立存储，避免污染 Provider 主表。
		edge.To("probe_config", Sub2APIProviderProbeConfig.Type).
			Unique(),
		edge.To("probe_runs", Sub2APIProviderProbeRun.Type),
		edge.To("probe_targets", Sub2APIProviderProbeTarget.Type),
	}
}

func (Sub2APIProvider) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("proxy_id"),
		index.Fields("deleted_at"),
		// 唯一索引：base_url + email（软删除时排除）
		index.Fields("base_url", "email").
			Unique().
			Annotations(
				entsql.IndexWhere("deleted_at IS NULL"),
			),
	}
}
