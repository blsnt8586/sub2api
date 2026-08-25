package migrations

import (
	"strings"
	"testing"
)

func TestGroupDynamicPricingMigrationKeepsEffectiveRateAndRefreshTriggers(t *testing.T) {
	content, err := FS.ReadFile("238_group_dynamic_pricing.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, required := range []string{
		"manual_rate_multiplier = rate_multiplier",
		"for update",
		"remote_group_multiplier",
		"max(a.remote_group_multiplier)",
		"account_count = 0",
		"next_status := 'error'",
		"round(source_max + markup, 4)",
		"next_rate := fallback_rate",
		"next_status := 'no_accounts'",
		"refresh_group_dynamic_pricing",
		"trg_account_groups_dynamic_pricing_insert",
		"trg_account_groups_dynamic_pricing_delete",
		"trg_accounts_dynamic_pricing_update",
		"trg_groups_dynamic_pricing_config_update",
		"dynamic_pricing_enabled is not distinct from new.dynamic_pricing_enabled",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("dynamic pricing migration missing %q", required)
		}
	}
	if strings.Contains(sql, "coalesce(remote_group_multiplier, rate_multiplier)") ||
		strings.Contains(sql, "when a.rate_multiplier") {
		t.Fatal("dynamic pricing must never treat accounts.rate_multiplier as a remote procurement rate")
	}
}
