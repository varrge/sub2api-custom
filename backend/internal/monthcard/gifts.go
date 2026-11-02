package monthcard

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

var ErrGiftPreviewChanged = errors.New("赠卡配置或符合条件的用户已变化，请重新预览")

type GiftConfig struct {
	Name          string          `json:"name"`
	GroupID       int64           `json:"group_id"`
	SourceGroupID int64           `json:"source_group_id"`
	IncludeFrozen bool            `json:"include_frozen"`
	QuotaUSD      decimal.Decimal `json:"quota_usd"`
	ValidDays     int             `json:"valid_days"`
}

func (c GiftConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" || utf8.RuneCountInString(c.Name) > 100 || c.GroupID <= 0 || c.SourceGroupID < 0 || c.ValidDays < 1 || c.ValidDays > 365 {
		return fmt.Errorf("%w: 请设置卡名、分组及 1 至 365 天有效期", ErrInvalid)
	}
	if c.QuotaUSD.Exponent() < -8 || c.QuotaUSD.Exponent() > 9 || !c.QuotaUSD.IsPositive() || c.QuotaUSD.GreaterThan(decimal.NewFromInt(1_000_000_000)) {
		return fmt.Errorf("%w: 额度须大于 0、不超过 1,000,000,000，最多 8 位小数", ErrInvalid)
	}
	return nil
}

type GiftPreview struct {
	RecipientCount int             `json:"recipient_count"`
	TotalQuotaUSD  decimal.Decimal `json:"total_quota_usd"`
	PreviewToken   string          `json:"preview_token"`
}

type GiftIssue struct {
	GiftConfig
	RequestID    string `json:"request_id"`
	PreviewToken string `json:"preview_token"`
}

type GiftBatch struct {
	ID int64 `json:"id"`
	GiftConfig
	GroupName      string    `json:"group_name"`
	ActorUserID    int64     `json:"actor_user_id"`
	RecipientCount int       `json:"recipient_count"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// A gift never qualifies its owner for another holder promotion. Exhausted paid
// cards still qualify; eligibility depends on time, ownership and payment.
const giftRecipients = `SELECT u.id FROM users u WHERE u.status='active' AND u.account_type='personal' AND u.deleted_at IS NULL
    AND EXISTS(SELECT 1 FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
        WHERE c.user_id=u.id AND o.user_id=u.id AND c.deleted_at IS NULL AND o.status IN ('COMPLETED','REFUND_FAILED')
        AND ($2::bigint=0 OR c.group_id=$2) AND (c.status='active' OR ($3 AND c.status='frozen'))
        AND c.starts_at<=$1 AND c.expires_at+c.paused_us*INTERVAL '1 microsecond'>COALESCE(c.frozen_at,$1)) ORDER BY u.id`

func giftUsers(ctx context.Context, q queryer, cfg GiftConfig, now time.Time, lock bool) ([]int64, error) {
	query := giftRecipients
	if lock {
		query += " FOR UPDATE OF u"
	}
	rows, err := q.QueryContext(ctx, query, now, cfg.SourceGroupID, cfg.IncludeFrozen)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func giftFingerprint(cfg GiftConfig, ids []int64) string {
	raw, _ := json.Marshal(struct {
		Config GiftConfig
		Users  []int64
	}{cfg, ids})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func giftGroup(ctx context.Context, q queryer, id int64) (string, string, error) {
	var name, platform string
	err := q.QueryRowContext(ctx, `SELECT name,platform FROM groups WHERE id=$1 AND status='active' AND deleted_at IS NULL AND subscription_type='subscription'`, id).Scan(&name, &platform)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("%w: 请选择有效的订阅分组", ErrInvalid)
	}
	return name, platform, err
}

func (s *Store) PreviewGift(ctx context.Context, cfg GiftConfig) (*GiftPreview, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := s.syncFreezePolicy(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	if _, _, err := giftGroup(ctx, s.db, cfg.GroupID); err != nil {
		return nil, err
	}
	ids, err := giftUsers(ctx, s.db, cfg, s.now(), false)
	if err != nil {
		return nil, err
	}
	return &GiftPreview{len(ids), cfg.QuotaUSD.Mul(decimal.NewFromInt(int64(len(ids)))), giftFingerprint(cfg, ids)}, nil
}

const giftBatchSelect = `SELECT b.id,b.name,b.group_id,COALESCE(b.source_group_id,0),b.include_frozen,b.quota_usd,b.valid_days,g.name,b.actor_user_id,b.recipient_count,b.created_at,b.expires_at
    FROM month_card_gift_batches b JOIN groups g ON g.id=b.group_id`

func scanGiftBatch(row rowScanner) (*GiftBatch, error) {
	var b GiftBatch
	err := row.Scan(&b.ID, &b.Name, &b.GroupID, &b.SourceGroupID, &b.IncludeFrozen, &b.QuotaUSD, &b.ValidDays, &b.GroupName, &b.ActorUserID, &b.RecipientCount, &b.CreatedAt, &b.ExpiresAt)
	return &b, notFound(err)
}

var giftRequestID = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,80}$`)

