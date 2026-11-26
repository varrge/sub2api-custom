-- Security audit rows remain available even when notifications are disabled.
-- Only metadata enters the outbound queue; never copy request_body or headers.
CREATE INDEX IF NOT EXISTS idx_push_notification_security_access_bucket
    ON push_notification_events ((values->>'notification_bucket'))
    WHERE event IN ('security.login_failed','security.suspicious_ip');

CREATE OR REPLACE FUNCTION capture_security_access_push_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    event_name TEXT;
    source_ip TEXT;
    bucket BIGINT;
    key TEXT;
    event_values JSONB;
BEGIN
    IF NEW.status_code >= 400 AND NEW.status_code < 500 AND (
        NEW.action IN ('auth.login', 'auth.login.2fa') OR
        (NEW.path = '/api/v1/auth/passkey/login/begin' AND NEW.method = 'POST') OR
        (NEW.path LIKE '/api/v1/auth/oauth/%/bind-login' AND NEW.method = 'POST')
    ) THEN
        event_name := 'security.login_failed';
        -- Old/compatibility audit client_ip may trust raw headers. Only the new
        -- middleware's trusted-chain attribution is eligible for notifications.
        source_ip := NEW.extra->>'security_client_ip';
    ELSIF NEW.action IN ('security.ip_probe', 'security.access_denied') THEN
        event_name := 'security.suspicious_ip';
        source_ip := NEW.client_ip;
    ELSE
        RETURN NEW;
    END IF;
    IF COALESCE(source_ip, '') = '' THEN RETURN NEW; END IF;
    IF NOT EXISTS (
        SELECT 1 FROM push_notification_config c,
            LATERAL jsonb_array_elements(c.channels) ch
        WHERE c.enabled AND ch->>'enabled' = 'true' AND ch->'events' ? event_name
    ) THEN RETURN NEW; END IF;

    -- Serialize the small cap check across instances; unique keys also make
    -- repeated audit inserts harmless. No network work runs in this transaction.
    PERFORM pg_advisory_xact_lock(hashtext('sub2api.security-access-push'));
    bucket := floor(extract(epoch FROM clock_timestamp()) / 600)::bigint;
    key := 'access:' || md5(source_ip) || ':' || bucket::text;
    IF EXISTS (SELECT 1 FROM push_notification_events WHERE event=event_name AND event_key=key)
        OR (SELECT count(*) FROM push_notification_events
            WHERE event IN ('security.login_failed','security.suspicious_ip')
            AND values->>'notification_bucket' = bucket::text) >= 60 THEN
        RETURN NEW;
    END IF;
    event_values := jsonb_build_object(
        -- Limit by queue time, not the possibly delayed/skewed audit time.
        -- Preserve occurred_at below so enabling a channel never replays old activity.
        'notification_bucket', bucket::text,
        'summary', CASE event_name WHEN 'security.login_failed' THEN '登录失败，请核对来源'
            ELSE '检测到疑似接口探测或重复未授权访问' END,
        'action', NEW.action,
        'client_ip', source_ip,
        'peer_ip', COALESCE(NEW.extra->>'peer_ip',''),
        'ip_source', COALESCE(NEW.extra->>'ip_source',''),
        'reason', COALESCE(NEW.extra->>'reason','login_failed'),
        'request_count', COALESCE(NEW.extra->>'request_count','1'),
        'user_id', COALESCE(NEW.actor_user_id::text,''),
        'actor_email', CASE WHEN NEW.actor_user_id IS NOT NULL THEN NEW.actor_email ELSE '' END,
        'method', NEW.method,
        'path', NEW.path,
        'status_code', NEW.status_code::text,
        'request_id', NEW.request_id
    );
    INSERT INTO push_notification_events(event,event_key,occurred_at,values)
        VALUES(event_name,key,NEW.created_at,event_values)
        ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS push_notification_security_access ON audit_logs;
CREATE TRIGGER push_notification_security_access AFTER INSERT ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION capture_security_access_push_event();
