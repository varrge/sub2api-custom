-- Independent month cards: deliberately preserve the legacy subscription indexes.
CREATE TABLE IF NOT EXISTS month_card_products (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id),
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_cny NUMERIC(20,8) NOT NULL CHECK (price_cny > 0),
    base_quota_usd NUMERIC(20,8) NOT NULL CHECK (base_quota_usd > 0),
    tiers JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tiers) = 'array'),
    max_members INTEGER NOT NULL CHECK (max_members >= 2),
    recruitment_hours INTEGER NOT NULL DEFAULT 48 CHECK (recruitment_hours > 0),
    for_sale BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS month_card_products_sale_idx ON month_card_products(for_sale,sort_order,id);

CREATE TABLE IF NOT EXISTS month_card_teams (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(40) NOT NULL UNIQUE,
    product_id BIGINT NOT NULL REFERENCES month_card_products(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    product_snapshot JSONB NOT NULL,
    member_count INTEGER NOT NULL DEFAULT 0 CHECK (member_count >= 0),
    current_quota_usd NUMERIC(20,8) NOT NULL CHECK (current_quota_usd > 0),
    max_members INTEGER NOT NULL CHECK (max_members >= 2),
    starts_at TIMESTAMPTZ NOT NULL,
    closes_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'recruiting' CHECK (status IN ('recruiting','full','closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (closes_at > starts_at),
    CHECK (member_count <= max_members)
);
CREATE INDEX IF NOT EXISTS month_card_teams_recruiting_idx ON month_card_teams(group_id,closes_at) WHERE status='recruiting';

CREATE TABLE IF NOT EXISTS month_card_cards (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(40) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    order_id BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id),
    product_id BIGINT NOT NULL REFERENCES month_card_products(id),
    product_name VARCHAR(100) NOT NULL,
    group_name VARCHAR(100) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    team_id BIGINT REFERENCES month_card_teams(id),
    total_quota_usd NUMERIC(20,8) NOT NULL CHECK (total_quota_usd > 0),
    total_used_usd NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (total_used_usd >= 0 AND total_used_usd <= total_quota_usd),
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','expired','revoked')),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at = starts_at + INTERVAL '720 hours'),
    -- Retain membership even after refunds: the same user can never rejoin a team.
    UNIQUE(team_id,user_id)
);
CREATE INDEX IF NOT EXISTS month_card_cards_user_group_idx ON month_card_cards(user_id,group_id,expires_at,id);
CREATE INDEX IF NOT EXISTS month_card_cards_team_idx ON month_card_cards(team_id);

CREATE TABLE IF NOT EXISTS month_card_priorities (
    user_id BIGINT NOT NULL REFERENCES users(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('card','legacy')),
    reference_id BIGINT NOT NULL CHECK (reference_id > 0),
    priority INTEGER NOT NULL CHECK (priority >= 0),
    PRIMARY KEY(user_id,group_id,kind,reference_id),
    UNIQUE(user_id,group_id,priority)
);
