-- Migration 192: 补偿 191_add_groups_model_pricing.sql 的副作用
--
-- 背景：191 使用了 goose 格式（-- +goose Up / -- +goose Down），
-- 但本项目的迁移执行器（migrations_runner.go）把整个 SQL 文件当原始 SQL 执行，
-- goose 指令仅是 Postgres 行注释，会被忽略。
-- 因此 191 的实际执行顺序为：ADD COLUMN → COMMENT → DROP COLUMN，
-- 净效果是 model_pricing 列被删除。
--
-- 本迁移：幂等地补回该列（ADD COLUMN IF NOT EXISTS），保证列始终存在。
-- 在已 applied 191 + canvas_model_pricing.sql 的环境（dev）上：列已存在，本迁移为 no-op。
-- 在仅 applied 191 的全新环境（生产）上：补回被删的列。
--
-- 注意：191 不能直接修改（migrations_runner.go 有 checksum 不可变校验）。

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS model_pricing JSONB DEFAULT NULL;

COMMENT ON COLUMN groups.model_pricing IS
    '模型定价配置（canvas/avi2api 平台专用）：
     {"video":{"seedance-2.0":{"per_count":0.08},"veo-3.1":{"per_second":0.02}},
      "image":{"gpt-image-2":{"1k":0.01,"2k":0.02,"4k":0.04}}}
     优先级：模型专属定价 > 分组全局定价 > 系统默认定价。
     nil = 未配置（回退全局价）；{} = 已配置但全部清空。';
