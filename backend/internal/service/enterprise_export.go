package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"io"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type MonthlyExportInput struct {
	Month      string
	Kind       string
	GroupID    int64
	APIKeyID   int64
	SnapshotAt time.Time
}

func MonthlyExportBounds(month string) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01", month, timezone.Location())
	if err != nil || start.Format("2006-01") != month {
		return time.Time{}, time.Time{}, infraerrors.BadRequest("INVALID_MONTH", "月份格式应为 YYYY-MM")
	}
	return start, start.AddDate(0, 1, 0), nil
}

// ExportMonthlyArchive reads all three statements in one repeatable-read
// transaction. A timestamp filter alone cannot freeze concurrent late inserts.
func (s *UsageService) ExportMonthlyArchive(ctx context.Context, userID int64, in MonthlyExportInput, w io.Writer) error {
	if s.entClient == nil {
		return fmt.Errorf("export database unavailable")
	}
	if in.APIKeyID != 0 {
		return infraerrors.BadRequest("ARCHIVE_KEY_FILTER", "完整对账包不支持按密钥筛选，请导出单独的消费报表")
	}
	if in.SnapshotAt.IsZero() {
		in.SnapshotAt = time.Now().UTC()
	}
	tx, err := s.entClient.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ctx = dbent.NewTxContext(ctx, tx)
	archive := zip.NewWriter(w)
	for _, kind := range []string{"summary", "details", "conversions"} {
		entry, err := archive.Create("statement-" + in.Month + "-" + kind + ".csv")
		if err != nil {
			return err
		}
		in.Kind = kind
		if err := s.ExportMonthly(ctx, userID, in, entry); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return tx.Commit()
}

// Every row comes from historical billing records. No current group price is
// used. One SQL statement gives a consistent snapshot without pagination gaps.
func (s *UsageService) ExportMonthly(ctx context.Context, userID int64, in MonthlyExportInput, w io.Writer) error {
	client := s.entClient
	if client == nil {
		return fmt.Errorf("export database unavailable")
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.IsEnterprise() {
		return infraerrors.Forbidden("ENTERPRISE_EXPORT_ONLY", "月度对账导出仅供企业账户使用")
	}
	start, end, err := MonthlyExportBounds(in.Month)
	if err != nil {
		return err
	}
	if in.GroupID < 0 || in.APIKeyID < 0 {
		return infraerrors.BadRequest("INVALID_EXPORT_FILTER", "分组和密钥ID必须为正整数")
	}
	if in.SnapshotAt.IsZero() {
		in.SnapshotAt = time.Now().UTC()
	}
	if in.APIKeyID > 0 {
		var own bool
		if err := enterpriseQueryOne(ctx, client, `SELECT EXISTS(SELECT 1 FROM api_keys WHERE id=$1 AND user_id=$2)`, []any{in.APIKeyID, userID}, &own); err != nil {
			return err
		}
		if !own {
			return infraerrors.Forbidden("EXPORT_KEY_NOT_OWNED", "无权导出此密钥")
		}
	}
	args := []any{userID, start, end, in.SnapshotAt, in.GroupID, in.APIKeyID}
	base := `WITH bills AS (
 SELECT l.id,l.created_at,l.request_id,COALESCE(l.group_id,0) group_id,COALESCE(g.name,'') group_name,l.api_key_id,COALESCE(k.name,'') api_key_name,l.model,l.input_tokens,l.output_tokens,l.cache_creation_tokens,l.cache_read_tokens,l.total_cost,l.actual_cost,l.rate_multiplier,l.billing_mode,COALESCE(l.image_count,0) image_count,COALESCE(l.image_size,'') image_size,COALESCE(l.video_count,0) video_count,COALESCE(l.video_resolution,'') video_resolution,COALESCE(l.video_duration_seconds,0) video_duration_seconds,
 CASE WHEN a.total IS NOT NULL THEN a.balance WHEN l.billing_type=0 THEN l.actual_cost ELSE 0 END balance_cost,
 CASE WHEN a.total IS NOT NULL THEN a.total-a.balance WHEN l.billing_type=1 THEN l.actual_cost ELSE 0 END entitlement_cost
 FROM usage_logs l LEFT JOIN groups g ON g.id=l.group_id LEFT JOIN api_keys k ON k.id=l.api_key_id
 LEFT JOIN LATERAL (SELECT SUM(amount_usd) total,COALESCE(SUM(amount_usd) FILTER(WHERE kind='balance'),0) balance FROM month_card_allocations WHERE request_id=l.request_id AND api_key_id=l.api_key_id) a ON TRUE
 WHERE l.user_id=$1 AND l.created_at>=$2 AND l.created_at<$3 AND l.created_at<=$4 AND ($5::bigint=0 OR l.group_id=$5) AND ($6::bigint=0 OR l.api_key_id=$6)
 ) `
	var query string
	var headers []string
	switch in.Kind {
	case "summary":
		query = base + `SELECT group_id,group_name,api_key_id,api_key_name,COUNT(*),SUM(input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens),SUM(total_cost),SUM(actual_cost),SUM(balance_cost),SUM(entitlement_cost),SUM(image_count),SUM(video_count),SUM(video_duration_seconds) FROM bills GROUP BY group_id,group_name,api_key_id,api_key_name ORDER BY group_id,api_key_id`
		headers = []string{"group_id", "group_name", "api_key_id", "api_key_name", "requests", "tokens", "base_cost_usd", "actual_cost_usd", "balance_debit_usd", "entitlement_used_usd", "images", "videos", "video_duration_seconds"}
	case "details":
		query = base + `SELECT id,created_at,request_id,group_id,group_name,api_key_id,api_key_name,model,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,total_cost,rate_multiplier,actual_cost,balance_cost,entitlement_cost,billing_mode,image_count,image_size,video_count,video_resolution,video_duration_seconds FROM bills ORDER BY created_at,id`
		headers = []string{"usage_id", "created_at", "request_id", "group_id", "group_name", "api_key_id", "api_key_name", "model", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "base_cost_usd", "rate_multiplier", "actual_cost_usd", "balance_debit_usd", "entitlement_used_usd", "billing_mode", "image_count", "image_size", "video_count", "video_resolution", "video_duration_seconds"}
	case "conversions":
		// Account conversion credits are not attributable to an API key.
		if in.APIKeyID != 0 {
			return infraerrors.BadRequest("CONVERSION_KEY_FILTER", "账户折算流水不支持按密钥筛选")
		}
		args = args[:5]
		query = `WITH events AS (
 SELECT e.id,e.operation_id,e.created_at,e.account_type,COALESCE(c.card_id,0) card_id,COALESCE(c.order_id,0) order_id,COALESCE(m.group_id,0) group_id,COALESCE(c.remaining_usd,e.remaining_usd) remaining_usd,e.conversion_rate,COALESCE(c.credited_usd,e.credited_usd) credited_usd,e.balance_before,e.balance_after,'conversion' event_kind FROM enterprise_conversions e LEFT JOIN enterprise_conversion_cards c ON c.conversion_id=e.id LEFT JOIN month_card_cards m ON m.id=c.card_id WHERE e.user_id=$1
 UNION ALL
 SELECT e.id,e.operation_id,c.refunded_at,e.account_type,c.card_id,c.order_id,m.group_id,0,e.conversion_rate,-c.credited_usd,NULL,NULL,'refund_reversal' FROM enterprise_conversions e JOIN enterprise_conversion_cards c ON c.conversion_id=e.id JOIN month_card_cards m ON m.id=c.card_id WHERE e.user_id=$1 AND c.refunded_at IS NOT NULL
 ) SELECT * FROM events WHERE created_at>=$2 AND created_at<$3 AND created_at<=$4 AND ($5::bigint=0 OR group_id=$5) ORDER BY created_at,id,card_id`
		headers = []string{"conversion_id", "operation_id", "created_at", "account_type", "card_id", "order_id", "group_id", "remaining_usd", "conversion_rate", "credited_usd", "balance_before", "balance_after", "event_kind"}
	default:
		return infraerrors.BadRequest("INVALID_EXPORT_KIND", "导出类型应为 summary、details 或 conversions")
	}
	if s.entClient == nil {
		return fmt.Errorf("export database unavailable")
	}
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	writer := csv.NewWriter(w)
	headers = append(headers, "statement_month", "timezone", "snapshot_at")
	if _, err = io.WriteString(w, "\xef\xbb\xbf"); err != nil {
		return err
	}
	if err := writer.Write(headers); err != nil {
		return err
	}
	n := len(headers) - 3
	values := make([]any, n)
	dest := make([]any, n)
	for i := range dest {
		dest[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		record := make([]string, 0, len(headers))
		for _, v := range values {
			var value string
			switch x := v.(type) {
			case nil:
				value = ""
			case []byte:
				value = string(x)
			case time.Time:
				value = x.In(timezone.Location()).Format(time.RFC3339Nano)
			case int64:
				value = strconv.FormatInt(x, 10)
			default:
				value = fmt.Sprint(x)
			}
			record = append(record, monthlyCSVCell(value))
		}
		record = append(record, in.Month, timezone.Location().String(), in.SnapshotAt.UTC().Format(time.RFC3339Nano))
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}
func monthlyCSVCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if strings.HasPrefix(trimmed, "-") {
		if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return value
		}
	}
	if trimmed != "" && strings.ContainsAny(trimmed[:1], "=+-@") {
		return "'" + value
	}
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") {
		return "'" + value
	}
	return value
}
