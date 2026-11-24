// Package ipban keeps a permanent, database-backed list of banned client
// addresses and decides when suspicious traffic earns a ban.
//
// Bans never expire on their own; an administrator lifts them from the admin
// panel (or, in an emergency, by setting revoked_at in SQL). Every instance
// keeps the active list in memory and reloads it from PostgreSQL every few
// seconds, so a ban created on one instance reaches the others quickly and a
// database outage never blocks legitimate traffic.
package ipban

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
)

// Reasons recorded on automatic bans.
const (
	ReasonHoneypotEmail      = "honeypot_email"
	ReasonSensitiveProbe     = "sensitive_path_probe"
	ReasonAdminLoginFailures = "admin_login_failures"
	ReasonAdminUnauthorized  = "admin_api_unauthorized"
	ReasonManual             = "manual"
)

var (
	ErrInvalid  = errors.New("封禁参数无效")
	ErrNotFound = errors.New("封禁记录不存在或已解除")
	ErrExists   = errors.New("该 IP 已在封禁名单中")
)

// Settings are stored as one JSON document in the settings table.
type Settings struct {
	Enabled bool `json:"enabled"`
	// HoneypotEmails are account names nobody legitimately uses; any login,
	// registration, verification or recovery request naming one is banned.
	HoneypotEmails []string `json:"honeypot_emails"`
	// BanSensitiveProbes bans /.env, /.git, wp-login.php style probes.
	BanSensitiveProbes bool `json:"ban_sensitive_probes"`
	// AdminLoginFailures failed logins against a real administrator account
	// within AdminLoginWindowMinutes ban the source. 0 disables.
	AdminLoginFailures      int `json:"admin_login_failures"`
	AdminLoginWindowMinutes int `json:"admin_login_window_minutes"`
	// AdminUnauthorized unauthenticated /api/v1/admin/* requests within
	// AdminUnauthorizedWindowMinutes ban the source. 0 disables.
	AdminUnauthorized              int `json:"admin_unauthorized"`
	AdminUnauthorizedWindowMinutes int `json:"admin_unauthorized_window_minutes"`
	// Allowlist entries (IP or CIDR) are never banned automatically or manually.
	Allowlist []string `json:"allowlist"`
}

// DefaultHoneypotEmails come from historical brute-force traffic.
var DefaultHoneypotEmails = []string{
	"admin@sub2api.local", "root@sub2api.local", "admin@sub2api.org", "admin@sub2api.com",
	"admin@example.com", "admin@test.com", "admin@admin.com",
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:                        true,
		HoneypotEmails:                 slices.Clone(DefaultHoneypotEmails),
		BanSensitiveProbes:             true,
		AdminLoginFailures:             5,
		AdminLoginWindowMinutes:        15,
		AdminUnauthorized:              10,
		AdminUnauthorizedWindowMinutes: 10,
		Allowlist:                      []string{},
	}
}

const settingsKey = "ip_ban_settings"

// Ban is one row of ip_bans.
type Ban struct {
	ID         int64          `json:"id"`
	IP         string         `json:"ip"`
	Reason     string         `json:"reason"`
	Source     string         `json:"source"`
	Detail     map[string]any `json:"detail"`
	Note       string         `json:"note"`
	CreatedBy  string         `json:"created_by"`
	CreatedAt  time.Time      `json:"created_at"`
	Hits       int64          `json:"hits"`
	LastHitAt  *time.Time     `json:"last_hit_at"`
	RevokedAt  *time.Time     `json:"revoked_at"`
	RevokedBy  string         `json:"revoked_by"`
	RevokeNote string         `json:"revoke_note"`
}

// Event describes the request that may lead to a ban.
type Event struct {
	IP     string
	Reason string
	Detail map[string]any
}

type snapshot struct {
	exact    map[netip.Addr]int64
	prefixes []prefixBan
}

type prefixBan struct {
	prefix netip.Prefix
	id     int64
}

