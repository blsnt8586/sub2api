-- Reading a conversation is independent of whether an administrator has replied.
-- Each administrator advances only to the last user message actually displayed.
CREATE TABLE IF NOT EXISTS radar_feedback_admin_reads (
    thread_id BIGINT NOT NULL REFERENCES radar_feedback_threads(id) ON DELETE CASCADE,
    admin_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_message_id BIGINT NOT NULL DEFAULT 0 CHECK (last_read_message_id >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (thread_id, admin_id)
);

CREATE INDEX IF NOT EXISTS idx_radar_feedback_admin_reads_admin
    ON radar_feedback_admin_reads (admin_id, thread_id);

CREATE INDEX IF NOT EXISTS idx_radar_feedback_user_messages_thread_id
    ON radar_feedback_messages (thread_id, id) WHERE author_type = 'user';
