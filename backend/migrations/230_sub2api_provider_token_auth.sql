-- 为 Sub2API 上游增加可持久化的 Token Pair 认证。
-- 旧 Provider 默认保留 password 模式；所有秘密字段均可空，升级不要求回填。

ALTER TABLE sub2api_providers
    ADD COLUMN IF NOT EXISTS auth_mode VARCHAR(32) NOT NULL DEFAULT 'password',
    ADD COLUMN IF NOT EXISTS access_token_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS refresh_token_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS access_token_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_token_refresh_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_auth_error TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'sub2api_providers_auth_mode_check'
          AND conrelid = 'sub2api_providers'::regclass
    ) THEN
        ALTER TABLE sub2api_providers
            ADD CONSTRAINT sub2api_providers_auth_mode_check
            CHECK (auth_mode IN ('password', 'token_pair'));
    END IF;
END
$$;

COMMENT ON COLUMN sub2api_providers.auth_mode IS '认证方式：password, token_pair';
COMMENT ON COLUMN sub2api_providers.access_token_encrypted IS 'AES-GCM 加密的上游 Access Token';
COMMENT ON COLUMN sub2api_providers.refresh_token_encrypted IS 'AES-GCM 加密的上游 Refresh Token';
COMMENT ON COLUMN sub2api_providers.access_token_expires_at IS '上游 Access Token 的保守失效时间';
COMMENT ON COLUMN sub2api_providers.last_token_refresh_at IS '最近一次持久化 Token 对的时间';
COMMENT ON COLUMN sub2api_providers.last_auth_error IS '最近一次认证错误（不含凭据正文）';
