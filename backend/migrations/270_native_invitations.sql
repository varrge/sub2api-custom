CREATE TABLE IF NOT EXISTS native_invitation_settings (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    settings JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO native_invitation_settings(id, settings) VALUES
(1, '{"enabled":false,"validity_days":30,"levels":[{"id":"default","name":"默认","spend_threshold":0,"lifetime_invite_limit":3,"unused_invite_limit":3,"rebate_rate":0}]}')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS native_invitation_overrides (
    user_id BIGINT PRIMARY KEY REFERENCES users(id),
    level_id VARCHAR(64) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS native_invitations (
    id BIGSERIAL PRIMARY KEY,
    redeem_id BIGINT NOT NULL UNIQUE REFERENCES redeem_codes(id) ON DELETE RESTRICT,
    inviter_id BIGINT NOT NULL REFERENCES users(id),
    request_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (inviter_id, request_id)
);
CREATE INDEX IF NOT EXISTS native_invitations_inviter_idx ON native_invitations(inviter_id, id DESC);

CREATE TABLE IF NOT EXISTS native_invitation_bindings (
    invitee_id BIGINT PRIMARY KEY REFERENCES users(id),
    invite_id BIGINT NOT NULL UNIQUE REFERENCES native_invitations(id),
    inviter_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (inviter_id <> invitee_id)
);

CREATE TABLE IF NOT EXISTS native_invitation_rebates (
    id BIGSERIAL PRIMARY KEY,
    invite_id BIGINT NOT NULL UNIQUE REFERENCES native_invitations(id),
    inviter_id BIGINT NOT NULL REFERENCES users(id),
    invitee_id BIGINT NOT NULL UNIQUE REFERENCES users(id),
    source_type VARCHAR(20) NOT NULL CHECK (source_type IN ('payment','redeem_code')),
    source_id BIGINT NOT NULL,
    source_currency VARCHAR(3) NOT NULL,
    source_occurred_at TIMESTAMPTZ NOT NULL,
    first_amount NUMERIC(20,8) NOT NULL CHECK (first_amount > 0),
    rebate_rate NUMERIC(10,6) NOT NULL CHECK (rebate_rate BETWEEN 0 AND 100),
    rebate_amount NUMERIC(20,2) NOT NULL CHECK (rebate_amount >= 0),
    level_id VARCHAR(64) NOT NULL,
    level_name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','revoked')),
    review_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS native_invitation_rebates_inviter_idx ON native_invitation_rebates(inviter_id, id DESC);
CREATE INDEX IF NOT EXISTS native_invitation_rebates_status_idx ON native_invitation_rebates(status, id DESC);

CREATE TABLE IF NOT EXISTS native_invitation_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT REFERENCES users(id),
    action VARCHAR(32) NOT NULL,
    rebate_id BIGINT REFERENCES native_invitation_rebates(id),
    user_id BIGINT REFERENCES users(id),
    reason TEXT NOT NULL DEFAULT '',
    balance_delta NUMERIC(20,2) NOT NULL DEFAULT 0,
    detail JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS native_invitation_audit_rebate_idx ON native_invitation_audit(rebate_id, id);