func (s *Store) IssueGift(ctx context.Context, actorID int64, req GiftIssue) (*GiftBatch, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if actorID <= 0 || !giftRequestID.MatchString(req.RequestID) || len(req.PreviewToken) != 64 {
		return nil, ErrInvalid
	}
	if err := s.syncFreezePolicy(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Replays serialize before inspecting a changing audience; a committed batch
	// always returns its original receipt, even if the caller lost the response.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "month-card-gift:"+req.RequestID); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(req)
	hash := sha256.Sum256(raw)
	requestHash := hex.EncodeToString(hash[:])
	var existingID, existingActor int64
	var existingHash string
	err = tx.QueryRowContext(ctx, `SELECT id,actor_user_id,request_hash FROM month_card_gift_batches WHERE request_id=$1`, req.RequestID).Scan(&existingID, &existingActor, &existingHash)
	if err == nil {
		if existingHash != requestHash || existingActor != actorID {
			return nil, ErrGiftPreviewChanged
		}
		b, err := scanGiftBatch(tx.QueryRowContext(ctx, giftBatchSelect+` WHERE b.id=$1`, existingID))
		if err != nil {
			return nil, err
		}
		return b, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	locked, err := giftUsers(ctx, tx, req.GiftConfig, s.now(), true)
	if err != nil {
		return nil, err
	}
	// Follow the same user -> order lock order as administrative quota changes.
	rows, err := tx.QueryContext(ctx, `SELECT o.id FROM payment_orders o WHERE o.id IN (SELECT order_id FROM month_card_cards WHERE user_id=ANY($1)) ORDER BY o.id FOR SHARE`, pq.Array(locked))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	ids, err := giftUsers(ctx, tx, req.GiftConfig, now, false)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 || giftFingerprint(req.GiftConfig, ids) != req.PreviewToken || giftFingerprint(req.GiftConfig, locked) != req.PreviewToken {
		return nil, ErrGiftPreviewChanged
	}
	name, platform, err := giftGroup(ctx, tx, req.GroupID)
	if err != nil {
		return nil, err
	}
	expires := now.Add(time.Duration(req.ValidDays) * 24 * time.Hour)
	var batchID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO month_card_gift_batches(request_id,request_hash,actor_user_id,name,group_id,source_group_id,include_frozen,quota_usd,valid_days,recipient_count,created_at,expires_at)
        VALUES($1,$2,$3,$4,$5,NULLIF($6,0),$7,$8,$9,$10,$11,$12) RETURNING id`, req.RequestID, requestHash, actorID, req.Name, req.GroupID, req.SourceGroupID, req.IncludeFrozen, req.QuotaUSD, req.ValidDays, len(ids), now, expires).Scan(&batchID)
	if err != nil {
		return nil, err
	}
	for _, userID := range ids {
		priority, err := materializeOrder(ctx, tx, userID, req.GroupID, now)
		if err != nil {
			return nil, err
		}
		code, err := newCode("G")
		if err != nil {
			return nil, err
		}
		var cardID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO month_card_cards(code,user_id,group_id,product_name,group_name,platform,total_quota_usd,weekly_quota_usd,total_quota_manual,starts_at,expires_at,gift_batch_id)
            VALUES($1,$2,$3,$4,$5,$6,$7,$7,TRUE,$8,$9,$10) RETURNING id`, code, userID, req.GroupID, req.Name, name, platform, req.QuotaUSD, now, expires, batchID).Scan(&cardID)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO month_card_priorities(user_id,group_id,kind,reference_id,priority) VALUES($1,$2,'card',$3,$4)`, userID, req.GroupID, cardID, priority); err != nil {
			return nil, err
		}
	}
	b, err := scanGiftBatch(tx.QueryRowContext(ctx, giftBatchSelect+` WHERE b.id=$1`, batchID))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) ListGiftBatches(ctx context.Context, page int) ([]GiftBatch, int64, error) {
	if page < 1 || page > 1000000 {
		return nil, 0, ErrInvalid
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM month_card_gift_batches`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, giftBatchSelect+` ORDER BY b.id DESC LIMIT 20 OFFSET $1`, (page-1)*20)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	batches := []GiftBatch{}
	for rows.Next() {
		b, err := scanGiftBatch(rows)
		if err != nil {
			return nil, 0, err
		}
		batches = append(batches, *b)
	}
	return batches, total, rows.Err()
}
