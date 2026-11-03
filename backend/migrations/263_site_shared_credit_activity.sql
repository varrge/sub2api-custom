ALTER TABLE site_shared_credit_pool
    ADD COLUMN name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN starts_at TIMESTAMPTZ,
    ADD COLUMN ends_at TIMESTAMPTZ,
    ADD CONSTRAINT site_shared_credit_activity_window CHECK (starts_at IS NULL OR ends_at IS NULL OR starts_at < ends_at);

ALTER TABLE site_shared_credit_changes
    ADD COLUMN old_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN old_starts_at TIMESTAMPTZ,
    ADD COLUMN starts_at TIMESTAMPTZ,
    ADD COLUMN old_ends_at TIMESTAMPTZ,
    ADD COLUMN ends_at TIMESTAMPTZ;
