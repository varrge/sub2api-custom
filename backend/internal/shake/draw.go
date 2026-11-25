package shake

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// ConnectionID is echoed in user responses for compatibility with the page's
// response validation; there is no remote connection any more.
const ConnectionID = "native"

// The user-facing JSON keeps the camelCase shape the existing /activities/box
// page already validates.

type UserCampaign struct {
	ID                    string    `json:"id"`
	ConnectionID          string    `json:"connectionId"`
	Name                  string    `json:"name"`
	Status                string    `json:"status"`
	StartAt               time.Time `json:"startAt"`
	EndAt                 time.Time `json:"endAt"`
	ActiveConfigVersionID string    `json:"activeConfigVersionId"`
}

type UserPrize struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Type          string             `json:"type"`
	Amount        *float64           `json:"amount"`
	Weight        float64            `json:"weight"`
	Probability   float64            `json:"probability"`
	Weights       map[string]float64 `json:"weights"`
	Probabilities map[string]float64 `json:"probabilities"`
	Rarity        string             `json:"rarity"`
	DisplayText   string             `json:"displayText"`
	Icon          string             `json:"icon"`
	SortOrder     int                `json:"sortOrder"`
}

type DrawPrize struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Amount *float64 `json:"amount"`
	Rarity string   `json:"rarity"`
}

type Draw struct {
	ID                string     `json:"id"`
	RequestID         string     `json:"requestId"`
	CampaignID        string     `json:"campaignId"`
	ConnectionID      string     `json:"connectionId"`
	UserID            string     `json:"userId"`
	Email             string     `json:"email"`
	CardTier          string     `json:"cardTier"`
	Status            string     `json:"status"`
	Prize             DrawPrize  `json:"prize"`
	ErrorMessage      string     `json:"errorMessage"`
	DispositionReason string     `json:"dispositionReason"`
	DispositionBy     string     `json:"dispositionBy"`
	CreatedAt         time.Time  `json:"createdAt"`
	DeliveredAt       *time.Time `json:"deliveredAt"`
}

// Progress explains each rule to the user.
type Progress struct {
	Source      string  `json:"source"`
	OrderType   string  `json:"orderType,omitempty"`
	Billing     string  `json:"billing,omitempty"`
	Mode        string  `json:"mode"`
	GroupID     *int64  `json:"groupId,omitempty"`
	GroupName   string  `json:"groupName,omitempty"`
	CardTier    string  `json:"cardTier"`
	Quantity    int     `json:"quantity"`
	Threshold   float64 `json:"threshold,omitempty"`
	Amount      float64 `json:"amount"`
	Remaining   float64 `json:"remaining"`
	CardsEarned int     `json:"cardsEarned"`
}

type Bootstrap struct {
	Campaign             *UserCampaign  `json:"campaign"`
	AvailableCards       int            `json:"availableCards"`
	AvailableCardsByTier map[string]int `json:"availableCardsByTier"`
	Progress             []Progress     `json:"progress"`
	Prizes               []UserPrize    `json:"prizes"`
	Draws                []Draw         `json:"draws"`
}

type DrawResult struct {
	Draw                 Draw           `json:"draw"`
	AvailableCards       int            `json:"availableCards"`
	AvailableCardsByTier map[string]int `json:"availableCardsByTier"`
	// BalanceCredited is internal: the handler invalidates caches when > 0.
	BalanceCredited bool `json:"-"`
}

type Recovery struct {
	Found                bool           `json:"found"`
	Draw                 *Draw          `json:"draw"`
	AvailableCards       *int           `json:"availableCards,omitempty"`
	AvailableCardsByTier map[string]int `json:"availableCardsByTier,omitempty"`
	Resolution           string         `json:"resolution,omitempty"`
	RequestID            string         `json:"requestId,omitempty"`
	CampaignID           string         `json:"campaignId,omitempty"`
	CardTier             string         `json:"cardTier,omitempty"`
	UserID               string         `json:"userId,omitempty"`
	ConnectionIDField    string         `json:"connectionId,omitempty"`
	BalanceCredited      bool           `json:"-"`
}

