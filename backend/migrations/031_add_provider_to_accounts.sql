-- 扩展 accounts 表，增加 Provider 关联字段

DO $$
BEGIN
    -- 添加 provider_id 字段（如果不存在）
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='accounts' AND column_name='provider_id') THEN
        ALTER TABLE accounts ADD COLUMN provider_id BIGINT REFERENCES sub2api_providers(id) ON DELETE SET NULL;
    END IF;

    -- 添加 provider_api_key_id 字段（如果不存在）
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='accounts' AND column_name='provider_api_key_id') THEN
        ALTER TABLE accounts ADD COLUMN provider_api_key_id BIGINT;
    END IF;

    -- 添加 remote_group_name 字段（如果不存在）
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='accounts' AND column_name='remote_group_name') THEN
        ALTER TABLE accounts ADD COLUMN remote_group_name VARCHAR(100);
    END IF;

    -- 添加 remote_group_multiplier 字段（如果不存在）
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='accounts' AND column_name='remote_group_multiplier') THEN
        ALTER TABLE accounts ADD COLUMN remote_group_multiplier DECIMAL(10,4);
    END IF;

    -- 添加 remote_group_synced_at 字段（如果不存在）
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='accounts' AND column_name='remote_group_synced_at') THEN
        ALTER TABLE accounts ADD COLUMN remote_group_synced_at TIMESTAMPTZ;
    END IF;
END $$;

-- 索引
CREATE INDEX IF NOT EXISTS idx_accounts_provider_id ON accounts(provider_id);

-- 注释
COMMENT ON COLUMN accounts.provider_id IS '关联的第三方 Sub2API Provider ID';
COMMENT ON COLUMN accounts.provider_api_key_id IS '远程 Sub2API 实例上的 APIKey ID';
COMMENT ON COLUMN accounts.remote_group_name IS '远程分组名称（缓存）';
COMMENT ON COLUMN accounts.remote_group_multiplier IS '远程分组倍率（缓存）';
COMMENT ON COLUMN accounts.remote_group_synced_at IS '最后同步时间';
