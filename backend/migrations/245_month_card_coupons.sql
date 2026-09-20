-- Payment discounts are separate from registration promo codes.
CREATE TABLE IF NOT EXISTS month_card_coupons (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('fixed', 'percent')),
    value NUMERIC(14,2) NOT NULL CHECK (value > 0 AND (kind <> 'percent' OR value < 100)),
    product_id BIGINT REFERENCES month_card_products(id),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    max_uses INTEGER NOT NULL DEFAULT 0 CHECK (max_uses >= 0),
    per_user_limit INTEGER NOT NULL DEFAULT 1 CHECK (per_user_limit > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS month_card_coupon_uses (
    order_id BIGINT PRIMARY KEY REFERENCES payment_orders(id),
    coupon_id BIGINT NOT NULL REFERENCES month_card_coupons(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    redeemed BOOLEAN NOT NULL DEFAULT FALSE,
    released BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS month_card_coupon_uses_coupon_user_idx
    ON month_card_coupon_uses(coupon_id, user_id);
