//go:build unit

package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProviderTokenAuthMigrationIsAdditive(t *testing.T) {
	content, err := FS.ReadFile("230_sub2api_provider_token_auth.sql")
	if err != nil {
		t.Fatalf("read token auth migration: %v", err)
	}
	sql := strings.ToUpper(string(content))
	for _, required := range []string{
		"ADD COLUMN IF NOT EXISTS AUTH_MODE",
		"DEFAULT 'PASSWORD'",
		"ADD COLUMN IF NOT EXISTS ACCESS_TOKEN_ENCRYPTED",
		"ADD COLUMN IF NOT EXISTS REFRESH_TOKEN_ENCRYPTED",
		"CHECK (AUTH_MODE IN ('PASSWORD', 'TOKEN_PAIR'))",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, destructive := range []string{"DROP COLUMN", "DROP TABLE", "DELETE FROM", "TRUNCATE"} {
		if strings.Contains(sql, destructive) {
			t.Fatalf("migration contains destructive statement %q", destructive)
		}
	}
}
