-- Route-level monitoring for Sub2API. This is additive: existing Provider
-- control-plane probe configuration and historical rows are preserved.

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS remote_group_id BIGINT;

CREATE TABLE IF NOT EXISTS sub2api_provider_probe_targets (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES sub2api_providers(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    provider_api_key_id BIGINT,
    remote_group_id BIGINT,
    remote_group_name TEXT,
    platform VARCHAR(50) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    interval_seconds INTEGER NOT NULL DEFAULT 1800 CHECK (interval_seconds BETWEEN 60 AND 86400),
    test_model VARCHAR(160),
    allow_media_probe BOOLEAN NOT NULL DEFAULT FALSE,
    timeout_seconds INTEGER NOT NULL DEFAULT 15 CHECK (timeout_seconds BETWEEN 3 AND 120),
    degraded_latency_ms INTEGER NOT NULL DEFAULT 2000 CHECK (degraded_latency_ms BETWEEN 100 AND 120000),
    failure_threshold INTEGER NOT NULL DEFAULT 3 CHECK (failure_threshold BETWEEN 1 AND 20),
    recovery_threshold INTEGER NOT NULL DEFAULT 2 CHECK (recovery_threshold BETWEEN 1 AND 20),
    last_run_at TIMESTAMPTZ,
    route_changed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_sub2api_probe_target_provider_account UNIQUE (provider_id, account_id)
);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_targets_due ON sub2api_provider_probe_targets(provider_id, enabled, last_run_at);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_targets_account ON sub2api_provider_probe_targets(account_id);

CREATE TABLE IF NOT EXISTS sub2api_provider_probe_target_runs (
    id BIGSERIAL PRIMARY KEY,
    target_id BIGINT NOT NULL REFERENCES sub2api_provider_probe_targets(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL REFERENCES sub2api_providers(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    provider_api_key_id BIGINT,
    remote_group_id BIGINT,
    remote_group_name TEXT,
    platform VARCHAR(50) NOT NULL DEFAULT '',
    model_id VARCHAR(160),
    status VARCHAR(20) NOT NULL CHECK (status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    latency_ms INTEGER,
    traffic_request_count INTEGER NOT NULL DEFAULT 0,
    traffic_success_rate NUMERIC(7,4),
    traffic_p95_latency_ms INTEGER,
    error_category VARCHAR(64),
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_target_runs_target_created ON sub2api_provider_probe_target_runs(target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_target_runs_provider_created ON sub2api_provider_probe_target_runs(provider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_target_runs_account_created ON sub2api_provider_probe_target_runs(account_id, created_at DESC);

DROP TRIGGER IF EXISTS update_sub2api_provider_probe_targets_updated_at ON sub2api_provider_probe_targets;
CREATE TRIGGER update_sub2api_provider_probe_targets_updated_at
    BEFORE UPDATE ON sub2api_provider_probe_targets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
