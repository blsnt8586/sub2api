CREATE TABLE IF NOT EXISTS token_park_rewards (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id VARCHAR(32) NOT NULL,
    campaign_version VARCHAR(32) NOT NULL,
    amount NUMERIC(20, 8) NOT NULL CHECK (amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, game_id, campaign_version)
);

CREATE INDEX IF NOT EXISTS token_park_rewards_user_idx ON token_park_rewards(user_id);
