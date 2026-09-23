package monthcard_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/stretchr/testify/require"
)

func TestPurchaseRulesReadPublishAndConsent(t *testing.T) {
	db := billingDB(t)
	b, err := os.ReadFile(filepath.Join("..", "..", "migrations", "247_month_card_purchase_rules.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(b))
	require.NoError(t, err)
	ctx := context.Background()
	store := monthcard.NewStore(db)
	user, err := store.Rules(ctx, 1)
	require.NoError(t, err)
	require.Len(t, user.Documents, 2)
	original := user.Documents
	require.Contains(t, original[1].Content, "月卡套餐不支持退款，订阅后不可退订，若因不可抗力因素无法供应，按比例原支付渠道退回。")
	require.Error(t, store.ValidateRulesConsent(ctx, 1, user.Publication, false))
	require.Error(t, store.ValidateRulesConsent(ctx, 1, user.Publication, true))
	require.NoError(t, store.ReadRule(ctx, 1, original[0].ID, original[0].Version))
	require.Error(t, store.ValidateRulesConsent(ctx, 1, user.Publication, true))
	require.NoError(t, store.ReadRule(ctx, 1, original[1].ID, original[1].Version))
	require.NoError(t, store.ValidateRulesConsent(ctx, 1, user.Publication, true))
	require.Error(t, store.ValidateRulesConsent(ctx, 2, user.Publication, true), "account isolation")
	require.Error(t, store.ValidateRulesConsent(ctx, 1, user.Publication, false), "every order requires explicit consent")
	user, err = monthcard.NewStore(db).Rules(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, user.Documents[0].ReadAt)
	readAt := *user.Documents[0].ReadAt
	require.NoError(t, store.ReadRule(ctx, 1, original[0].ID, original[0].Version))
	user, err = store.Rules(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, readAt, *user.Documents[0].ReadAt)
	admin, err := store.AdminRules(ctx)
	require.NoError(t, err)
	oldDraft := admin.DraftRevision
	admin.Documents[0].Content += "\n新规则"
	saved, err := store.SaveRules(ctx, oldDraft, admin.Documents)
	require.NoError(t, err)
	_, err = store.SaveRules(ctx, oldDraft, admin.Documents)
	require.Error(t, err, "stale draft rejected")
	require.NoError(t, store.ValidateRulesConsent(ctx, 1, user.Publication, true), "draft is not published")
	_, err = store.PublishRules(ctx, oldDraft)
	require.Error(t, err)
	published, err := store.PublishRules(ctx, saved.DraftRevision)
	require.NoError(t, err)
	require.Error(t, store.ValidateRulesConsent(ctx, 1, user.Publication, true), "old checkout must refresh")
	require.Error(t, store.ReadRule(ctx, 1, original[0].ID, original[0].Version), "cannot mark obsolete rules read")
	fresh, err := store.Rules(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, fresh.Documents[0].ReadAt)
	require.NotNil(t, fresh.Documents[1].ReadAt)
	require.NoError(t, store.ReadRule(ctx, 1, fresh.Documents[0].ID, fresh.Documents[0].Version))
	published.Documents[0], published.Documents[1] = published.Documents[1], published.Documents[0]
	saved, err = store.SaveRules(ctx, published.DraftRevision, published.Documents)
	require.NoError(t, err)
	published, err = store.PublishRules(ctx, saved.DraftRevision)
	require.NoError(t, err)
	require.NoError(t, store.ValidateRulesConsent(ctx, 1, published.Publication, true), "sort only does not require rereading")
	published.Documents[1].Content = original[0].Content
	saved, err = store.SaveRules(ctx, published.DraftRevision, published.Documents)
	require.NoError(t, err)
	published, err = store.PublishRules(ctx, saved.DraftRevision)
	require.NoError(t, err)
	require.NotEqual(t, original[0].Version, published.PublishedDocuments[1].Version)
	require.Error(t, store.ValidateRulesConsent(ctx, 1, published.Publication, true), "restoring an old text is a new update")
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_rule_publications`).Scan(&count))
	require.Equal(t, 4, count)
	published.Documents[0].Active = false
	published.Documents[1].Active = false
	_, err = store.SaveRules(ctx, published.DraftRevision, published.Documents)
	require.Error(t, err, "cannot remove all rules")
}

func TestPurchaseRulesPublishWaitsForOrderSnapshot(t *testing.T) {
	db := billingDB(t)
	b, err := os.ReadFile(filepath.Join("..", "..", "migrations", "247_month_card_purchase_rules.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(b))
	require.NoError(t, err)
	ctx := context.Background()
	store := monthcard.NewStore(db)
	rules, err := store.Rules(ctx, 1)
	require.NoError(t, err)
	for _, d := range rules.Documents {
		require.NoError(t, store.ReadRule(ctx, 1, d.ID, d.Version))
	}
	admin, err := store.AdminRules(ctx)
	require.NoError(t, err)
	admin.Documents[0].Title = "新版拼团规则"
	saved, err := store.SaveRules(ctx, admin.DraftRevision, admin.Documents)
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	evidence, err := monthcard.ValidateRulesConsent(ctx, tx, 1, rules.Publication, true, true)
	require.NoError(t, err)
	require.Equal(t, rules.Documents[0].Content, evidence.Documents[0].Content)
	require.NotNil(t, evidence.Documents[0].ReadAt)
	timeout, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	_, err = store.PublishRules(timeout, saved.DraftRevision)
	require.Error(t, err, "publisher cannot overtake an accepted order transaction")
	require.NoError(t, tx.Commit())
	_, err = store.PublishRules(ctx, saved.DraftRevision)
	require.NoError(t, err)
	require.Equal(t, "拼团规则", evidence.Documents[0].Title)
	require.Error(t, store.ValidateRulesConsent(ctx, 1, rules.Publication, true))
}
