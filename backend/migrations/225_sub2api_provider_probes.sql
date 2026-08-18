-- Provider health probe configuration and append-only run history.
-- This migration is additive and safe for existing production databases.

CREATE TABLE IF NOT EXISTS sub2api_provider_probe_configs (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL UNIQUE REFERENCES sub2api_providers(id) ON DELETE CASCADE,
    control_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    control_interval_seconds INTEGER NOT NULL DEFAULT 300 CHECK (control_interval_seconds BETWEEN 60 AND 86400),
    data_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    data_interval_seconds INTEGER NOT NULL DEFAULT 1800 CHECK (data_interval_seconds BETWEEN 300 AND 86400),
    selected_account_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    allow_media_probe BOOLEAN NOT NULL DEFAULT FALSE,
    timeout_seconds INTEGER NOT NULL DEFAULT 15 CHECK (timeout_seconds BETWEEN 3 AND 120),
    degraded_latency_ms INTEGER NOT NULL DEFAULT 2000 CHECK (degraded_latency_ms BETWEEN 100 AND 120000),
    failure_threshold INTEGER NOT NULL DEFAULT 3 CHECK (failure_threshold BETWEEN 1 AND 20),
    recovery_threshold INTEGER NOT NULL DEFAULT 2 CHECK (recovery_threshold BETWEEN 1 AND 20),
    last_control_run_at TIMESTAMPTZ,
    last_data_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_configs_due ON sub2api_provider_probe_configs(control_enabled, last_control_run_at);

CREATE TABLE IF NOT EXISTS sub2api_provider_probe_runs (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES sub2api_providers(id) ON DELETE CASCADE,
    overall_status VARCHAR(20) NOT NULL CHECK (overall_status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    control_status VARCHAR(20) NOT NULL CHECK (control_status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    data_status VARCHAR(20) NOT NULL CHECK (data_status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    traffic_status VARCHAR(20) NOT NULL CHECK (traffic_status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    login_latency_ms INTEGER,
    health_latency_ms INTEGER,
    keys_latency_ms INTEGER,
    groups_latency_ms INTEGER,
    data_probe_count INTEGER NOT NULL DEFAULT 0,
    data_probe_success INTEGER NOT NULL DEFAULT 0,
    data_probe_failed INTEGER NOT NULL DEFAULT 0,
    traffic_request_count INTEGER NOT NULL DEFAULT 0,
    traffic_success_rate NUMERIC(7,4),
    traffic_p95_latency_ms INTEGER,
    error_category VARCHAR(64),
    error_message TEXT,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_runs_provider_created ON sub2api_provider_probe_runs(provider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sub2api_probe_runs_created ON sub2api_provider_probe_runs(created_at DESC);

DROP TRIGGER IF EXISTS update_sub2api_provider_probe_configs_updated_at ON sub2api_provider_probe_configs;
CREATE TRIGGER update_sub2api_provider_probe_configs_updated_at
    BEFORE UPDATE ON sub2api_provider_probe_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
