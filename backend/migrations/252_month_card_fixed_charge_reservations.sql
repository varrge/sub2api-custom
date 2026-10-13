-- Fixed cache charges retain their original entitlement snapshot until the
-- usage transaction commits or the upstream operation is explicitly aborted.
CREATE TABLE IF NOT EXISTS month_card_fixed_charge_reservations (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    snapshot JSONB NOT NULL,
    amount_usd NUMERIC(20,8) NOT NULL CHECK (amount_usd > 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS month_card_fixed_charge_reservations_user_idx
    ON month_card_fixed_charge_reservations(user_id, created_at, id);
