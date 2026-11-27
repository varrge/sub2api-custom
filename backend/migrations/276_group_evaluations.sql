CREATE TABLE IF NOT EXISTS group_evaluation_configs (
    group_id BIGINT PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL DEFAULT 1,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    candy_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    pelican_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    automatic BOOLEAN NOT NULL DEFAULT FALSE,
    interval_seconds INTEGER NOT NULL DEFAULT 600 CHECK (interval_seconds BETWEEN 10 AND 86400),
    model VARCHAR(200) NOT NULL DEFAULT '',
    effort VARCHAR(10) NOT NULL DEFAULT 'low' CHECK (effort IN ('low','medium','high')),
    runs INTEGER NOT NULL DEFAULT 3 CHECK (runs BETWEEN 1 AND 10),
    daily_limit INTEGER NOT NULL DEFAULT 100 CHECK (daily_limit BETWEEN 1 AND 1000),
    usage_date DATE,
    used_requests INTEGER NOT NULL DEFAULT 0,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    lease_token UUID,
    leased_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (NOT enabled OR (model<>'' AND (candy_enabled OR pelican_enabled)))
);
CREATE INDEX IF NOT EXISTS group_evaluation_due ON group_evaluation_configs(next_run_at)
    WHERE enabled AND automatic;

CREATE TABLE IF NOT EXISTS group_evaluation_records (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('candy','pelican')),
    status VARCHAR(12) NOT NULL CHECK (status IN ('passed','mismatch','generated','error')),
    model VARCHAR(200) NOT NULL,
    effort VARCHAR(10) NOT NULL,
    answer TEXT NOT NULL DEFAULT '',
    html TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    expected_answer VARCHAR(20) NOT NULL DEFAULT '',
    prompt_version VARCHAR(80) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (octet_length(answer)<=8192 AND octet_length(html)<=65536 AND octet_length(error)<=512)
);
CREATE INDEX IF NOT EXISTS group_evaluation_recent ON group_evaluation_records(group_id,kind,created_at DESC,id DESC);
