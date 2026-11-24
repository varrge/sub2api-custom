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
	s, db := legacySecurityPushDB(t)
	raw, err := os.ReadFile("../../migrations/273_admin_login_failure_push_scope.sql")
	require.NoError(t, err)
	sqlExec(t, db, string(raw))
	return s, db
}

func legacySecurityPushDB(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	s, db := pushDB(t)
	sqlExec(t, db, `CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT NOT NULL,role TEXT NOT NULL,deleted_at TIMESTAMPTZ);
		INSERT INTO users VALUES(1,'admin@example.test','admin',NULL),(2,'user@example.test','user',NULL),
			(3,'deleted@example.test','admin',NOW()),
			(4,'duplicate@example.test','admin',NULL),(5,'DUPLICATE@example.test','user',NULL)`)
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
		"token":                "extra-secret-must-not-leak",
		"login_target_user_id": 1,
	})
	storedClient := client
	path := "/api/v1/auth/login"
	if action == "auth.login" {
		storedClient = "192.0.2.99" // legacy attribution must not override strict metadata
	}
	switch action {
	case "auth.login.2fa":
		path = "/api/v1/auth/login/2fa"
	case "auth.register":
		path = "/api/v1/auth/register"
	case "security.ip_probe":
		path = "/.env*"
	}
	_, err := db.Exec(`INSERT INTO audit_logs(action,client_ip,status_code,path,method,extra,request_body,credential_masked)
	 VALUES($1,$2,$3,$5,'POST',$4,'body-secret-must-not-leak','masked-secret-must-not-leak')`,
		action, storedClient, status, string(extra), path)
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
		if kind == "security.login_failed" {
			require.Contains(t, body, "admin@example.test")
			require.Contains(t, body, "不代表账号本人操作")
			require.NotContains(t, body, "未确认关联账号")
		} else {
			require.Contains(t, body, "未确认关联账号")
		}
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
			VALUES($1,'192.0.2.99',429,$2,'POST',jsonb_build_object('security_client_ip',$3::text,'login_target_user_id',1))`,
			action, target, fmt.Sprintf("203.0.113.%d", i+1))
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events WHERE event='security.login_failed'`).Scan(&count))
	require.Equal(t, 2, count, "usernameless passkey begin is not an identified administrator login")
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
		if kind == "security.login_failed" {
			delete(e.Values, "actor_email")
			delete(e.Values, "user_id")
			e.Values["target_email"] = "<script>bad</script>"
		}
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

