ALTER TABLE sub2api_provider_probe_target_runs
    ADD COLUMN IF NOT EXISTS answer_probe BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE sub2api_provider_probe_targets
    ADD COLUMN IF NOT EXISTS answer_probe_interval_seconds INTEGER NOT NULL DEFAULT 300;
