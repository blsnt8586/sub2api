-- Retire the fork-specific automatic group multiplier feature.
-- Keep the legacy columns from 238_group_dynamic_pricing.sql for rollback and
-- schema compatibility, but stop all triggers/functions from changing the
-- upstream-owned groups.rate_multiplier automatically.

DROP TRIGGER IF EXISTS trg_account_groups_dynamic_pricing_insert ON account_groups;
DROP TRIGGER IF EXISTS trg_account_groups_dynamic_pricing_delete ON account_groups;
DROP TRIGGER IF EXISTS trg_account_groups_dynamic_pricing_update ON account_groups;
DROP TRIGGER IF EXISTS trg_accounts_dynamic_pricing_update ON accounts;
DROP TRIGGER IF EXISTS trg_groups_dynamic_pricing_insert ON groups;
DROP TRIGGER IF EXISTS trg_groups_dynamic_pricing_config_update ON groups;

-- Groups that were in dynamic mode used manual_rate_multiplier as their
-- configured static fallback. Restore that value before the legacy columns are
-- ignored by the application. Existing static groups keep rate_multiplier.
UPDATE groups
   SET rate_multiplier = manual_rate_multiplier,
       dynamic_pricing_enabled = FALSE,
       dynamic_source_max_multiplier = NULL,
       dynamic_pricing_updated_at = NULL,
       dynamic_pricing_status = 'manual',
       updated_at = NOW()
 WHERE dynamic_pricing_enabled = TRUE
   AND manual_rate_multiplier IS NOT NULL
   AND manual_rate_multiplier > 0;

-- Disable the feature for any remaining rows without changing their static
-- rate. This also makes the migration idempotent on partially upgraded DBs.
UPDATE groups
   SET dynamic_pricing_enabled = FALSE,
       dynamic_source_max_multiplier = NULL,
       dynamic_pricing_updated_at = NULL,
       dynamic_pricing_status = 'manual'
 WHERE dynamic_pricing_enabled IS DISTINCT FROM FALSE;

DROP FUNCTION IF EXISTS refresh_dynamic_pricing_after_account_group_insert();
DROP FUNCTION IF EXISTS refresh_dynamic_pricing_after_account_group_delete();
DROP FUNCTION IF EXISTS refresh_dynamic_pricing_after_account_group_update();
DROP FUNCTION IF EXISTS refresh_dynamic_pricing_after_account_update();
DROP FUNCTION IF EXISTS refresh_dynamic_pricing_after_group_config();
DROP FUNCTION IF EXISTS refresh_group_dynamic_pricing(BIGINT);
