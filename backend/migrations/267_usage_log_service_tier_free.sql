-- Snapshot the waived tier surcharge; changing group policy must not relabel history.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS service_tier_free BOOLEAN NOT NULL DEFAULT FALSE;
