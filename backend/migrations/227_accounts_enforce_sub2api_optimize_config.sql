-- 参与定时优化必须同时配置倍率下限、倍率上限和测试模型。
-- 历史版本曾允许“已参与 + 下限为空”的记录存在；升级时只关闭参与开关，
-- 保留账号已填写的其他值，避免定时任务继续静默空跑。
UPDATE accounts
   SET sub2api_optimize_enabled = FALSE,
       updated_at = NOW()
 WHERE sub2api_optimize_enabled = TRUE
   AND (
       sub2api_min_multiplier IS NULL
       OR sub2api_max_multiplier IS NULL
       OR sub2api_test_model IS NULL
       OR BTRIM(sub2api_test_model) = ''
   );

ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS check_sub2api_optimize_required_config;

ALTER TABLE accounts
    ADD CONSTRAINT check_sub2api_optimize_required_config
    CHECK (
        sub2api_optimize_enabled = FALSE
        OR (
            sub2api_min_multiplier IS NOT NULL
            AND sub2api_max_multiplier IS NOT NULL
            AND sub2api_test_model IS NOT NULL
            AND BTRIM(sub2api_test_model) <> ''
        )
    );
