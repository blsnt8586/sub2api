-- =============================================================================
-- Sub2API 二开升级脚本
-- 包含：上游管理 + 定时优化 + 倍率区间 等全部新增表结构
-- 执行方法：psql -U <user> -d <dbname> -f sub2api_upgrade.sql
-- 幂等：可重复执行，已存在的表/列/约束不会报错
-- =============================================================================

BEGIN;

-- -----------------------------------------------------------------------------
-- 029: 确保 updated_at 触发器函数存在
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- -----------------------------------------------------------------------------
-- 030: 创建 sub2api_providers 表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sub2api_providers (
    id                 BIGSERIAL PRIMARY KEY,
    name               VARCHAR(100) NOT NULL,
    base_url           VARCHAR(500) NOT NULL,
    status             VARCHAR(20)  NOT NULL DEFAULT 'active',
    notes              TEXT,
    email              VARCHAR(200) NOT NULL,
    password_encrypted TEXT         NOT NULL,
    api_path_keys      VARCHAR(100),
    api_path_groups    VARCHAR(100),
    last_sync_at       TIMESTAMPTZ,
    last_sync_status   VARCHAR(20),
    last_sync_error    TEXT,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    CONSTRAINT check_provider_status      CHECK (status IN ('active', 'inactive')),
    CONSTRAINT check_provider_sync_status CHECK (last_sync_status IS NULL OR last_sync_status IN ('success', 'failed', 'pending'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sub2api_providers_unique
    ON sub2api_providers(base_url, email)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sub2api_providers_status     ON sub2api_providers(status);
CREATE INDEX IF NOT EXISTS idx_sub2api_providers_deleted_at ON sub2api_providers(deleted_at);

DROP TRIGGER IF EXISTS update_sub2api_providers_updated_at ON sub2api_providers;
CREATE TRIGGER update_sub2api_providers_updated_at
    BEFORE UPDATE ON sub2api_providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- -----------------------------------------------------------------------------
-- 031: accounts 表增加 Provider 关联字段
-- -----------------------------------------------------------------------------
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='accounts' AND column_name='provider_id') THEN
        ALTER TABLE accounts ADD COLUMN provider_id BIGINT REFERENCES sub2api_providers(id) ON DELETE SET NULL;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='accounts' AND column_name='provider_api_key_id') THEN
        ALTER TABLE accounts ADD COLUMN provider_api_key_id BIGINT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='accounts' AND column_name='remote_group_name') THEN
        ALTER TABLE accounts ADD COLUMN remote_group_name VARCHAR(100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='accounts' AND column_name='remote_group_multiplier') THEN
        ALTER TABLE accounts ADD COLUMN remote_group_multiplier DECIMAL(10,4);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='accounts' AND column_name='remote_group_synced_at') THEN
        ALTER TABLE accounts ADD COLUMN remote_group_synced_at TIMESTAMPTZ;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_accounts_provider_id ON accounts(provider_id);

-- -----------------------------------------------------------------------------
-- 158: sub2api_providers 移除 platform 列（早期设计遗留）
-- -----------------------------------------------------------------------------
ALTER TABLE sub2api_providers DROP COLUMN IF EXISTS platform;

-- -----------------------------------------------------------------------------
-- 159: accounts 增加定时优化字段
-- -----------------------------------------------------------------------------
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sub2api_max_multiplier DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS sub2api_test_model     VARCHAR(100);

-- -----------------------------------------------------------------------------
-- 160: 定时优化调度表 + 执行日志表
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sub2api_optimize_schedules (
    id          BIGSERIAL   PRIMARY KEY,
    provider_id BIGINT      NOT NULL REFERENCES sub2api_providers(id) ON DELETE CASCADE,
    cron_expr   VARCHAR(64) NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider_id)
);

CREATE TABLE IF NOT EXISTS sub2api_optimize_logs (
    id          BIGSERIAL   PRIMARY KEY,
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

-- -----------------------------------------------------------------------------
-- 161: accounts 增加 sub2api_optimize_enabled 开关
-- -----------------------------------------------------------------------------
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sub2api_optimize_enabled BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE accounts
   SET sub2api_optimize_enabled = TRUE
 WHERE sub2api_max_multiplier IS NOT NULL
   AND deleted_at IS NULL;

-- -----------------------------------------------------------------------------
-- 162: sub2api_providers 增加 provider_type 字段
-- -----------------------------------------------------------------------------
ALTER TABLE sub2api_providers
    ADD COLUMN IF NOT EXISTS provider_type VARCHAR(50) NOT NULL DEFAULT 'sub2api';

UPDATE sub2api_providers SET provider_type = 'sub2api'
 WHERE provider_type IS NULL OR provider_type = '';

ALTER TABLE sub2api_providers DROP CONSTRAINT IF EXISTS check_provider_type;
ALTER TABLE sub2api_providers ADD CONSTRAINT check_provider_type
    CHECK (provider_type IN ('sub2api'));

CREATE INDEX IF NOT EXISTS idx_sub2api_providers_provider_type ON sub2api_providers(provider_type);

-- -----------------------------------------------------------------------------
-- 163: accounts 增加 sub2api_min_multiplier（倍率下限）
-- -----------------------------------------------------------------------------
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sub2api_min_multiplier DECIMAL(10,4);

-- -----------------------------------------------------------------------------
-- 164: sub2api_min_multiplier CHECK 约束
-- -----------------------------------------------------------------------------
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS check_sub2api_min_multiplier;
ALTER TABLE accounts ADD CONSTRAINT check_sub2api_min_multiplier
    CHECK (sub2api_min_multiplier IS NULL OR sub2api_min_multiplier >= 0);

ALTER TABLE accounts DROP CONSTRAINT IF EXISTS check_sub2api_min_le_max;
ALTER TABLE accounts ADD CONSTRAINT check_sub2api_min_le_max
    CHECK (
        sub2api_min_multiplier IS NULL
        OR sub2api_max_multiplier IS NULL
        OR sub2api_min_multiplier <= sub2api_max_multiplier
    );

-- =============================================================================
-- 确保 schema_migrations 表存在（迁移 runner 用，防止新二进制重复跑已执行的迁移）
-- =============================================================================
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename   TEXT PRIMARY KEY,
    checksum   TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 将以上迁移文件标记为已执行（checksum 填 'manual'，runner 遇到已记录的文件会跳过）
INSERT INTO schema_migrations (filename, checksum) VALUES
    ('029_ensure_update_updated_at_column_function.sql', 'manual'),
    ('030_create_sub2api_providers.sql',                 'manual'),
    ('031_add_provider_to_accounts.sql',                 'manual'),
    ('158_sub2api_providers_drop_platform.sql',          'manual'),
    ('159_accounts_add_sub2api_optimize_fields.sql',     'manual'),
    ('160_sub2api_optimize_schedule_and_log.sql',        'manual'),
    ('161_accounts_add_sub2api_optimize_enabled.sql',    'manual'),
    ('162_sub2api_providers_add_provider_type.sql',      'manual'),
    ('163_accounts_add_sub2api_min_multiplier.sql',      'manual'),
    ('164_accounts_sub2api_min_multiplier_constraints.sql', 'manual')
ON CONFLICT (filename) DO NOTHING;  -- 已存在则跳过，不覆盖

COMMIT;

-- 执行结束，输出确认
DO $$ BEGIN RAISE NOTICE 'Sub2API 升级脚本执行完成'; END $$;
