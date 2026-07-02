-- 账号新增定时优化字段
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sub2api_max_multiplier DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS sub2api_test_model     VARCHAR(100);

COMMENT ON COLUMN accounts.sub2api_max_multiplier IS '定时优化最高可接受倍率，NULL 表示不参与定时优化';
COMMENT ON COLUMN accounts.sub2api_test_model     IS '定时优化测试模型，NULL 时按平台使用默认模型';
