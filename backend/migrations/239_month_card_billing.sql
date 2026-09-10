-- Historical windows are immutable identities, so a late completion never borrows
-- quota from the window currently displayed on a card or legacy subscription.
CREATE TABLE IF NOT EXISTS month_card_period_usage (
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('card','legacy')),
    entitlement_id BIGINT NOT NULL,
    period_kind VARCHAR(10) NOT NULL CHECK (period_kind IN ('daily','weekly','monthly')),
    window_start TIMESTAMPTZ NOT NULL,
    generation BIGINT NOT NULL DEFAULT 0,
    used_usd NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (used_usd >= 0),
    PRIMARY KEY (kind, entitlement_id, period_kind, window_start, generation)
);

-- A manual daily reset retains the same calendar-midnight window_start. Give
-- that freshly granted quota a separate identity while in-flight work keeps its
-- original generation. The trigger also covers existing repository/admin paths.
CREATE TABLE IF NOT EXISTS month_card_legacy_window_generations (
    subscription_id BIGINT NOT NULL REFERENCES user_subscriptions(id),
    period_kind VARCHAR(10) NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    generation BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY(subscription_id,period_kind,window_start)
);
CREATE OR REPLACE FUNCTION month_card_legacy_reset_generation() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF current_setting('sub2api.month_card_explicit_reset',true) = '1' THEN
        RETURN NEW;
    END IF;
    IF NEW.daily_window_start IS NOT DISTINCT FROM OLD.daily_window_start
       AND NEW.daily_window_start IS NOT NULL AND NEW.daily_usage_usd < OLD.daily_usage_usd THEN
        INSERT INTO month_card_legacy_window_generations VALUES(NEW.id,'daily',NEW.daily_window_start,1)
        ON CONFLICT(subscription_id,period_kind,window_start) DO UPDATE
        SET generation=month_card_legacy_window_generations.generation+1;
    END IF;
    IF NEW.weekly_window_start IS NOT DISTINCT FROM OLD.weekly_window_start
       AND NEW.weekly_window_start IS NOT NULL AND NEW.weekly_usage_usd < OLD.weekly_usage_usd THEN
        INSERT INTO month_card_legacy_window_generations VALUES(NEW.id,'weekly',NEW.weekly_window_start,1)
        ON CONFLICT(subscription_id,period_kind,window_start) DO UPDATE
        SET generation=month_card_legacy_window_generations.generation+1;
    END IF;
    IF NEW.monthly_window_start IS NOT DISTINCT FROM OLD.monthly_window_start
       AND NEW.monthly_window_start IS NOT NULL AND NEW.monthly_usage_usd < OLD.monthly_usage_usd THEN
        INSERT INTO month_card_legacy_window_generations VALUES(NEW.id,'monthly',NEW.monthly_window_start,1)
        ON CONFLICT(subscription_id,period_kind,window_start) DO UPDATE
        SET generation=month_card_legacy_window_generations.generation+1;
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS month_card_legacy_reset_generation_trigger ON user_subscriptions;
CREATE TRIGGER month_card_legacy_reset_generation_trigger
BEFORE UPDATE ON user_subscriptions FOR EACH ROW EXECUTE FUNCTION month_card_legacy_reset_generation();

CREATE TABLE IF NOT EXISTS month_card_allocations (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(200) NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('card','legacy','balance')),
    entitlement_id BIGINT NOT NULL DEFAULT 0,
    amount_usd NUMERIC(20,8) NOT NULL CHECK (amount_usd > 0),
    weekly_window_start TIMESTAMPTZ,
    started_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (request_id, api_key_id, kind, entitlement_id)
);
CREATE INDEX IF NOT EXISTS month_card_allocations_user_created_idx
    ON month_card_allocations(user_id, created_at DESC, id DESC);

-- Store a complete immutable command before attempting monetary effects. A
-- crashed gateway or transient settlement failure can replay the same dedup key.
CREATE TABLE IF NOT EXISTS month_card_billing_pending (
    request_id VARCHAR(200) NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    request_fingerprint VARCHAR(64) NOT NULL,
    command JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id)
);
CREATE INDEX IF NOT EXISTS month_card_billing_pending_retry_idx ON month_card_billing_pending(next_retry_at);

-- Async video completion carries the create-time entitlement through restarts
-- and Redis eviction. Routing ownership is stored alongside the billing snapshot.
CREATE TABLE IF NOT EXISTS month_card_video_pending (
    request_id VARCHAR(200) NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(request_id,api_key_id,user_id)
);