func round8(v float64) float64 { return math.Round(v*1e8) / 1e8 }

func userPrizes(prizes []Prize) []UserPrize {
	var totals Weights
	for _, p := range prizes {
		totals.Low += p.Weights.Low
		totals.Medium += p.Weights.Medium
		totals.High += p.Weights.High
	}
	pct := func(w, t float64) float64 {
		if t <= 0 {
			return 0
		}
		return round8(w / t * 100)
	}
	out := make([]UserPrize, 0, len(prizes))
	for _, p := range prizes {
		probs := map[string]float64{"low": pct(p.Weights.Low, totals.Low), "medium": pct(p.Weights.Medium, totals.Medium), "high": pct(p.Weights.High, totals.High)}
		out = append(out, UserPrize{ID: p.ID, Name: p.Name, Type: p.Type, Amount: p.Amount,
			Weight: p.Weights.Low, Probability: probs["low"],
			Weights:       map[string]float64{"low": p.Weights.Low, "medium": p.Weights.Medium, "high": p.Weights.High},
			Probabilities: probs, Rarity: p.Rarity, DisplayText: p.DisplayText, Icon: p.Icon, SortOrder: p.SortOrder})
	}
	return out
}

func toUserCampaign(c *Campaign) *UserCampaign {
	return &UserCampaign{ID: c.ID, ConnectionID: ConnectionID, Name: c.Config.Name, Status: c.Status,
		StartAt: c.Config.StartsAt, EndAt: c.Config.EndsAt, ActiveConfigVersionID: strconv.Itoa(c.ConfigVersion)}
}

type rowsQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func countCards(ctx context.Context, q rowsQueryer, campaignID string, userID int64) (int, map[string]int, error) {
	counts := map[string]int{"low": 0, "medium": 0, "high": 0}
	rows, err := q.QueryContext(ctx, `SELECT card_tier,count(*) FROM shake_cards WHERE campaign_id=$1 AND user_id=$2 AND status='available' GROUP BY card_tier`, campaignID, userID)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = rows.Close() }()
	total := 0
	for rows.Next() {
		var tier string
		var n int
		if err := rows.Scan(&tier, &n); err != nil {
			return 0, nil, err
		}
		counts[tier] = n
		total += n
	}
	return total, counts, rows.Err()
}

const drawColumns = `d.id,d.request_id,d.campaign_id,d.user_id,COALESCE(u.email,''),d.card_tier,d.status,d.prize_id,d.prize_name,d.prize_type,d.prize_amount::float8,d.prize_rarity,d.created_at`

func scanDraw(row interface{ Scan(...any) error }) (Draw, error) {
	var d Draw
	var id, userID int64
	var amount sql.NullFloat64
	if err := row.Scan(&id, &d.RequestID, &d.CampaignID, &userID, &d.Email, &d.CardTier, &d.Status,
		&d.Prize.ID, &d.Prize.Name, &d.Prize.Type, &amount, &d.Prize.Rarity, &d.CreatedAt); err != nil {
		return d, err
	}
	d.ID = strconv.FormatInt(id, 10)
	d.UserID = strconv.FormatInt(userID, 10)
	d.ConnectionID = ConnectionID
	if amount.Valid {
		v := amount.Float64
		d.Prize.Amount = &v
	}
	if d.Status == "delivered" {
		t := d.CreatedAt
		d.DeliveredAt = &t
	}
	return d, nil
}

