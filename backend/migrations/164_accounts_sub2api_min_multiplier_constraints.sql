-- 为 sub2api_min_multiplier 补充 CHECK 约束（163 只加了列，遗漏了约束）。
-- 应用层已在 UpdateAccountOptimizeSettings 校验，这里作为数据库最后防线，
-- 防止绕过应用直接写库导致「负下限」或「下限 > 上限」的非法配置。
-- 幂等：先删后建。

-- 下限非负
ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS check_sub2api_min_multiplier;
ALTER TABLE accounts
    ADD CONSTRAINT check_sub2api_min_multiplier
    CHECK (sub2api_min_multiplier IS NULL OR sub2api_min_multiplier >= 0);

-- 下限不得超过上限（任一为 null 时不约束）
ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS check_sub2api_min_le_max;
ALTER TABLE accounts
    ADD CONSTRAINT check_sub2api_min_le_max
    CHECK (
        sub2api_min_multiplier IS NULL
        OR sub2api_max_multiplier IS NULL
        OR sub2api_min_multiplier <= sub2api_max_multiplier
    );
