-- Route security-relevant administrator audit entries into the push-notification
-- queue. The queue worker performs outbound HTTP; this trigger only inserts rows
-- in the same transaction as the audit entry.
CREATE OR REPLACE FUNCTION capture_admin_activity_push_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    recent INTEGER;
BEGIN
    IF NEW.action IN ('auth.password.forgot', 'auth.password.reset') THEN
        INSERT INTO push_notification_events(event, event_key, occurred_at, values)
        VALUES ('admin.activity', 'audit:' || NEW.id::text, NEW.created_at, jsonb_build_object(
            'summary', NEW.action,
            'action', NEW.action,
            'method', NEW.method,
            'path', NEW.path,
            'client_ip', NEW.client_ip,
            'status_code', NEW.status_code::text,
            'request_id', NEW.request_id
        ))
        ON CONFLICT DO NOTHING;
        RETURN NEW;
    END IF;
    IF NEW.path NOT LIKE '/api/v1/admin/%' THEN
        RETURN NEW;
    END IF;
    IF NEW.action NOT IN (
        'admin.accounts.export',
        'admin.accounts.import',
        'admin.proxies.export',
        'admin.redeem_codes.export',
        'admin.backups.create',
        'admin.backups.download',
        'admin.backups.restore',
        'admin.backups.delete',
        'admin.backups.s3_config.update',
        'admin.data_management.s3_config.update',
        'admin.admin_api_key.read',
        'admin.admin_api_key.regenerate',
        'admin.admin_api_key.delete',
        'admin.audit_log.clear',
        'admin.settings.update',
        'admin.users.create',
        'admin.users.update',
        'admin.users.delete',
        'admin.users.api_keys.read',
        'admin.groups.api_keys.read',
        'admin.push_notifications.update'
    ) THEN
        RETURN NEW;
    END IF;
    -- Bound notification volume: a looping or hostile caller must not turn the
    -- admin audit stream into an outbound HTTP flood.
    SELECT count(*) INTO recent FROM push_notification_events
     WHERE event = 'admin.activity' AND occurred_at > clock_timestamp() - interval '10 minutes';
    IF recent >= 60 THEN
        RETURN NEW;
    END IF;
    INSERT INTO push_notification_events(event, event_key, occurred_at, values)
    VALUES ('admin.activity', 'audit:' || NEW.id::text, NEW.created_at, jsonb_build_object(
        'summary', NEW.action,
        'action', NEW.action,
        'method', NEW.method,
        'path', NEW.path,
        'client_ip', NEW.client_ip,
        'actor_email', NEW.actor_email,
        'auth_method', NEW.auth_method,
        'status_code', NEW.status_code::text,
        'request_id', NEW.request_id
    ))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS push_notification_admin_activity ON audit_logs;
CREATE TRIGGER push_notification_admin_activity AFTER INSERT ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION capture_admin_activity_push_event();

-- Deliveries are operational breadcrumbs, not the audit trail itself.
CREATE INDEX IF NOT EXISTS push_notification_deliveries_created_idx
    ON push_notification_deliveries(created_at);
