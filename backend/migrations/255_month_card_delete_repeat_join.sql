-- Each paid order grants its own card and recruitment seat, including repeat buyers.
ALTER TABLE month_card_cards
    DROP CONSTRAINT IF EXISTS month_card_cards_team_id_user_id_key;

-- Keep tombstones for payment idempotency, billing history and recruitment counts.
ALTER TABLE month_card_cards
    ADD COLUMN deleted_at TIMESTAMPTZ,
    ADD COLUMN deleted_by BIGINT,
    ADD COLUMN retain_team_seat BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT month_card_cards_deletion_check CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND NOT retain_team_seat)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND deleted_by > 0 AND status = 'revoked')
    );
