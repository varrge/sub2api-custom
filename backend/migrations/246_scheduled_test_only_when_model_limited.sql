-- Existing plans retain unconditional cron behavior.
ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS only_when_model_limited BOOLEAN NOT NULL DEFAULT false;
