package invitation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// queryOne closes rows before the caller sends another query on a transaction.
func queryOne(ctx context.Context, c *dbent.Client, q string, args []any, dest ...any) error {
	rows, err := c.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

func (s *Store) withTx(ctx context.Context, f func(*dbent.Client) error) error {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := f(tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

func activeUserType(ctx context.Context, c *dbent.Client, userID int64, lock bool) (string, error) {
	q := `SELECT account_type FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active'`
	if lock {
		// No key columns change, so do not block foreign-key checks that reference this user.
		q += postgresLock(c, " FOR NO KEY UPDATE")
	}
	var kind string
	err := queryOne(ctx, c, q, []any{userID}, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if kind != "personal" && kind != "enterprise" {
		return "", ErrInvalid
	}
	return kind, nil
}

func personalUser(ctx context.Context, c *dbent.Client, userID int64, lock bool) error {
	kind, err := activeUserType(ctx, c, userID, lock)
	if err != nil {
		return err
	}
	if kind != "personal" {
		return ErrPersonalOnly
	}
	return nil
}

func adminUser(ctx context.Context, c *dbent.Client, userID int64) error {
	var allowed bool
	err := queryOne(ctx, c, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL)`, []any{userID}, &allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrInvalid
	}
	return nil
}

func readSettings(ctx context.Context, c *dbent.Client, lock string) (*Settings, error) {
	var data []byte
	if err := queryOne(ctx, c, `SELECT settings FROM native_invitation_settings WHERE id=1`+lock, nil, &data); err != nil {
		return nil, err
	}
	var cfg Settings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *Store) Settings(ctx context.Context) (*Settings, error) {
	return readSettings(ctx, s.client, "")
}

func validateSettings(cfg *Settings) error {
	if cfg.ValidityDays < 1 || cfg.ValidityDays > 3650 || len(cfg.Levels) == 0 || len(cfg.Levels) > 100 ||
		cfg.AutoApproveHours < 0 || cfg.AutoApproveHours > maxAutoApproveHours {
		return ErrInvalid
	}
	ids := make(map[string]bool, len(cfg.Levels))
	for i := range cfg.Levels {
		l := &cfg.Levels[i]
		l.ID, l.Name = strings.TrimSpace(l.ID), strings.TrimSpace(l.Name)
		if l.ID == "" || len(l.ID) > 64 || l.Name == "" || utf8.RuneCountInString(l.Name) > 100 ||
			ids[l.ID] || l.LifetimeInviteLimit < 0 || l.UnusedInviteLimit < 0 ||
			l.LifetimeInviteLimit > math.MaxInt32 || l.UnusedInviteLimit > math.MaxInt32 ||
			!finite(l.SpendThreshold) || l.SpendThreshold < 0 || l.SpendThreshold >= 1e12 ||
			!finite(l.RebateRate) || l.RebateRate < 0 || l.RebateRate > 100 ||
			decimal.NewFromFloat(l.RebateRate).Exponent() < -6 {
			return ErrInvalid
		}
		ids[l.ID] = true
	}
	sort.Slice(cfg.Levels, func(i, j int) bool { return cfg.Levels[i].SpendThreshold < cfg.Levels[j].SpendThreshold })
	if cfg.Levels[0].SpendThreshold != 0 {
		return ErrInvalid
	}
	for i := 1; i < len(cfg.Levels); i++ {
		if cfg.Levels[i].SpendThreshold == cfg.Levels[i-1].SpendThreshold {
			return ErrInvalid
		}
	}
	return nil
}

// One year; a longer wait is indistinguishable from manual review.
const maxAutoApproveHours = 8760

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

func (s *Store) SaveSettings(ctx context.Context, cfg Settings, actorID int64) (*Settings, error) {
	cfg.Levels = append([]Level(nil), cfg.Levels...)
	if err := validateSettings(&cfg); err != nil {
		return nil, err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	err = s.withTx(ctx, func(c *dbent.Client) error {
		if err := adminUser(ctx, c, actorID); err != nil {
			return err
		}
		before, err := readSettings(ctx, c, " FOR UPDATE")
		if err != nil {
			return err
		}
		rows, err := c.QueryContext(ctx, `SELECT DISTINCT level_id FROM native_invitation_overrides`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			if findLevel(cfg.Levels, id) == nil {
				return ErrLevelInUse
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if _, err := c.ExecContext(ctx, `UPDATE native_invitation_settings SET settings=$1::jsonb, updated_at=NOW() WHERE id=1`, string(data)); err != nil {
			return err
		}
		return audit(ctx, c, actorID, "settings", nil, nil, "", decimal.Zero, map[string]any{"before": before, "after": cfg})
	})
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func findLevel(levels []Level, id string) *Level {
	for i := range levels {
		if levels[i].ID == id {
			return &levels[i]
		}
	}
	return nil
}

func userLevel(ctx context.Context, c *dbent.Client, cfg *Settings, userID int64) (*Overview, error) {
	o := &Overview{Enabled: cfg.Enabled, ValidityDays: cfg.ValidityDays, AutoApproveHours: cfg.AutoApproveHours, Levels: cfg.Levels}
	var total decimal.Decimal
	err := queryOne(ctx, c, `SELECT COALESCE(SUM(value),0) FROM redeem_codes WHERE used_by=$1 AND status='used' AND type='balance' AND value>0`, []any{userID}, &total)
	if err != nil {
		return nil, err
	}
	o.RechargeTotal, _ = total.Float64()
	err = queryOne(ctx, c, `SELECT level_id FROM native_invitation_overrides WHERE user_id=$1`, []any{userID}, &o.OverrideLevelID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	o.Level = cfg.Levels[0]
	for _, l := range cfg.Levels {
		if total.GreaterThanOrEqual(decimal.NewFromFloat(l.SpendThreshold)) {
			o.Level = l
			continue
		}
		next := l
		o.NextLevel = &next
		o.NextLevelRemaining, _ = decimal.NewFromFloat(l.SpendThreshold).Sub(total).Float64()
		break
	}
	if override := findLevel(cfg.Levels, o.OverrideLevelID); override != nil {
		o.Level = *override
	}
	return o, nil
}

func inviteCounts(ctx context.Context, c *dbent.Client, userID int64) (int64, int64, error) {
	var lifetime, unused int64
	err := queryOne(ctx, c, `SELECT
		COUNT(*) FILTER (WHERE b.invitee_id IS NOT NULL OR rc.status='used' OR (rc.status='unused' AND i.expires_at>NOW())),
		COUNT(*) FILTER (WHERE b.invitee_id IS NULL AND rc.status='unused' AND i.expires_at>NOW())
		FROM native_invitations i JOIN redeem_codes rc ON rc.id=i.redeem_id
		LEFT JOIN native_invitation_bindings b ON b.invite_id=i.id WHERE i.inviter_id=$1`,
		[]any{userID}, &lifetime, &unused)
	return lifetime, unused, err
}

func (s *Store) Overview(ctx context.Context, userID int64) (*Overview, error) {
	kind, err := activeUserType(ctx, s.client, userID, false)
	if err != nil {
		return nil, err
	}
	cfg, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	o, err := userLevel(ctx, s.client, cfg, userID)
	if err != nil {
		return nil, err
	}
	o.LifetimeUsed, o.UnusedCount, err = inviteCounts(ctx, s.client, userID)
	if err != nil {
		return nil, err
	}
	o.RebateEnabled = kind == "personal"
	if !o.RebateEnabled {
		o.AutoApproveHours = 0
		o.Level.RebateRate = 0
		for i := range o.Levels {
			o.Levels[i].RebateRate = 0
		}
		if o.NextLevel != nil {
			o.NextLevel.RebateRate = 0
		}
		return o, nil
	}
	err = queryOne(ctx, s.client, `SELECT
		COALESCE(SUM(rebate_amount) FILTER (WHERE status='pending'),0),
		COALESCE(SUM(rebate_amount) FILTER (WHERE status='approved'),0),
		COALESCE(SUM(rebate_amount) FILTER (WHERE status IN ('rejected','revoked')),0)
		FROM native_invitation_rebates WHERE inviter_id=$1`, []any{userID},
		&o.Summary.Pending, &o.Summary.Paid, &o.Summary.RejectedOrRevoked)
	return o, err
}

func (s *Store) Apply(ctx context.Context, userID int64, requestID string) (*Invite, error) {
	request, err := uuid.Parse(requestID)
	if err != nil || request == uuid.Nil {
		return nil, ErrInvalid
	}
	var out *Invite
	err = s.withTx(ctx, func(c *dbent.Client) error {
		kind, err := activeUserType(ctx, c, userID, true)
		if err != nil {
			return err
		}
		var id int64
		err = queryOne(ctx, c, `SELECT id FROM native_invitations WHERE inviter_id=$1 AND request_id=$2`, []any{userID, request.String()}, &id)
		if err == nil {
			out, err = getInvite(ctx, c, id)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		cfg, err := readSettings(ctx, c, " FOR SHARE")
		if err != nil {
			return err
		}
		if !cfg.Enabled {
			return ErrDisabled
		}
		o, err := userLevel(ctx, c, cfg, userID)
		if err != nil {
			return err
		}
		lifetime, unused, err := inviteCounts(ctx, c, userID)
		if err != nil {
			return err
		}
		if (o.Level.LifetimeInviteLimit > 0 && lifetime >= int64(o.Level.LifetimeInviteLimit)) ||
			(o.Level.UnusedInviteLimit > 0 && unused >= int64(o.Level.UnusedInviteLimit)) {
			return ErrQuota
		}
		var redeemID int64
		for attempt := 0; attempt < 4; attempt++ {
			var random [15]byte
			if _, err := rand.Read(random[:]); err != nil {
				return err
			}
			code := "NI" + strings.ToUpper(hex.EncodeToString(random[:]))
			err = queryOne(ctx, c, `INSERT INTO redeem_codes(code,type,value,status,created_at,expires_at,validity_days)
				VALUES ($1,'invitation',0,'unused',NOW(),NOW()+$2*INTERVAL '1 day',30)
				ON CONFLICT (code) DO NOTHING RETURNING id`, []any{code, cfg.ValidityDays}, &redeemID)
			if err == nil {
				break
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if redeemID == 0 {
			return ErrConflict
		}
		err = queryOne(ctx, c, `INSERT INTO native_invitations(redeem_id,inviter_id,request_id,expires_at,rebate_eligible)
			SELECT id,$2,$3,expires_at,$4 FROM redeem_codes WHERE id=$1 RETURNING id`, []any{redeemID, userID, request.String(), kind == "personal"}, &id)
		if err != nil {
			return err
		}
		out, err = getInvite(ctx, c, id)
		return err
	})
	if out != nil {
		privateInvite(out)
	}
	return out, err
}

func (s *Store) SetOverride(ctx context.Context, userID int64, levelID string, actorID int64) error {
	return s.withTx(ctx, func(c *dbent.Client) error {
		if err := adminUser(ctx, c, actorID); err != nil {
			return err
		}
		if levelID == "" {
			// An override must remain removable after account deletion/conversion.
			var id int64
			if err := queryOne(ctx, c, `SELECT id FROM users WHERE id=$1 FOR NO KEY UPDATE`, []any{userID}, &id); err != nil {
				return notFound(err)
			}
		} else {
			if _, err := activeUserType(ctx, c, userID, true); err != nil {
				return err
			}
		}
		cfg, err := readSettings(ctx, c, " FOR SHARE")
		if err != nil {
			return err
		}
		if levelID != "" && findLevel(cfg.Levels, levelID) == nil {
			return ErrInvalid
		}
		var previous string
		err = queryOne(ctx, c, `SELECT level_id FROM native_invitation_overrides WHERE user_id=$1`, []any{userID}, &previous)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if levelID == "" {
			_, err = c.ExecContext(ctx, `DELETE FROM native_invitation_overrides WHERE user_id=$1`, userID)
		} else {
			_, err = c.ExecContext(ctx, `INSERT INTO native_invitation_overrides(user_id,level_id) VALUES($1,$2)
				ON CONFLICT(user_id) DO UPDATE SET level_id=EXCLUDED.level_id,updated_at=NOW()`, userID, levelID)
		}
		if err != nil {
			return err
		}
		return audit(ctx, c, actorID, "override", nil, userID, "", decimal.Zero, map[string]any{"before": previous, "after": levelID})
	})
}

// audit records actorID as NULL when it is nil (an automatic system action).
func audit(ctx context.Context, c *dbent.Client, actorID any, action string, rebateID, userID any, reason string, delta decimal.Decimal, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = c.ExecContext(ctx, `INSERT INTO native_invitation_audit(actor_id,action,rebate_id,user_id,reason,balance_delta,detail)
		VALUES($1,$2,$3,$4,$5,$6,$7::jsonb)`, actorID, action, rebateID, userID, reason, delta.String(), string(data))
	return err
}
