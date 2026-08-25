-- Healthy account probes perform the upstream cheaper-group check after six
-- consecutive healthy samples by default. The target-level field remains
-- configurable per account (1..20); this only migrates the previous default.

ALTER TABLE sub2api_provider_probe_targets
    ALTER COLUMN cost_optimize_healthy_threshold SET DEFAULT 6;

UPDATE sub2api_provider_probe_targets
   SET cost_optimize_healthy_threshold = 6
 WHERE cost_optimize_healthy_threshold = 3;
