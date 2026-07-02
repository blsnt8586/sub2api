-- 上游定时优化配置表（每个 Provider 最多一条）
CREATE TABLE IF NOT EXISTS sub2api_optimize_schedules (
    id          BIGSERIAL PRIMARY KEY,
    provider_id BIGINT      NOT NULL REFERENCES sub2api_providers(id) ON DELETE CASCADE,
    cron_expr   VARCHAR(64) NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider_id)
);

-- 每次定时优化运行的日志记录
CREATE TABLE IF NOT EXISTS sub2api_optimize_logs (
    id          BIGSERIAL PRIMARY KEY,
    schedule_id BIGINT      NOT NULL REFERENCES sub2api_optimize_schedules(id) ON DELETE CASCADE,
    status      VARCHAR(16) NOT NULL DEFAULT 'success',
    total       INT         NOT NULL DEFAULT 0,
    optimized   INT         NOT NULL DEFAULT 0,
    skipped     INT         NOT NULL DEFAULT 0,
    failed      INT         NOT NULL DEFAULT 0,
    detail      JSONB,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sub2api_optimize_logs_schedule_created
    ON sub2api_optimize_logs(schedule_id, created_at DESC);
