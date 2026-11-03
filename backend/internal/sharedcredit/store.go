// Package sharedcredit accounts for the site's shared model-base-price budget.
// Billing callers lock the user before the pool and commit all charges together.
package sharedcredit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("共享额度参数无效")
var ErrChanged = errors.New("共享额度配置已变化，请刷新后重试")
var ErrDebt = errors.New("账户余额低于 0 元，请先充值至 0 元或以上再使用共享额度")

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

type State struct {
	Name         string          `json:"name"`
	StartsAt     *time.Time      `json:"starts_at"`
	EndsAt       *time.Time      `json:"ends_at"`
	Status       string          `json:"status"`
	Enabled      bool            `json:"enabled"`
	GroupIDs     []int64         `json:"group_ids"`
	TotalUSD     decimal.Decimal `json:"total_usd"`
	UsedUSD      decimal.Decimal `json:"used_usd"`
	ReservedUSD  decimal.Decimal `json:"reserved_usd"`
	RemainingUSD decimal.Decimal `json:"remaining_usd"`
	Revision     int64           `json:"revision"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
type Update struct {
	Name     string     `json:"name"`
	StartsAt *time.Time `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
	Enabled  *bool      `json:"enabled"`
	GroupIDs []int64    `json:"group_ids"`
	TotalUSD string     `json:"total_usd"`
	Revision int64      `json:"revision"`
}
type Allocation struct {
	ID             int64           `json:"id"`
	RequestID      string          `json:"request_id"`
	UserID         int64           `json:"user_id"`
	APIKeyID       int64           `json:"api_key_id"`
	GroupID        int64           `json:"group_id"`
	Model          string          `json:"model"`
	AmountUSD      decimal.Decimal `json:"amount_usd"`
	CoveredCostUSD decimal.Decimal `json:"covered_cost_usd"`
	CreatedAt      time.Time       `json:"created_at"`
}
type History struct {
	Items []Allocation `json:"items"`
	Total int64        `json:"total"`
	Page  int          `json:"page"`
}
type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const columns = `enabled,group_ids,total_usd,used_usd,reserved_usd,total_usd-used_usd-reserved_usd,revision,updated_at,name,starts_at,ends_at`

func (s *State) ActiveAt(now time.Time) bool {
	return s.Enabled && s.EndsAt != nil && now.Before(*s.EndsAt) && (s.StartsAt == nil || !now.Before(*s.StartsAt))
}

func (s *State) statusAt(now time.Time) string {
	switch {
	case !s.Enabled:
		return "disabled"
	case s.EndsAt == nil:
		return "incomplete"
	case !now.Before(*s.EndsAt):
		return "ended"
	case s.StartsAt != nil && now.Before(*s.StartsAt):
		return "scheduled"
	case len(s.GroupIDs) == 0:
		return "no_groups"
	case !s.RemainingUSD.IsPositive():
		return "exhausted"
	default:
		return "active"
	}
}

func read(ctx context.Context, q queryer, lock bool) (*State, error) {
	query := `SELECT ` + columns + ` FROM site_shared_credit_pool WHERE id=TRUE`
	if lock {
		query += ` FOR UPDATE`
	}
	s := &State{GroupIDs: []int64{}}
	err := q.QueryRowContext(ctx, query).Scan(&s.Enabled, pq.Array(&s.GroupIDs), &s.TotalUSD, &s.UsedUSD, &s.ReservedUSD, &s.RemainingUSD, &s.Revision, &s.UpdatedAt, &s.Name, &s.StartsAt, &s.EndsAt)
	s.Status = s.statusAt(time.Now())
	return s, err
}
func (s *Store) Get(ctx context.Context) (*State, error) { return read(ctx, s.db, false) }

