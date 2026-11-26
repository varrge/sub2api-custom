package shake

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// LeaderboardSource is the consumption leaderboard as seen by reward jobs. It
// is implemented in the handler layer over ActivityLeaderboardService, which
// freezes immutable manifests (activity_reward_manifests).
type LeaderboardSource interface {
	Current(ctx context.Context) (*LeaderboardEvent, error)
	Settle(ctx context.Context, campaignID, day, fingerprint string) (manifest, sha string, err error)
}

// ErrSettlementPending means the day is not frozen yet; the job retries.
var ErrSettlementPending = errors.New("shake: leaderboard settlement pending")

type LeaderboardEvent struct {
	Enabled           bool            `json:"enabled"`
	AutoRewards       bool            `json:"auto_rewards"`
	Period            string          `json:"period"`
	CampaignID        string          `json:"campaign_id"`
	StartsAt          time.Time       `json:"starts_at"`
	EndsAt            time.Time       `json:"ends_at"`
	Policy            json.RawMessage `json:"reward_policy"`
	PolicyFingerprint string          `json:"policy_fingerprint"`
}

type rewardPolicy struct {
	Period       string `json:"period"`
	DelayMinutes int    `json:"delay_minutes"`
	Rewards      struct {
		Ranked []struct {
			Rank     int    `json:"rank"`
			CardTier string `json:"card_tier"`
			Quantity int    `json:"quantity"`
		} `json:"ranked"`
		Participation struct {
			Enabled         bool   `json:"enabled"`
			CardTier        string `json:"card_tier"`
			Quantity        int    `json:"quantity"`
			IncludeZeroCost bool   `json:"include_zero_cost"`
		} `json:"participation"`
	} `json:"rewards"`
}

func (p rewardPolicy) award(rank int) (string, int) {
	if rank > 0 {
		for _, r := range p.Rewards.Ranked {
			if r.Rank == rank {
				return r.CardTier, r.Quantity
			}
		}
	}
	r := p.Rewards.Participation
	if r.Enabled && (rank > 0 || r.IncludeZeroCost) {
		return r.CardTier, r.Quantity
	}
	return "", 0
}

