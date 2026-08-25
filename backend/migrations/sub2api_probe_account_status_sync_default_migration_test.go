package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProbeAccountStatusSyncDefaultMigrationEnablesDisplay(t *testing.T) {
	content, err := FS.ReadFile("236_sub2api_probe_account_status_sync_default.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	if !strings.Contains(sql, "alter column account_status_sync_enabled set default true") {
		t.Fatal("migration must make the display-only exhausted indicator default to true")
	}
	if !strings.Contains(sql, "update sub2api_provider_probe_configs") || !strings.Contains(sql, "set account_status_sync_enabled = true") {
		t.Fatal("migration must enable the indicator for existing configs")
	}
}
