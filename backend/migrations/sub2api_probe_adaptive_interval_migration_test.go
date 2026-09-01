//go:build unit

package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProbeAdaptiveIntervalMigrationIsSelfContained(t *testing.T) {
	content, err := FS.ReadFile("241_sub2api_probe_adaptive_interval.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, column := range []string{
		"adaptive_interval_enabled",
		"healthy_interval_seconds",
		"healthy_interval_threshold",
		"stable_healthy_interval_seconds",
		"stable_healthy_threshold",
		"consecutive_healthy",
	} {
		if !strings.Contains(sql, "add column if not exists "+column) {
			t.Fatalf("migration must add %s before using it", column)
		}
	}
	for _, check := range []string{
		"healthy_interval_seconds between 30 and 86400",
		"healthy_interval_threshold between 1 and 100",
		"stable_healthy_interval_seconds between 30 and 86400",
		"stable_healthy_threshold between 1 and 100",
		"consecutive_healthy >= 0",
	} {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration missing check %q", check)
		}
	}
	if !strings.Contains(sql, "where allow_media_probe = true") {
		t.Fatal("migration must keep existing media probes off adaptive cadence")
	}
	if !strings.Contains(sql, "greatest(interval_seconds, healthy_interval_seconds)") ||
		!strings.Contains(sql, "greatest(interval_seconds, healthy_interval_seconds, stable_healthy_interval_seconds)") {
		t.Fatal("migration must not reduce an existing target's configured cadence")
	}
}
