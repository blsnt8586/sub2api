CREATE TABLE IF NOT EXISTS provider_account_daily_cost_snapshots (
    provider_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    stat_date DATE NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_tokens BIGINT NOT NULL DEFAULT 0,
    estimated_upstream_cost NUMERIC(20, 10) NOT NULL DEFAULT 0,
    local_revenue NUMERIC(20, 10) NOT NULL DEFAULT 0,
    gross_profit NUMERIC(20, 10) NOT NULL DEFAULT 0,
    multiplier_snapshot NUMERIC(20, 10),
    cost_source TEXT NOT NULL DEFAULT 'local_estimate',
    sampled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider_id, account_id, stat_date)
);

CREATE INDEX IF NOT EXISTS idx_provider_account_daily_cost_date
    ON provider_account_daily_cost_snapshots (provider_id, stat_date DESC);

ALTER TABLE sub2api_provider_probe_targets
    ALTER COLUMN enabled SET DEFAULT false;
