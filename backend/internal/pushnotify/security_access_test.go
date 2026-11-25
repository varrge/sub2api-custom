package pushnotify

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func securityPushDB(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	s, db := pushDB(t)
	for _, file := range []string{"180_audit_logs.sql", "269_admin_activity_push_notifications.sql", "272_security_access_push_notifications.sql"} {
		raw, err := os.ReadFile("../../migrations/" + file)
		require.NoError(t, err)
		sqlExec(t, db, string(raw))
	}
	return s, db
}

func enableSecurityPush(t *testing.T, s *Store) Config {
	t.Helper()
	c, err := s.Get(context.Background())
	require.NoError(t, err)
	ch := testChannel()
	ch.Events = []string{"security.login_failed", "security.suspicious_ip"}
	c.Enabled, c.Channels = true, []Channel{ch}
	c, err = s.Save(context.Background(), c)
	require.NoError(t, err)
	return c
}

func insertSecurityAudit(db *sql.DB, action, client string, status int) error {
	extra, _ := json.Marshal(map[string]any{
		"security_client_ip": client, "peer_ip": "10.0.0.1", "ip_source": "trusted_proxy_chain",
		"request_count": 3, "reason": "test-reason",
		"token": "extra-secret-must-not-leak",
	})
	storedClient := client
	if action == "auth.login" {
		storedClient = "192.0.2.99" // legacy attribution must not override strict metadata
	}
	_, err := db.Exec(`INSERT INTO audit_logs(action,client_ip,status_code,path,method,extra,request_body,credential_masked)
	 VALUES($1,$2,$3,'/api/v1/auth/login','POST',$4,'body-secret-must-not-leak','masked-secret-must-not-leak')`,
		action, storedClient, status, string(extra))
	return err
}

func TestPostgresSecurityAccessEventsDeliverMetadataOnlyAndRespectSettings(t *testing.T) {
	s, db := securityPushDB(t)
	ctx := context.Background()
	require.NoError(t, insertSecurityAudit(db, "auth.login", "203.0.113.10", 401))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events`).Scan(&count))
	require.Zero(t, count, "disabled notifications must not enqueue security events")
	enableSecurityPush(t, s)
	for _, action := range []string{"auth.login", "auth.login.2fa"} {
		require.NoError(t, insertSecurityAudit(db, action, "203.0.113.10", 401))
	}
	require.NoError(t, insertSecurityAudit(db, "security.ip_probe", "203.0.113.10", 404))
	require.NoError(t, insertSecurityAudit(db, "auth.login", "203.0.113.11", 200))
	require.NoError(t, insertSecurityAudit(db, "auth.login", "203.0.113.11", 503))
	require.NoError(t, insertSecurityAudit(db, "auth.register", "203.0.113.11", 400))
	sqlExec(t, db, `INSERT INTO audit_logs(action,client_ip,status_code) VALUES('auth.login','spoofed-legacy-ip',401)`)
	require.NoError(t, s.Collect(ctx))
	require.NoError(t, s.Collect(ctx))
	rows, err := db.Query(`SELECT event,body FROM push_notification_deliveries ORDER BY event`)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	kinds := []string{}
	for rows.Next() {
		var kind, body string
		require.NoError(t, rows.Scan(&kind, &body))
		kinds = append(kinds, kind)
		require.Contains(t, body, "203.0.113.10")
		require.Contains(t, body, "10.0.0.1")
		require.Contains(t, body, "未确认关联账号")
		require.NotContains(t, body, "secret-must-not-leak")
		require.NotContains(t, body, "spoofed-legacy")
		require.NotContains(t, body, "192.0.2.99")
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"security.login_failed", "security.suspicious_ip"}, kinds)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM audit_logs`).Scan(&count))
	require.Equal(t, 8, count, "dedupe only notifications, not the audit trail")
}

