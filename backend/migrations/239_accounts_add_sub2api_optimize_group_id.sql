-- Optional remote-group restriction for account-level automatic optimization.
-- NULL preserves the previous platform plus multiplier filtering.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sub2api_optimize_group_id BIGINT;

ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_sub2api_optimize_group_id_positive;
ALTER TABLE accounts
    ADD CONSTRAINT accounts_sub2api_optimize_group_id_positive
    CHECK (sub2api_optimize_group_id IS NULL OR sub2api_optimize_group_id > 0);

COMMENT ON COLUMN accounts.sub2api_optimize_group_id IS
    '定时优化指定远程分组 ID，NULL 表示不限制';
