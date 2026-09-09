//go:build unit

package migrations

import (
	"strings"
	"testing"
)

func TestAPIKeySmartGroupsMigrationAddsColumnsBeforeConstraints(t *testing.T) {
	content, err := FS.ReadFile("243_api_key_smart_groups.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, column := range []string{
		"smart_group_enabled",
		"smart_group_ids",
		"smart_group_failure_threshold",
		"smart_group_recovery_interval_seconds",
		"smart_group_consecutive_failures",
		"smart_group_healthy_since",
		"smart_group_last_probe_at",
		"smart_group_last_switch_at",
		"smart_group_probe_lease_until",
		"smart_group_last_switch_reason",
		"smart_group_last_error",
	} {
		fragment := "add column if not exists " + column
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration must add %s before using it", column)
		}
	}
	for _, constraint := range []string{
		"chk_api_keys_smart_group_failure_threshold",
		"chk_api_keys_smart_group_recovery_interval",
		"chk_api_keys_smart_group_candidates",
	} {
		if !strings.Contains(sql, constraint) {
			t.Fatalf("migration missing constraint %s", constraint)
		}
	}
	if !strings.Contains(sql, "smart_group_failure_threshold between 1 and 20") ||
		!strings.Contains(sql, "smart_group_recovery_interval_seconds between 60 and 86400") ||
		!strings.Contains(sql, "jsonb_array_length(smart_group_ids) between 2 and 10") {
		t.Fatal("migration is missing smart group policy bounds")
	}
}
