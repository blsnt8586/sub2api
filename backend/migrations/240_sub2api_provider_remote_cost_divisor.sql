-- Allow each remote Sub2API provider to normalize monetary overview values.
-- A value of 10 means the displayed amount is upstream amount / 10.

ALTER TABLE sub2api_providers
    ADD COLUMN IF NOT EXISTS remote_cost_divisor DECIMAL(20,8) NOT NULL DEFAULT 1.0;

UPDATE sub2api_providers
   SET remote_cost_divisor = 1.0
 WHERE remote_cost_divisor IS NULL
    OR remote_cost_divisor <= 0;

ALTER TABLE sub2api_providers
    DROP CONSTRAINT IF EXISTS sub2api_providers_remote_cost_divisor_positive;

ALTER TABLE sub2api_providers
    ADD CONSTRAINT sub2api_providers_remote_cost_divisor_positive
    CHECK (remote_cost_divisor > 0);

COMMENT ON COLUMN sub2api_providers.remote_cost_divisor IS
    '远程概览金额换算除数；展示金额 = 上游金额 ÷ 此值';
