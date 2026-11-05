CREATE TABLE IF NOT EXISTS provider_account_hybrid_health (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL REFERENCES sub2api_providers(id) ON DELETE CASCADE,
    policy JSONB NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    phase TEXT NOT NULL DEFAULT 'observing' CHECK (phase IN ('observing', 'recovering')),
    observe_since TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    quarantine_token TEXT NOT NULL DEFAULT '',
    admin_generation TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    recovery_successes INT NOT NULL DEFAULT 0,
    last_probe_at TIMESTAMPTZ,
    last_probe_status TEXT NOT NULL DEFAULT '',
    last_probe_latency_ms INT,
    next_probe_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_provider_account_hybrid_health_provider ON provider_account_hybrid_health(provider_id);
