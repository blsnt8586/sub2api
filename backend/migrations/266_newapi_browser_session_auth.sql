ALTER TABLE sub2api_providers DROP CONSTRAINT IF EXISTS sub2api_providers_auth_mode_check;
ALTER TABLE sub2api_providers ADD CONSTRAINT sub2api_providers_auth_mode_check
    CHECK ((provider_type = 'sub2api' AND auth_mode IN ('password', 'token_pair'))
        OR (provider_type = 'newapi' AND auth_mode IN ('password', 'access_token', 'browser_session')));