type RewardJob struct {
	ID                int64      `json:"id"`
	CampaignID        string     `json:"campaign_id"`
	SourceCampaignID  string     `json:"source_campaign_id"`
	SourceStartsAt    time.Time  `json:"source_starts_at"`
	SourceEndsAt      time.Time  `json:"source_ends_at"`
	Period            string     `json:"period"`
	DelayMinutes      int        `json:"delay_minutes"`
	PolicyFingerprint string     `json:"policy_fingerprint"`
	Enabled           bool       `json:"enabled"`
	LastError         string     `json:"last_error"`
	LastCheckedAt     *time.Time `json:"last_checked_at"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedDays     int        `json:"completed_days"`
	Recipients        int        `json:"recipients"`
	CardsGranted      int        `json:"cards_granted"`
	policy            rewardPolicy
}

const day = 24 * time.Hour

func isMidnight(t time.Time) bool {
	l := t.In(China)
	return l.Hour() == 0 && l.Minute() == 0 && l.Second() == 0 && l.Nanosecond() == 0
}

func validateEvent(ev *LeaderboardEvent) (rewardPolicy, error) {
	var p rewardPolicy
	if ev == nil || !ev.Enabled || !ev.AutoRewards || (ev.Period != "daily" && ev.Period != "event") ||
		!strings.HasPrefix(ev.CampaignID, "activity-") || !ev.EndsAt.After(ev.StartsAt) || ev.EndsAt.Sub(ev.StartsAt) > 366*day {
		return p, fmt.Errorf("%w: 请先在消费榜设置中配置结算周期并开启自动奖励", ErrInvalid)
	}
	if err := json.Unmarshal(ev.Policy, &p); err != nil || p.Period != ev.Period || p.DelayMinutes < 10 || p.DelayMinutes > 1440 {
		return p, fmt.Errorf("%w: 消费榜奖励配置无效", ErrInvalid)
	}
	sum := sha256.Sum256(ev.Policy)
	if hex.EncodeToString(sum[:]) != ev.PolicyFingerprint {
		return p, fmt.Errorf("%w: 消费榜奖励配置校验失败", ErrInvalid)
	}
	if p.Period == "daily" && (!isMidnight(ev.StartsAt) || !isMidnight(ev.EndsAt)) {
		return p, fmt.Errorf("%w: 按天结算要求开始和结束都在北京时间零点", ErrInvalid)
	}
	return p, nil
}

// CreateRewardJob binds the current leaderboard event to a box campaign.
func (s *Store) CreateRewardJob(ctx context.Context, src LeaderboardSource, campaignID string, actor int64) (*RewardJob, error) {
	ev, err := src.Current(ctx)
	if err != nil {
		return nil, err
	}
	p, err := validateEvent(ev)
	if err != nil {
		return nil, err
	}
	var id int64
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		c, err := s.getCampaign(ctx, tx, campaignID, "FOR SHARE")
		if err != nil {
			return err
		}
		required := ev.EndsAt.Add(time.Duration(p.DelayMinutes)*time.Minute + day)
		if c.StoredStatus == "ended" || c.Config.StartsAt.After(ev.StartsAt) || c.Config.EndsAt.Before(required) {
			return fmt.Errorf("%w: 摇摇乐活动须覆盖消费活动，并在最后一次结算后至少保留24小时（需结束于 %s 之后）", ErrConflict, required.In(China).Format("2006-01-02 15:04"))
		}
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('shake_reward_jobs'))`); err != nil {
			return err
		}
		var conflict bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM shake_reward_jobs WHERE source_campaign_id=$1
			OR (source_starts_at<$3 AND source_ends_at>$2))`, ev.CampaignID, ev.StartsAt, ev.EndsAt).Scan(&conflict); err != nil {
			return err
		}
		if conflict {
			return fmt.Errorf("%w: 此消费活动已有发奖任务，或统计日期与已有任务重叠；禁止重复发奖", ErrConflict)
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO shake_reward_jobs(campaign_id,source_campaign_id,source_starts_at,source_ends_at,policy,policy_fingerprint,created_by)
			VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, campaignID, ev.CampaignID, ev.StartsAt, ev.EndsAt, []byte(ev.Policy), ev.PolicyFingerprint, actor).Scan(&id); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "rewards.schedule", campaignID, 0, map[string]any{"job_id": id, "source": ev.CampaignID})
	})
	if err != nil {
		return nil, err
	}
	return s.RewardJob(ctx, id)
}

// RefreshPolicy re-snapshots the leaderboard policy before its event starts.
func (s *Store) RefreshPolicy(ctx context.Context, src LeaderboardSource, jobID, actor int64) (*RewardJob, error) {
	job, err := s.RewardJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if !s.now().Before(job.SourceStartsAt) {
		return nil, fmt.Errorf("%w: 消费活动已开始，奖励配置快照已锁定", ErrConflict)
	}
	ev, err := src.Current(ctx)
	if err != nil {
		return nil, err
	}
	p, err := validateEvent(ev)
	if err != nil {
		return nil, err
	}
	if ev.CampaignID != job.SourceCampaignID {
		return nil, fmt.Errorf("%w: 只能同步同一消费活动（起止时间不能变）的奖励规则", ErrConflict)
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		c, err := s.getCampaign(ctx, tx, job.CampaignID, "FOR SHARE")
		if err != nil {
			return err
		}
		if c.Config.EndsAt.Before(ev.EndsAt.Add(time.Duration(p.DelayMinutes)*time.Minute + day)) {
			return fmt.Errorf("%w: 新的结算延迟超出摇摇乐活动有效期", ErrConflict)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE shake_reward_jobs SET policy=$2,policy_fingerprint=$3,last_error='' WHERE id=$1`, jobID, []byte(ev.Policy), ev.PolicyFingerprint); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "rewards.refresh", job.CampaignID, 0, map[string]any{"job_id": jobID, "from": job.PolicyFingerprint, "to": ev.PolicyFingerprint})
	})
	if err != nil {
		return nil, err
	}
	return s.RewardJob(ctx, jobID)
}

