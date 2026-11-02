CREATE TABLE month_card_gift_batches (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(80) NOT NULL UNIQUE,
    request_hash VARCHAR(64) NOT NULL,
    actor_user_id BIGINT NOT NULL,
    name VARCHAR(100) NOT NULL,
    group_id BIGINT NOT NULL REFERENCES groups(id),
    source_group_id BIGINT REFERENCES groups(id),
    include_frozen BOOLEAN NOT NULL,
    quota_usd NUMERIC(20,8) NOT NULL CHECK (quota_usd > 0),
    valid_days INTEGER NOT NULL CHECK (valid_days BETWEEN 1 AND 365),
    recipient_count INTEGER NOT NULL CHECK (recipient_count > 0),
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > created_at)
);

ALTER TABLE month_card_cards
    ALTER COLUMN order_id DROP NOT NULL,
    ALTER COLUMN product_id DROP NOT NULL,
    ADD COLUMN gift_batch_id BIGINT REFERENCES month_card_gift_batches(id);

-- Keep the purchased-card duration rule while allowing fixed gift durations.
DO $$ DECLARE constraint_name TEXT;
BEGIN
    FOR constraint_name IN SELECT conname FROM pg_constraint
        WHERE conrelid='month_card_cards'::regclass AND contype='c'
        AND pg_get_constraintdef(oid) LIKE '%expires_at =%720%'
    LOOP
        EXECUTE format('ALTER TABLE month_card_cards DROP CONSTRAINT %I', constraint_name);
    END LOOP;
END $$;

ALTER TABLE month_card_cards ADD CONSTRAINT month_card_card_origin_check CHECK (
    (gift_batch_id IS NULL AND order_id IS NOT NULL AND product_id IS NOT NULL
        AND expires_at = starts_at + INTERVAL '720 hours')
    OR (gift_batch_id IS NOT NULL AND order_id IS NULL AND product_id IS NULL AND team_id IS NULL
        AND expires_at > starts_at AND expires_at <= starts_at + INTERVAL '8760 hours'
        AND frozen_at IS NULL AND paused_us=0 AND status<>'frozen')
);
CREATE UNIQUE INDEX month_card_gift_recipient_idx ON month_card_cards(gift_batch_id,user_id) WHERE gift_batch_id IS NOT NULL;
