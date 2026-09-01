//go:build unit

package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProviderRemoteCostDivisorMigrationIsIdempotent(t *testing.T) {
	content, err := FS.ReadFile("240_sub2api_provider_remote_cost_divisor.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"add column if not exists remote_cost_divisor",
		"default 1.0",
		"drop constraint if exists sub2api_providers_remote_cost_divisor_positive",
		"check (remote_cost_divisor > 0)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