func (s *Store) SetRewardJobEnabled(ctx context.Context, jobID int64, enabled bool, actor int64) (*RewardJob, error) {
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE shake_reward_jobs SET enabled=$2 WHERE id=$1`, jobID, enabled)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return ErrNotFound
		}
		return audit(ctx, tx, actor, "rewards.toggle", "", 0, map[string]any{"job_id": jobID, "enabled": enabled})
	})
	if err != nil {
		return nil, err
	}
	return s.RewardJob(ctx, jobID)
}

const jobColumns = `j.id,j.campaign_id,j.source_campaign_id,j.source_starts_at,j.source_ends_at,j.policy,j.policy_fingerprint,j.enabled,j.last_error,j.last_checked_at,j.created_at,
	(SELECT count(*) FROM shake_reward_days d WHERE d.job_id=j.id),
	(SELECT COALESCE(SUM(recipients),0) FROM shake_reward_days d WHERE d.job_id=j.id),
	(SELECT COALESCE(SUM(cards),0) FROM shake_reward_days d WHERE d.job_id=j.id)`

func scanJob(row interface{ Scan(...any) error }) (*RewardJob, error) {
	var j RewardJob
	var raw []byte
	var checked sql.NullTime
	if err := row.Scan(&j.ID, &j.CampaignID, &j.SourceCampaignID, &j.SourceStartsAt, &j.SourceEndsAt, &raw, &j.PolicyFingerprint,
		&j.Enabled, &j.LastError, &checked, &j.CreatedAt, &j.CompletedDays, &j.Recipients, &j.CardsGranted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &j.policy); err != nil {
		return nil, err
	}
	j.Period, j.DelayMinutes = j.policy.Period, j.policy.DelayMinutes
	if checked.Valid {
		j.LastCheckedAt = &checked.Time
	}
	return &j, nil
}

func (s *Store) RewardJob(ctx context.Context, id int64) (*RewardJob, error) {
	return scanJob(s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM shake_reward_jobs j WHERE j.id=$1`, id))
}

func (s *Store) RewardJobs(ctx context.Context) ([]RewardJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM shake_reward_jobs j ORDER BY j.created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []RewardJob{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

type manifestEntry struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Amount   string `json:"amount"`
	Rank     int    `json:"rank"`
	CardTier string `json:"card_tier"`
	Quantity int    `json:"quantity"`
}

type manifest struct {
	CampaignID        string          `json:"campaign_id"`
	Day               string          `json:"day"`
	StartsAt          time.Time       `json:"starts_at"`
	EndsAt            time.Time       `json:"ends_at"`
	Rule              string          `json:"rule"`
	PolicyFingerprint string          `json:"policy_fingerprint"`
	Entries           []manifestEntry `json:"entries"`
}

// validateManifest re-derives every award from the job's own policy snapshot,
// so a changed leaderboard setting can never alter what this job grants.
func validateManifest(raw, sha string, job *RewardJob, dayName string, start, end time.Time) (*manifest, error) {
	sum := sha256.Sum256([]byte(raw))
	if len(raw) > 32<<20 || hex.EncodeToString(sum[:]) != sha {
		return nil, errors.New("结算清单校验失败")
	}
	var m manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, errors.New("结算清单格式错误")
	}
	if m.CampaignID != job.SourceCampaignID || m.Day != dayName || !m.StartsAt.Equal(start) || !m.EndsAt.Equal(end) || len(m.Entries) > 100000 {
		return nil, errors.New("结算清单与活动不匹配")
	}
	if m.Rule != "configured-cards-v2" || m.PolicyFingerprint != job.PolicyFingerprint {
		return nil, errors.New("结算使用的奖励配置与绑定快照不一致")
	}
	seen := map[int64]bool{}
	rank := 0
	var prev *decimal.Decimal
	zeroSeen := false
	for _, e := range m.Entries {
		if e.UserID <= 0 || seen[e.UserID] {
			return nil, errors.New("清单存在无效或重复用户")
		}
		seen[e.UserID] = true
		amt, err := decimal.NewFromString(e.Amount)
		if err != nil || amt.IsNegative() {
			return nil, errors.New("结算金额格式错误")
		}
		if prev != nil && amt.GreaterThan(*prev) {
			return nil, errors.New("结算排名顺序错误")
		}
		prev = &amt
		if amt.IsPositive() {
			rank++
			if zeroSeen || e.Rank != rank {
				return nil, errors.New("结算排名不连续")
			}
		} else {
			zeroSeen = true
			if e.Rank != 0 {
				return nil, errors.New("零费用调用仅享参与奖")
			}
		}
		tier, qty := job.policy.award(e.Rank)
		if e.CardTier != tier || e.Quantity != qty {
			return nil, errors.New("奖励与活动规则不匹配")
		}
	}
	return &m, nil
}