// Service is safe for concurrent use.
type Service struct {
	db *sql.DB

	bans      atomic.Pointer[snapshot]
	settings  atomic.Pointer[compiledSettings]
	protected []netip.Prefix

	mu       sync.Mutex
	counters map[string]*window

	hitMu sync.Mutex
	hits  map[int64]int64

	// adminIPs are addresses that recently completed authenticated
	// administrator requests; automatic rules never ban them.
	adminMu       sync.Mutex
	adminIPs      map[netip.Addr]time.Time
	adminLoadedAt time.Time

	onBan func(Ban)

	cancel context.CancelFunc
	done   chan struct{}
	stop   sync.Once
}

type compiledSettings struct {
	Settings
	honeypots map[string]struct{}
	allow     []netip.Prefix
}

type window struct {
	start time.Time
	count int
}

// alwaysProtected covers loopback, private, link-local, CGNAT and documentation
// space. Traffic from these is a proxy or internal component, never a client
// that can be banned.
var alwaysProtected = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.168.0.0/16", "::1/128", "::/128", "fc00::/7", "fe80::/10",
)

// New builds a service. trusted lists proxy addresses/CIDRs (for example
// server.trusted_proxies) that must never be banned.
func New(db *sql.DB, trusted []string) *Service {
	s := &Service{db: db, counters: map[string]*window{}, hits: map[int64]int64{}, adminIPs: map[netip.Addr]time.Time{}}
	s.protected = append(slices.Clone(alwaysProtected), parsePrefixes(trusted)...)
	s.bans.Store(&snapshot{exact: map[netip.Addr]int64{}})
	s.settings.Store(compile(DefaultSettings()))
	return s
}

// Start loads state and refreshes it periodically until Stop.
func (s *Service) Start() {
	if s == nil || s.db == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	s.refresh(ctx)
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.flushHits(context.Background())
				return
			case <-ticker.C:
				s.refresh(ctx)
			}
		}
	}()
}

func (s *Service) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.stop.Do(func() { s.cancel(); <-s.done })
}

// OnBan registers a callback for newly created bans (used for logging).
func (s *Service) OnBan(fn func(Ban)) { s.onBan = fn }

func (s *Service) refresh(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.loadSettings(runCtx); err != nil && runCtx.Err() == nil {
		slog.Warn("ip ban settings reload failed", "error", err)
	}
	if err := s.loadBans(runCtx); err != nil && runCtx.Err() == nil {
		slog.Warn("ip ban list reload failed", "error", err)
	}
	s.flushHits(runCtx)
	now := time.Now()
	s.pruneCounters(now)
	if now.Sub(s.adminLoadedAt) >= 10*time.Minute {
		if err := s.loadAdminIPs(runCtx, now); err != nil && runCtx.Err() == nil {
			slog.Warn("ip ban admin source reload failed", "error", err)
		}
	}
}

const adminIPTTL = 30 * 24 * time.Hour

// loadAdminIPs remembers addresses used by administrators in the last 30 days
// so an owner working from a new network is not banned by a typo.
func (s *Service) loadAdminIPs(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT a.client_ip, MAX(a.created_at) FROM audit_logs a
		WHERE a.actor_user_id IN (SELECT id FROM users WHERE role='admin' AND deleted_at IS NULL)
		  AND a.status_code < 400 AND a.created_at > NOW() - INTERVAL '30 days'
		GROUP BY a.client_ip`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	loaded := map[netip.Addr]time.Time{}
	for rows.Next() {
		var raw string
		var at time.Time
		if err := rows.Scan(&raw, &at); err != nil {
			return err
		}
		if a, ok := parseAddr(raw); ok && !s.isInternal(a) {
			loaded[a] = at
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.adminMu.Lock()
	defer s.adminMu.Unlock()
	for a, at := range s.adminIPs {
		if now.Sub(at) < adminIPTTL && at.After(loaded[a]) {
			loaded[a] = at
		}
	}
	s.adminIPs = loaded
	s.adminLoadedAt = now
	return nil
}

// MarkAdmin records an address that just completed an administrator request.
func (s *Service) MarkAdmin(ip string) {
	if s == nil {
		return
	}
	a, ok := parseAddr(ip)
	if !ok || s.isInternal(a) {
		return
	}
	s.adminMu.Lock()
	s.adminIPs[a] = time.Now()
	s.adminMu.Unlock()
}

func (s *Service) isRecentAdmin(a netip.Addr) bool {
	s.adminMu.Lock()
	defer s.adminMu.Unlock()
	at, ok := s.adminIPs[a]
	return ok && time.Since(at) < adminIPTTL
}

// IsAdminAccount reports whether the login target is an existing
// administrator, by server-resolved user ID or by a unique email match.
func (s *Service) IsAdminAccount(ctx context.Context, userID int64, email string) (bool, error) {
	var admin bool
	var err error
	if userID > 0 {
		err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND deleted_at IS NULL)`, userID).Scan(&admin)
	} else if email = normalizeEmail(email); email != "" {
		err = s.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT bool_and(role='admin') AND count(*)=1 FROM users
			WHERE deleted_at IS NULL AND LOWER(TRIM(email))=$1), FALSE)`, email).Scan(&admin)
	}
	return admin, err
}

func (s *Service) loadSettings(ctx context.Context) error {
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return err
	}
	s.settings.Store(compile(cfg))
	return nil
}

func (s *Service) loadBans(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, ip FROM ip_bans WHERE revoked_at IS NULL`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	next := &snapshot{exact: map[netip.Addr]int64{}}
	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		next.add(raw, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.bans.Store(next)
	return nil
}

