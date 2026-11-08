-- 269: Store optional configured-answer probe prompts and raw upstream answers.
ALTER TABLE sub2api_provider_probe_targets
    ADD COLUMN IF NOT EXISTS answer_probe_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS answer_probe_prompt TEXT;

ALTER TABLE sub2api_provider_probe_target_runs
    ADD COLUMN IF NOT EXISTS response_text TEXT;
