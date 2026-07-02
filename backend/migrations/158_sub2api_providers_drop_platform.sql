-- 从 sub2api_providers 表移除 platform 字段
-- 平台由关联账号决定，Provider 无需单独指定平台

ALTER TABLE sub2api_providers DROP COLUMN IF EXISTS platform;
