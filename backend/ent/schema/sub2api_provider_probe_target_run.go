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

// Sub2APIProviderProbeTargetRun stores immutable data-plane evidence. The
// routing group is copied because an optimizer may legitimately switch it.
type Sub2APIProviderProbeTargetRun struct {
	ent.Schema
}

func (Sub2APIProviderProbeTargetRun) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sub2api_provider_probe_target_runs"}}
}

func (Sub2APIProviderProbeTargetRun) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("target_id"),
		field.Int64("provider_id"),
		field.Int64("account_id"),
		field.Int64("provider_api_key_id").Optional().Nillable(),
		field.Int64("remote_group_id").Optional().Nillable(),
		field.String("remote_group_name").MaxLen(100).Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("platform").MaxLen(50).Default(""),
		field.String("model_id").MaxLen(160).Optional().Nillable(),
		field.Enum("status").Values("healthy", "degraded", "unhealthy", "unknown"),
		field.Int("latency_ms").Optional().Nillable(),
		field.Int("traffic_request_count").Default(0),
		field.Float("traffic_success_rate").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "decimal(7,4)"}),
		field.Int("traffic_p95_latency_ms").Optional().Nillable(),
		field.String("error_category").Optional().Nillable().MaxLen(64),
		field.String("error_message").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("started_at").Default(time.Now),
		field.Time("finished_at").Default(time.Now),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Sub2APIProviderProbeTargetRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("target", Sub2APIProviderProbeTarget.Type).Ref("runs").Field("target_id").Unique().Required(),
	}
}

func (Sub2APIProviderProbeTargetRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("target_id", "created_at"),
		index.Fields("provider_id", "created_at"),
		index.Fields("account_id", "created_at"),
	}
}
