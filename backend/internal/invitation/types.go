// Package invitation owns one-use registration invitations and reviewed rewards.
// Registration and acquisition hooks must receive the caller's transaction client.
package invitation

import (
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrNotFound     = infraerrors.NotFound("INVITATION_NOT_FOUND", "邀请记录不存在")
	ErrPersonalOnly = infraerrors.Forbidden("INVITATION_PERSONAL_ONLY", "仅个人账户可以参与邀请返利")
	ErrDisabled     = infraerrors.Forbidden("INVITATION_DISABLED", "暂未开放新邀请码申请")
	ErrQuota        = infraerrors.Conflict("INVITATION_QUOTA_EXCEEDED", "邀请码申请额度已用完")
	ErrInvalid      = infraerrors.BadRequest("INVITATION_INVALID", "邀请参数无效")
	ErrConflict     = infraerrors.Conflict("INVITATION_CONFLICT", "邀请记录状态已变化")
	ErrLevelInUse   = infraerrors.Conflict("INVITATION_LEVEL_IN_USE", "仍有用户被指定为该等级，请先将其恢复为自动等级再删除")
)

type Store struct{ client *dbent.Client }

func NewStore(client *dbent.Client) *Store { return &Store{client: client} }

type Level struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	SpendThreshold      float64 `json:"spend_threshold"`
	LifetimeInviteLimit int     `json:"lifetime_invite_limit"`
	UnusedInviteLimit   int     `json:"unused_invite_limit"`
	// RebateRate is a percentage: 10 means 10%.
	RebateRate float64 `json:"rebate_rate"`
}

type Settings struct {
	Enabled      bool `json:"enabled"`
	ValidityDays int  `json:"validity_days"`
	// AutoApproveHours credits a pending rebate automatically once it has waited this
	// long. 0 keeps every rebate in the manual review queue.
	AutoApproveHours int     `json:"auto_approve_hours"`
	Levels           []Level `json:"levels"`
}

type Summary struct {
	Pending           float64 `json:"pending"`
	Paid              float64 `json:"paid"`
	RejectedOrRevoked float64 `json:"rejected_or_revoked"`
}

type Overview struct {
	Enabled       bool `json:"enabled"`
	RebateEnabled bool `json:"rebate_enabled"`
	ValidityDays  int  `json:"validity_days"`
	// AutoApproveHours mirrors the setting so the inviter knows how long a reward can wait.
	AutoApproveHours int   `json:"auto_approve_hours"`
	Level            Level `json:"level"`
	// Levels is the full ladder, lowest threshold first, so a user can see what each level offers.
	Levels             []Level `json:"levels"`
	RechargeTotal      float64 `json:"recharge_total"`
	NextLevel          *Level  `json:"next_level"`
	NextLevelRemaining float64 `json:"next_level_remaining"`
	OverrideLevelID    string  `json:"override_level_id"`
	LifetimeUsed       int64   `json:"lifetime_used"`
	UnusedCount        int64   `json:"unused_count"`
	Summary            Summary `json:"summary"`
}

type Invite struct {
	ID             int64      `json:"id"`
	RebateEligible bool       `json:"rebate_eligible"`
	Code           string     `json:"code"`
	InviterID      int64      `json:"inviter_id"`
	InviterEmail   string     `json:"inviter_email"`
	InviteeID      *int64     `json:"invitee_id"`
	InviteeEmail   string     `json:"invitee_email"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	UsedAt         *time.Time `json:"used_at"`
}

type Rebate struct {
	ID             int64  `json:"id"`
	InviteID       int64  `json:"invite_id"`
	InviteCode     string `json:"invite_code"`
	InviterID      int64  `json:"inviter_id"`
	InviterEmail   string `json:"inviter_email"`
	InviteeID      *int64 `json:"invitee_id"`
	InviteeEmail   string `json:"invitee_email"`
	SourceType     string `json:"source_type"`
	SourceID       int64  `json:"source_id"`
	SourceCurrency string `json:"source_currency"`
	// SourceStatus is the current payment order status (admin view only, "" for card codes),
	// so a refunded first payment is visible before the reward is credited.
	SourceStatus     string    `json:"source_status"`
	SourceOccurredAt time.Time `json:"source_occurred_at"`
	FirstAmount      float64   `json:"first_amount"`
	RebateRate       float64   `json:"rebate_rate"`
	RebateAmount     float64   `json:"rebate_amount"`
	LevelName        string    `json:"level_name"`
	Status           string    `json:"status"`
	ReviewReason     string    `json:"review_reason"`
	// AutoApproved is true when the waiting period, not an administrator, credited the reward.
	AutoApproved bool       `json:"auto_approved"`
	CreatedAt    time.Time  `json:"created_at"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
}

// Override is one user whose level was set by an administrator.
type Override struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	LevelID   string    `json:"level_id"`
	UpdatedAt time.Time `json:"updated_at"`
}
