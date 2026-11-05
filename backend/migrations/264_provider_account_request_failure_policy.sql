UPDATE provider_account_hybrid_health
SET policy = (
    (CASE WHEN policy = '{
      "enabled": true,
      "window_seconds": 600,
      "minimum_samples": 3,
      "degraded_latency_ms": 25000,
      "timeout_ms": 60000,
      "failure_threshold": 2,
      "affected_users_threshold": 3,
      "session_avoid_seconds": 300,
      "probe_interval_seconds": 60,
      "probe_timeout_seconds": 60,
      "recovery_successes": 2
    }'::jsonb THEN jsonb_set(policy, '{window_seconds}', '300'::jsonb) ELSE policy END)
    - 'affected_users_threshold'
  ) || jsonb_build_object(
    'account_failure_threshold', COALESCE(policy->'account_failure_threshold', '3'::jsonb),
    'recovery_delay_seconds', COALESCE(policy->'recovery_delay_seconds', '300'::jsonb)
  ),
  revision = revision + 1,
  observe_since = CASE WHEN phase = 'observing' THEN NOW() ELSE observe_since END,
  next_probe_at = CASE WHEN phase = 'recovering' THEN GREATEST(COALESCE(next_probe_at, NOW()), NOW() + INTERVAL '5 minutes') ELSE next_probe_at END,
  updated_at = NOW();
