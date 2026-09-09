-- Add opt-in smart group routing to API keys while preserving group_id as the
-- single authoritative group used for request routing and billing.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS smart_group_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS smart_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS smart_group_failure_threshold INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS smart_group_recovery_interval_seconds INTEGER NOT NULL DEFAULT 900,
    ADD COLUMN IF NOT EXISTS smart_group_consecutive_failures INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS smart_group_healthy_since TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS smart_group_last_probe_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS smart_group_last_switch_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS smart_group_probe_lease_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS smart_group_last_switch_reason VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS smart_group_last_error TEXT NOT NULL DEFAULT '';

UPDATE api_keys
   SET smart_group_ids = '[]'::jsonb
 WHERE smart_group_ids IS NULL
    OR jsonb_typeof(smart_group_ids) IS DISTINCT FROM 'array';

UPDATE api_keys
   SET smart_group_failure_threshold = 3
 WHERE smart_group_failure_threshold < 1
    OR smart_group_failure_threshold > 20;

UPDATE api_keys
   SET smart_group_recovery_interval_seconds = 900
 WHERE smart_group_recovery_interval_seconds < 60
    OR smart_group_recovery_interval_seconds > 86400;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_api_keys_smart_group_failure_threshold'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT chk_api_keys_smart_group_failure_threshold
            CHECK (smart_group_failure_threshold BETWEEN 1 AND 20);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_api_keys_smart_group_recovery_interval'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT chk_api_keys_smart_group_recovery_interval
            CHECK (smart_group_recovery_interval_seconds BETWEEN 60 AND 86400);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_api_keys_smart_group_candidates'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT chk_api_keys_smart_group_candidates
            CHECK (
                jsonb_typeof(smart_group_ids) = 'array'
                AND (
                    smart_group_enabled = FALSE
                    OR (group_id IS NOT NULL AND jsonb_array_length(smart_group_ids) BETWEEN 2 AND 10)
                )
            );
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_api_keys_smart_group_enabled
    ON api_keys (id)
    WHERE deleted_at IS NULL AND smart_group_enabled = TRUE;
