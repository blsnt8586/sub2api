-- Migration 193: platform="jimeng" 统一重命名为 "canvas"
--
-- 背景：platform=jimeng 是历史遗留标识（源自"即梦"原生平台），
-- 2026-08 重构将 avi2api 网关统一命名为 canvas。
-- 代码已全面切换到 PlatformCanvas 常量；存量数据库行需同步。
--
-- 影响三张表：accounts / groups / user_platform_quotas
--
-- 幂等：WHERE platform='jimeng' 过滤已迁移的行，重复运行安全。
--
-- H2 防御：user_platform_quotas 在 (user_id, platform) 上有局部唯一索引
-- （WHERE deleted_at IS NULL）。若某 user 已同时存在 platform='jimeng'
-- 和 platform='canvas' 的未删除行，直接 UPDATE 会触发唯一约束冲突。
-- 先软删除重复的 jimeng 行（保留 canvas 行），再执行 UPDATE。

-- user_platform_quotas: 先软删有冲突的 jimeng 行
UPDATE user_platform_quotas uq
SET    deleted_at = NOW(),
       updated_at = NOW()
WHERE  uq.platform    = 'jimeng'
  AND  uq.deleted_at  IS NULL
  AND  EXISTS (
      SELECT 1
      FROM   user_platform_quotas uq2
      WHERE  uq2.user_id     = uq.user_id
        AND  uq2.platform    = 'canvas'
        AND  uq2.deleted_at  IS NULL
  );

-- user_platform_quotas: 再安全重命名剩余 jimeng 行
UPDATE user_platform_quotas
SET    platform   = 'canvas',
       updated_at = NOW()
WHERE  platform = 'jimeng';

-- accounts: platform 字段（无跨行唯一约束，直接更新）
UPDATE accounts
SET    platform   = 'canvas',
       updated_at = NOW()
WHERE  platform = 'jimeng';

-- groups: platform 字段（无跨行唯一约束，直接更新）
UPDATE groups
SET    platform   = 'canvas',
       updated_at = NOW()
WHERE  platform = 'jimeng';

-- 注：accounts.credentials 里的 vendor 字段无需迁移。
-- GetCanvasVendor() 把空串和 "avi2api" 都当默认 vendor，"leonardo"（历史）同样兜底。
