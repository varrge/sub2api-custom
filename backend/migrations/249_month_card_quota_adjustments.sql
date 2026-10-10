-- NULL preserves the original total / 4 rule until an administrator adjusts a card.
ALTER TABLE month_card_cards
    ADD COLUMN IF NOT EXISTS weekly_quota_usd NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS total_quota_manual BOOLEAN NOT NULL DEFAULT FALSE;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='month_card_cards'::regclass AND conname='month_card_weekly_quota_check') THEN
        ALTER TABLE month_card_cards ADD CONSTRAINT month_card_weekly_quota_check
            CHECK (weekly_quota_usd IS NULL OR (weekly_quota_usd > 0 AND weekly_quota_usd <= total_quota_usd));
    END IF;
END $$;

-- Keep an immutable operation history alongside the generic admin request audit.
-- Actor IDs are historical identifiers, not FKs: an admin may later be deleted.
CREATE TABLE IF NOT EXISTS month_card_quota_adjustments (
    id BIGSERIAL PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES month_card_cards(id),
    actor_user_id BIGINT NOT NULL,
    previous_total_usd NUMERIC(20,8) NOT NULL,
    total_usd NUMERIC(20,8) NOT NULL,
    previous_weekly_usd NUMERIC(20,8) NOT NULL,
    weekly_usd NUMERIC(20,8) NOT NULL,
    set_total BOOLEAN NOT NULL,
    set_weekly BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS month_card_quota_adjustments_card_idx ON month_card_quota_adjustments(card_id,id);
