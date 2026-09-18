-- Preserve the exact admitted entitlement and accounting configuration through
-- asynchronous completion, key/group edits, expiry, and billing retries.
-- Historical jobs retain their original balance-hold billing contract.
ALTER TABLE batch_image_jobs ADD COLUMN IF NOT EXISTS billing_snapshot JSONB;
