-- Key restrictions are independent of group model permissions and default to
-- unrestricted for existing keys and older writers that omit this column.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS model_allowlist JSONB NOT NULL DEFAULT '{"enabled":false,"models":[]}'::jsonb;