func (sn *snapshot) add(raw string, id int64) {
	if p, err := netip.ParsePrefix(raw); err == nil {
		p = p.Masked()
		if p.Bits() == p.Addr().BitLen() {
			sn.exact[p.Addr()] = id
		} else {
			sn.prefixes = append(sn.prefixes, prefixBan{prefix: p, id: id})
		}
		return
	}
	if a, err := netip.ParseAddr(raw); err == nil {
		sn.exact[a.Unmap()] = id
	}
}

func (sn *snapshot) match(a netip.Addr) (int64, bool) {
	if id, ok := sn.exact[a]; ok {
		return id, true
	}
	for _, p := range sn.prefixes {
		if p.prefix.Contains(a) {
			return p.id, true
		}
	}
	return 0, false
}

// Check reports whether ip is banned and counts the hit.
func (s *Service) Check(ip string) bool {
	if s == nil {
		return false
	}
	a, ok := parseAddr(ip)
	if !ok || s.isProtected(a) {
		return false
	}
	id, banned := s.bans.Load().match(a)
	if banned {
		s.hitMu.Lock()
		s.hits[id]++
		s.hitMu.Unlock()
	}
	return banned
}

func (s *Service) flushHits(ctx context.Context) {
	s.hitMu.Lock()
	pending := s.hits
	s.hits = map[int64]int64{}
	s.hitMu.Unlock()
	for id, n := range pending {
		if _, err := s.db.ExecContext(ctx, `UPDATE ip_bans SET hits=hits+$2, last_hit_at=NOW() WHERE id=$1`, id, n); err != nil {
			return
		}
	}
}

// Protected reports whether ip can never be banned (internal, trusted proxy
// or administrator allowlist).
func (s *Service) Protected(ip string) bool {
	a, ok := parseAddr(ip)
	return !ok || s.isProtected(a)
}

