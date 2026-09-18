-- Keep the legacy primary projection so old binaries can continue to operate.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS multi_group_enabled BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE api_keys SET group_id = NULL WHERE group_id IN (SELECT id FROM groups WHERE deleted_at IS NOT NULL);
UPDATE api_keys SET group_ids = jsonb_build_array(group_id) WHERE group_id IS NOT NULL AND group_ids = '[]'::jsonb;
CREATE INDEX IF NOT EXISTS idx_api_keys_group_ids ON api_keys USING gin(group_ids);

-- Legacy writers only change group_id. Collapse their new selection to one group,
-- rather than resurrecting the previous ordered list when the application upgrades.
-- The sticky permission boundary survives legacy edits and configuration shrinking.
CREATE OR REPLACE FUNCTION sync_api_key_group_configuration() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.group_ids = '[]'::jsonb AND NEW.group_id IS NOT NULL THEN
            NEW.group_ids := jsonb_build_array(NEW.group_id);
        END IF;
    ELSIF NEW.group_ids IS NOT DISTINCT FROM OLD.group_ids AND NEW.group_id IS DISTINCT FROM OLD.group_id THEN
        NEW.group_ids := CASE WHEN NEW.group_id IS NULL THEN '[]'::jsonb ELSE jsonb_build_array(NEW.group_id) END;
    END IF;
    IF jsonb_typeof(NEW.group_ids) <> 'array' THEN
        RAISE EXCEPTION 'api key group_ids must be an array';
    END IF;
    IF EXISTS (SELECT 1 FROM jsonb_array_elements(NEW.group_ids) v WHERE jsonb_typeof(v) <> 'number' OR v::text !~ '^[1-9][0-9]*$') THEN
        RAISE EXCEPTION 'api key group_ids must contain positive integer IDs';
    END IF;
    IF (SELECT count(*) FROM jsonb_array_elements(NEW.group_ids)) <> (SELECT count(DISTINCT v) FROM jsonb_array_elements(NEW.group_ids) v) THEN
        RAISE EXCEPTION 'api key group_ids must not contain duplicates';
    END IF;
    -- Serialize configuration writes with soft/hard deletion of every bound group.
    PERFORM id FROM groups WHERE id IN (SELECT value::bigint FROM jsonb_array_elements_text(NEW.group_ids))
        AND deleted_at IS NULL ORDER BY id FOR SHARE;
    IF (SELECT count(*) FROM groups WHERE id IN (SELECT value::bigint FROM jsonb_array_elements_text(NEW.group_ids)) AND deleted_at IS NULL)
        <> jsonb_array_length(NEW.group_ids) THEN
        RAISE EXCEPTION 'api key group_ids references a missing or deleted group';
    END IF;
    NEW.group_id := (NEW.group_ids ->> 0)::bigint;
    NEW.multi_group_enabled := NEW.multi_group_enabled OR jsonb_array_length(NEW.group_ids) > 1;
    IF TG_OP = 'UPDATE' THEN
        NEW.multi_group_enabled := NEW.multi_group_enabled OR OLD.multi_group_enabled;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS api_key_group_configuration ON api_keys;
CREATE TRIGGER api_key_group_configuration BEFORE INSERT OR UPDATE OF group_id, group_ids, multi_group_enabled ON api_keys
FOR EACH ROW EXECUTE FUNCTION sync_api_key_group_configuration();

-- Deletion is the only event that removes a temporarily unavailable binding.
-- Cover both application soft deletes and physical deletes, including old binaries.
CREATE OR REPLACE FUNCTION remove_deleted_api_key_group() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' OR (NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL) THEN
        UPDATE api_keys SET group_ids = COALESCE((
            SELECT jsonb_agg(v ORDER BY ord) FROM jsonb_array_elements(group_ids) WITH ORDINALITY AS ids(v, ord)
            WHERE v <> to_jsonb(OLD.id)
        ), '[]'::jsonb), updated_at = NOW() WHERE group_ids @> jsonb_build_array(OLD.id);
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS api_key_remove_deleted_group ON groups;
CREATE TRIGGER api_key_remove_deleted_group BEFORE DELETE OR UPDATE OF deleted_at ON groups
FOR EACH ROW EXECUTE FUNCTION remove_deleted_api_key_group();

-- New asynchronous jobs retain their original billing group. Historical jobs had
-- no reliable group snapshot, so leave them null rather than guess from edited keys.
ALTER TABLE batch_image_jobs ADD COLUMN IF NOT EXISTS group_id BIGINT NULL;
