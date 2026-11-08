CREATE TABLE IF NOT EXISTS radar_feedback_threads (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'replied', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_radar_feedback_threads_user_updated
    ON radar_feedback_threads (user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_radar_feedback_threads_status_updated
    ON radar_feedback_threads (status, updated_at DESC);

CREATE TABLE IF NOT EXISTS radar_feedback_messages (
    id BIGSERIAL PRIMARY KEY,
    thread_id BIGINT NOT NULL REFERENCES radar_feedback_threads(id) ON DELETE CASCADE,
    author_type TEXT NOT NULL CHECK (author_type IN ('user', 'admin')),
    author_id BIGINT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_radar_feedback_messages_thread_created
    ON radar_feedback_messages (thread_id, created_at ASC);

CREATE TABLE IF NOT EXISTS radar_feedback_attachments (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES radar_feedback_messages(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    content BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_radar_feedback_attachments_message
    ON radar_feedback_attachments (message_id);