func listDraws(ctx context.Context, q rowsQueryer, where string, limit int, args ...any) ([]Draw, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+drawColumns+` FROM shake_draws d JOIN users u ON u.id=d.user_id WHERE `+where+
		fmt.Sprintf(` ORDER BY d.id DESC LIMIT %d`, limit), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Draw{}
	for rows.Next() {
		d, err := scanDraw(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Bootstrap builds the user page. It first issues the user's pending cards so
// a purchase or usage just made shows up without waiting for the worker.
func (s *Store) Bootstrap(ctx context.Context, userID int64) (*Bootstrap, error) {
	b := &Bootstrap{AvailableCardsByTier: map[string]int{"low": 0, "medium": 0, "high": 0}, Progress: []Progress{}, Prizes: []UserPrize{}, Draws: []Draw{}}
	c, err := s.CurrentCampaign(ctx)
	if err != nil || c == nil {
		return b, err
	}
	if c.Status == "active" {
		if _, err := s.SyncCampaign(ctx, c.ID, userID); err != nil {
			return nil, err
		}
	}
	b.Campaign = toUserCampaign(c)
	b.Prizes = userPrizes(c.Config.Prizes)
	if b.AvailableCards, b.AvailableCardsByTier, err = countCards(ctx, s.db, c.ID, userID); err != nil {
		return nil, err
	}
	if b.Draws, err = listDraws(ctx, s.db, `d.campaign_id=$1 AND d.user_id=$2`, 50, c.ID, userID); err != nil {
		return nil, err
	}
	b.Progress, err = s.progress(ctx, c, userID)
	return b, err
}

func (s *Store) progress(ctx context.Context, c *Campaign, userID int64) ([]Progress, error) {
	out := []Progress{}
	for _, r := range c.Config.Rules {
		p := Progress{Source: r.Source, OrderType: r.OrderType, Billing: r.Billing, Mode: r.Mode, GroupID: r.GroupID,
			CardTier: r.CardTier, Quantity: r.Quantity, Threshold: r.Threshold}
		if r.GroupID != nil {
			_ = s.db.QueryRowContext(ctx, `SELECT name FROM groups WHERE id=$1`, *r.GroupID).Scan(&p.GroupName)
		}
		if r.Mode == "per_order" {
			if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(cards),0) FROM shake_order_credits WHERE campaign_id=$1 AND user_id=$2 AND rule_key=$3`,
				c.ID, userID, r.Key()).Scan(&p.CardsEarned); err != nil {
				return nil, err
			}
			out = append(out, p)
			continue
		}
		var raw string
		var units int
		err := s.db.QueryRowContext(ctx, `SELECT amount::text,units FROM shake_progress WHERE campaign_id=$1 AND user_id=$2 AND rule_key=$3`, c.ID, userID, r.Key()).Scan(&raw, &units)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		amount, _ := decimal.NewFromString(raw)
		threshold := decimal.NewFromFloat(r.Threshold)
		if threshold.IsPositive() {
			// Show progress toward the next card after the units already paid.
			rem := amount.Sub(threshold.Mul(decimal.NewFromInt(int64(units))))
			if rem.IsNegative() {
				rem = decimal.Zero
			}
			if rem.GreaterThan(threshold) {
				rem = threshold
			}
			p.Amount, _ = rem.Round(6).Float64()
			p.Remaining, _ = threshold.Sub(rem).Round(6).Float64()
		}
		p.CardsEarned = units * r.Quantity
		out = append(out, p)
	}
	return out, nil
}

// pick draws a prize with a cryptographically random point in the weight total.
func pick(prizes []Prize, tier string) (Prize, error) {
	total := 0.0
	for _, p := range prizes {
		total += p.Weights.For(tier)
	}
	if !(total > 0) {
		return Prize{}, fmt.Errorf("%w: 奖池对该卡种没有可抽奖品", ErrConflict)
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Prize{}, err
	}
	point := float64(binary.BigEndian.Uint64(b[:])>>11) / float64(1<<53) * total
	for _, p := range prizes {
		point -= p.Weights.For(tier)
		if point < 0 && p.Weights.For(tier) > 0 {
			return p, nil
		}
	}
	for i := len(prizes) - 1; i >= 0; i-- {
		if prizes[i].Weights.For(tier) > 0 {
			return prizes[i], nil
		}
	}
	return Prize{}, ErrConflict
}

func validRequestID(id string) bool {
	if len(id) < 8 || len(id) > 100 {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == ':' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func (s *Store) drawByRequest(ctx context.Context, q rowsQueryer, userID int64, requestID string) (*Draw, error) {
	d, err := scanDraw(q.QueryRowContext(ctx, `SELECT `+drawColumns+` FROM shake_draws d JOIN users u ON u.id=d.user_id WHERE d.user_id=$1 AND d.request_id=$2`, userID, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Draw consumes one card and delivers its prize in a single transaction: the
// result is either fully recorded (card consumed, balance credited, extra card
// issued) or nothing happened. Replays with the same requestID return the
// recorded draw and never consume another card.
func (s *Store) Draw(ctx context.Context, userID int64, requestID, campaignID, tier string) (*DrawResult, error) {
	if !validRequestID(requestID) || !validTier(tier) || campaignID == "" {
		return nil, ErrInvalid
	}
	var result *DrawResult
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Serialize this user's draws; the unique (user_id, request_id) is the final guard.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('shake_draw'), hashtext($1))`, strconv.FormatInt(userID, 10)); err != nil {
			return err
		}
		if prior, err := s.drawByRequest(ctx, tx, userID, requestID); err != nil || prior != nil {
			if err != nil {
				return err
			}
			if prior.CampaignID != campaignID || prior.CardTier != tier {
				return fmt.Errorf("%w: 原抽奖请求与活动或卡种不匹配", ErrConflict)
			}
			result = &DrawResult{Draw: *prior}
			return nil
		}
		c, err := s.lockOpenCampaign(ctx, tx, campaignID)
		if err != nil {
			return err
		}
		// Lock the user row for update now: a balance prize updates it later in
		// this transaction, and upgrading a share lock could deadlock.
		if err := checkPersonalLock(ctx, tx, userID, "FOR NO KEY UPDATE"); err != nil {
			return err
		}
		var cardID int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM shake_cards WHERE campaign_id=$1 AND user_id=$2 AND card_tier=$3 AND status='available'
			ORDER BY granted_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, c.ID, userID, tier).Scan(&cardID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoCard
		}
		if err != nil {
			return err
		}
		prize, err := pick(c.Config.Prizes, tier)
		if err != nil {
			return err
		}
		var amount *string
		if prize.Type == "balance" && prize.Amount != nil {
			value := decimal.NewFromFloat(*prize.Amount).Round(8).String()
			amount = &value
		}
		var drawID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO shake_draws(request_id,campaign_id,config_version,user_id,card_tier,prize_id,prize_name,prize_type,prize_amount,prize_rarity)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			requestID, c.ID, c.ConfigVersion, userID, tier, prize.ID, prize.Name, prize.Type, amount, prize.Rarity).Scan(&drawID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE shake_cards SET status='consumed',consumed_at=NOW(),draw_id=$2 WHERE id=$1 AND status='available'`, cardID, drawID); err != nil {
			return err
		}
		credited := false
		switch prize.Type {
		case "balance":
			if err := creditBalance(ctx, tx, userID, *amount, fmt.Sprintf("摇摇乐奖励：%s / %s / #%d", c.Config.Name, prize.Name, drawID)); err != nil {
				return err
			}
			credited = true
		case "extra_draw":
			if _, err := insertCards(ctx, tx, c.ID, userID, tier, SourceExtraDraw, fmt.Sprintf("draw:%d", drawID), 1); err != nil {
				return err
			}
		}
		d, err := scanDraw(tx.QueryRowContext(ctx, `SELECT `+drawColumns+` FROM shake_draws d JOIN users u ON u.id=d.user_id WHERE d.id=$1`, drawID))
		if err != nil {
			return err
		}
		result = &DrawResult{Draw: d, BalanceCredited: credited}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.AvailableCards, result.AvailableCardsByTier, err = countCards(ctx, s.db, result.Draw.CampaignID, userID); err != nil {
		return nil, err
	}
	return result, nil
}

// creditBalance adds a prize to the wallet and writes a balance-history row
// that is not counted as a recharge.
func creditBalance(ctx context.Context, tx *sql.Tx, userID int64, amount, notes string) error {
	res, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2`, amount, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrNotFound
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at,notes,created_at)
		VALUES($1,$2,$3,'used',$4,NOW(),$5,NOW())`, randomHex(16), RedeemTypeShakeBalance, amount, userID, notes)
	return err
}

// Recover looks up a request without ever drawing. A request that never
// reached the server is reported as closed only after its campaign ended.
func (s *Store) Recover(ctx context.Context, userID int64, requestID, campaignID, tier string) (*Recovery, error) {
	if !validRequestID(requestID) || !validTier(tier) || campaignID == "" {
		return nil, ErrInvalid
	}
	var result *Recovery
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Drain in-flight draws before declaring a request absent after expiry.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('shake_draw'), hashtext($1))`, strconv.FormatInt(userID, 10)); err != nil {
			return err
		}
		var err error
		result, err = s.recoverRequest(ctx, tx, userID, requestID, campaignID, tier)
		return err
	})
	return result, err
}

