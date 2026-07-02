-- 为 accounts 增加 sub2api_min_multiplier（定时优化倍率下限）字段。
-- 语义：定时优化只在 [下限, 上限] 区间内挑最便宜的可用分组。
-- 目的：最便宜的分组往往是超卖/特价/不稳定区（易被上游 GROUP_DISABLED 或返回 5xx），
--       下限用于设一个质量底线——拿一点成本换稳定性。
-- null = 未设下限，保持原行为（从最便宜的候选开始试）。
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sub2api_min_multiplier DECIMAL(10,4);

COMMENT ON COLUMN accounts.sub2api_min_multiplier IS '定时优化最低可接受倍率，null 表示不设下限';
