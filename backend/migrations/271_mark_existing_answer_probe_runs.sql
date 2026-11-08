UPDATE sub2api_provider_probe_target_runs
SET answer_probe = TRUE
WHERE response_text IS NOT NULL AND answer_probe = FALSE;
