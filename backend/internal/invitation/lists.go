package invitation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const inviteStatus = `CASE WHEN b.invitee_id IS NOT NULL OR rc.status='used' THEN 'used'
	     WHEN rc.status<>'unused' THEN 'disabled'
	     WHEN i.expires_at<=NOW() THEN 'expired' ELSE 'unused' END`

const inviteFrom = ` FROM native_invitations i JOIN redeem_codes rc ON rc.id=i.redeem_id
	JOIN users u ON u.id=i.inviter_id
	LEFT JOIN native_invitation_bindings b ON b.invite_id=i.id
	LEFT JOIN users e ON e.id=b.invitee_id`

const inviteSelect = `SELECT i.id,rc.code,i.inviter_id,u.email,b.invitee_id,COALESCE(e.email,''),` + inviteStatus + `,
	i.created_at,i.expires_at,rc.used_at` + inviteFrom

// ListFilter narrows list results. Query matches the code for everyone; only the
// admin view may also match account emails, otherwise search would defeat masking.
type ListFilter struct {
	UserID int64
	Status string
	Query  string
}

func (f *ListFilter) normalize(admin bool, statuses ...string) error {
	f.Status, f.Query = strings.TrimSpace(f.Status), strings.TrimSpace(f.Query)
	if f.UserID < 0 || (!admin && f.UserID == 0) || utf8.RuneCountInString(f.Query) > 200 {
		return ErrInvalid
	}
	if f.Status == "" {
		return nil
	}
	for _, s := range statuses {
		if f.Status == s {
			return nil
		}
	}
	return ErrInvalid
}

// pattern returns an ILIKE pattern with user wildcards escaped, or "" for no search.
func (f ListFilter) pattern() string {
	if f.Query == "" {
		return ""
	}
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.Query) + "%"
}

func queryClause(admin bool, n string) string {
	if admin {
		return ` AND (` + n + `::text='' OR rc.code ILIKE ` + n + ` OR u.email ILIKE ` + n + ` OR e.email ILIKE ` + n + `)`
	}
	return ` AND (` + n + `::text='' OR rc.code ILIKE ` + n + `)`
}

type scanner interface{ Scan(...any) error }

func scanInvite(row scanner) (*Invite, error) {
	i := new(Invite)
	err := row.Scan(&i.ID, &i.Code, &i.InviterID, &i.InviterEmail, &i.InviteeID, &i.InviteeEmail,
		&i.Status, &i.CreatedAt, &i.ExpiresAt, &i.UsedAt)
	return i, err
}

