-- Add the account-level cost-optimization fields introduced after the original
-- probe-target table migration. The split migration is ordered before 237 so
-- 237 can safely set the new healthy-streak default on existing installations.

ALTER TABLE sub2api_provider_probe_targets
    ADD COLUMN IF NOT EXISTS degraded_optimize_threshold INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS cost_optimize_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS cost_optimize_interval_seconds INTEGER NOT NULL DEFAULT 21600,
    ADD COLUMN IF NOT EXISTS cost_optimize_healthy_threshold INTEGER NOT NULL DEFAULT 6,
    ADD COLUMN IF NOT EXISTS last_cost_optimize_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_degraded_optimize_threshold_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_degraded_optimize_threshold_check
            CHECK (degraded_optimize_threshold BETWEEN 1 AND 20);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_cost_optimize_interval_seconds_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_cost_optimize_interval_seconds_check
            CHECK (cost_optimize_interval_seconds BETWEEN 1800 AND 86400);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'sub2api_probe_targets_cost_optimize_healthy_threshold_check'
           AND conrelid = 'sub2api_provider_probe_targets'::regclass
    ) THEN
        ALTER TABLE sub2api_provider_probe_targets
            ADD CONSTRAINT sub2api_probe_targets_cost_optimize_healthy_threshold_check
            CHECK (cost_optimize_healthy_threshold BETWEEN 1 AND 20);
    END IF;
END
$$;
