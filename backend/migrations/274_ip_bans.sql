-- Permanent application-level IP bans. Rows are never deleted by the
-- application: unbanning records revoked_at so the history stays auditable.
-- Emergency unban without the admin UI:
--   UPDATE ip_bans SET revoked_at=NOW(), revoked_by='sql', revoke_note='emergency'
--    WHERE ip='<ip>' AND revoked_at IS NULL;
-- Running instances reload the active list within about 10 seconds.
CREATE TABLE IF NOT EXISTS ip_bans (
    id          BIGSERIAL PRIMARY KEY,
    -- Canonical IPv4 address, IPv6 /64 network (auto bans) or a manual CIDR.
    ip          TEXT NOT NULL,
    reason      TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT 'auto' CHECK (source IN ('auto','manual','backfill')),
    detail      JSONB NOT NULL DEFAULT '{}',
    note        TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    hits        BIGINT NOT NULL DEFAULT 0,
    last_hit_at TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    revoked_by  TEXT NOT NULL DEFAULT '',
    revoke_note TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ip_bans_active_ip ON ip_bans(ip) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ip_bans_created ON ip_bans(created_at DESC, id DESC);

-- Backfill every public source that ever tried a *@sub2api.local account
-- through the audited authentication endpoints. These addresses targeted the
-- historical default administrator identity, which no real account uses.
CREATE OR REPLACE FUNCTION pg_temp.ip_ban_key(raw TEXT) RETURNS TEXT LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE addr INET;
BEGIN
    BEGIN
        addr := host(raw::inet)::inet;
    EXCEPTION WHEN others THEN
        RETURN NULL;
    END;
    IF family(addr) = 6 AND addr << '::ffff:0:0/96'::inet THEN
        RETURN NULL; -- mapped IPv4 never reaches audit storage in canonical form
    END IF;
    IF addr << '10.0.0.0/8' OR addr << '172.16.0.0/12' OR addr << '192.168.0.0/16'
        OR addr << '127.0.0.0/8' OR addr << '169.254.0.0/16' OR addr << '100.64.0.0/10'
        OR addr << '0.0.0.0/8' OR addr << 'fc00::/7' OR addr << 'fe80::/10'
        OR addr = '::1'::inet OR addr = '::'::inet THEN
        RETURN NULL;
    END IF;
    IF family(addr) = 6 THEN
        RETURN network(set_masklen(addr, 64))::text;
    END IF;
    RETURN host(addr);
END;
$$;

WITH attempts AS (
    SELECT pg_temp.ip_ban_key(a.client_ip) AS ip,
           LOWER(substring(a.request_body FROM '"(?:email|username)"\s*:\s*"([^"]{1,255}@sub2api\.local)"')) AS email,
           a.created_at
      FROM audit_logs a
     WHERE a.path LIKE '/api/v1/auth/%'
       AND a.request_body ~* '"(email|username)"\s*:\s*"[^"]{1,255}@sub2api\.local"'
), known_user_sources AS (
    -- Addresses where any real account (administrator or customer) completed
    -- an authenticated request are usually shared exits (VPN, proxy, campus
    -- NAT). They are left for an administrator to review instead of being
    -- banned blindly; new decoy attempts after deployment still ban at once.
    SELECT DISTINCT pg_temp.ip_ban_key(client_ip) AS ip
      FROM audit_logs
     WHERE actor_user_id IS NOT NULL AND status_code < 400
), grouped AS (
    SELECT ip, COUNT(*) AS attempts, MIN(created_at) AS first_seen, MAX(created_at) AS last_seen,
           (ARRAY_AGG(DISTINCT email ORDER BY email))[1:10] AS emails
      FROM attempts
     WHERE ip IS NOT NULL AND email IS NOT NULL
     GROUP BY ip
)
INSERT INTO ip_bans(ip, reason, source, detail, created_by, note)
SELECT g.ip, 'honeypot_email', 'backfill',
       jsonb_build_object('attempted_emails', to_jsonb(g.emails), 'attempts', g.attempts,
                          'first_seen', g.first_seen, 'last_seen', g.last_seen),
       'migration', '历史审计中尝试登录 @sub2api.local 账号'
  FROM grouped g
 WHERE NOT EXISTS (SELECT 1 FROM known_user_sources s WHERE s.ip = g.ip)
   AND NOT EXISTS (
        SELECT 1 FROM users u
         WHERE u.deleted_at IS NULL AND LOWER(TRIM(u.email)) = ANY(g.emails))
ON CONFLICT (ip) WHERE revoked_at IS NULL DO NOTHING;

-- Queue a push notification for every new automatic or manual ban. Created
-- after the backfill so historical sources do not flood the channels.
CREATE OR REPLACE FUNCTION capture_ip_ban_push_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    bucket BIGINT;
BEGIN
    IF NEW.source = 'backfill' THEN RETURN NEW; END IF;
    IF NOT EXISTS (
        SELECT 1 FROM push_notification_config c,
            LATERAL jsonb_array_elements(c.channels) ch
        WHERE c.enabled AND ch->>'enabled' = 'true' AND ch->'events' ? 'security.ip_banned'
    ) THEN RETURN NEW; END IF;
    -- Cap a burst (for example a distributed scan) at 60 notifications per
    -- 10 minutes; every ban is still listed in the admin panel.
    PERFORM pg_advisory_xact_lock(hashtext('sub2api.ip-ban-push'));
    bucket := floor(extract(epoch FROM clock_timestamp()) / 600)::bigint;
    IF (SELECT count(*) FROM push_notification_events
         WHERE event = 'security.ip_banned' AND values->>'notification_bucket' = bucket::text) >= 60 THEN
        RETURN NEW;
    END IF;
    INSERT INTO push_notification_events(event, event_key, occurred_at, values)
    VALUES ('security.ip_banned', 'ban:' || NEW.id::text, NEW.created_at, jsonb_build_object(
        'notification_bucket', bucket::text,
        'summary', CASE NEW.source WHEN 'manual' THEN '管理员手动封禁了 IP' ELSE '已自动永久封禁可疑 IP' END,
        'action', 'security.ip_banned',
        'client_ip', NEW.ip,
        'reason', NEW.reason,
        'target_email', COALESCE(NEW.detail->>'attempted_email', ''),
        'path', COALESCE(NEW.detail->>'path', ''),
        'request_count', COALESCE(NEW.detail->>'count', '1'),
        'actor_email', CASE NEW.source WHEN 'manual' THEN NEW.created_by ELSE '' END,
        'ban_id', NEW.id::text
    )) ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS push_notification_ip_ban ON ip_bans;
CREATE TRIGGER push_notification_ip_ban AFTER INSERT ON ip_bans
    FOR EACH ROW EXECUTE FUNCTION capture_ip_ban_push_event();
