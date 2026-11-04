CREATE TABLE IF NOT EXISTS push_notification_config (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(id),
    revision BIGINT NOT NULL DEFAULT 1,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    channels JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO push_notification_config(id) VALUES(TRUE) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS push_notification_deliveries (
    id BIGSERIAL PRIMARY KEY,
    channel_id TEXT NOT NULL,
    channel_name TEXT NOT NULL,
    event TEXT NOT NULL,
    event_key TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','sending','sent','failed','cancelled')),
    attempts INTEGER NOT NULL DEFAULT 0,
    invalidated BOOLEAN NOT NULL DEFAULT FALSE,
    error TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    UNIQUE(channel_id,event,event_key)
);
CREATE INDEX IF NOT EXISTS push_notification_deliveries_pending_idx
    ON push_notification_deliveries(next_attempt_at,id) WHERE status IN ('pending','sending');
-- Capture transitions in the transaction; outbound HTTP only runs in the worker.
CREATE TABLE IF NOT EXISTS push_notification_events (
    event TEXT NOT NULL,
    event_key TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    values JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY(event,event_key)
);
CREATE INDEX IF NOT EXISTS push_notification_events_time_idx ON push_notification_events(event,occurred_at);

CREATE OR REPLACE FUNCTION capture_push_notification_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE event_name TEXT; event_values JSONB;
BEGIN
    IF TG_TABLE_NAME='payment_orders' THEN
        IF NEW.status<>'COMPLETED' OR (TG_OP='UPDATE' AND OLD.status='COMPLETED') THEN RETURN NEW; END IF;
        event_name := CASE NEW.order_type WHEN 'balance' THEN 'payment.recharged' WHEN 'subscription' THEN 'subscription.purchased' WHEN 'month_card' THEN 'monthcard.purchased' ELSE NULL END;
        event_values := jsonb_build_object('user_id',NEW.user_id::text,'order_id',NEW.id::text,'amount',NEW.pay_amount::text,'currency',UPPER(COALESCE(NULLIF(NEW.provider_snapshot->>'currency',''),'CNY')));
        IF NEW.order_type='month_card' THEN
            -- Failed joins/coupons also become COMPLETED before automatic refunds.
            -- Only an issued card proves successful month-card fulfillment.
            IF NOT EXISTS(SELECT 1 FROM month_card_cards WHERE order_id=NEW.id) THEN RETURN NEW; END IF;
            SELECT event_values || jsonb_build_object('product_name',c.product_name,'team_code',COALESCE(t.code,''))
              INTO event_values FROM month_card_cards c LEFT JOIN month_card_teams t ON t.id=c.team_id
             WHERE c.order_id=NEW.id;
        END IF;
    ELSIF TG_TABLE_NAME='month_card_teams' THEN
        IF NEW.status<>'full' OR (TG_OP='UPDATE' AND OLD.status='full') THEN RETURN NEW; END IF;
        event_name := 'team.full';
        event_values := jsonb_build_object('team_code',NEW.code,'product_name',COALESCE(NEW.product_snapshot->>'name','月卡'),'member_count',NEW.member_count::text);
    ELSIF TG_TABLE_NAME='ops_alert_events' THEN
        event_name := 'ops.alert';
        event_values := jsonb_build_object('summary',COALESCE(NEW.title,'运行告警'),'description',COALESCE(NEW.description,''),'severity',NEW.severity);
    END IF;
    IF event_name IS NOT NULL THEN
        INSERT INTO push_notification_events(event,event_key,values) VALUES(event_name,NEW.id::text,event_values) ON CONFLICT DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS push_notification_payment ON payment_orders;
CREATE TRIGGER push_notification_payment AFTER INSERT OR UPDATE OF status ON payment_orders
    FOR EACH ROW EXECUTE FUNCTION capture_push_notification_event();
DROP TRIGGER IF EXISTS push_notification_team ON month_card_teams;
CREATE TRIGGER push_notification_team AFTER INSERT OR UPDATE OF status ON month_card_teams
    FOR EACH ROW EXECUTE FUNCTION capture_push_notification_event();
DROP TRIGGER IF EXISTS push_notification_ops ON ops_alert_events;
CREATE TRIGGER push_notification_ops AFTER INSERT ON ops_alert_events
    FOR EACH ROW EXECUTE FUNCTION capture_push_notification_event();
