-- 确保 update_updated_at_column() 触发器函数存在。
--
-- 背景：030_create_sub2api_providers.sql（及后续若干表）通过
--   CREATE TRIGGER ... EXECUTE FUNCTION update_updated_at_column()
-- 引用该函数，但历史迁移中并没有任何文件显式创建它（早期版本由已被
-- 重整删除的迁移或外部初始化建立）。这导致「全新空库从头全量迁移」时，
-- 030 会因 "function update_updated_at_column() does not exist" 失败。
--
-- 本迁移用 CREATE OR REPLACE 幂等地补齐该函数，且编号 029 保证在 030 之前执行：
--   - 全新库：先建函数，030 起的触发器得以正常创建；
--   - 已有库：函数早已存在，CREATE OR REPLACE 以相同定义替换，无副作用。
--
-- 函数作用：行更新时自动将 updated_at 刷新为当前时间。

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_updated_at_column() IS '触发器函数：行更新时自动刷新 updated_at 为当前时间';
