-- Align newly created account probe routes with the account-plane defaults.
-- Existing target rows are intentionally preserved; these ALTER statements
-- only affect future inserts that omit the column values.

ALTER TABLE sub2api_provider_probe_targets
    ALTER COLUMN interval_seconds SET DEFAULT 30,
    ALTER COLUMN timeout_seconds SET DEFAULT 60,
    ALTER COLUMN degraded_latency_ms SET DEFAULT 5000;
