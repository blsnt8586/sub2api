-- 生成内容（图片/视频/音频）的元数据表。真正的字节存在对象存储（Wasabi），
-- 本表只存 s3_key 引用与前端展示所需的元数据。media_key 是前端的 storageKey
-- （如 image:<nanoid> / video:<nanoid>），画布节点里存的就是它，稳定不变。
CREATE TABLE IF NOT EXISTS canvas_media (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL,
    media_key   TEXT        NOT NULL,
    kind        TEXT        NOT NULL,
    s3_key      TEXT        NOT NULL,
    mime_type   TEXT        NOT NULL DEFAULT '',
    bytes       BIGINT      NOT NULL DEFAULT 0,
    width       INTEGER,
    height      INTEGER,
    duration_ms INTEGER,
    pinned      BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 同一用户下 media_key 唯一，支撑幂等 upsert 与按 key 取回。
CREATE UNIQUE INDEX IF NOT EXISTS uq_canvas_media_user_key
    ON canvas_media (user_id, media_key);

-- 列表查询：按用户 + 类型取最近生成的内容。
CREATE INDEX IF NOT EXISTS idx_canvas_media_user_kind
    ON canvas_media (user_id, kind, created_at DESC);

-- 生命周期清理：按创建时间扫未收藏的过期内容（pinned 的永久保留）。
CREATE INDEX IF NOT EXISTS idx_canvas_media_expiry
    ON canvas_media (created_at) WHERE pinned = false;
