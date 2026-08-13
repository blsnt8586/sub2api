-- Prepare legacy quota rows for migration 193's jimeng -> canvas rename.
-- Standard installations could not persist jimeng rows under migration 157's
-- constraint, but some fork deployments widened it manually. Allow both names
-- before the data update so those installations can upgrade without failure.
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'jimeng', 'canvas'))
    NOT VALID;

ALTER TABLE user_platform_quotas
    VALIDATE CONSTRAINT user_platform_quotas_platform_check;
