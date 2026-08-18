-- Account probe targets use a 30-second default cadence. Migration 232
-- changed the column default but the original migration 226 check constraint
-- still rejected values below 60, causing target creation to fail.

ALTER TABLE sub2api_provider_probe_targets
    DROP CONSTRAINT IF EXISTS sub2api_provider_probe_targets_interval_seconds_check;

ALTER TABLE sub2api_provider_probe_targets
    ADD CONSTRAINT sub2api_provider_probe_targets_interval_seconds_check
    CHECK (interval_seconds BETWEEN 30 AND 86400);
