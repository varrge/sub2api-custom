ALTER TABLE users ADD COLUMN IF NOT EXISTS account_type VARCHAR(20) NOT NULL DEFAULT 'personal'
    CHECK (account_type IN ('personal', 'enterprise'));
ALTER TABLE users ADD CONSTRAINT users_enterprise_role_check CHECK (account_type <> 'enterprise' OR role <> 'admin');
ALTER TABLE user_group_rate_multipliers ADD COLUMN IF NOT EXISTS enterprise_rate_multiplier NUMERIC(20,8)
    CHECK (enterprise_rate_multiplier > 0 AND enterprise_rate_multiplier <> 'NaN'::numeric);

ALTER TABLE month_card_cards DROP CONSTRAINT IF EXISTS month_card_cards_status_check;
ALTER TABLE month_card_cards ADD CONSTRAINT month_card_cards_status_check
    CHECK (status IN ('active','frozen','expired','revoked','converted'));

CREATE TABLE enterprise_conversions (
    id BIGSERIAL PRIMARY KEY,
    operation_id VARCHAR(100) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    actor_id BIGINT NOT NULL REFERENCES users(id),
    account_type VARCHAR(20) NOT NULL CHECK (account_type IN ('personal','enterprise')),
    remaining_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    conversion_rate NUMERIC(20,8) NOT NULL DEFAULT 0,
    credited_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    balance_before NUMERIC(20,8) NOT NULL,
    balance_after NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX enterprise_conversions_user_created ON enterprise_conversions(user_id,created_at);
CREATE TABLE enterprise_conversion_cards (
    conversion_id BIGINT NOT NULL REFERENCES enterprise_conversions(id),
    card_id BIGINT NOT NULL UNIQUE REFERENCES month_card_cards(id),
    order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    remaining_usd NUMERIC(20,8) NOT NULL,
    credited_usd NUMERIC(20,8) NOT NULL,
    PRIMARY KEY (conversion_id,card_id)
);

-- Durable barriers survive process failure. A stale barrier requires an operator
-- to reconcile its request before removing it; it never expires into free money.
CREATE TABLE enterprise_account_requests (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX enterprise_account_requests_user ON enterprise_account_requests(user_id);
ALTER TABLE enterprise_conversion_cards ADD COLUMN refunded_at TIMESTAMPTZ;
-- Preserve the actual consumer rate in historical statements.
ALTER TABLE usage_logs ALTER COLUMN rate_multiplier TYPE NUMERIC(20,8);
