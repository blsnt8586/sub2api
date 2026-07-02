-- 定时优化「是否参与」独立布尔字段。
-- 与 sub2api_max_multiplier/sub2api_test_model 解耦：关闭参与时保留倍率上限与测试模型的历史值，
-- 仅用该开关控制是否纳入定时优化，避免关闭后丢失已配置的数据。
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sub2api_optimize_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- 历史数据：已设置倍率上限的账号视为已参与，迁移后保持参与状态
UPDATE accounts
   SET sub2api_optimize_enabled = TRUE
 WHERE sub2api_max_multiplier IS NOT NULL
   AND deleted_at IS NULL;
