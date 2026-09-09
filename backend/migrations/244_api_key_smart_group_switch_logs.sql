-- Persist successful automatic API-key smart-group route changes.
-- Group names are stored as snapshots so history remains useful after a group
-- is renamed or removed.  Rows are retained for three days by the service and
-- pruned on every switch as well as by its hourly cleanup task.

CREATE TABLE IF NOT EXISTS api_key_smart_group_switch_logs (
    id              BIGSERIAL PRIMARY KEY,
    api_key_id      BIGINT NOT NULL,
    from_group_id   BIGINT NOT NULL,
    from_group_name VARCHAR(100) NOT NULL DEFAULT '',
    to_group_id     BIGINT NOT NULL,
    to_group_name   VARCHAR(100) NOT NULL DEFAULT '',
    reason          VARCHAR(32) NOT NULL,
    switched_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_key_smart_group_switch_logs_key_time
    ON api_key_smart_group_switch_logs (api_key_id, switched_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_api_key_smart_group_switch_logs_time
    ON api_key_smart_group_switch_logs (switched_at);