func (s *Store) recoverRequest(ctx context.Context, tx *sql.Tx, userID int64, requestID, campaignID, tier string) (*Recovery, error) {
	d, err := s.drawByRequest(ctx, tx, userID, requestID)
	if err != nil {
		return nil, err
	}
	if d != nil {
		if d.CampaignID != campaignID || d.CardTier != tier {
			return nil, fmt.Errorf("%w: 原抽奖请求与活动或卡种不匹配", ErrConflict)
		}
		total, byTier, err := countCards(ctx, tx, d.CampaignID, userID)
		if err != nil {
			return nil, err
		}
		return &Recovery{Found: true, Draw: d, AvailableCards: &total, AvailableCardsByTier: byTier}, nil
	}
	c, err := s.getCampaign(ctx, tx, campaignID, "FOR SHARE")
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	// Absence is proven once no draw can ever be created for the request: the
	// campaign ended, or it does not exist here at all (e.g. a request left in
	// the browser from the former KaWang service), since Draw requires an
	// existing running campaign.
	if c == nil || c.Status == "ended" {
		return &Recovery{Found: false, Resolution: "closed_without_draw", RequestID: requestID, CampaignID: campaignID,
			CardTier: tier, UserID: strconv.FormatInt(userID, 10), ConnectionIDField: ConnectionID}, nil
	}
	return &Recovery{Found: false}, nil
}

