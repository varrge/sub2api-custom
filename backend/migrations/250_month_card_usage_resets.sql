-- A reset opens a new counter generation. Requests already admitted continue
-- settling against their original counters, including durable async retries.
ALTER TABLE month_card_cards
    ADD COLUMN IF NOT EXISTS total_usage_generation BIGINT NOT NULL DEFAULT 0 CHECK (total_usage_generation >= 0),
    ADD COLUMN IF NOT EXISTS weekly_usage_generation BIGINT NOT NULL DEFAULT 0 CHECK (weekly_usage_generation >= 0);

-- Current total usage stays on the card; each reset archives the previous cap
-- and usage so old requests cannot refill the newly reset visible counter.
CREATE TABLE IF NOT EXISTS month_card_total_usage_history (
    card_id BIGINT NOT NULL REFERENCES month_card_cards(id),
    generation BIGINT NOT NULL CHECK (generation >= 0),
    quota_usd NUMERIC(20,8) NOT NULL CHECK (quota_usd > 0),
    used_usd NUMERIC(20,8) NOT NULL CHECK (used_usd >= 0 AND used_usd <= quota_usd),
    PRIMARY KEY (card_id,generation)
);

CREATE TABLE IF NOT EXISTS month_card_usage_resets (
    id BIGSERIAL PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES month_card_cards(id),
    -- Historical actor identity, matching quota-adjustment audits. An FK
    -- would lock the actor after card locks and break the global lock order.
    actor_user_id BIGINT NOT NULL,
    reset_total BOOLEAN NOT NULL,
    reset_weekly BOOLEAN NOT NULL,
    previous_total_used_usd NUMERIC(20,8) NOT NULL,
    previous_weekly_used_usd NUMERIC(20,8) NOT NULL,
    total_generation BIGINT NOT NULL,
    weekly_generation BIGINT NOT NULL,
    weekly_window_start TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (reset_total OR reset_weekly)
);
CREATE INDEX IF NOT EXISTS month_card_usage_resets_card_idx ON month_card_usage_resets(card_id,id);
