-- Site-sponsored usage is settled atomically with the existing billing ledger.
CREATE TABLE site_shared_credit_pool (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    group_ids BIGINT[] NOT NULL DEFAULT '{}',
    total_usd NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (total_usd >= 0),
    used_usd NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (used_usd >= 0),
    reserved_usd NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (reserved_usd >= 0),
    revision BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (used_usd + reserved_usd <= total_usd)
);
INSERT INTO site_shared_credit_pool(id) VALUES(TRUE);

CREATE TABLE site_shared_credit_allocations (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    amount_usd NUMERIC(20,8) NOT NULL CHECK (amount_usd > 0),
    covered_cost_usd NUMERIC(20,8) NOT NULL CHECK (covered_cost_usd >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(request_id, api_key_id)
);
CREATE INDEX site_shared_credit_allocations_user_idx ON site_shared_credit_allocations(user_id, id DESC);

CREATE TABLE site_shared_credit_holds (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    amount_usd NUMERIC(20,8) NOT NULL CHECK (amount_usd > 0),
    covered_cost_usd NUMERIC(20,8) NOT NULL CHECK (covered_cost_usd >= 0),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE site_shared_credit_changes (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT NOT NULL,
    old_total_usd NUMERIC(20,8) NOT NULL,
    total_usd NUMERIC(20,8) NOT NULL,
    old_enabled BOOLEAN NOT NULL,
    enabled BOOLEAN NOT NULL,
    old_group_ids BIGINT[] NOT NULL,
    group_ids BIGINT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
