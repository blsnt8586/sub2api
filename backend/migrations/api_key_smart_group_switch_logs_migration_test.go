//go:build unit

package migrations

import (
	"strings"
	"testing"
)

func TestAPIKeySmartGroupSwitchLogsMigrationCreatesRetainedHistoryTable(t *testing.T) {
	content, err := FS.ReadFile("244_api_key_smart_group_switch_logs.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	if !strings.Contains(sql, "create table if not exists api_key_smart_group_switch_logs") {
		t.Fatal("migration must create the smart-group switch log table")
	}
	for _, column := range []string{
		"api_key_id",
		"from_group_id",
		"from_group_name",
		"to_group_id",
		"to_group_name",
		"reason",
		"switched_at",
	} {
		if !strings.Contains(sql, column) {
			t.Fatalf("migration missing %s", column)
		}
	}
	if !strings.Contains(sql, "idx_api_key_smart_group_switch_logs_key_time") ||
		!strings.Contains(sql, "idx_api_key_smart_group_switch_logs_time") {
		t.Fatal("migration must index key and time for bounded history queries and cleanup")
	}
}
