CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sub2api_optimize_logs_provider_created
    ON sub2api_optimize_logs (provider_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sub2api_optimize_logs_provider_trigger_created
    ON sub2api_optimize_logs (provider_id, trigger, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sub2api_optimize_logs_provider_status_created
    ON sub2api_optimize_logs (provider_id, status, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sub2api_optimize_logs_detail_gin
    ON sub2api_optimize_logs USING GIN (detail jsonb_path_ops);
