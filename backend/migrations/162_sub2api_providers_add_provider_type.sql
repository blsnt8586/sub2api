-- 为 sub2api_providers 增加 provider_type（上游类型）字段。
-- 语义：标识上游实例的协议/接口类型，决定同步、登录、分组等接口逻辑走哪套实现。
-- 与历史上被删除的 platform 字段（账号平台 anthropic/openai/gemini，见 158 迁移）语义不同——
-- 一个上游实例下可挂多种平台的账号，provider_type 描述的是上游本身的协议类型。
-- 当前仅支持 sub2api，后续扩展其他上游协议时放宽 CHECK 约束即可。
ALTER TABLE sub2api_providers
    ADD COLUMN IF NOT EXISTS provider_type VARCHAR(50) NOT NULL DEFAULT 'sub2api';

-- 历史数据：存量上游全部视为 sub2api（列默认值已覆盖，这里显式兜底 NULL/空值）
UPDATE sub2api_providers
   SET provider_type = 'sub2api'
 WHERE provider_type IS NULL
    OR provider_type = '';

-- CHECK 约束：限定受支持的上游类型（幂等：先删后建）
ALTER TABLE sub2api_providers
    DROP CONSTRAINT IF EXISTS check_provider_type;
ALTER TABLE sub2api_providers
    ADD CONSTRAINT check_provider_type CHECK (provider_type IN ('sub2api'));

-- 索引：按上游类型过滤（未来多类型时有用）
CREATE INDEX IF NOT EXISTS idx_sub2api_providers_provider_type
    ON sub2api_providers(provider_type);

COMMENT ON COLUMN sub2api_providers.provider_type IS '上游类型：sub2api（当前唯一），决定接口逻辑走哪套实现';
