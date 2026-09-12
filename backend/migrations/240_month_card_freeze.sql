-- A frozen card keeps its paid clock and quota window paused. `expires_at`
-- remains the unpaused anchor; paused_us is added when the card is read.
ALTER TABLE month_card_cards DROP CONSTRAINT IF EXISTS month_card_cards_status_check;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='month_card_cards_status_check') THEN
        ALTER TABLE month_card_cards ADD CONSTRAINT month_card_cards_status_check CHECK (status IN ('active','frozen','expired','revoked'));
    END IF;
END $$;
ALTER TABLE month_card_cards ADD COLUMN IF NOT EXISTS paused_us BIGINT NOT NULL DEFAULT 0 CHECK (paused_us >= 0);
ALTER TABLE month_card_cards ADD COLUMN IF NOT EXISTS frozen_at TIMESTAMPTZ;
ALTER TABLE month_card_cards ADD COLUMN IF NOT EXISTS thawed_at TIMESTAMPTZ;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='month_card_cards_frozen_clock_check') THEN
        ALTER TABLE month_card_cards ADD CONSTRAINT month_card_cards_frozen_clock_check CHECK (status <> 'frozen' OR frozen_at IS NOT NULL);
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS month_card_cards_frozen_idx ON month_card_cards(user_id,group_id) WHERE status='frozen';

ALTER TABLE month_card_teams DROP CONSTRAINT IF EXISTS month_card_teams_status_check;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='month_card_teams_status_check') THEN
        ALTER TABLE month_card_teams ADD CONSTRAINT month_card_teams_status_check CHECK (status IN ('recruiting','full','closed','cancelled'));
    END IF;
END $$;
ALTER TABLE month_card_teams ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ;
