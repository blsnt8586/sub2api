package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProbeCostOptimizeThresholdMigrationDefaultsToSix(t *testing.T) {
	content, err := FS.ReadFile("237_sub2api_probe_cost_optimize_threshold_default.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	if !strings.Contains(sql, "alter column cost_optimize_healthy_threshold set default 6") {
		t.Fatal("migration must set the six-probe default")
	}
	if !strings.Contains(sql, "set cost_optimize_healthy_threshold = 6") {
		t.Fatal("migration must update rows that still carry the old default")
	}
}
