//go:build unit

package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProbeAccountStatusSyncMigrationDefaultsToDisabled(t *testing.T) {
	content, err := FS.ReadFile("235_sub2api_probe_account_status_sync.sql")
	if err != nil {
		t.Fatalf("read account status sync migration: %v", err)
	}
	sql := strings.ToUpper(strings.Join(strings.Fields(string(content)), " "))
	for _, required := range []string{
		"ADD COLUMN IF NOT EXISTS ACCOUNT_STATUS_SYNC_ENABLED BOOLEAN NOT NULL DEFAULT FALSE",
		"ADD COLUMN IF NOT EXISTS ACCOUNT_STATUS_FAILURE_THRESHOLD INTEGER NOT NULL DEFAULT 3",
		"ADD COLUMN IF NOT EXISTS ACCOUNT_STATUS_RECOVERY_THRESHOLD INTEGER NOT NULL DEFAULT 2",
		"CHECK (ACCOUNT_STATUS_FAILURE_THRESHOLD BETWEEN 1 AND 20)",
		"CHECK (ACCOUNT_STATUS_RECOVERY_THRESHOLD BETWEEN 1 AND 20)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(sql, "UPDATE SUB2API_PROVIDER_PROBE_CONFIGS") {
		t.Fatal("migration must not opt existing providers into account status control")
	}
}
