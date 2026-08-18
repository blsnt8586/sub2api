//go:build unit

package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProbeTargetIntervalMigrationAlignsThirtySecondDefault(t *testing.T) {
	content, err := FS.ReadFile("233_sub2api_probe_target_interval_constraint.sql")
	if err != nil {
		t.Fatalf("read account probe interval migration: %v", err)
	}
	sql := strings.ToUpper(strings.Join(strings.Fields(string(content)), " "))
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS SUB2API_PROVIDER_PROBE_TARGETS_INTERVAL_SECONDS_CHECK",
		"ADD CONSTRAINT SUB2API_PROVIDER_PROBE_TARGETS_INTERVAL_SECONDS_CHECK",
		"CHECK (INTERVAL_SECONDS BETWEEN 30 AND 86400)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}