// CheckDebt uses the current SQL balance, never an authentication/cache snapshot.
func (s *Store) CheckDebt(ctx context.Context, userID int64) error {
	var balance decimal.Decimal
	if err := s.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&balance); err != nil {
		return err
	}
	if balance.IsNegative() {
		return ErrDebt
	}
	return nil
}
func (s *Store) Available(ctx context.Context, groupID int64) (bool, error) {
	if s == nil || s.db == nil || groupID <= 0 {
		return false, nil
	}
	// Reads remain lock-free unless expired crash leftovers need reclaiming.
	var expired bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM site_shared_credit_holds WHERE expires_at<=NOW())`).Scan(&expired); err != nil {
		return false, err
	}
	if expired {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return false, err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err = lockPool(ctx, tx); err != nil {
			return false, err
		}
		if err = tx.Commit(); err != nil {
			return false, err
		}
	}
	state, err := s.Get(ctx)
	if err != nil {
		return false, err
	}
	return state.ActiveAt(time.Now()) && slices.Contains(state.GroupIDs, groupID) && state.RemainingUSD.IsPositive(), nil
}
func parseTotal(raw string) (decimal.Decimal, error) {
	if len(raw) == 0 || len(raw) > 22 {
		return decimal.Zero, ErrInvalid
	}
	for _, r := range raw {
		if (r < '0' || r > '9') && r != '.' {
			return decimal.Zero, ErrInvalid
		}
	}
	total, err := decimal.NewFromString(raw)
	if err != nil || total.IsNegative() || total.Exponent() < -8 || total.GreaterThan(decimal.NewFromInt(1_000_000_000)) {
		return decimal.Zero, ErrInvalid
	}
	return total, nil
}
func (s *Store) Update(ctx context.Context, actorID int64, req Update) (*State, error) {
	if actorID <= 0 || req.Enabled == nil || req.Revision <= 0 || req.GroupIDs == nil {
		return nil, ErrInvalid
	}
	total, err := parseTotal(req.TotalUSD)
	if err != nil {
		return nil, err
	}
	return s.update(ctx, actorID, req.Revision, func(state *State) {
		state.Name = strings.TrimSpace(req.Name)
		state.StartsAt = req.StartsAt
		state.EndsAt = req.EndsAt
		state.Enabled = *req.Enabled
		state.TotalUSD = total
		state.GroupIDs = slices.Clone(req.GroupIDs)
	})
}
func (s *Store) SetGroup(ctx context.Context, actorID, groupID, revision int64, enabled bool) (*State, error) {
	if actorID <= 0 || groupID <= 0 || revision <= 0 {
		return nil, ErrInvalid
	}
	return s.update(ctx, actorID, revision, func(state *State) {
		ids := []int64{}
		for _, id := range state.GroupIDs {
			if id != groupID {
				ids = append(ids, id)
			}
		}
		if enabled {
			ids = append(ids, groupID)
		}
		state.GroupIDs = ids
	})
}
func (s *Store) update(ctx context.Context, actorID, revision int64, change func(*State)) (*State, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	old, err := lockPool(ctx, tx)
	if err != nil {
		return nil, err
	}
	if old.Revision != revision {
		return nil, ErrChanged
	}
	next := *old
	change(&next)
	if utf8.RuneCountInString(next.Name) > 100 {
		return nil, fmt.Errorf("%w：活动名称不能超过 100 个字符", ErrInvalid)
	}
	if (next.Enabled && next.EndsAt == nil) || (next.StartsAt != nil && next.EndsAt != nil && !next.StartsAt.Before(*next.EndsAt)) {
		return nil, fmt.Errorf("%w：启用活动必须设置结束时间，且结束时间须晚于开始时间", ErrInvalid)
	}
	if next.TotalUSD.LessThan(old.UsedUSD.Add(old.ReservedUSD)) {
		return nil, fmt.Errorf("%w：总额度不能低于已用与预留额度之和", ErrInvalid)
	}
	if len(next.GroupIDs) > 10000 {
		return nil, ErrInvalid
	}
	slices.Sort(next.GroupIDs)
	for i, id := range next.GroupIDs {
		if id <= 0 || (i > 0 && id == next.GroupIDs[i-1]) {
			return nil, ErrInvalid
		}
	}
	var valid int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(next.GroupIDs)).Scan(&valid); err != nil {
		return nil, err
	}
	if valid != len(next.GroupIDs) {
		return nil, fmt.Errorf("%w：包含不存在的分组", ErrInvalid)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE site_shared_credit_pool SET enabled=$1,total_usd=$2,group_ids=$3,name=$4,starts_at=$5,ends_at=$6,revision=revision+1,updated_at=NOW() WHERE id=TRUE`, next.Enabled, next.TotalUSD, pq.Array(next.GroupIDs), next.Name, next.StartsAt, next.EndsAt); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO site_shared_credit_changes(actor_user_id,old_total_usd,total_usd,old_enabled,enabled,old_group_ids,group_ids,old_name,name,old_starts_at,starts_at,old_ends_at,ends_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, actorID, old.TotalUSD, next.TotalUSD, old.Enabled, next.Enabled, pq.Array(old.GroupIDs), pq.Array(next.GroupIDs), old.Name, next.Name, old.StartsAt, next.StartsAt, old.EndsAt, next.EndsAt); err != nil {
		return nil, err
	}
	state, err := read(ctx, tx, false)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return state, nil
}
func (s *Store) History(ctx context.Context, page int) (*History, error) {
	if page < 1 || page > 1_000_000 {
		return nil, ErrInvalid
	}
	h := &History{Items: []Allocation{}, Page: page}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM site_shared_credit_allocations`).Scan(&h.Total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,user_id,api_key_id,group_id,model,amount_usd,covered_cost_usd,created_at FROM site_shared_credit_allocations ORDER BY id DESC LIMIT 20 OFFSET $1`, (page-1)*20)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var a Allocation
		if err = rows.Scan(&a.ID, &a.RequestID, &a.UserID, &a.APIKeyID, &a.GroupID, &a.Model, &a.AmountUSD, &a.CoveredCostUSD, &a.CreatedAt); err != nil {
			return nil, err
		}
		h.Items = append(h.Items, a)
	}
	return h, rows.Err()
}

