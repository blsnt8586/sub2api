-- Add an optional outbound proxy to Sub2API providers. Existing providers keep
-- proxy_id=NULL and therefore preserve the current direct-connection behavior.

ALTER TABLE sub2api_providers
    ADD COLUMN IF NOT EXISTS proxy_id BIGINT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conname = 'sub2api_providers_proxy_id_fkey'
           AND conrelid = 'sub2api_providers'::regclass
    ) THEN
        ALTER TABLE sub2api_providers
            ADD CONSTRAINT sub2api_providers_proxy_id_fkey
            FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_sub2api_providers_proxy_id
    ON sub2api_providers(proxy_id)
    WHERE deleted_at IS NULL AND proxy_id IS NOT NULL;
