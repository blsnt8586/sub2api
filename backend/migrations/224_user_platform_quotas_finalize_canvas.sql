-- Finalize the quota-platform constraint after the legacy jimeng -> canvas rename.
-- This intentionally repeats migration 223's final state. Deployments that
-- already applied 193/223 before receiving the prerequisite 192a migration will
-- apply 192a later, so a higher-numbered migration must tighten the temporary
-- jimeng+canvas constraint again.
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'canvas'));
