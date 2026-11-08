ALTER TABLE provider_account_daily_cost_snapshots
    ADD COLUMN IF NOT EXISTS base_cost NUMERIC(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_cost_available BOOLEAN NOT NULL DEFAULT FALSE;

-- Existing estimates cannot safely be inverted: multiplier_snapshot records
-- account_rate_multiplier, while provider profit uses remote_group_multiplier.
-- Those legacy rows remain unavailable until a later bounded rebuild has the
-- original usage records available; new hourly snapshots write exact base cost.
