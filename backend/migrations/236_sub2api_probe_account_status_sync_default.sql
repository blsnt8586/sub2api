-- Group-exhaustion errors are safe display-only diagnostics: they never turn
-- off scheduling. Enable the indicator by default for existing probe configs so
-- the recovery-aware state machine is visible without an extra setup step.

ALTER TABLE sub2api_provider_probe_configs
    ALTER COLUMN account_status_sync_enabled SET DEFAULT TRUE;

UPDATE sub2api_provider_probe_configs
   SET account_status_sync_enabled = TRUE
 WHERE account_status_sync_enabled = FALSE;
