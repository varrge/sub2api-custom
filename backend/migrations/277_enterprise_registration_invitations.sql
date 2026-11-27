-- Codes issued by enterprises never gain rebate eligibility after conversion.
ALTER TABLE native_invitations
    ADD COLUMN IF NOT EXISTS rebate_eligible BOOLEAN NOT NULL DEFAULT TRUE;
