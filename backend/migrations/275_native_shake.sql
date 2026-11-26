-- Native lucky box (摇摇乐). Replaces the KaWang remote service; KaWang history
-- is intentionally not imported. Prizes and usage use USD balance units; month-card order amounts use CNY.

CREATE TABLE IF NOT EXISTS shake_campaigns (
    id TEXT PRIMARY KEY,
    status VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','ended')),
    config_version INTEGER NOT NULL DEFAULT 1,
    config JSONB NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    end_reason TEXT NOT NULL DEFAULT '',
    CHECK (ends_at > starts_at)
);
CREATE INDEX IF NOT EXISTS shake_campaigns_window_idx ON shake_campaigns(status, starts_at, ends_at);

-- Every saved configuration is kept; draws reference the version they used.
CREATE TABLE IF NOT EXISTS shake_config_versions (
    campaign_id TEXT NOT NULL REFERENCES shake_campaigns(id),
    version INTEGER NOT NULL,
    config JSONB NOT NULL,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, version)
);

-- Threshold rules are recomputed from source totals (usage_logs / payment_orders),
-- so late billing rows are never lost. units = thresholds already paid out; it
-- only grows, so lowering a total (refund, rule edit) never revokes cards.
CREATE TABLE IF NOT EXISTS shake_progress (
    campaign_id TEXT NOT NULL REFERENCES shake_campaigns(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    rule_key TEXT NOT NULL,
    amount NUMERIC(30,10) NOT NULL DEFAULT 0,
    units INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, user_id, rule_key)
);
CREATE INDEX IF NOT EXISTS shake_progress_user_idx ON shake_progress(user_id, campaign_id);

-- A per-order rule credits an order once per campaign; later rule edits
-- (quantity, tier) never make the same order earn again.
CREATE TABLE IF NOT EXISTS shake_order_credits (
    campaign_id TEXT NOT NULL REFERENCES shake_campaigns(id),
    order_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id),
    rule_key TEXT NOT NULL,
    card_tier VARCHAR(8) NOT NULL,
    cards INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, order_id)
);

CREATE TABLE IF NOT EXISTS shake_draws (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(100) NOT NULL,
    campaign_id TEXT NOT NULL REFERENCES shake_campaigns(id),
    config_version INTEGER NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id),
    card_tier VARCHAR(8) NOT NULL CHECK (card_tier IN ('low','medium','high')),
    prize_id TEXT NOT NULL,
    prize_name TEXT NOT NULL,
    prize_type VARCHAR(16) NOT NULL CHECK (prize_type IN ('balance','extra_draw','empty')),
    prize_amount NUMERIC(20,8),
    prize_rarity VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'delivered' CHECK (status IN ('delivered','voided')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, request_id)
);
CREATE INDEX IF NOT EXISTS shake_draws_campaign_idx ON shake_draws(campaign_id, id DESC);
CREATE INDEX IF NOT EXISTS shake_draws_user_idx ON shake_draws(user_id, id DESC);

CREATE TABLE IF NOT EXISTS shake_cards (
    id BIGSERIAL PRIMARY KEY,
    campaign_id TEXT NOT NULL REFERENCES shake_campaigns(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    card_tier VARCHAR(8) NOT NULL CHECK (card_tier IN ('low','medium','high')),
    source VARCHAR(20) NOT NULL CHECK (source IN ('order','usage','manual','leaderboard','activity_task','extra_draw')),
    -- Deterministic per issuance unit; retries and concurrent workers cannot double-issue.
    source_ref TEXT NOT NULL,
    status VARCHAR(12) NOT NULL DEFAULT 'available' CHECK (status IN ('available','consumed','expired')),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at TIMESTAMPTZ,
    draw_id BIGINT REFERENCES shake_draws(id),
    UNIQUE (source, source_ref)
);
CREATE INDEX IF NOT EXISTS shake_cards_user_idx ON shake_cards(campaign_id, user_id, status, card_tier);

CREATE TABLE IF NOT EXISTS shake_manual_grants (
    id BIGSERIAL PRIMARY KEY,
    campaign_id TEXT NOT NULL REFERENCES shake_campaigns(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    card_tier VARCHAR(8) NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity BETWEEN 1 AND 100),
    reason TEXT NOT NULL,
    granted_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Consumption-leaderboard reward jobs bind one frozen leaderboard event to a box campaign.
CREATE TABLE IF NOT EXISTS shake_reward_jobs (
    id BIGSERIAL PRIMARY KEY,
    campaign_id TEXT NOT NULL REFERENCES shake_campaigns(id),
    source_campaign_id TEXT NOT NULL UNIQUE,
    source_starts_at TIMESTAMPTZ NOT NULL,
    source_ends_at TIMESTAMPTZ NOT NULL,
    policy JSONB NOT NULL,
    policy_fingerprint CHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_error TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shake_reward_days (
    job_id BIGINT NOT NULL REFERENCES shake_reward_jobs(id),
    day DATE NOT NULL,
    sha256 CHAR(64) NOT NULL,
    recipients INTEGER NOT NULL,
    cards INTEGER NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (job_id, day)
);

CREATE TABLE IF NOT EXISTS shake_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT,
    action VARCHAR(40) NOT NULL,
    campaign_id TEXT,
    user_id BIGINT,
    detail JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
