UPDATE provider_account_hybrid_health
SET policy = jsonb_set(policy, '{degraded_latency_ms}', '25000'::jsonb),
    revision = revision + 1,
    observe_since = NOW(),
    updated_at = NOW()
WHERE policy = '{
  "enabled": true,
  "window_seconds": 600,
  "minimum_samples": 3,
  "degraded_latency_ms": 5000,
  "timeout_ms": 60000,
  "failure_threshold": 2,
  "affected_users_threshold": 3,
  "session_avoid_seconds": 300,
  "probe_interval_seconds": 60,
  "probe_timeout_seconds": 60,
  "recovery_successes": 2
}'::jsonb;
