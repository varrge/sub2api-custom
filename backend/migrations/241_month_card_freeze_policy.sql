-- Global administrator controlled window during which users may freeze cards.
CREATE TABLE IF NOT EXISTS month_card_freeze_policy (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);
INSERT INTO month_card_freeze_policy(id, enabled)
VALUES (TRUE, FALSE)
ON CONFLICT (id) DO NOTHING;