// Covered converts sponsored base-price usage into the personal charge avoided.
func Covered(base, actual, share decimal.Decimal) decimal.Decimal {
	if !base.IsPositive() || !actual.IsPositive() || !share.IsPositive() {
		return decimal.Zero
	}
	return decimal.Min(actual, actual.Mul(share).DivRound(base, 8)).Round(8)
}
func lockPool(ctx context.Context, tx *sql.Tx) (*State, error) {
	state, err := read(ctx, tx, true)
	if err != nil {
		return nil, err
	}
	var expired decimal.Decimal
	err = tx.QueryRowContext(ctx, `WITH gone AS (
 DELETE FROM site_shared_credit_holds h WHERE expires_at<=NOW()
 AND NOT EXISTS(SELECT 1 FROM month_card_billing_pending p WHERE p.user_id=h.user_id AND p.command->>'FixedChargeReservationID'=h.id)
 RETURNING amount_usd) SELECT COALESCE(SUM(amount_usd),0) FROM gone`).Scan(&expired)
	if err != nil {
		return nil, err
	}
	if expired.IsPositive() {
		if _, err = tx.ExecContext(ctx, `UPDATE site_shared_credit_pool SET reserved_usd=reserved_usd-$1 WHERE id=TRUE`, expired); err != nil {
			return nil, err
		}
		state.ReservedUSD = state.ReservedUSD.Sub(expired)
		state.RemainingUSD = state.RemainingUSD.Add(expired)
	}
	return state, nil
}

type Hold struct {
	ID                        string
	UserID, APIKeyID, GroupID int64
	AmountUSD, CoveredCostUSD decimal.Decimal
	ExpiresAt                 *time.Time
}

func (s *Store) Renew(ctx context.Context, id string, userID, keyID int64, ttl time.Duration) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE site_shared_credit_holds SET expires_at=NOW()+$4*INTERVAL '1 microsecond' WHERE id=$1 AND user_id=$2 AND api_key_id=$3 AND expires_at>NOW()`, id, userID, keyID, ttl.Microseconds())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func GetHoldTx(ctx context.Context, tx *sql.Tx, id string, userID, keyID int64) (*Hold, error) {
	h := &Hold{ID: id, UserID: userID, APIKeyID: keyID}
	err := tx.QueryRowContext(ctx, `SELECT group_id,amount_usd,covered_cost_usd,expires_at FROM site_shared_credit_holds WHERE id=$1 AND user_id=$2 AND api_key_id=$3`, id, userID, keyID).Scan(&h.GroupID, &h.AmountUSD, &h.CoveredCostUSD, &h.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return h, err
}

// ReserveTx requires the user accounting lock; idempotency belongs to the caller.
func ReserveTx(ctx context.Context, tx *sql.Tx, h Hold, base, actual decimal.Decimal) (decimal.Decimal, error) {
	if !base.IsPositive() || h.GroupID <= 0 {
		return decimal.Zero, nil
	}
	state, err := read(ctx, tx, false)
	if err != nil {
		return decimal.Zero, err
	}
	if !state.ActiveAt(time.Now()) || !slices.Contains(state.GroupIDs, h.GroupID) {
		return decimal.Zero, nil
	}
	var balance decimal.Decimal
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL`, h.UserID).Scan(&balance); err != nil {
		return decimal.Zero, err
	}
	if balance.IsNegative() {
		return decimal.Zero, ErrDebt
	}
	state, err = lockPool(ctx, tx)
	if err != nil {
		return decimal.Zero, err
	}
	if !state.ActiveAt(time.Now()) || !slices.Contains(state.GroupIDs, h.GroupID) {
		return decimal.Zero, nil
	}
	share := decimal.Min(base.Round(8), state.RemainingUSD)
	if !share.IsPositive() {
		return decimal.Zero, nil
	}
	covered := Covered(base, actual, share)
	if _, err = tx.ExecContext(ctx, `INSERT INTO site_shared_credit_holds(id,user_id,api_key_id,group_id,amount_usd,covered_cost_usd,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, h.ID, h.UserID, h.APIKeyID, h.GroupID, share, covered, h.ExpiresAt); err != nil {
		return decimal.Zero, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE site_shared_credit_pool SET reserved_usd=reserved_usd+$1 WHERE id=TRUE`, share)
	return covered, err
}
func (s *Store) Reserve(ctx context.Context, h Hold, base, actual decimal.Decimal) (decimal.Decimal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return decimal.Zero, err
	}
	defer func() { _ = tx.Rollback() }()
	var balance decimal.Decimal
	if err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, h.UserID).Scan(&balance); err != nil {
		return decimal.Zero, err
	}
	if balance.IsNegative() {
		return decimal.Zero, ErrDebt
	}
	covered, err := ReserveTx(ctx, tx, h, base, actual)
	if err != nil {
		return decimal.Zero, err
	}
	return covered, tx.Commit()
}
func ReleaseTx(ctx context.Context, tx *sql.Tx, id string, userID, keyID int64) (*Hold, error) {
	if _, err := lockPool(ctx, tx); err != nil {
		return nil, err
	}
	h, err := GetHoldTx(ctx, tx, id, userID, keyID)
	if err != nil || h == nil {
		return h, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM site_shared_credit_holds WHERE id=$1`, id); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE site_shared_credit_pool SET reserved_usd=reserved_usd-$1 WHERE id=TRUE`, h.AmountUSD)
	return h, err
}
func (s *Store) Release(ctx context.Context, id string, userID, keyID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = ReleaseTx(ctx, tx, id, userID, keyID); err != nil {
		return err
	}
	return tx.Commit()
}

