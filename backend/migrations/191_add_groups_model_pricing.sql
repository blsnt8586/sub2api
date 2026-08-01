-- +goose Up
-- 为分组表添加模型定价配置字段（即梦 jimeng 平台专用）
-- 支持按模型设置不同的价格，优先级：模型专属定价 > 分组全局定价 > 系统默认定价
ALTER TABLE groups ADD COLUMN IF NOT EXISTS model_pricing JSONB DEFAULT NULL;

COMMENT ON COLUMN groups.model_pricing IS '模型定价配置（jimeng 平台）：{"video":{"seedance-v1":{"per_count":0.08}},"image":{"leonardo-phoenix":{"1k":0.01,"2k":0.02}}}';

-- +goose Down
ALTER TABLE groups DROP COLUMN IF EXISTS model_pricing;
