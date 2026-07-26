-- 用户资产表。data 存完整 Asset 对象（data 字段随 kind 变化），
-- kind/title 冗余出来供列表查询过滤。client_id 是前端 nanoid。
CREATE TABLE IF NOT EXISTS canvas_assets (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    client_id  TEXT        NOT NULL,
    kind       TEXT        NOT NULL,
    title      TEXT        NOT NULL DEFAULT '',
    data       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_canvas_assets_user_client
    ON canvas_assets (user_id, client_id);
CREATE INDEX IF NOT EXISTS idx_canvas_assets_user_kind
    ON canvas_assets (user_id, kind, updated_at DESC);

-- 用户 AI 配置表。每个用户一行（user_id 主键），存整个 AiConfig JSON。
CREATE TABLE IF NOT EXISTS canvas_configs (
    user_id    BIGINT      PRIMARY KEY,
    config     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
