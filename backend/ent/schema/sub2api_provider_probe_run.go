package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Sub2APIProviderProbeRun is an append-only result of one provider probe cycle.
type Sub2APIProviderProbeRun struct {
	ent.Schema
}

func (Sub2APIProviderProbeRun) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sub2api_provider_probe_runs"}}
}

func (Sub2APIProviderProbeRun) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("provider_id"),
		field.Enum("overall_status").Values("healthy", "degraded", "unhealthy", "unknown"),
		field.Enum("control_status").Values("healthy", "degraded", "unhealthy", "unknown"),
		field.Enum("data_status").Values("healthy", "degraded", "unhealthy", "unknown"),
		field.Enum("traffic_status").Values("healthy", "degraded", "unhealthy", "unknown"),
		field.Int("login_latency_ms").Optional().Nillable(),
		field.Int("health_latency_ms").Optional().Nillable(),
		field.Int("keys_latency_ms").Optional().Nillable(),
		field.Int("groups_latency_ms").Optional().Nillable(),
		field.Int("data_probe_count").Default(0),
		field.Int("data_probe_success").Default(0),
		field.Int("data_probe_failed").Default(0),
		field.Int("traffic_request_count").Default(0),
		field.Float("traffic_success_rate").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "decimal(7,4)"}),
		field.Int("traffic_p95_latency_ms").Optional().Nillable(),
		field.String("error_category").Optional().Nillable().MaxLen(64),
		field.String("error_message").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("details", map[string]any{}).Default(map[string]any{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("started_at").Default(time.Now),
		field.Time("finished_at").Default(time.Now),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Sub2APIProviderProbeRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("provider", Sub2APIProvider.Type).
			Ref("probe_runs").
			Field("provider_id").
			Unique().
			Required(),
	}
}

func (Sub2APIProviderProbeRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider_id", "created_at"),
		index.Fields("created_at"),
	}
}
