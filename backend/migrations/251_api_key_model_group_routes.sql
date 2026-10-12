-- Model-specific group selections remain attached to each API key.
-- Missing entries preserve existing routing. Empty arrays explicitly deny it.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS model_group_routes JSONB NOT NULL DEFAULT '{}'::jsonb;
