-- New API reuses provider/account relationships; only protocol and auth checks change.
ALTER TABLE sub2api_providers DROP CONSTRAINT IF EXISTS check_provider_type;
ALTER TABLE sub2api_providers ADD CONSTRAINT check_provider_type
    CHECK (provider_type IN ('sub2api', 'newapi'));
ALTER TABLE sub2api_providers DROP CONSTRAINT IF EXISTS sub2api_providers_auth_mode_check;
ALTER TABLE sub2api_providers ADD CONSTRAINT sub2api_providers_auth_mode_check
    CHECK ((provider_type = 'sub2api' AND auth_mode IN ('password', 'token_pair'))
        OR (provider_type = 'newapi' AND auth_mode IN ('password', 'access_token')));
COMMENT ON COLUMN sub2api_providers.email IS 'Sub2API email; New API username or numeric user ID for management token auth';
COMMENT ON COLUMN sub2api_providers.provider_type IS 'Control-plane protocol: sub2api, newapi';
