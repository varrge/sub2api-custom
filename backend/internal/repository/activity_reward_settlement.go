package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *activityLeaderboardRepository) FreezeDailyRewards(ctx context.Context, campaignID, day string, start, end, now time.Time, policies ...service.ActivityRewardPolicy) (*service.ActivityRewardSnapshot, error) {
	policy := (&service.ActivityLeaderboardConfig{}).RewardPolicy()
	if len(policies) > 0 {
		policy = policies[0]
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var existing service.ActivityRewardSnapshot
	err = tx.QueryRowContext(ctx, `SELECT manifest,sha256 FROM activity_reward_manifests WHERE campaign_id=$1 AND day=$2`, campaignID, day).Scan(&existing.Manifest, &existing.SHA256)
	if err == nil {
		var manifest service.ActivityRewardManifest
		if json.Unmarshal([]byte(existing.Manifest), &manifest) != nil {
			return nil, service.ErrActivitySettlementInvalid
		}
		if manifest.PolicyFingerprint != "" && manifest.PolicyFingerprint != policy.Fingerprint() {
			return nil, service.ErrActivitySettlementInvalid
		}
		if !manifest.StartsAt.Equal(start) || !manifest.EndsAt.Equal(end) {
			return nil, service.ErrActivitySettlementInvalid
		}
		return &existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	// Recovery can still change costs or insert missing usage. Never freeze while
	// any older unresolved billing command could affect this day.
	var pending bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
SELECT 1 FROM month_card_billing_pending p
WHERE p.created_at<$2
 OR (p.command->'MonthCardSnapshot'->>'started_at')::timestamptz<$2
 OR EXISTS(
  SELECT 1 FROM usage_logs ul WHERE ul.request_id=p.request_id AND ul.api_key_id=p.api_key_id
    AND ul.created_at >= $1 AND ul.created_at < $2)
)`, start, end).Scan(&pending)
	if err != nil {
		return nil, err
	}
	if pending {
		return nil, service.ErrActivitySettlementPending
	}
	rows, err := tx.QueryContext(ctx, `
SELECT ul.user_id,COALESCE(u.email,''),SUM(ul.actual_cost)::text,
       SUM(ul.actual_cost)>0
FROM usage_logs ul JOIN users u ON u.id=ul.user_id
WHERE ul.created_at >= $1 AND ul.created_at < $2 AND ul.actual_cost >= 0
  AND u.role='user' AND u.account_type='personal' AND u.deleted_at IS NULL
GROUP BY ul.user_id,u.email
ORDER BY SUM(ul.actual_cost) DESC,
         MAX(ul.created_at) FILTER(WHERE ul.actual_cost>0) ASC NULLS LAST,ul.user_id ASC`, start, end)
	if err != nil {
		return nil, err
	}
	manifest := service.ActivityRewardManifest{
		CampaignID: campaignID, Day: day, StartsAt: start, EndsAt: end,
		SettledAt: now, Rule: "configured-cards-v2", Policy: policy, PolicyFingerprint: policy.Fingerprint(), Entries: []service.ActivityRewardEntry{},
	}
	rank := 0
	for rows.Next() {
		entry := service.ActivityRewardEntry{}
		var positive bool
		if err := rows.Scan(&entry.UserID, &entry.Email, &entry.Amount, &positive); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if positive {
			rank++
			entry.Rank = rank
		}
		entry.CardTier, entry.Quantity = policy.Award(entry.Rank)
		manifest.Entries = append(manifest.Entries, entry)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	snapshot := &service.ActivityRewardSnapshot{Manifest: string(raw), SHA256: hex.EncodeToString(digest[:])}
	// Concurrent transactions either persist one immutable snapshot or return a
	// serialization error for retry; they never overwrite a previously frozen list.
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_reward_manifests(campaign_id,day,starts_at,ends_at,settled_at,manifest,sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		campaignID, day, start, end, now, string(raw), snapshot.SHA256)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return snapshot, nil
}
