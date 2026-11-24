-- Rejected/rate-limited password requests must not scan every user while auditing.
-- Non-unique so legacy normalized-email collisions remain detectable.
CREATE INDEX IF NOT EXISTS idx_users_active_login_email
    ON users ((LOWER(TRIM(email)))) WHERE deleted_at IS NULL;

-- Keep audit history and sent deliveries, but do not send/replay the previous
-- all-user login-failure queue after narrowing the existing subscription.
LOCK TABLE audit_logs IN SHARE ROW EXCLUSIVE MODE;
SELECT id FROM push_notification_config WHERE id=TRUE FOR UPDATE;
UPDATE push_notification_deliveries
    SET invalidated=TRUE,
        status=CASE WHEN status='pending' THEN 'cancelled' ELSE status END,
        error='登录失败通知已收窄为管理员账号'
    WHERE event='security.login_failed' AND status IN ('pending','sending');
DELETE FROM push_notification_events WHERE event='security.login_failed';

CREATE OR REPLACE FUNCTION capture_security_access_push_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    event_name TEXT;
    source_ip TEXT;
    bucket BIGINT;
    key TEXT;
    event_values JSONB;
    target_id BIGINT;
    target_email TEXT;
BEGIN
    IF NEW.status_code >= 400 AND NEW.status_code < 500 AND NEW.method='POST' AND (
        NEW.path IN ('/api/v1/auth/login', '/api/v1/auth/login/2fa', '/api/v1/auth/passkey/login/finish') OR
        NEW.path LIKE '/api/v1/auth/oauth/%/bind-login'
    ) THEN
        event_name := 'security.login_failed';
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

    IF event_name='security.login_failed' THEN
        -- This ID is set only by server-side account/session/credential lookups.
        -- An attempted target is not an authenticated actor.
        IF NEW.extra ? 'login_target_user_id' THEN
            BEGIN
                target_id := (NEW.extra->>'login_target_user_id')::bigint;
            EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range THEN
                RETURN NEW;
            END;
            SELECT u.email INTO target_email FROM users u
                WHERE u.id=target_id AND u.role='admin' AND u.deleted_at IS NULL;
        ELSIF NEW.path='/api/v1/auth/login' OR NEW.path LIKE '/api/v1/auth/oauth/%/bind-login' THEN
            -- Before captcha/rate-limit rejection, email is only a lookup hint.
            -- Match GetByEmail semantics, including rejecting ambiguous matches.
            SELECT MIN(u.id), MIN(u.email) INTO target_id, target_email FROM users u
                WHERE u.deleted_at IS NULL
                  AND LOWER(TRIM(u.email))=LOWER(TRIM(NEW.extra->>'login_target_email'))
                HAVING count(*)=1 AND bool_and(u.role='admin');
        END IF;
        IF target_email IS NULL THEN RETURN NEW; END IF;
    END IF;

    PERFORM pg_advisory_xact_lock(hashtext('sub2api.security-access-push'));
    bucket := floor(extract(epoch FROM clock_timestamp()) / 600)::bigint;
    -- New login key namespace avoids a cancelled old delivery suppressing the
    -- first administrator failure from the same IP after this migration.
    key := CASE event_name WHEN 'security.login_failed' THEN 'admin-login:' ELSE 'access:' END
        || md5(source_ip) || ':' || bucket::text;
    IF EXISTS (SELECT 1 FROM push_notification_events WHERE event=event_name AND event_key=key)
        OR (SELECT count(*) FROM push_notification_events
            WHERE event IN ('security.login_failed','security.suspicious_ip')
            AND values->>'notification_bucket' = bucket::text) >= 60 THEN
        RETURN NEW;
    END IF;
    event_values := jsonb_build_object(
        'notification_bucket', bucket::text,
        'summary', CASE event_name WHEN 'security.login_failed' THEN '管理员账号登录失败，请核对来源'
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
    IF event_name='security.login_failed' THEN
        event_values := (event_values - 'user_id' - 'actor_email') ||
            jsonb_build_object('target_user_id',target_id::text,'target_email',target_email);
    END IF;
    INSERT INTO push_notification_events(event,event_key,occurred_at,values)
        VALUES(event_name,key,NEW.created_at,event_values)
        ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;
