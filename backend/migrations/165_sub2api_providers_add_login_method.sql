-- 165: 为 sub2api_providers 添加 login_method 字段
-- login_method: http（默认，直接 HTTP 登录）或 browser（浏览器登录，用于 Turnstile 平台）

ALTER TABLE sub2api_providers
    ADD COLUMN IF NOT EXISTS login_method VARCHAR(20) NOT NULL DEFAULT 'http';

ALTER TABLE sub2api_providers
    ADD CONSTRAINT sub2api_providers_login_method_check
    CHECK (login_method IN ('http', 'browser'));
