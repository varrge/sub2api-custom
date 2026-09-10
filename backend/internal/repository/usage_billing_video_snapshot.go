package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageBillingRepository) StoreGrokVideoPending(ctx context.Context, requestID string, userID, apiKeyID int64, pending *service.GrokVideoPendingBilling) error {
	raw, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO month_card_video_pending(request_id,user_id,api_key_id,snapshot) VALUES($1,$2,$3,$4) ON CONFLICT(request_id,api_key_id,user_id) DO NOTHING`, requestID, userID, apiKeyID, raw)
	return err
}
func (r *usageBillingRepository) LoadGrokVideoPending(ctx context.Context, requestID string, userID, apiKeyID int64) (*service.GrokVideoPendingBilling, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT snapshot FROM month_card_video_pending WHERE request_id=$1 AND user_id=$2 AND api_key_id=$3`, requestID, userID, apiKeyID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pending service.GrokVideoPendingBilling
	if err := json.Unmarshal(raw, &pending); err != nil {
		return nil, err
	}
	return &pending, nil
}