func TestPostgresSecurityPushConcurrentDedupAndGlobalCap(t *testing.T) {
	s, db := securityPushDB(t)
	enableSecurityPush(t, s)
	var wg sync.WaitGroup
	errs := make(chan error, 80)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- insertSecurityAudit(db, "auth.login", "203.0.113.20", 401)
		}()
	}
	wg.Wait()
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events`).Scan(&count))
	require.Equal(t, 1, count)
	for i := 0; i < 80; i++ {
		require.NoError(t, insertSecurityAudit(db, "security.ip_probe", fmt.Sprintf("198.51.100.%d", i+1), 404))
	}
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events`).Scan(&count))
	require.Equal(t, 60, count)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM audit_logs`).Scan(&count))
	require.Equal(t, 110, count)
}

func TestPostgresSecurityPushCapsDelayedAuditInsertsWithoutReplayingHistory(t *testing.T) {
	s, db := securityPushDB(t)
	enableSecurityPush(t, s)
	// Async audit batches can be written after their event-time bucket ended.
	// All 80 distinct IPs must still share the current queue-time cap.
	sqlExec(t, db, `INSERT INTO audit_logs(action,client_ip,status_code,created_at)
		SELECT 'security.ip_probe','198.51.100.'||i,404,NOW()-INTERVAL '1 hour'
		FROM generate_series(1,80) i`)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events`).Scan(&count))
	require.Equal(t, 60, count)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM audit_logs`).Scan(&count))
	require.Equal(t, 80, count, "notification limits must not discard audit records")
	var buckets int
	require.NoError(t, db.QueryRow(`SELECT count(DISTINCT values->>'notification_bucket') FROM push_notification_events`).Scan(&buckets))
	require.Equal(t, 1, buckets)
	require.NoError(t, s.Collect(context.Background()))
	history, err := s.History(context.Background())
	require.NoError(t, err)
	require.Empty(t, history, "old activity written late must not be replayed after enabling")
}

func TestPostgresSecurityPushIncludesPasskeyAndOAuthLoginFailures(t *testing.T) {
	s, db := securityPushDB(t)
	enableSecurityPush(t, s)
	for i, target := range []string{
		"/api/v1/auth/passkey/login/begin",
		"/api/v1/auth/passkey/login/finish",
		"/api/v1/auth/oauth/github/bind-login",
	} {
		action := "auth.passkey.login.begin.create"
		if strings.HasSuffix(target, "/finish") {
			action = "auth.login"
		}
		if strings.Contains(target, "/oauth/") {
			action = "auth.oauth.github.bind-login.create"
		}
		sqlExec(t, db, `INSERT INTO audit_logs(action,client_ip,status_code,path,method,extra)
			VALUES($1,'192.0.2.99',429,$2,'POST',jsonb_build_object('security_client_ip',$3::text))`,
			action, target, fmt.Sprintf("203.0.113.%d", i+1))
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events WHERE event='security.login_failed'`).Scan(&count))
	require.Equal(t, 3, count)
}

func TestPostgresSecurityPushNoReplayAfterEnablingAndRollback(t *testing.T) {
	s, db := securityPushDB(t)
	c := enableSecurityPush(t, s)
	require.NoError(t, insertSecurityAudit(db, "auth.login", "203.0.113.30", 429))
	c.Enabled = false
	var err error
	c, err = s.Save(context.Background(), c)
	require.NoError(t, err)
	c.Enabled = true
	_, err = s.Save(context.Background(), c)
	require.NoError(t, err)
	require.NoError(t, s.Collect(context.Background()))
	history, err := s.History(context.Background())
	require.NoError(t, err)
	require.Empty(t, history, "reenabling must not replay previous security activity")
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO audit_logs(action,client_ip,status_code) VALUES('security.ip_probe','198.51.100.99',404)`)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events WHERE values->>'client_ip'='198.51.100.99'`).Scan(&count))
	require.Zero(t, count)
}

func TestSecurityPushPreviewsAndMarkdownEscaping(t *testing.T) {
	for _, kind := range []string{"security.login_failed", "security.suspicious_ip"} {
		e := Sample(kind)
		require.Equal(t, kind, e.Kind)
		require.NotEmpty(t, e.Values["peer_ip"])
		require.NotEmpty(t, e.Values["client_ip"])
		e.Values["actor_email"] = "<script>bad</script>"
		e.Values["user_id"] = "7"
		decorateEvent(&e)
		c := testChannel()
		c.BodyTemplate = "{{details}}\n{{peer_ip}}"
		m, err := Render(c, e, "test")
		require.NoError(t, err)
		require.Contains(t, m.Body, "&lt;script&gt;")
		require.NotContains(t, m.Body, "<script>")
		require.Contains(t, m.Body, "10 分钟")
		require.False(t, strings.Contains(m.Body, "secret"))
		require.WithinDuration(t, time.Now(), e.At, time.Second)
	}
}
