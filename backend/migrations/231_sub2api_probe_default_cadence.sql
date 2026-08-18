-- Keep new probe records aligned with the control-plane/account-plane split.
-- Existing rows are intentionally preserved because their intervals may be
-- operator choices rather than inherited defaults.

ALTER TABLE sub2api_provider_probe_configs
    ALTER COLUMN control_interval_seconds SET DEFAULT 1800;

ALTER TABLE sub2api_provider_probe_targets
    ALTER COLUMN interval_seconds SET DEFAULT 300;