func (s *Service) isInternal(a netip.Addr) bool {
	for _, p := range s.protected {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func (s *Service) isProtected(a netip.Addr) bool {
	if s.isInternal(a) {
		return true
	}
	for _, p := range s.settings.Load().allow {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// IsHoneypotEmail reports whether email is a configured decoy account.
func (s *Service) IsHoneypotEmail(email string) bool {
	if s == nil {
		return false
	}
	cfg := s.settings.Load()
	if !cfg.Enabled {
		return false
	}
	_, ok := cfg.honeypots[normalizeEmail(email)]
	return ok
}

// Current returns the active settings snapshot.
func (s *Service) Current() Settings { return s.settings.Load().Settings }

// Observe counts a suspicious request and bans the source once the configured
// threshold is reached. It returns true when a ban was created.
func (s *Service) Observe(ctx context.Context, ev Event) bool {
	if s == nil || s.db == nil {
		return false
	}
	cfg := s.settings.Load()
	if !cfg.Enabled {
		return false
	}
	threshold, minutes := 1, 0
	switch ev.Reason {
	case ReasonHoneypotEmail:
	case ReasonSensitiveProbe:
		if !cfg.BanSensitiveProbes {
			return false
		}
	case ReasonAdminLoginFailures:
		threshold, minutes = cfg.AdminLoginFailures, cfg.AdminLoginWindowMinutes
	case ReasonAdminUnauthorized:
		threshold, minutes = cfg.AdminUnauthorized, cfg.AdminUnauthorizedWindowMinutes
	default:
		return false
	}
	if threshold <= 0 {
		return false
	}
	a, ok := parseAddr(ev.IP)
	if !ok || s.isProtected(a) || s.isRecentAdmin(a) {
		return false
	}
	key := banKey(a)
	if threshold > 1 {
		count, reached := s.count(ev.Reason+"|"+key, time.Duration(minutes)*time.Minute, threshold, time.Now())
		if !reached {
			return false
		}
		if ev.Detail == nil {
			ev.Detail = map[string]any{}
		}
		ev.Detail["count"] = count
		ev.Detail["window_minutes"] = minutes
	}
	if _, banned := s.bans.Load().match(a); banned {
		return false
	}
	ban, err := s.create(ctx, key, ev.Reason, "auto", ev.Detail, "", "system")
	if err != nil {
		if !errors.Is(err, ErrExists) {
			slog.Warn("ip ban insert failed", "error", err)
		}
		return false
	}
	return ban != nil
}

func (s *Service) count(key string, span time.Duration, threshold int, now time.Time) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.counters[key]
	if w == nil || now.Sub(w.start) >= span {
		if w == nil && len(s.counters) >= 20000 {
			return 0, false
		}
		w = &window{start: now}
		s.counters[key] = w
	}
	w.count++
	if w.count >= threshold {
		delete(s.counters, key)
		return w.count, true
	}
	return w.count, false
}

func (s *Service) pruneCounters(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, w := range s.counters {
		if now.Sub(w.start) >= time.Hour {
			delete(s.counters, k)
		}
	}
}

func (s *Service) create(ctx context.Context, ip, reason, source string, detail map[string]any, note, by string) (*Ban, error) {
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	insCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	b := Ban{IP: ip, Reason: reason, Source: source, Detail: detail, Note: note, CreatedBy: by}
	err = s.db.QueryRowContext(insCtx, `INSERT INTO ip_bans(ip,reason,source,detail,note,created_by) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT (ip) WHERE revoked_at IS NULL DO NOTHING RETURNING id, created_at`,
		ip, reason, source, string(raw), note, by).Scan(&b.ID, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrExists
	}
	if err != nil {
		return nil, err
	}
	s.addLocal(ip, b.ID)
	slog.Warn("ip banned", "ip", ip, "reason", reason, "source", source, "ban_id", b.ID)
	if s.onBan != nil {
		s.onBan(b)
	}
	return &b, nil
}

func (s *Service) addLocal(ip string, id int64) {
	cur := s.bans.Load()
	next := &snapshot{exact: make(map[netip.Addr]int64, len(cur.exact)+1), prefixes: slices.Clone(cur.prefixes)}
	for k, v := range cur.exact {
		next.exact[k] = v
	}
	next.add(ip, id)
	s.bans.Store(next)
}

// ManualBan bans an IP or CIDR on an administrator's request.
func (s *Service) ManualBan(ctx context.Context, target, note, by string) (*Ban, error) {
	key, err := normalizeTarget(target)
	if err != nil {
		return nil, err
	}
	p, _ := netip.ParsePrefix(key)
	if !p.IsValid() {
		a, _ := netip.ParseAddr(key)
		p = netip.PrefixFrom(a, a.BitLen())
	}
	if s.overlapsProtected(p) {
		return nil, fmt.Errorf("%w：该地址属于内网、受信代理或白名单，不能封禁", ErrInvalid)
	}
	if len([]rune(note)) > 500 {
		return nil, fmt.Errorf("%w：备注过长", ErrInvalid)
	}
	return s.create(ctx, key, ReasonManual, "manual", map[string]any{}, strings.TrimSpace(note), by)
}

func (s *Service) overlapsProtected(p netip.Prefix) bool {
	all := append(slices.Clone(s.protected), s.settings.Load().allow...)
	for _, q := range all {
		if q.Overlaps(p) {
			return true
		}
	}
	return false
}

// Revoke lifts an active ban.
func (s *Service) Revoke(ctx context.Context, id int64, note, by string) error {
	if len([]rune(note)) > 500 {
		return fmt.Errorf("%w：备注过长", ErrInvalid)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE ip_bans SET revoked_at=NOW(), revoked_by=$2, revoke_note=$3 WHERE id=$1 AND revoked_at IS NULL`, id, by, strings.TrimSpace(note))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	slog.Warn("ip ban revoked", "ban_id", id, "by", by)
	return s.loadBans(ctx)
}

// ListQuery filters List.
type ListQuery struct {
	Status   string // active (default), revoked, all
	Reason   string
	Search   string
	Page     int
	PageSize int
}

// List returns bans newest first with the total count.
func (s *Service) List(ctx context.Context, q ListQuery) ([]Ban, int64, error) {
	where := []string{"TRUE"}
	args := []any{}
	switch q.Status {
	case "revoked":
		where = append(where, "revoked_at IS NOT NULL")
	case "all":
	default:
		where = append(where, "revoked_at IS NULL")
	}
	if q.Reason != "" {
		args = append(args, q.Reason)
		where = append(where, fmt.Sprintf("reason=$%d", len(args)))
	}
	if term := strings.TrimSpace(q.Search); term != "" {
		args = append(args, "%"+escapeLike(term)+"%")
		where = append(where, fmt.Sprintf("(ip ILIKE $%[1]d OR note ILIKE $%[1]d OR detail::text ILIKE $%[1]d)", len(args)))
	}
	cond := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_bans WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if q.PageSize <= 0 || q.PageSize > 200 {
		q.PageSize = 20
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT id,ip,reason,source,detail,note,created_by,created_at,hits,last_hit_at,revoked_at,revoked_by,revoke_note
		FROM ip_bans WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, cond, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []Ban{}
	for rows.Next() {
		var b Ban
		var raw []byte
		if err := rows.Scan(&b.ID, &b.IP, &b.Reason, &b.Source, &raw, &b.Note, &b.CreatedBy, &b.CreatedAt, &b.Hits, &b.LastHitAt, &b.RevokedAt, &b.RevokedBy, &b.RevokeNote); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(raw, &b.Detail)
		if b.Detail == nil {
			b.Detail = map[string]any{}
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

// Stats summarises active bans by reason.
func (s *Service) Stats(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT reason, COUNT(*) FROM ip_bans WHERE revoked_at IS NULL GROUP BY reason`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int64{}
	for rows.Next() {
		var r string
		var n int64
		if err := rows.Scan(&r, &n); err != nil {
			return nil, err
		}
		out[r] = n
	}
	return out, rows.Err()
}

// GetSettings reads stored settings, falling back to defaults.
func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	cfg := DefaultSettings()
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, settingsKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultSettings(), nil
	}
	return sanitize(cfg), nil
}

// SaveSettings validates and stores settings.
func (s *Service) SaveSettings(ctx context.Context, cfg Settings) (Settings, error) {
	cfg = sanitize(cfg)
	if err := validate(cfg); err != nil {
		return Settings{}, err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return Settings{}, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW())
		ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value, updated_at=EXCLUDED.updated_at`, settingsKey, string(raw)); err != nil {
		return Settings{}, err
	}
	s.settings.Store(compile(cfg))
	return cfg, nil
}

// HoneypotConflicts returns configured decoy emails that belong to existing
// accounts. Such entries are ignored (never banned on) until removed.
func (s *Service) HoneypotConflicts(ctx context.Context, emails []string) ([]string, error) {
	if len(emails) == 0 {
		return []string{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT LOWER(TRIM(email)) FROM users WHERE deleted_at IS NULL AND LOWER(TRIM(email)) = ANY($1)`, pq.Array(emails))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AccountExists reports whether a non-deleted user owns email.
func (s *Service) AccountExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE deleted_at IS NULL AND LOWER(TRIM(email))=$1)`, normalizeEmail(email)).Scan(&exists)
	return exists, err
}

func sanitize(cfg Settings) Settings {
	seen := map[string]bool{}
	emails := []string{}
	for _, e := range cfg.HoneypotEmails {
		e = normalizeEmail(e)
		if e != "" && !seen[e] {
			seen[e] = true
			emails = append(emails, e)
		}
	}
	cfg.HoneypotEmails = emails
	allow := []string{}
	for _, a := range cfg.Allowlist {
		if a = strings.TrimSpace(a); a != "" && !slices.Contains(allow, a) {
			allow = append(allow, a)
		}
	}
	cfg.Allowlist = allow
	return cfg
}

func validate(cfg Settings) error {
	if len(cfg.HoneypotEmails) > 200 {
		return fmt.Errorf("%w：诱饵邮箱最多 200 个", ErrInvalid)
	}
	for _, e := range cfg.HoneypotEmails {
		if len(e) > 255 || !strings.Contains(e, "@") {
			return fmt.Errorf("%w：诱饵邮箱格式无效：%s", ErrInvalid, e)
		}
	}
	if len(cfg.Allowlist) > 200 {
		return fmt.Errorf("%w：白名单最多 200 条", ErrInvalid)
	}
	for _, a := range cfg.Allowlist {
		if len(parsePrefixes([]string{a})) == 0 {
			return fmt.Errorf("%w：白名单地址无效：%s", ErrInvalid, a)
		}
	}
	for _, n := range []int{cfg.AdminLoginFailures, cfg.AdminUnauthorized} {
		if n < 0 || n > 1000 {
			return fmt.Errorf("%w：次数阈值须在 0–1000 之间", ErrInvalid)
		}
	}
	for _, m := range []int{cfg.AdminLoginWindowMinutes, cfg.AdminUnauthorizedWindowMinutes} {
		if m < 1 || m > 1440 {
			return fmt.Errorf("%w：统计窗口须在 1–1440 分钟之间", ErrInvalid)
		}
	}
	return nil
}

func compile(cfg Settings) *compiledSettings {
	c := &compiledSettings{Settings: cfg, honeypots: map[string]struct{}{}}
	for _, e := range cfg.HoneypotEmails {
		c.honeypots[normalizeEmail(e)] = struct{}{}
	}
	c.allow = parsePrefixes(cfg.Allowlist)
	return c
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// parseAddr accepts a bare address (IPv4-mapped IPv6 is unmapped).
func parseAddr(raw string) (netip.Addr, bool) {
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	a, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap().WithZone(""), true
}

// banKey is the stored form of an automatic ban: the address for IPv4 and the
// /64 network for IPv6 (clients typically control a whole /64).
func banKey(a netip.Addr) string {
	if a.Is6() {
		p, _ := a.Prefix(64)
		return p.String()
	}
	return a.String()
}

func normalizeTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w：请填写 IP 或网段", ErrInvalid)
	}
	if strings.Contains(raw, "/") {
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return "", fmt.Errorf("%w：网段格式无效", ErrInvalid)
		}
		p = netip.PrefixFrom(p.Addr().Unmap(), bitsUnmapped(p)).Masked()
		if (p.Addr().Is4() && p.Bits() < 16) || (p.Addr().Is6() && p.Bits() < 32) {
			return "", fmt.Errorf("%w：网段过大（IPv4 至少 /16，IPv6 至少 /32）", ErrInvalid)
		}
		if p.Bits() == p.Addr().BitLen() {
			return p.Addr().String(), nil
		}
		return p.String(), nil
	}
	a, ok := parseAddr(raw)
	if !ok {
		return "", fmt.Errorf("%w：IP 格式无效", ErrInvalid)
	}
	return a.String(), nil
}

func bitsUnmapped(p netip.Prefix) int {
	if p.Addr().Is4In6() {
		return max(p.Bits()-96, 0)
	}
	return p.Bits()
}

func parsePrefixes(values []string) []netip.Prefix {
	out := []netip.Prefix{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if p, err := netip.ParsePrefix(v); err == nil {
			out = append(out, netip.PrefixFrom(p.Addr().Unmap(), bitsUnmapped(p)).Masked())
			continue
		}
		if a, ok := parseAddr(v); ok {
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
		}
	}
	return out
}

func mustPrefixes(values ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		out = append(out, netip.MustParsePrefix(v))
	}
	return out
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
