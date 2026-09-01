-- Add independently configurable healthy-state probe backoff. The base
-- interval remains the recovery cadence for new, degraded, and unhealthy
-- routes; only consecutive healthy results use the slower intervals.

ALTER TABLE sub2api_provider_probe_targets
    ADD COLUMN IF NOT EXISTS adaptive_interval_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS healthy_interval_seconds INTEGER NOT NULL DEFAULT 120,
    ADD COLUMN IF NOT EXISTS healthy_interval_threshold INTEGER NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS stable_healthy_interval_seconds INTEGER NOT NULL DEFAULT 300,
    ADD COLUMN IF NOT EXISTS stable_healthy_threshold INTEGER NOT NULL DEFAULT 6,
    ADD COLUMN IF NOT EXISTS consecutive_healthy INTEGER NOT NULL DEFAULT 0;

-- Existing installations may already use a base interval above the new
-- defaults. Never let adaptive mode make those routes probe more frequently,
-- and keep every migrated row immediately valid for partial configuration
-- updates from the UI.
UPDATE sub2api_provider_probe_targets
   SET healthy_interval_seconds = GREATEST(interval_seconds, healthy_interval_seconds),
       stable_healthy_interval_seconds = GREATEST(interval_seconds, healthy_interval_seconds, stable_healthy_interval_seconds);

-- Media probes already have a six-hour safety floor. Keep existing media
-- routes on their explicitly configured fixed cadence until an operator
-- supplies adaptive intervals that also satisfy that floor.
UPDATE sub2api_provider_probe_targets
   SET adaptive_interval_enabled = FALSE
 WHERE allow_media_probe = TRUE;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_healthy_interval_seconds_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_healthy_interval_seconds_check
            CHECK (healthy_interval_seconds BETWEEN 30 AND 86400);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_healthy_interval_threshold_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_healthy_interval_threshold_check
            CHECK (healthy_interval_threshold BETWEEN 1 AND 100);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_stable_interval_seconds_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_stable_interval_seconds_check
            CHECK (stable_healthy_interval_seconds BETWEEN 30 AND 86400);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_stable_healthy_threshold_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_stable_healthy_threshold_check
            CHECK (stable_healthy_threshold BETWEEN 1 AND 100);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_consecutive_healthy_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_consecutive_healthy_check
            CHECK (consecutive_healthy >= 0);
    END IF;
END
$$;

COMMENT ON COLUMN sub2api_provider_probe_targets.interval_seconds IS
    'Base cadence used before a healthy streak and after degraded/unhealthy results';
COMMENT ON COLUMN sub2api_provider_probe_targets.healthy_interval_seconds IS
    'Cadence after healthy_interval_threshold consecutive healthy probes';
COMMENT ON COLUMN sub2api_provider_probe_targets.stable_healthy_interval_seconds IS
    'Cadence after stable_healthy_threshold consecutive healthy probes';
