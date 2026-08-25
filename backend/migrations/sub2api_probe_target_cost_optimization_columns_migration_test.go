package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProbeTargetCostOptimizationColumnsMigrationAddsFieldsBeforeDefaultMigration(t *testing.T) {
	content, err := FS.ReadFile("236a_sub2api_probe_target_cost_optimization_columns.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, column := range []string{
		"degraded_optimize_threshold",
		"cost_optimize_enabled",
		"cost_optimize_interval_seconds",
		"cost_optimize_healthy_threshold",
		"last_cost_optimize_at",
	} {
		if !strings.Contains(sql, "add column if not exists "+column) {
			t.Fatalf("migration must add %s", column)
		}
	}
	if !strings.Contains(sql, "cost_optimize_interval_seconds between 1800 and 86400") {
		t.Fatal("migration must enforce the 30-minute to 24-hour interval range")
	}
	if !strings.Contains(sql, "cost_optimize_healthy_threshold between 1 and 20") {
		t.Fatal("migration must enforce the healthy probe threshold range")
	}
}
