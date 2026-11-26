package groupeval

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

const configColumns = `g.id,g.name,g.platform,COALESCE(e.revision,0),COALESCE(e.enabled,false),
 COALESCE(e.candy_enabled,true),COALESCE(e.pelican_enabled,false),COALESCE(e.automatic,false),
 COALESCE(e.interval_seconds,600),COALESCE(e.model,''),COALESCE(e.effort,'low'),
 COALESCE(e.runs,3),COALESCE(e.daily_limit,100),e.last_run_at,e.next_run_at,
 COALESCE(e.leased_until>NOW(),false)`

type scanner interface{ Scan(...any) error }

func scanConfig(row scanner) (Config, error) {
	var c Config
	err := row.Scan(&c.GroupID, &c.GroupName, &c.GroupPlatform, &c.Revision, &c.Enabled, &c.CandyEnabled,
		&c.PelicanEnabled, &c.Automatic, &c.IntervalSeconds, &c.Model, &c.Effort,
		&c.Runs, &c.DailyLimit, &c.LastRunAt, &c.NextRunAt, &c.Running)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return c, err
}

func (s *Store) List(ctx context.Context) ([]Config, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+configColumns+` FROM groups g
 LEFT JOIN group_evaluation_configs e ON e.group_id=g.id
 WHERE g.deleted_at IS NULL AND g.status='active' ORDER BY g.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Config{}
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) Get(ctx context.Context, id int64) (Config, error) {
	return scanConfig(s.db.QueryRowContext(ctx, `SELECT `+configColumns+` FROM groups g
 LEFT JOIN group_evaluation_configs e ON e.group_id=g.id
 WHERE g.id=$1 AND g.deleted_at IS NULL AND g.status='active'`, id))
}

func (s *Store) Save(ctx context.Context, c Config) (Config, error) {
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Config{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var name, platform string
	err = tx.QueryRowContext(ctx, `SELECT name,platform FROM groups WHERE id=$1 AND deleted_at IS NULL AND status='active' FOR UPDATE`, c.GroupID).Scan(&name, &platform)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, ErrNotFound
	}
	if err != nil {
		return Config{}, err
	}
	if c.Enabled && platform != "openai" {
		return Config{}, ErrInvalid
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO group_evaluation_configs(group_id,revision) VALUES($1,0) ON CONFLICT DO NOTHING`, c.GroupID); err != nil {
		return Config{}, err
	}
	var revision int64
	var busy bool
	if err = tx.QueryRowContext(ctx, `SELECT revision,COALESCE(leased_until>NOW(),false) FROM group_evaluation_configs WHERE group_id=$1 FOR UPDATE`, c.GroupID).Scan(&revision, &busy); err != nil {
		return Config{}, err
	}
	if busy {
		return Config{}, ErrBusy
	}
	if revision != c.Revision {
		return Config{}, ErrConflict
	}
	_, err = tx.ExecContext(ctx, `UPDATE group_evaluation_configs SET revision=revision+1,
 enabled=$2,candy_enabled=$3,pelican_enabled=$4,automatic=$5,interval_seconds=$6,
 model=$7,effort=$8,runs=$9,daily_limit=$10,updated_at=NOW(),
 next_run_at=CASE WHEN $2 AND $5 THEN NOW()+make_interval(secs=>$6::integer) ELSE NULL END
 WHERE group_id=$1`, c.GroupID, c.Enabled, c.CandyEnabled, c.PelicanEnabled, c.Automatic,
		c.IntervalSeconds, c.Model, c.Effort, c.Runs, c.DailyLimit)
	if err != nil {
		return Config{}, err
	}
	if err = tx.Commit(); err != nil {
		return Config{}, err
	}
	return s.Get(ctx, c.GroupID)
}