func TestPostgresLoginFailureRequiresActualAdministratorTarget(t *testing.T) {
	s, db := securityPushDB(t)
	enableSecurityPush(t, s)
	for i, tc := range []struct {
		name, path, email string
		id                any
		status            int
		want              bool
	}{
		{"admin password", "/api/v1/auth/login", "", 1, 401, true},
		{"admin captcha", "/api/v1/auth/login", " ADMIN@example.test ", nil, 400, true},
		{"admin rate limited", "/api/v1/auth/login", "admin@example.test", nil, 429, true},
		{"admin 2fa", "/api/v1/auth/login/2fa", "", 1, 401, true},
		{"admin passkey", "/api/v1/auth/passkey/login/finish", "", 1, 400, true},
		{"admin oauth bind", "/api/v1/auth/oauth/oidc/bind-login", "admin@example.test", nil, 400, true},
		{"user password", "/api/v1/auth/login", "", 2, 401, false},
		{"user captcha", "/api/v1/auth/login", "user@example.test", nil, 400, false},
		{"user 2fa", "/api/v1/auth/login/2fa", "", 2, 401, false},
		{"user passkey", "/api/v1/auth/passkey/login/finish", "", 2, 400, false},
		{"user oauth", "/api/v1/auth/oauth/oidc/bind-login", "user@example.test", nil, 400, false},
		{"unknown email", "/api/v1/auth/login", "unknown@example.test", nil, 401, false},
		{"ambiguous normalized email", "/api/v1/auth/login", "duplicate@example.test", nil, 401, false},
		{"deleted admin", "/api/v1/auth/login", "", 3, 401, false},
		{"deleted email", "/api/v1/auth/login", "deleted@example.test", nil, 401, false},
		{"missing target", "/api/v1/auth/login", "", nil, 401, false},
		{"unknown target", "/api/v1/auth/login", "", 999, 401, false},
		{"malformed ID", "/api/v1/auth/login", "", "bad", 401, false},
		{"overflow ID", "/api/v1/auth/login", "", "999999999999999999999", 401, false},
		{"ID takes precedence", "/api/v1/auth/login", "admin@example.test", 2, 401, false},
		{"2fa cannot claim email", "/api/v1/auth/login/2fa", "admin@example.test", nil, 401, false},
		{"passkey cannot claim email", "/api/v1/auth/passkey/login/finish", "admin@example.test", nil, 401, false},
		{"begin has no target", "/api/v1/auth/passkey/login/begin", "", 1, 429, false},
		{"success", "/api/v1/auth/login", "", 1, 200, false},
		{"server error", "/api/v1/auth/login", "", 1, 503, false},
		{"nonlogin action", "/api/v1/auth/register", "", 1, 400, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fmt.Sprintf("203.0.113.%d", i+1)
			extra := map[string]any{"security_client_ip": client, "login_target_email": tc.email, "role": "admin", "user_id": 1}
			if tc.id != nil {
				extra["login_target_user_id"] = tc.id
			}
			raw, err := json.Marshal(extra)
			require.NoError(t, err)
			sqlExec(t, db, `INSERT INTO audit_logs(action,method,path,status_code,actor_user_id,actor_email,actor_role,extra,request_body)
				VALUES('auth.login','POST',$1,$2,1,'claimed-admin@example.test','admin',$3,'{"role":"admin","user_id":1}')`,
				tc.path, tc.status, string(raw))
			var count int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events WHERE values->>'client_ip'=$1`, client).Scan(&count))
			if !tc.want {
				require.Zero(t, count)
				return
			}
			require.Equal(t, 1, count)
			var values map[string]string
			require.NoError(t, db.QueryRow(`SELECT values FROM push_notification_events WHERE values->>'client_ip'=$1`, client).Scan(&raw))
			require.NoError(t, json.Unmarshal(raw, &values))
			require.Equal(t, "1", values["target_user_id"])
			require.Equal(t, "admin@example.test", values["target_email"])
			require.Empty(t, values["actor_email"], "target identity must not become actor identity")
			require.Empty(t, values["user_id"])
		})
	}
}

func TestPostgresRegularLoginFailuresDoNotConsumeAdministratorNotificationLimit(t *testing.T) {
	s, db := securityPushDB(t)
	enableSecurityPush(t, s)
	sqlExec(t, db, `INSERT INTO audit_logs(action,method,path,status_code,extra)
		SELECT 'auth.login','POST','/api/v1/auth/login',401,
			jsonb_build_object('security_client_ip','203.0.113.'||i,'login_target_user_id',2)
		FROM generate_series(1,80) i`)
	require.NoError(t, insertSecurityAudit(db, "auth.login", "203.0.113.1", 401))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_events`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestPostgresAdminLoginHintLookupHasMatchingIndex(t *testing.T) {
	_, db := securityPushDB(t)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	// Tiny fixtures normally favor a sequential scan; ensure an indexed plan is
	// possible for the exact normalized, active-user predicate used by the trigger.
	_, err = tx.Exec(`SET LOCAL enable_seqscan=off`)
	require.NoError(t, err)
	var plan string
	err = tx.QueryRow(`EXPLAIN (FORMAT JSON)
		SELECT MIN(id), MIN(email) FROM users
		WHERE deleted_at IS NULL AND LOWER(TRIM(email))=LOWER(TRIM(' ADMIN@example.test '))
		HAVING count(*)=1 AND bool_and(role='admin')`).Scan(&plan)
	require.NoError(t, err)
	require.Contains(t, plan, "idx_users_active_login_email")
}

func TestPostgresAdminLoginMigrationCancelsOldQueueWithoutErasingAuditOrSentHistory(t *testing.T) {
	s, db := legacySecurityPushDB(t)
	enableSecurityPush(t, s)
	for _, client := range []string{"203.0.113.1", "203.0.113.2", "203.0.113.3"} {
		require.NoError(t, insertSecurityAudit(db, "auth.login", client, 401))
	}
	require.NoError(t, insertSecurityAudit(db, "security.ip_probe", "203.0.113.4", 404))
	require.NoError(t, s.Collect(context.Background()))
	sqlExec(t, db, `UPDATE push_notification_deliveries SET status='sent' WHERE event_key LIKE 'access:'||md5('203.0.113.1')||':%'`)
	sqlExec(t, db, `UPDATE push_notification_deliveries SET status='sending' WHERE event_key LIKE 'access:'||md5('203.0.113.2')||':%'`)
	// Also cover an old event that has not yet been collected.
	require.NoError(t, insertSecurityAudit(db, "auth.login", "203.0.113.5", 401))
	raw, err := os.ReadFile("../../migrations/273_admin_login_failure_push_scope.sql")
	require.NoError(t, err)
	sqlExec(t, db, string(raw))
	require.NoError(t, s.Collect(context.Background()))
	var count int
	for query, want := range map[string]int{
		`SELECT count(*) FROM audit_logs`: 5,
		`SELECT count(*) FROM push_notification_events WHERE event='security.login_failed'`:                                               0,
		`SELECT count(*) FROM push_notification_deliveries WHERE event='security.login_failed'`:                                           3,
		`SELECT count(*) FROM push_notification_deliveries WHERE event='security.login_failed' AND status='sent' AND NOT invalidated`:     1,
		`SELECT count(*) FROM push_notification_deliveries WHERE event='security.login_failed' AND status='cancelled' AND invalidated`:    1,
		`SELECT count(*) FROM push_notification_deliveries WHERE event='security.login_failed' AND status='sending' AND invalidated`:      1,
		`SELECT count(*) FROM push_notification_deliveries WHERE event='security.suspicious_ip' AND status='pending' AND NOT invalidated`: 1,
	} {
		require.NoError(t, db.QueryRow(query).Scan(&count))
		require.Equal(t, want, count, query)
	}
	// The old cancelled delivery for this IP must not suppress the corrected event.
	require.NoError(t, insertSecurityAudit(db, "auth.login", "203.0.113.3", 401))
	require.NoError(t, s.Collect(context.Background()))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM push_notification_deliveries
		WHERE event='security.login_failed' AND status='pending' AND NOT invalidated`).Scan(&count))
	require.Equal(t, 1, count)
}
