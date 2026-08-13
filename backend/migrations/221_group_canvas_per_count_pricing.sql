-- canvas 平台异步任务按次定价：图像 / 音频。
-- 对应 ent schema groups.canvas_image_price_per_count / canvas_audio_price_per_count。
-- NULL = 使用全局视频价格兜底；显式 0 = 免费；>0 = 分组覆盖价。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS canvas_image_price_per_count DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS canvas_audio_price_per_count DECIMAL(20,8);
