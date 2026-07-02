-- 创建 sub2api_providers 表
-- 用于存储第三方 Sub2API 实例的管理凭证

CREATE TABLE IF NOT EXISTS sub2api_providers (
    id BIGSERIAL PRIMARY KEY,

    -- 基本信息
    name VARCHAR(100) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    notes TEXT,

    -- 认证信息（阶段1明文存储，阶段7加密）
    email VARCHAR(200) NOT NULL,
    password_encrypted TEXT NOT NULL,

    -- API 路径配置（自动检测后缓存）
    api_path_keys VARCHAR(100),
    api_path_groups VARCHAR(100),

    -- 同步状态
    last_sync_at TIMESTAMPTZ,
    last_sync_status VARCHAR(20),
    last_sync_error TEXT,

    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- 约束
    CONSTRAINT check_provider_platform CHECK (platform IN ('anthropic', 'openai', 'gemini')),
    CONSTRAINT check_provider_status CHECK (status IN ('active', 'inactive')),
    CONSTRAINT check_provider_sync_status CHECK (last_sync_status IS NULL OR last_sync_status IN ('success', 'failed', 'pending'))
);

-- 唯一索引：同一个 base_url + email 只能有一个（软删除时排除）
CREATE UNIQUE INDEX IF NOT EXISTS idx_sub2api_providers_unique
    ON sub2api_providers(base_url, email)
    WHERE deleted_at IS NULL;

-- 索引
CREATE INDEX IF NOT EXISTS idx_sub2api_providers_platform ON sub2api_providers(platform);
CREATE INDEX IF NOT EXISTS idx_sub2api_providers_status ON sub2api_providers(status);
CREATE INDEX IF NOT EXISTS idx_sub2api_providers_deleted_at ON sub2api_providers(deleted_at);

-- 触发器：自动更新 updated_at
DROP TRIGGER IF EXISTS update_sub2api_providers_updated_at ON sub2api_providers;
CREATE TRIGGER update_sub2api_providers_updated_at
    BEFORE UPDATE ON sub2api_providers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 注释
COMMENT ON TABLE sub2api_providers IS '第三方 Sub2API Provider 配置表';
COMMENT ON COLUMN sub2api_providers.name IS 'Provider 显示名称';
COMMENT ON COLUMN sub2api_providers.base_url IS 'Provider 基础 URL，如 https://api.example.com';
COMMENT ON COLUMN sub2api_providers.platform IS '平台类型：anthropic, openai, gemini';
COMMENT ON COLUMN sub2api_providers.email IS '登录邮箱';
COMMENT ON COLUMN sub2api_providers.password_encrypted IS '登录密码（阶段1明文，阶段7加密）';
COMMENT ON COLUMN sub2api_providers.api_path_keys IS 'APIKey 列表路径（自动检测后缓存）';
COMMENT ON COLUMN sub2api_providers.api_path_groups IS '分组列表路径（自动检测后缓存）';
COMMENT ON COLUMN sub2api_providers.last_sync_at IS '最后同步时间';
COMMENT ON COLUMN sub2api_providers.last_sync_status IS '最后同步状态：success, failed, pending';
COMMENT ON COLUMN sub2api_providers.last_sync_error IS '同步错误信息';