// Admin views.

type CardTotals struct {
	Tier      string `json:"tier"`
	Available int    `json:"available"`
	Consumed  int    `json:"consumed"`
	Expired   int    `json:"expired"`
}

type CampaignStats struct {
	Cards     []CardTotals `json:"cards"`
	Users     int          `json:"users"`
	Draws     int          `json:"draws"`
	BalanceUS float64      `json:"balance_paid"`
}

func (s *Store) Stats(ctx context.Context, campaignID string) (*CampaignStats, error) {
	st := &CampaignStats{Cards: []CardTotals{}}
	rows, err := s.db.QueryContext(ctx, `SELECT card_tier,
		count(*) FILTER (WHERE status='available'),count(*) FILTER (WHERE status='consumed'),count(*) FILTER (WHERE status='expired')
		FROM shake_cards WHERE campaign_id=$1 GROUP BY card_tier ORDER BY card_tier`, campaignID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t CardTotals
		if err := rows.Scan(&t.Tier, &t.Available, &t.Consumed, &t.Expired); err != nil {
			_ = rows.Close()
			return nil, err
		}
		st.Cards = append(st.Cards, t)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT (SELECT count(DISTINCT user_id) FROM shake_cards WHERE campaign_id=$1),
		count(*),COALESCE(SUM(prize_amount) FILTER (WHERE prize_type='balance' AND status='delivered'),0)::float8
		FROM shake_draws WHERE campaign_id=$1`, campaignID).Scan(&st.Users, &st.Draws, &st.BalanceUS)
	return st, err
}

type DrawFilter struct {
	CampaignID string
	UserID     int64
	Status     string
}

func (s *Store) AdminDraws(ctx context.Context, f DrawFilter) ([]Draw, error) {
	where, args := "TRUE", []any{}
	if f.CampaignID != "" {
		args = append(args, f.CampaignID)
		where += fmt.Sprintf(" AND d.campaign_id=$%d", len(args))
	}
	if f.UserID > 0 {
		args = append(args, f.UserID)
		where += fmt.Sprintf(" AND d.user_id=$%d", len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND d.status=$%d", len(args))
	}
	return listDraws(ctx, s.db, where, 500, args...)
}

// VoidDraw reverses a delivered draw: the balance prize is taken back and an
// extra card it produced is expired if still unused. The consumed card is not
// returned. Used for fraud or mistaken pools.
func (s *Store) VoidDraw(ctx context.Context, drawID int64, reason string, actor int64) (*Draw, bool, error) {
	if l := len([]rune(reason)); l < 2 || l > 500 {
		return nil, false, fmt.Errorf("%w: 请填写2-500字的作废原因", ErrInvalid)
	}
	var out Draw
	credited := false
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		d, err := scanDraw(tx.QueryRowContext(ctx, `SELECT `+drawColumns+` FROM shake_draws d JOIN users u ON u.id=d.user_id WHERE d.id=$1 FOR UPDATE OF d`, drawID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if d.Status != "delivered" {
			return fmt.Errorf("%w: 该记录已作废", ErrConflict)
		}
		userID, _ := strconv.ParseInt(d.UserID, 10, 64)
		if d.Prize.Type == "balance" && d.Prize.Amount != nil {
			amt := decimal.NewFromFloat(*d.Prize.Amount).Round(8).Neg().String()
			if err := creditBalance(ctx, tx, userID, amt, fmt.Sprintf("摇摇乐奖励作废：#%d %s", drawID, reason)); err != nil {
				return err
			}
			credited = true
		}
		if d.Prize.Type == "extra_draw" {
			if _, err := tx.ExecContext(ctx, `UPDATE shake_cards SET status='expired' WHERE source='extra_draw' AND source_ref=$1 AND status='available'`, fmt.Sprintf("draw:%d:1", drawID)); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE shake_draws SET status='voided' WHERE id=$1`, drawID); err != nil {
			return err
		}
		d.Status = "voided"
		d.DeliveredAt = nil
		d.DispositionReason = reason
		out = d
		return audit(ctx, tx, actor, "draw.void", d.CampaignID, userID, map[string]any{"draw_id": drawID, "reason": reason})
	})
	return &out, credited, err
}

type UserCards struct {
	UserID   int64          `json:"user_id"`
	Email    string         `json:"email"`
	ByTier   map[string]int `json:"by_tier"`
	Total    int            `json:"total"`
	Progress []Progress     `json:"progress"`
}

func (s *Store) AdminUser(ctx context.Context, campaignID string, userID int64) (*UserCards, error) {
	c, err := s.Campaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	u := &UserCards{UserID: userID}
	if err := s.db.QueryRowContext(ctx, `SELECT email FROM users WHERE id=$1`, userID).Scan(&u.Email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if u.Total, u.ByTier, err = countCards(ctx, s.db, campaignID, userID); err != nil {
		return nil, err
	}
	u.Progress, err = s.progress(ctx, c, userID)
	return u, err
}
