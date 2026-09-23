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

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RuleDraft struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Active  bool   `json:"active"`
}
type PublishedRule struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Content string     `json:"content"`
	Version string     `json:"version"`
	ReadAt  *time.Time `json:"read_at,omitempty"`
}
type AdminRules struct {
	DraftRevision      int64           `json:"draft_revision"`
	Publication        int64           `json:"publication"`
	Documents          []RuleDraft     `json:"documents"`
	PublishedDocuments []PublishedRule `json:"published_documents"`
	PublishedAt        time.Time       `json:"published_at"`
}
type UserRules struct {
	Publication int64           `json:"publication"`
	Documents   []PublishedRule `json:"documents"`
}
type RulesConsent struct {
	Publication int64           `json:"publication"`
	UserID      int64           `json:"user_id"`
	AcceptedAt  time.Time       `json:"accepted_at"`
	Documents   []PublishedRule `json:"documents"`
}

var ruleIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

func rulesChanged() error {
	return infraerrors.Conflict("MONTH_CARD_RULES_CHANGED", "购买规则已更新，请重新阅读并确认")
}
func rulesRequired() error {
	return infraerrors.BadRequest("MONTH_CARD_RULES_REQUIRED", "请先阅读全部购买规则并勾选同意")
}
func loadRules(ctx context.Context, db CouponDB, lock string) (*AdminRules, error) {
	rows, err := db.QueryContext(ctx, `SELECT draft_revision,publication,documents,published_documents,published_at FROM month_card_rules WHERE id=TRUE `+lock)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return nil, errors.New("month card rules are unavailable")
	}
	var r AdminRules
	var draft, published []byte
	if err = rows.Scan(&r.DraftRevision, &r.Publication, &draft, &published, &r.PublishedAt); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(draft, &r.Documents); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(published, &r.PublishedDocuments); err != nil {
		return nil, err
	}
	return &r, nil
}
func (s *Store) AdminRules(ctx context.Context) (*AdminRules, error) { return loadRules(ctx, s.db, "") }
func loadReadTimes(ctx context.Context, db CouponDB, userID int64, docs []PublishedRule) error {
	rows, err := db.QueryContext(ctx, `SELECT document_id,version,read_at FROM month_card_rule_reads WHERE user_id=$1`, userID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	read := map[string]time.Time{}
	for rows.Next() {
		var id, version string
		var at time.Time
		if err = rows.Scan(&id, &version, &at); err != nil {
			return err
		}
		read[id+":"+version] = at
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for i := range docs {
		if at, ok := read[docs[i].ID+":"+docs[i].Version]; ok {
			docs[i].ReadAt = &at
		}
	}
	return nil
}
func (s *Store) Rules(ctx context.Context, userID int64) (*UserRules, error) {
	r, err := loadRules(ctx, s.db, "")
	if err != nil {
		return nil, err
	}
	if err = loadReadTimes(ctx, s.db, userID, r.PublishedDocuments); err != nil {
		return nil, err
	}
	return &UserRules{Publication: r.Publication, Documents: r.PublishedDocuments}, nil
}
func (s *Store) ReadRule(ctx context.Context, userID int64, id, version string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := loadRules(ctx, tx, "FOR SHARE")
	if err != nil {
		return err
	}
	found := false
	for _, d := range r.PublishedDocuments {
		if d.ID == id && d.Version == version {
			found = true
			break
		}
	}
	if !found {
		return rulesChanged()
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO month_card_rule_reads(user_id,document_id,version,read_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, userID, id, version, s.now())
	if err != nil {
		return err
	}
	return tx.Commit()
}
func validateRuleDrafts(docs []RuleDraft) error {
	if len(docs) == 0 || len(docs) > 20 {
		return infraerrors.BadRequest("INVALID_RULES", "请保留 1–20 份规则")
	}
	ids := map[string]bool{}
	active := 0
	for i := range docs {
		d := &docs[i]
		d.Title = strings.TrimSpace(d.Title)
		d.Content = strings.TrimSpace(d.Content)
		if !ruleIDPattern.MatchString(d.ID) || ids[d.ID] || d.Title == "" || d.Content == "" || utf8.RuneCountInString(d.Title) > 120 || utf8.RuneCountInString(d.Content) > 50000 {
			return infraerrors.BadRequest("INVALID_RULES", "规则编号、标题或正文无效")
		}
		ids[d.ID] = true
		if d.Active {
			active++
		}
	}
	if active == 0 {
		return infraerrors.BadRequest("INVALID_RULES", "至少保留一份启用的规则")
	}
	return nil
}
func (s *Store) SaveRules(ctx context.Context, revision int64, docs []RuleDraft) (*AdminRules, error) {
	if err := validateRuleDrafts(docs); err != nil {
		return nil, err
	}
	data, err := json.Marshal(docs)
	if err != nil {
		return nil, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE month_card_rules SET documents=$1,draft_revision=draft_revision+1 WHERE id=TRUE AND draft_revision=$2`, string(data), revision)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, infraerrors.Conflict("RULES_DRAFT_CHANGED", "规则草稿已被修改，请刷新后重试")
	}
	return s.AdminRules(ctx)
}
func publishDocuments(docs []RuleDraft, previous []PublishedRule, publication int64) []PublishedRule {
	result := make([]PublishedRule, 0, len(docs))
	for _, d := range docs {
		if !d.Active {
			continue
		}
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%s", publication, d.ID, d.Title, d.Content)))
		version := hex.EncodeToString(sum[:])
		for _, old := range previous {
			if old.ID == d.ID && old.Title == d.Title && old.Content == d.Content {
				version = old.Version
				break
			}
		}
		result = append(result, PublishedRule{ID: d.ID, Title: d.Title, Content: d.Content, Version: version})
	}
	return result
}
func (s *Store) PublishRules(ctx context.Context, revision int64) (*AdminRules, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := loadRules(ctx, tx, "FOR UPDATE")
	if err != nil {
		return nil, err
	}
	if revision != r.DraftRevision {
		return nil, infraerrors.Conflict("RULES_DRAFT_CHANGED", "规则草稿已被修改，请刷新后重试")
	}
	if err = validateRuleDrafts(r.Documents); err != nil {
		return nil, err
	}
	docs := publishDocuments(r.Documents, r.PublishedDocuments, r.Publication+1)
	data, err := json.Marshal(docs)
	if err != nil {
		return nil, err
	}
	old, _ := json.Marshal(r.PublishedDocuments)
	if string(old) == string(data) {
		return r, nil
	}
	r.Publication++
	r.DraftRevision++
	r.PublishedDocuments = docs
	r.PublishedAt = s.now()
	_, err = tx.ExecContext(ctx, `INSERT INTO month_card_rule_publications(publication,documents,published_at) VALUES($1,$2,$3)`, r.Publication, string(data), r.PublishedAt)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE month_card_rules SET publication=$1,published_documents=$2,published_at=$3,draft_revision=$4 WHERE id=TRUE`, r.Publication, string(data), r.PublishedAt, r.DraftRevision)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}

// ValidateRulesConsent rechecks the current publication and account read receipts.
// With lock=true, call on the order transaction: publishing waits until the
// order and its complete consent snapshot have committed together.
func ValidateRulesConsent(ctx context.Context, db CouponDB, userID, publication int64, accepted, lock bool) (*RulesConsent, error) {
	if !accepted || publication <= 0 {
		return nil, rulesRequired()
	}
	clause := ""
	if lock {
		clause = "FOR SHARE"
	}
	r, err := loadRules(ctx, db, clause)
	if err != nil {
		return nil, fmt.Errorf("load purchase rules: %w", err)
	}
	if r.Publication != publication {
		return nil, rulesChanged()
	}
	if len(r.PublishedDocuments) == 0 {
		return nil, rulesRequired()
	}
	if err = loadReadTimes(ctx, db, userID, r.PublishedDocuments); err != nil {
		return nil, err
	}
	for _, d := range r.PublishedDocuments {
		if d.ReadAt == nil {
			return nil, rulesRequired()
		}
	}
	return &RulesConsent{Publication: r.Publication, UserID: userID, AcceptedAt: time.Now(), Documents: r.PublishedDocuments}, nil
}
func (s *Store) ValidateRulesConsent(ctx context.Context, userID, publication int64, accepted bool) error {
	_, err := ValidateRulesConsent(ctx, s.db, userID, publication, accepted, false)
	return err
}

// Keep SQL transaction support explicit alongside Ent's raw-query adapter.
var _ CouponDB = (*sql.Tx)(nil)