func (s *Store) Records(ctx context.Context, c Config, admin bool) (map[string][]Record, error) {
	result := map[string][]Record{"candy": {}, "pelican": {}}
	for _, kind := range []string{"candy", "pelican"} {
		if !admin && ((kind == "candy" && !c.CandyEnabled) || (kind == "pelican" && !c.PelicanEnabled)) {
			continue
		}
		rows, err := s.db.QueryContext(ctx, `SELECT id,kind,status,model,effort,created_at,answer,html,error,duration_ms,expected_answer,prompt_version
 FROM group_evaluation_records WHERE group_id=$1 AND kind=$2 ORDER BY created_at DESC,id DESC LIMIT $3`, c.GroupID, kind, HistoryLimit)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r Record
			if err = rows.Scan(&r.ID, &r.Kind, &r.Status, &r.Model, &r.Effort, &r.CreatedAt, &r.Answer, &r.HTML, &r.Error, &r.DurationMS, &r.ExpectedAnswer, &r.PromptVersion); err != nil {
				_ = rows.Close()
				return nil, err
			}
			result[kind] = append(result[kind], r)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) Due(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT e.group_id FROM group_evaluation_configs e JOIN groups g ON g.id=e.group_id
 WHERE g.deleted_at IS NULL AND g.status='active' AND e.enabled AND e.automatic AND e.next_run_at<=NOW()
 AND (e.leased_until IS NULL OR e.leased_until<=NOW()) ORDER BY e.next_run_at,e.group_id LIMIT 20`)
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

type Claim struct {
	Config
	Token string
}

// Claim reserves the whole batch up front. A single transaction-scoped lock
// serializes admission across instances; expired worker leases are recoverable.
func (s *Store) Claim(ctx context.Context, id int64, scheduled bool) (Claim, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(276,1)`); err != nil {
		return Claim{}, err
	}
	c, err := scanConfig(tx.QueryRowContext(ctx, `SELECT `+configColumns+` FROM groups g
 JOIN group_evaluation_configs e ON e.group_id=g.id
 WHERE g.id=$1 AND g.deleted_at IS NULL AND g.status='active' FOR UPDATE OF e`, id))
	if err != nil {
		return Claim{}, err
	}
	if !c.Enabled || (scheduled && (!c.Automatic || c.NextRunAt == nil || c.NextRunAt.After(time.Now()))) {
		return Claim{}, ErrDisabled
	}
	if c.Running {
		return Claim{}, ErrBusy
	}
	if err = c.Validate(); err != nil {
		return Claim{}, err
	}
	var active, used int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_evaluation_configs WHERE leased_until>NOW()`).Scan(&active); err != nil {
		return Claim{}, err
	}
	if active >= 2 {
		return Claim{}, ErrBusy
	}
	if err = tx.QueryRowContext(ctx, `SELECT CASE WHEN usage_date=(NOW() AT TIME ZONE 'UTC')::date THEN used_requests ELSE 0 END
 FROM group_evaluation_configs WHERE group_id=$1`, id).Scan(&used); err != nil {
		return Claim{}, err
	}
	if used+c.RequestsPerRun() > c.DailyLimit {
		// Avoid rescanning a capped group every second.
		if _, err = tx.ExecContext(ctx, `UPDATE group_evaluation_configs SET next_run_at=((NOW() AT TIME ZONE 'UTC')::date+1)::timestamp AT TIME ZONE 'UTC' WHERE group_id=$1`, id); err != nil {
			return Claim{}, err
		}
		if err = tx.Commit(); err != nil {
			return Claim{}, err
		}
		return Claim{}, ErrBudget
	}
	token := uuid.NewString()
	_, err = tx.ExecContext(ctx, `UPDATE group_evaluation_configs SET lease_token=$2,leased_until=NOW()+INTERVAL '20 minutes',
 usage_date=(NOW() AT TIME ZONE 'UTC')::date,used_requests=$3 WHERE group_id=$1`, id, token, used+c.RequestsPerRun())
	if err != nil {
		return Claim{}, err
	}
	if err = tx.Commit(); err != nil {
		return Claim{}, err
	}
	return Claim{Config: c, Token: token}, nil
}

func (s *Store) Append(ctx context.Context, claim Claim, r Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var owned int64
	if err = tx.QueryRowContext(ctx, `SELECT group_id FROM group_evaluation_configs
 WHERE group_id=$1 AND lease_token=$2 AND leased_until>NOW() FOR UPDATE`, claim.GroupID, claim.Token).Scan(&owned); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO group_evaluation_records(group_id,kind,status,model,effort,answer,html,error,duration_ms,expected_answer,prompt_version)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, claim.GroupID, r.Kind, r.Status, r.Model, r.Effort, r.Answer, r.HTML, r.Error, r.DurationMS, r.ExpectedAnswer, r.PromptVersion)
	if err != nil {
		return err
	}
	// Evaluation output is untrusted and potentially large; bound stored history
	// as well as public responses, independently for each test kind.
	_, err = tx.ExecContext(ctx, `DELETE FROM group_evaluation_records WHERE group_id=$1 AND kind=$2 AND id NOT IN
 (SELECT id FROM group_evaluation_records WHERE group_id=$1 AND kind=$2 ORDER BY created_at DESC,id DESC LIMIT $3)`, claim.GroupID, r.Kind, HistoryLimit)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Finish(ctx context.Context, claim Claim) error {
	_, err := s.db.ExecContext(ctx, `UPDATE group_evaluation_configs SET lease_token=NULL,leased_until=NULL,last_run_at=NOW(),
 next_run_at=CASE WHEN enabled AND automatic THEN NOW()+make_interval(secs=>interval_seconds) ELSE NULL END
 WHERE group_id=$1 AND lease_token=$2`, claim.GroupID, claim.Token)
	return err
}
