package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ClearTestedModelLimits removes only unchanged limits observed before testing.
// Both the comparison and deletion execute in one UPDATE; concurrent 429 writes
// cannot be overwritten by an older successful probe.
func (r *accountRepository) ClearTestedModelLimits(ctx context.Context, id int64, observed service.ScheduledModelLimitSnapshot) (bool, error) {
	models, err := json.Marshal(observed.Models)
	if err != nil {
		return false, err
	}
	client := clientFromContext(ctx, r.client)
	result, err := client.ExecContext(ctx, `
 UPDATE accounts SET
  extra = CASE WHEN $2::jsonb <> 'null'::jsonb AND $2::jsonb <> '{}'::jsonb THEN
   jsonb_set(COALESCE(extra,'{}'::jsonb), '{model_rate_limits}', COALESCE((
    SELECT jsonb_object_agg(l.key, l.value)
    FROM jsonb_each(COALESCE(extra->'model_rate_limits','{}'::jsonb)) l
    WHERE NOT COALESCE($2::jsonb->l.key = l.value, false)
   ), '{}'::jsonb)) ELSE extra END,
  rate_limited_at = CASE WHEN $4::timestamptz IS NOT NULL
   AND rate_limited_at IS NOT DISTINCT FROM $3::timestamptz AND rate_limit_reset_at = $4::timestamptz
   THEN NULL ELSE rate_limited_at END,
  rate_limit_reset_at = CASE WHEN $4::timestamptz IS NOT NULL
   AND rate_limited_at IS NOT DISTINCT FROM $3::timestamptz AND rate_limit_reset_at = $4::timestamptz
   THEN NULL ELSE rate_limit_reset_at END,
  updated_at = NOW()
 WHERE id = $1 AND deleted_at IS NULL AND (
  EXISTS (SELECT 1 FROM jsonb_each(COALESCE(extra->'model_rate_limits','{}'::jsonb)) l
   WHERE $2::jsonb->l.key = l.value)
  OR ($4::timestamptz IS NOT NULL AND rate_limited_at IS NOT DISTINCT FROM $3::timestamptz AND rate_limit_reset_at = $4::timestamptz)
 )`, id, string(models), observed.RateLimitedAt, observed.RateLimitResetAt)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
		logger.LegacyPrintf("repository.account", "[SchedulerOutbox] enqueue tested model recovery failed: account=%d err=%v", id, err)
	}
	r.syncSchedulerAccountSnapshot(ctx, id)
	return true, nil
}
