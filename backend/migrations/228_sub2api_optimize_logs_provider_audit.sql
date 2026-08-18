-- Promote optimization logs from schedule-owned history to Provider-owned
-- audit records. The expand/backfill/contract order keeps existing rows valid
-- throughout the migration and preserves history when a schedule is deleted.
ALTER TABLE sub2api_optimize_logs
    ADD COLUMN IF NOT EXISTS provider_id BIGINT,
    ADD COLUMN IF NOT EXISTS trigger VARCHAR(32);

UPDATE sub2api_optimize_logs AS logs
   SET provider_id = schedules.provider_id
  FROM sub2api_optimize_schedules AS schedules
 WHERE logs.provider_id IS NULL
   AND logs.schedule_id = schedules.id;

UPDATE sub2api_optimize_logs
   SET trigger = 'legacy'
 WHERE trigger IS NULL OR BTRIM(trigger) = '';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM sub2api_optimize_logs WHERE provider_id IS NULL) THEN
        RAISE EXCEPTION 'sub2api_optimize_logs provider_id backfill is incomplete';
    END IF;
END
$$;

ALTER TABLE sub2api_optimize_logs
    ALTER COLUMN provider_id SET NOT NULL,
    ALTER COLUMN trigger SET DEFAULT 'legacy',
    ALTER COLUMN trigger SET NOT NULL,
    ALTER COLUMN schedule_id DROP NOT NULL;

-- Replace the historical ON DELETE CASCADE FK with SET NULL so deleting a
-- schedule never destroys Provider audit records. Constraint names differ
-- between the original SQL migration and Ent-created databases, so discover
-- every schedule_id FK instead of assuming one generated name.
DO $$
DECLARE
    fk_name TEXT;
BEGIN
    FOR fk_name IN
        SELECT kcu.constraint_name
          FROM information_schema.key_column_usage AS kcu
          JOIN information_schema.table_constraints AS tc
            ON tc.constraint_schema = kcu.constraint_schema
           AND tc.constraint_name = kcu.constraint_name
           AND tc.table_schema = kcu.table_schema
           AND tc.table_name = kcu.table_name
         WHERE kcu.table_schema = 'public'
           AND kcu.table_name = 'sub2api_optimize_logs'
           AND kcu.column_name = 'schedule_id'
           AND tc.constraint_type = 'FOREIGN KEY'
    LOOP
        EXECUTE format('ALTER TABLE sub2api_optimize_logs DROP CONSTRAINT %I', fk_name);
    END LOOP;
END
$$;

ALTER TABLE sub2api_optimize_logs
    ADD CONSTRAINT fk_sub2api_optimize_logs_schedule
    FOREIGN KEY (schedule_id)
    REFERENCES sub2api_optimize_schedules(id)
    ON DELETE SET NULL
    NOT VALID;

ALTER TABLE sub2api_optimize_logs
    VALIDATE CONSTRAINT fk_sub2api_optimize_logs_schedule;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM information_schema.key_column_usage AS kcu
          JOIN information_schema.table_constraints AS tc
            ON tc.constraint_schema = kcu.constraint_schema
           AND tc.constraint_name = kcu.constraint_name
           AND tc.table_schema = kcu.table_schema
           AND tc.table_name = kcu.table_name
         WHERE kcu.table_schema = 'public'
           AND kcu.table_name = 'sub2api_optimize_logs'
           AND kcu.column_name = 'provider_id'
           AND tc.constraint_type = 'FOREIGN KEY'
    ) THEN
        ALTER TABLE sub2api_optimize_logs
            ADD CONSTRAINT fk_sub2api_optimize_logs_provider
            FOREIGN KEY (provider_id)
            REFERENCES sub2api_providers(id)
            ON DELETE CASCADE
            NOT VALID;
    END IF;
END
$$;

ALTER TABLE sub2api_optimize_logs
    VALIDATE CONSTRAINT fk_sub2api_optimize_logs_provider;

ALTER TABLE sub2api_optimize_logs
    DROP CONSTRAINT IF EXISTS check_sub2api_optimize_logs_trigger;

ALTER TABLE sub2api_optimize_logs
    ADD CONSTRAINT check_sub2api_optimize_logs_trigger
    CHECK (trigger IN ('cron', 'schedule_now', 'probe_unhealthy', 'manual_account', 'manual_all', 'legacy'));

COMMENT ON COLUMN sub2api_optimize_logs.provider_id IS '所属上游 Provider ID，审计主归属';
COMMENT ON COLUMN sub2api_optimize_logs.schedule_id IS '关联的定时配置 ID，仅 cron/schedule_now 使用，可空';
COMMENT ON COLUMN sub2api_optimize_logs.trigger IS '触发方式：cron/schedule_now/probe_unhealthy/manual_account/manual_all/legacy';
