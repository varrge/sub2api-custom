-- Expand coupon scope without changing existing single-product/all-product codes.
ALTER TABLE month_card_coupons ADD COLUMN IF NOT EXISTS product_ids BIGINT[];
UPDATE month_card_coupons
SET product_ids = CASE WHEN product_id IS NULL THEN '{}'::BIGINT[] ELSE ARRAY[product_id] END
WHERE product_ids IS NULL;
ALTER TABLE month_card_coupons ALTER COLUMN product_ids SET DEFAULT '{}'::BIGINT[];
ALTER TABLE month_card_coupons ALTER COLUMN product_ids SET NOT NULL;
ALTER TABLE month_card_coupons DROP CONSTRAINT IF EXISTS month_card_coupons_product_ids_check;
ALTER TABLE month_card_coupons ADD CONSTRAINT month_card_coupons_product_ids_check
    CHECK (cardinality(product_ids) <= 1000 AND 0 < ALL(product_ids) AND array_position(product_ids, NULL) IS NULL);

-- Zero removes only the per-customer cap; the global cap remains independent.
ALTER TABLE month_card_coupons DROP CONSTRAINT IF EXISTS month_card_coupons_per_user_limit_check;
ALTER TABLE month_card_coupons ADD CONSTRAINT month_card_coupons_per_user_limit_check CHECK (per_user_limit >= 0);

-- An old application may temporarily write during an application-only rollback.
-- Keep its single-product writes restrictive when the new version resumes.
CREATE OR REPLACE FUNCTION sync_month_card_coupon_legacy_product() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF cardinality(NEW.product_ids) = 0 AND NEW.product_id IS NOT NULL THEN
            NEW.product_ids := ARRAY[NEW.product_id];
        END IF;
    ELSIF NEW.product_id IS DISTINCT FROM OLD.product_id
        AND NEW.product_ids IS NOT DISTINCT FROM OLD.product_ids THEN
        NEW.product_ids := CASE WHEN NEW.product_id IS NULL THEN '{}'::BIGINT[] ELSE ARRAY[NEW.product_id] END;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS month_card_coupon_legacy_product_sync ON month_card_coupons;
CREATE TRIGGER month_card_coupon_legacy_product_sync
    BEFORE INSERT OR UPDATE OF product_id, product_ids ON month_card_coupons
    FOR EACH ROW EXECUTE FUNCTION sync_month_card_coupon_legacy_product();
