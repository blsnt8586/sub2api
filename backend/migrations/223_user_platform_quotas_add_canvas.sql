-- Keep the database quota-platform constraint aligned with domain.AllPlatforms.
-- Canvas replaced the historical jimeng platform identifier in migration 193,
-- but migration 157's constraint still allowed only the five upstream platforms.
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'canvas'));