// RunRewardJobs settles every ended period whose delay has passed and grants
// cards. Each period grants exactly once (shake_reward_days primary key).
func (s *Store) RunRewardJobs(ctx context.Context, src LeaderboardSource, onlyID int64) (int, error) {
	jobs, err := s.RewardJobs(ctx)
	if err != nil {
		return 0, err
	}
	granted := 0
	for i := range jobs {
		job := &jobs[i]
		if !job.Enabled || (onlyID > 0 && job.ID != onlyID) {
			continue
		}
		n, runErr := s.runJob(ctx, src, job)
		granted += n
		msg := ""
		if runErr != nil {
			slog.Warn("shake reward job not complete; will retry", "job_id", job.ID, "error", runErr)
			msg = "结算或发卡未完成：" + runErr.Error() + "；系统会每分钟重试。"
			if len([]rune(msg)) > 300 {
				msg = string([]rune(msg)[:300])
			}
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE shake_reward_jobs SET last_error=$2,last_checked_at=NOW() WHERE id=$1`, job.ID, msg); err != nil {
			return granted, err
		}
	}
	return granted, nil
}

func (s *Store) runJob(ctx context.Context, src LeaderboardSource, job *RewardJob) (int, error) {
	granted := 0
	now := s.now()
	delay := time.Duration(job.DelayMinutes) * time.Minute
	for start := job.SourceStartsAt; start.Before(job.SourceEndsAt); {
		end := start.Add(day)
		if job.Period == "event" {
			end = job.SourceEndsAt
		}
		if end.Add(delay).After(now) {
			break
		}
		dayName := start.In(China).Format("2006-01-02")
		var done bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM shake_reward_days WHERE job_id=$1 AND day=$2)`, job.ID, dayName).Scan(&done); err != nil {
			return granted, err
		}
		if !done {
			raw, sha, err := src.Settle(ctx, job.SourceCampaignID, dayName, job.PolicyFingerprint)
			if err != nil {
				if errors.Is(err, ErrSettlementPending) {
					return granted, errors.New(dayName + " 账单仍在恢复，暂未冻结")
				}
				return granted, err
			}
			m, err := validateManifest(raw, sha, job, dayName, start, end)
			if err != nil {
				return granted, err
			}
			n, err := s.grantDay(ctx, job, dayName, sha, m)
			if err != nil {
				return granted, err
			}
			granted += n
		}
		start = end
	}
	return granted, nil
}

func (s *Store) grantDay(ctx context.Context, job *RewardJob, dayName, sha string, m *manifest) (int, error) {
	cards := 0
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var enabled bool
		if err := tx.QueryRowContext(ctx, `SELECT enabled FROM shake_reward_jobs WHERE id=$1 FOR UPDATE`, job.ID).Scan(&enabled); err != nil {
			return err
		}
		if !enabled {
			return nil
		}
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT sha256 FROM shake_reward_days WHERE job_id=$1 AND day=$2`, job.ID, dayName).Scan(&existing)
		if err == nil {
			if existing != sha {
				return errors.New("结算清单发生变化，已停止发奖")
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := s.lockOpenCampaign(ctx, tx, job.CampaignID); err != nil {
			return errors.New("摇摇乐活动未开启或已结束，未发卡")
		}
		recipients := 0
		for _, e := range m.Entries {
			if e.Quantity == 0 {
				continue
			}
			var ok bool
			// Users deleted or converted to enterprise after the freeze are skipped.
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL AND account_type='personal')`, e.UserID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				continue
			}
			n, err := insertCards(ctx, tx, job.CampaignID, e.UserID, e.CardTier, SourceLeaderboard, fmt.Sprintf("job:%d:%s:%d", job.ID, dayName, e.UserID), e.Quantity)
			if err != nil {
				return err
			}
			recipients++
			cards += n
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO shake_reward_days(job_id,day,sha256,recipients,cards) VALUES($1,$2,$3,$4,$5)`, job.ID, dayName, sha, recipients, cards)
		return err
	})
	return cards, err
}
