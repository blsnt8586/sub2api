package migrations

import (
	"strings"
	"testing"
)

func TestGroupDynamicPricingRetirementMigrationDisablesAutomaticRates(t *testing.T) {
	content, err := FS.ReadFile("242_remove_group_dynamic_pricing.sql")
	if err != nil {
		t.Fatalf("read dynamic pricing retirement migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, required := range []string{
		"drop trigger if exists trg_account_groups_dynamic_pricing_insert",
		"drop trigger if exists trg_account_groups_dynamic_pricing_delete",
		"drop trigger if exists trg_account_groups_dynamic_pricing_update",
		"drop trigger if exists trg_accounts_dynamic_pricing_update",
		"drop trigger if exists trg_groups_dynamic_pricing_insert",
		"drop trigger if exists trg_groups_dynamic_pricing_config_update",
		"set rate_multiplier = manual_rate_multiplier",
		"set dynamic_pricing_enabled = false",
		"drop function if exists refresh_group_dynamic_pricing(bigint)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("retirement migration missing %q", required)
		}
	}
	if strings.Contains(sql, "drop column") {
		t.Fatal("retirement migration must retain legacy columns for rollback compatibility")
	}
}
