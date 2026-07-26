-- 画布项目表。data 存整个 CanvasProject 对象（nodes/connections/chatSessions/viewport 等），
-- title 冗余出来供列表查询与排序。client_id 是前端生成的 nanoid，按 user 隔离。
CREATE TABLE IF NOT EXISTS canvas_projects (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    client_id  TEXT        NOT NULL,
    title      TEXT        NOT NULL DEFAULT '',
    data       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 同一用户下 client_id 唯一，支撑按 (user_id, client_id) 的 upsert。
CREATE UNIQUE INDEX IF NOT EXISTS uq_canvas_projects_user_client
    ON canvas_projects (user_id, client_id);

-- 列表查询：按用户取最近更新的画布。
CREATE INDEX IF NOT EXISTS idx_canvas_projects_user_updated
    ON canvas_projects (user_id, updated_at DESC);
