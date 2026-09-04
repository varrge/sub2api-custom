ALTER TABLE groups ADD COLUMN IF NOT EXISTS temporary_rate_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS temporary_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS temporary_rate_starts_at TIMESTAMPTZ NULL;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS temporary_rate_ends_at TIMESTAMPTZ NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'groups_temporary_rate_multiplier_positive'
          AND conrelid = 'groups'::regclass
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_temporary_rate_multiplier_positive
            CHECK (temporary_rate_multiplier > 0);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'groups_temporary_rate_window_valid'
          AND conrelid = 'groups'::regclass
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_temporary_rate_window_valid
            CHECK (
                (temporary_rate_starts_at IS NULL AND temporary_rate_ends_at IS NULL)
                OR (
                    temporary_rate_starts_at IS NOT NULL
                    AND temporary_rate_ends_at IS NOT NULL
                    AND temporary_rate_starts_at < temporary_rate_ends_at
                )
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'groups_temporary_rate_enabled_has_window'
          AND conrelid = 'groups'::regclass
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_temporary_rate_enabled_has_window
            CHECK (
                NOT temporary_rate_enabled
                OR (temporary_rate_starts_at IS NOT NULL AND temporary_rate_ends_at IS NOT NULL)
            );
    END IF;
END $$;

-- These fields are part of the API-key auth snapshot. Keep out-of-band group
-- edits from leaving cached billing data stale (latest body: migration 193).
CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.allow_image_generation IS NOT DISTINCT FROM NEW.allow_image_generation
       AND OLD.platform IS NOT DISTINCT FROM NEW.platform
       AND OLD.subscription_type IS NOT DISTINCT FROM NEW.subscription_type
       AND OLD.rate_multiplier IS NOT DISTINCT FROM NEW.rate_multiplier
       AND OLD.temporary_rate_enabled IS NOT DISTINCT FROM NEW.temporary_rate_enabled
       AND OLD.temporary_rate_multiplier IS NOT DISTINCT FROM NEW.temporary_rate_multiplier
       AND OLD.temporary_rate_starts_at IS NOT DISTINCT FROM NEW.temporary_rate_starts_at
       AND OLD.temporary_rate_ends_at IS NOT DISTINCT FROM NEW.temporary_rate_ends_at
       AND OLD.peak_rate_enabled IS NOT DISTINCT FROM NEW.peak_rate_enabled
       AND OLD.peak_start IS NOT DISTINCT FROM NEW.peak_start
       AND OLD.peak_end IS NOT DISTINCT FROM NEW.peak_end
       AND OLD.peak_rate_multiplier IS NOT DISTINCT FROM NEW.peak_rate_multiplier
       AND OLD.profit_control_enabled IS NOT DISTINCT FROM NEW.profit_control_enabled
       AND OLD.profit_min_margin IS NOT DISTINCT FROM NEW.profit_min_margin
       AND OLD.profit_safety_buffer IS NOT DISTINCT FROM NEW.profit_safety_buffer
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.group_id = target_group_id
      AND k.deleted_at IS NULL
      AND k.key <> '';
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
