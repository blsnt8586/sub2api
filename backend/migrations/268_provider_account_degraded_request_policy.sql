-- Add independent slow-request controls without replacing account-specific settings.
UPDATE provider_account_hybrid_health
SET policy = policy || jsonb_build_object(
      'degraded_threshold', COALESCE(policy->'degraded_threshold', '2'::jsonb),
      'account_degraded_threshold', COALESCE(policy->'account_degraded_threshold', '5'::jsonb)
    ),
    revision = revision + 1,
    observe_since = CASE WHEN phase = 'observing' THEN NOW() ELSE observe_since END,
    recovery_successes = 0,
    updated_at = NOW()
WHERE NOT (policy ? 'degraded_threshold' AND policy ? 'account_degraded_threshold');
