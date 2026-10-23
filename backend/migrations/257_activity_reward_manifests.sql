CREATE TABLE IF NOT EXISTS activity_reward_manifests (
    campaign_id TEXT NOT NULL,
    day DATE NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ NOT NULL,
    manifest TEXT NOT NULL,
    sha256 CHAR(64) NOT NULL,
    PRIMARY KEY (campaign_id, day),
    CHECK (ends_at > starts_at)
);
