-- 252 and 253 used the same constraint name; fresh databases kept 252's weaker check.
ALTER TABLE channel_model_pricing
    DROP CONSTRAINT IF EXISTS channel_model_pricing_ultrafast_multiplier_positive;

ALTER TABLE channel_model_pricing
    ADD CONSTRAINT channel_model_pricing_ultrafast_multiplier_positive
    CHECK (ultrafast_multiplier IS NULL OR (ultrafast_multiplier > 0 AND ultrafast_multiplier < 'Infinity'::numeric));
