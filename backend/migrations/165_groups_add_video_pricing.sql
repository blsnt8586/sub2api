-- Migration 165: 为 groups 表新增视频计费价格字段（即梦 jimeng 平台）
-- video_price_per_count: 每次视频生成价格（USD/次），nil=使用默认
-- video_price_per_second: 每秒视频价格（USD/秒），非 nil 时优先按秒计费

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_price_per_count  DECIMAL(20, 8) DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS video_price_per_second DECIMAL(20, 8) DEFAULT NULL;