func getInvite(ctx context.Context, c *dbent.Client, id int64) (*Invite, error) {
	rows, err := c.QueryContext(ctx, inviteSelect+` WHERE i.id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return scanInvite(rows)
}

func pagination(page, size int) (int, int, error) {
	// 1000 matches the largest page size the shared table settings can offer.
	if page < 1 || size < 1 || size > 1000 || page > 10_000_000 {
		return 0, 0, ErrInvalid
	}
	return size, (page - 1) * size, nil
}

func (s *Store) ListInvites(ctx context.Context, userID int64, page, pageSize int, admin bool) ([]Invite, int64, error) {
	return s.SearchInvites(ctx, ListFilter{UserID: userID}, page, pageSize, admin)
}

func (s *Store) SearchInvites(ctx context.Context, f ListFilter, page, pageSize int, admin bool) ([]Invite, int64, error) {
	size, offset, err := pagination(page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if err := f.normalize(admin, "unused", "used", "expired", "disabled"); err != nil {
		return nil, 0, err
	}
	if !admin {
		if err := personalUser(ctx, s.client, f.UserID, false); err != nil {
			return nil, 0, err
		}
	}
	where := ` WHERE ($1::bigint=0 OR i.inviter_id=$1) AND ($2::text='' OR ` + inviteStatus + `=$2)` + queryClause(admin, "$3")
	args := []any{f.UserID, f.Status, f.pattern()}
	var total int64
	if err := queryOne(ctx, s.client, `SELECT COUNT(*)`+inviteFrom+where, args, &total); err != nil {
		return nil, 0, err
	}
	rows, err := s.client.QueryContext(ctx, inviteSelect+where+` ORDER BY i.id DESC LIMIT $4 OFFSET $5`, append(args, size, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]Invite, 0)
	for rows.Next() {
		i, err := scanInvite(rows)
		if err != nil {
			return nil, 0, err
		}
		if !admin {
			privateInvite(i)
		}
		result = append(result, *i)
	}
	return result, total, rows.Err()
}

const rebateFrom = ` FROM native_invitation_rebates r JOIN native_invitations i ON i.id=r.invite_id
	JOIN redeem_codes rc ON rc.id=i.redeem_id JOIN users u ON u.id=r.inviter_id JOIN users e ON e.id=r.invitee_id
	LEFT JOIN payment_orders po ON r.source_type='payment' AND po.id=r.source_id`

const rebateSelect = `SELECT r.id,r.invite_id,rc.code,r.inviter_id,u.email,r.invitee_id,e.email,
	r.source_type,r.source_id,r.source_currency,COALESCE(po.status,''),r.source_occurred_at,r.first_amount,r.rebate_rate,
	r.rebate_amount,r.level_name,r.status,r.review_reason,r.created_at,r.reviewed_at,
	EXISTS(SELECT 1 FROM native_invitation_audit a WHERE a.rebate_id=r.id AND a.action='approve' AND a.actor_id IS NULL)` + rebateFrom

func scanRebate(row scanner) (*Rebate, error) {
	r := new(Rebate)
	err := row.Scan(&r.ID, &r.InviteID, &r.InviteCode, &r.InviterID, &r.InviterEmail, &r.InviteeID, &r.InviteeEmail,
		&r.SourceType, &r.SourceID, &r.SourceCurrency, &r.SourceStatus, &r.SourceOccurredAt, &r.FirstAmount, &r.RebateRate,
		&r.RebateAmount, &r.LevelName, &r.Status, &r.ReviewReason, &r.CreatedAt, &r.ReviewedAt, &r.AutoApproved)
	return r, err
}

func getRebate(ctx context.Context, c *dbent.Client, id int64) (*Rebate, error) {
	rows, err := c.QueryContext(ctx, rebateSelect+` WHERE r.id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return scanRebate(rows)
}

func (s *Store) ListRebates(ctx context.Context, userID int64, status string, page, pageSize int, admin bool) ([]Rebate, int64, error) {
	return s.SearchRebates(ctx, ListFilter{UserID: userID, Status: status}, page, pageSize, admin)
}

func (s *Store) SearchRebates(ctx context.Context, f ListFilter, page, pageSize int, admin bool) ([]Rebate, int64, error) {
	size, offset, err := pagination(page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if err := f.normalize(admin, "pending", "approved", "rejected", "revoked"); err != nil {
		return nil, 0, err
	}
	if !admin {
		if err := personalUser(ctx, s.client, f.UserID, false); err != nil {
			return nil, 0, err
		}
	}
	where := ` WHERE ($1::bigint=0 OR r.inviter_id=$1) AND ($2::text='' OR r.status=$2)` + queryClause(admin, "$3")
	args := []any{f.UserID, f.Status, f.pattern()}
	var total int64
	if err := queryOne(ctx, s.client, `SELECT COUNT(*)`+rebateFrom+where, args, &total); err != nil {
		return nil, 0, err
	}
	rows, err := s.client.QueryContext(ctx, rebateSelect+where+` ORDER BY r.id DESC LIMIT $4 OFFSET $5`, append(args, size, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]Rebate, 0)
	for rows.Next() {
		r, err := scanRebate(rows)
		if err != nil {
			return nil, 0, err
		}
		if !admin {
			r.InviteeID = nil
			r.InviteeEmail = maskEmail(r.InviteeEmail)
			r.SourceID = 0
			r.SourceStatus = ""
		}
		result = append(result, *r)
	}
	return result, total, rows.Err()
}

// Stats is the admin work-queue summary across all inviters.
type Stats struct {
	TotalInvites    int64   `json:"total_invites"`
	UsedInvites     int64   `json:"used_invites"`
	ActiveInvites   int64   `json:"active_invites"`
	PendingCount    int64   `json:"pending_count"`
	PendingAmount   float64 `json:"pending_amount"`
	ApprovedCount   int64   `json:"approved_count"`
	ApprovedAmount  float64 `json:"approved_amount"`
	DeclinedCount   int64   `json:"declined_count"`
	DeclinedAmount  float64 `json:"declined_amount"`
	OverriddenUsers int64   `json:"overridden_users"`
}

func (s *Store) Stats(ctx context.Context) (*Stats, error) {
	st := new(Stats)
	err := queryOne(ctx, s.client, `SELECT COUNT(*),
		COUNT(*) FILTER (WHERE `+inviteStatus+`='used'),
		COUNT(*) FILTER (WHERE `+inviteStatus+`='unused')`+inviteFrom, nil,
		&st.TotalInvites, &st.UsedInvites, &st.ActiveInvites)
	if err != nil {
		return nil, err
	}
	err = queryOne(ctx, s.client, `SELECT
		COUNT(*) FILTER (WHERE status='pending'),COALESCE(SUM(rebate_amount) FILTER (WHERE status='pending'),0),
		COUNT(*) FILTER (WHERE status='approved'),COALESCE(SUM(rebate_amount) FILTER (WHERE status='approved'),0),
		COUNT(*) FILTER (WHERE status IN ('rejected','revoked')),
		COALESCE(SUM(rebate_amount) FILTER (WHERE status IN ('rejected','revoked')),0),
		(SELECT COUNT(*) FROM native_invitation_overrides)
		FROM native_invitation_rebates`, nil,
		&st.PendingCount, &st.PendingAmount, &st.ApprovedCount, &st.ApprovedAmount,
		&st.DeclinedCount, &st.DeclinedAmount, &st.OverriddenUsers)
	return st, err
}

// Overrides lists users whose level was set by an administrator, newest change first.
func (s *Store) Overrides(ctx context.Context, page, pageSize int) ([]Override, int64, error) {
	size, offset, err := pagination(page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := queryOne(ctx, s.client, `SELECT COUNT(*) FROM native_invitation_overrides`, nil, &total); err != nil {
		return nil, 0, err
	}
	rows, err := s.client.QueryContext(ctx, `SELECT o.user_id,u.email,o.level_id,o.updated_at
		FROM native_invitation_overrides o JOIN users u ON u.id=o.user_id
		ORDER BY o.updated_at DESC,o.user_id DESC LIMIT $1 OFFSET $2`, size, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]Override, 0)
	for rows.Next() {
		var o Override
		if err := rows.Scan(&o.UserID, &o.Email, &o.LevelID, &o.UpdatedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, o)
	}
	return result, total, rows.Err()
}

func privateInvite(i *Invite) { i.InviteeID = nil; i.InviteeEmail = maskEmail(i.InviteeEmail) }

func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" {
		return "***"
	}
	_, n := utf8.DecodeRuneInString(local)
	// Mask the domain as well: private/custom domains can identify a person.
	suffix := ""
	if i := strings.LastIndexByte(domain, '.'); i >= 0 {
		suffix = domain[i:]
	}
	return local[:n] + "***@***" + suffix
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
