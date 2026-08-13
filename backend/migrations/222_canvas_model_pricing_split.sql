-- Split the fork's Canvas pricing object from upstream's generic group pricing
-- array. Existing Canvas deployments stored an object in model_pricing, while
-- upstream v0.1.176 stores an array there. JSON shape makes the migration safe
-- and idempotent for both installations.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS canvas_model_pricing JSONB;

UPDATE groups
SET canvas_model_pricing = COALESCE(canvas_model_pricing, model_pricing),
    model_pricing = NULL
WHERE model_pricing IS NOT NULL
  AND jsonb_typeof(model_pricing) = 'object';

COMMENT ON COLUMN groups.canvas_model_pricing IS
    'Canvas image/video per-model pricing overrides';
