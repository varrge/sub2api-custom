package campaign

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type Treasure struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Clue     string `json:"clue"`
	Revealed bool   `json:"revealed"`
	Found    bool   `json:"found"`
	Line     string `json:"line"`
}
type TreasureChallenge struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

var treasureLocations = []struct {
	id, path, name, clue, line string
	day                        int
}{
	{"welcome", "/dashboard", "启航鲸", "旅程开始的地方，今天的航行一眼可见。", "今天也要元气满满地出航呀！", 0},
	{"key", "/keys", "钥匙鲸", "打开新世界，需要一把属于你的钥匙。", "钥匙保管好，好运我来保管！", 0},
	{"footprints", "/usage", "足迹鲸", "每一次探索，都会留下小小的足迹。", "被你发现了！我正在数今天的浪花呢。", 1},
	{"friends", "/group-buy", "伙伴鲸", "一个人是散步，凑齐伙伴就是派对。", "人多热闹，自己组队也很酷！", 2},
	{"home", "/profile", "归航鲸", "航行很远以后，别忘了看看自己的模样。", "集齐好运，也别忘了照顾自己哦！", 3},
}

func readTreasures(ctx context.Context, q queryer, cfg Config, userID int64, now time.Time) ([]Treasure, error) {
	rows, err := q.QueryContext(ctx, `SELECT treasure_id FROM activity_campaign_treasures WHERE campaign_id=$1 AND user_id=$2 AND found_at IS NOT NULL`, WaveThree, userID)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		found[id] = true
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	result := make([]Treasure, 0, len(treasureLocations))
	for _, loc := range treasureLocations {
		t := Treasure{ID: loc.id, Name: loc.name, Found: found[loc.id]}
		t.Revealed = t.Found || !now.Before(cfg.StartsAt.Add(time.Duration(loc.day)*24*time.Hour))
		if t.Revealed {
			t.Clue = loc.clue
		}
		if t.Found {
			t.Line = loc.line
		}
		result = append(result, t)
	}
	return result, nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Visiting an allowed route issues an expiring, account-bound challenge.
// Browsing is not a proof of human attention; this blocks forged/replayed IDs,
// not automation by an authenticated user who visits every allowed route.
func (s *Store) VisitTreasure(ctx context.Context, userID int64, path string) (*TreasureChallenge, error) {
	var challenge *TreasureChallenge
	for _, loc := range treasureLocations {
		if path == loc.path {
			challenge = &TreasureChallenge{ID: loc.id, Name: loc.name}
			break
		}
	}
	if challenge == nil {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	global, err := readEnabled(ctx, tx, "FOR SHARE")
	if err != nil {
		return nil, err
	}
	cfg, err := readConfig(ctx, tx, WaveThree, "FOR SHARE")
	if err != nil {
		return nil, err
	}
	if err = checkUser(ctx, tx, userID, "FOR UPDATE"); err != nil {
		return nil, err
	}
	now := s.now()
	if cfg.Status(global, now) != "active" || s.connectionID == "" || cfg.BoxCampaignID == "" {
		return nil, nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_campaign_participants(campaign_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, WaveThree, userID)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return nil, err
	}
	challenge.Token = hex.EncodeToString(b)
	result, err := tx.ExecContext(ctx, `INSERT INTO activity_campaign_treasures(campaign_id,user_id,treasure_id,token_hash,token_expires_at)
		VALUES($1,$2,$3,$4,$5) ON CONFLICT(campaign_id,user_id,treasure_id)
		DO UPDATE SET token_hash=EXCLUDED.token_hash,token_expires_at=EXCLUDED.token_expires_at
		WHERE activity_campaign_treasures.found_at IS NULL`, WaveThree, userID, challenge.ID, tokenHash(challenge.Token), now.Add(10*time.Minute))
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	return challenge, nil
}

func (s *Store) FindTreasure(ctx context.Context, userID int64, id, token string) error {
	valid := false
	for _, loc := range treasureLocations {
		valid = valid || id == loc.id
	}
	if !valid || len(token) != 64 {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	global, err := readEnabled(ctx, tx, "FOR SHARE")
	if err != nil {
		return err
	}
	cfg, err := readConfig(ctx, tx, WaveThree, "FOR SHARE")
	if err != nil {
		return err
	}
	if err = checkUser(ctx, tx, userID, "FOR UPDATE"); err != nil {
		return err
	}
	now := s.now()
	if cfg.Status(global, now) != "active" {
		return ErrClosed
	}
	if s.connectionID == "" || cfg.BoxCampaignID == "" {
		return ErrNotReady
	}
	var hash string
	var expires time.Time
	var found sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT token_hash,token_expires_at,found_at FROM activity_campaign_treasures
		WHERE campaign_id=$1 AND user_id=$2 AND treasure_id=$3`, WaveThree, userID, id).Scan(&hash, &expires, &found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalid
	}
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(hash), []byte(tokenHash(token))) != 1 {
		return ErrInvalid
	}
	if found.Valid {
		return tx.Commit()
	}
	if !now.Before(expires) {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_campaign_treasures SET found_at=$4 WHERE campaign_id=$1 AND user_id=$2 AND treasure_id=$3`, WaveThree, userID, id, now); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM activity_campaign_treasures WHERE campaign_id=$1 AND user_id=$2 AND found_at IS NOT NULL`, WaveThree, userID).Scan(&count); err != nil {
		return err
	}
	for task, threshold := range map[string]int{"treasure_three": 3, "treasure_five": 5} {
		if count >= threshold {
			if err = insertReward(ctx, tx, cfg, userID, task, "low", 1, s.connectionID, now, nil); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
