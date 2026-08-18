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
)

// Sub2APIProviderProbeTarget is a monitorable business route: one Provider,
// one local account/API key, and its current remote group assignment.
type Sub2APIProviderProbeTarget struct {
	ent.Schema
}

func (Sub2APIProviderProbeTarget) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sub2api_provider_probe_targets"}}
}

func (Sub2APIProviderProbeTarget) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (Sub2APIProviderProbeTarget) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("provider_id"),
		field.Int64("account_id"),
		field.Int64("provider_api_key_id").Optional().Nillable(),
		field.Int64("remote_group_id").Optional().Nillable(),
		field.String("remote_group_name").MaxLen(100).Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("platform").MaxLen(50).Default(""),
		field.Bool("enabled").Default(false),
		field.Int("interval_seconds").Default(30).Range(30, 86400),
		field.String("test_model").MaxLen(160).Optional().Nillable(),
		field.Bool("allow_media_probe").Default(false),
		field.Int("timeout_seconds").Default(60).Range(3, 120),
		field.Int("degraded_latency_ms").Default(5000).Range(100, 120000),
		field.Int("failure_threshold").Default(3).Range(1, 20),
		field.Int("recovery_threshold").Default(2).Range(1, 20),
		field.Time("last_run_at").Optional().Nillable(),
		field.Time("route_changed_at").Optional().Nillable(),
	}
}

func (Sub2APIProviderProbeTarget) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("provider", Sub2APIProvider.Type).Ref("probe_targets").Field("provider_id").Unique().Required(),
		edge.From("account", Account.Type).Ref("sub2api_probe_targets").Field("account_id").Unique().Required(),
		edge.To("runs", Sub2APIProviderProbeTargetRun.Type),
	}
}

func (Sub2APIProviderProbeTarget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider_id", "account_id").Unique(),
		index.Fields("provider_id", "enabled", "last_run_at"),
		index.Fields("account_id"),
	}
}
