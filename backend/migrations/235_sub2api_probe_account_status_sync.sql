-- Allow provider probes to opt in to persistent local account status control.
-- The feature is disabled by default so existing probe deployments remain
-- observational until an administrator explicitly enables it.

ALTER TABLE sub2api_provider_probe_configs
    ADD COLUMN IF NOT EXISTS account_status_sync_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS account_status_failure_threshold INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS account_status_recovery_threshold INTEGER NOT NULL DEFAULT 2;

ALTER TABLE sub2api_provider_probe_configs
    DROP CONSTRAINT IF EXISTS sub2api_probe_configs_account_status_failure_threshold_check,
    DROP CONSTRAINT IF EXISTS sub2api_probe_configs_account_status_recovery_threshold_check;

ALTER TABLE sub2api_provider_probe_configs
    ADD CONSTRAINT sub2api_probe_configs_account_status_failure_threshold_check
        CHECK (account_status_failure_threshold BETWEEN 1 AND 20),
    ADD CONSTRAINT sub2api_probe_configs_account_status_recovery_threshold_check
        CHECK (account_status_recovery_threshold BETWEEN 1 AND 20);
