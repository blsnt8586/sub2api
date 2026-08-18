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

// Sub2APIProviderProbeConfig stores the opt-in policy for provider health probes.
// Control-plane checks are enabled by default; data-plane and media probes are not.
type Sub2APIProviderProbeConfig struct {
	ent.Schema
}

func (Sub2APIProviderProbeConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sub2api_provider_probe_configs"}}
}

func (Sub2APIProviderProbeConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (Sub2APIProviderProbeConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("provider_id"),
		field.Bool("control_enabled").Default(true),
		field.Int("control_interval_seconds").Default(1800).Range(60, 86400),
		field.Bool("data_enabled").Default(false),
		field.Int("data_interval_seconds").Default(1800).Range(300, 86400),
		field.JSON("selected_account_ids", []int64{}).Default([]int64{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Bool("allow_media_probe").Default(false),
		field.Int("timeout_seconds").Default(15).Range(3, 120),
		field.Int("degraded_latency_ms").Default(2000).Range(100, 120000),
		field.Int("failure_threshold").Default(3).Range(1, 20),
		field.Int("recovery_threshold").Default(2).Range(1, 20),
		field.Time("last_control_run_at").Optional().Nillable(),
		field.Time("last_data_run_at").Optional().Nillable(),
	}
}

func (Sub2APIProviderProbeConfig) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("provider", Sub2APIProvider.Type).
			Ref("probe_config").
			Field("provider_id").
			Unique().
			Required(),
	}
}

func (Sub2APIProviderProbeConfig) Indexes() []ent.Index {
	return []ent.Index{index.Fields("provider_id").Unique()}
}
