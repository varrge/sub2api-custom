// Package monthcard stores independently expiring subscription entitlements.
// It has no dependency on the service package so payment and billing can share it.
package monthcard

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound      = errors.New("month card resource not found")
	ErrInvalid       = errors.New("invalid month card operation")
	ErrCannotJoin    = errors.New("cannot join month card team")
	ErrTeamCancelled = errors.New("month card team recruitment was cancelled; manual refund required")
)

type Tier struct {
	Members  int     `json:"members"`
	QuotaUSD float64 `json:"quota_usd"`
}

type Product struct {
	ID               int64   `json:"id"`
	GroupID          int64   `json:"group_id"`
	GroupName        string  `json:"group_name"`
	Platform         string  `json:"platform"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	PriceCNY         float64 `json:"price_cny"`
	BaseQuotaUSD     float64 `json:"base_quota_usd"`
	Tiers            []Tier  `json:"tiers"`
	MaxMembers       int     `json:"max_members"`
	RecruitmentHours int     `json:"recruitment_hours"`
	ForSale          bool    `json:"for_sale"`
	SortOrder        int     `json:"sort_order"`
}

type Team struct {
	ID              int64     `json:"id"`
	Code            string    `json:"code"`
	ProductID       int64     `json:"product_id"`
	Product         Product   `json:"product"`
	MemberCount     int       `json:"member_count"`
	CurrentQuotaUSD float64   `json:"current_quota_usd"`
	NextQuotaUSD    float64   `json:"next_quota_usd"`
	NextMembers     int       `json:"next_members"`
	StartsAt        time.Time `json:"starts_at"`
	ClosesAt        time.Time `json:"closes_at"`
	Status          string    `json:"status"`
	Joined          bool      `json:"joined"`
}

type Card struct {
	ID                int64      `json:"id"`
	UserID            int64      `json:"user_id"`
	GroupID           int64      `json:"group_id"`
	OrderID           int64      `json:"order_id"`
	Code              string     `json:"code"`
	GroupName         string     `json:"group_name"`
	Platform          string     `json:"platform"`
	ProductName       string     `json:"product_name"`
	TeamCode          string     `json:"team_code"`
	Status            string     `json:"status"`
	TeamID            *int64     `json:"team_id"`
	TotalQuotaUSD     float64    `json:"total_quota_usd"`
	TotalUsedUSD      float64    `json:"total_used_usd"`
	WeeklyQuotaUSD    float64    `json:"weekly_quota_usd"`
	WeeklyUsedUSD     float64    `json:"weekly_used_usd"`
	StartsAt          time.Time  `json:"starts_at"`
	ExpiresAt         time.Time  `json:"expires_at"`
	FrozenAt          *time.Time `json:"frozen_at,omitempty"`
	PausedUS          int64      `json:"-"`
	RemainingSeconds  int64      `json:"remaining_seconds"`
	WeeklyWindowStart time.Time  `json:"weekly_window_start"`
	WeeklyWindowEnd   time.Time  `json:"weekly_window_end"`
	Priority          int        `json:"priority"`
	FreezeAllowed     bool       `json:"freeze_allowed"`
}

type Purchase struct {
	Mode     string  `json:"mode"`
	Product  Product `json:"product"`
	TeamID   *int64  `json:"team_id"`
	TeamCode string  `json:"team_code"`
}

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func NewStore(db *sql.DB) *Store { return &Store{db: db, now: time.Now} }

type rowScanner interface{ Scan(...any) error }
type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func newCode(prefix string) (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate month card code: %w", err)
	}
	return prefix + hex.EncodeToString(b[:]), nil
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// User rows serialize settlement, reordering and revocation. Fulfillment and
// revocation take user -> order -> team -> cards; billing never locks a team.
func lockUser(ctx context.Context, tx *sql.Tx, userID int64) error {
	var id int64
	return notFound(tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&id))
}

// Purchasing an offered subscription grants its entitlement. Standard-group
// binding restrictions must not require that entitlement before it is bought;
// API key binding remains gated by an active paid card or legacy subscription.
func checkPurchaseEligibility(ctx context.Context, q queryer, userID, groupID int64) error {
	var eligible bool
	err := q.QueryRowContext(ctx, `SELECT u.status='active' AND u.deleted_at IS NULL
		AND g.status='active' AND g.deleted_at IS NULL AND g.subscription_type='subscription'
		FROM users u CROSS JOIN groups g WHERE u.id=$1 AND g.id=$2`, userID, groupID).Scan(&eligible)
	if err != nil {
		return notFound(err)
	}
	if !eligible {
		return fmt.Errorf("%w: user or subscription group is unavailable", ErrInvalid)
	}
	return nil
}