// ConsumeTx runs after the billing dedup claim. Accepted holds remain sponsored
// even when an administrator disables the pool or removes their group.
func ConsumeTx(ctx context.Context, tx *sql.Tx, requestID, holdID string, keyID, userID, groupID int64, model string, base, actual decimal.Decimal) (decimal.Decimal, decimal.Decimal, error) {
	if groupID <= 0 || (!base.IsPositive() && holdID == "") {
		return decimal.Zero, decimal.Zero, nil
	}
	// Disabled and excluded traffic must not serialize behind the global budget.
	if holdID == "" {
		state, err := read(ctx, tx, false)
		if err != nil {
			return decimal.Zero, decimal.Zero, err
		}
		if !state.ActiveAt(time.Now()) || !slices.Contains(state.GroupIDs, groupID) || (!state.RemainingUSD.IsPositive() && !state.ReservedUSD.IsPositive()) {
			return decimal.Zero, decimal.Zero, nil
		}
	}
	var balance decimal.Decimal
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&balance); err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	if balance.IsNegative() && holdID == "" {
		// The request may already have completed upstream: charge personal usage,
		// rather than dropping the bill or granting a new subsidy to a debtor.
		return decimal.Zero, decimal.Zero, nil
	}
	state, err := lockPool(ctx, tx)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	held := decimal.Zero
	if holdID != "" {
		h, err := GetHoldTx(ctx, tx, holdID, userID, keyID)
		if err != nil {
			return decimal.Zero, decimal.Zero, err
		}
		if h != nil {
			if h.GroupID != groupID {
				return decimal.Zero, decimal.Zero, errors.New("shared credit hold group mismatch")
			}
			held = h.AmountUSD
			if _, err = tx.ExecContext(ctx, `DELETE FROM site_shared_credit_holds WHERE id=$1`, holdID); err != nil {
				return decimal.Zero, decimal.Zero, err
			}
		}
	}
	share := decimal.Min(decimal.Max(base.Round(8), decimal.Zero), held)
	if !balance.IsNegative() && state.ActiveAt(time.Now()) && slices.Contains(state.GroupIDs, groupID) {
		share = decimal.Min(decimal.Max(base.Round(8), decimal.Zero), held.Add(state.RemainingUSD))
	}
	covered := Covered(base, actual, share)
	if _, err = tx.ExecContext(ctx, `UPDATE site_shared_credit_pool SET reserved_usd=reserved_usd-$1,used_usd=used_usd+$2 WHERE id=TRUE`, held, share); err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	if share.IsPositive() {
		if _, err = tx.ExecContext(ctx, `INSERT INTO site_shared_credit_allocations(request_id,api_key_id,user_id,group_id,model,amount_usd,covered_cost_usd) VALUES($1,$2,$3,$4,$5,$6,$7)`, requestID, keyID, userID, groupID, model, share, covered); err != nil {
			return decimal.Zero, decimal.Zero, err
		}
	}
	return share, covered, nil
}
