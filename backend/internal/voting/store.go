package voting

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func checkAccess(ctx context.Context, tx *sql.Tx, userID int64, lock string) error {
	var enabled string
	err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1 `+lock, FeatureKey).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && enabled != "true") {
		return ErrClosed
	}
	if err != nil {
		return err
	}
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT status='active' AND deleted_at IS NULL FROM users WHERE id=$1 `+lock, userID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
		return ErrIneligible
	}
	return err
}

func readPoll(ctx context.Context, tx *sql.Tx, id int64, lock string) (Poll, error) {
	p := Poll{MyOptionIDs: []int{}, Options: []Option{}}
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT id,title,description,image,status,revision,options,created_at,updated_at FROM voting_polls WHERE id=$1 `+lock, id).
		Scan(&p.ID, &p.Title, &p.Description, &p.Image, &p.Status, &p.Revision, &raw, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	var options []OptionInput
	if err = json.Unmarshal(raw, &options); err != nil {
		return p, err
	}
	for i, o := range options {
		p.Options = append(p.Options, Option{ID: i + 1, Title: o.Title, Image: o.Image})
	}
	return p, nil
}

func (s *Store) List(ctx context.Context, userID int64, admin bool, page, pageSize int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 12
	}
	result := Page{Items: []Summary{}, Page: page, PageSize: pageSize}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	if !admin {
		if err = checkAccess(ctx, tx, userID, ""); err != nil {
			return result, err
		}
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM voting_polls WHERE ($1 OR status<>'draft')`, admin).Scan(&result.Total); err != nil {
		return result, err
	}
	result.Pages = (result.Total + pageSize - 1) / pageSize
	rows, err := tx.QueryContext(ctx, `SELECT p.id,p.title,p.description,p.status,p.revision,p.created_at,p.updated_at,
	 (SELECT COUNT(*) FROM voting_ballots b WHERE b.poll_id=p.id),
	 EXISTS(SELECT 1 FROM voting_ballots b WHERE b.poll_id=p.id AND b.user_id=$2)
	 FROM voting_polls p WHERE ($1 OR p.status<>'draft') ORDER BY p.id DESC LIMIT $3 OFFSET $4`, admin, userID, pageSize, int64(page-1)*int64(pageSize))
	if err != nil {
		return result, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var p Summary
		if err = rows.Scan(&p.ID, &p.Title, &p.Description, &p.Status, &p.Revision, &p.CreatedAt, &p.UpdatedAt, &p.VoterCount, &p.HasVoted); err != nil {
			return result, err
		}
		result.Items = append(result.Items, p)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (s *Store) Get(ctx context.Context, id, userID int64, admin bool) (Poll, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Poll{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if !admin {
		if err = checkAccess(ctx, tx, userID, ""); err != nil {
			return Poll{}, err
		}
	}
	p, err := readPoll(ctx, tx, id, "")
	if err != nil {
		return p, err
	}
	if !admin && p.Status == "draft" {
		return Poll{}, ErrNotFound
	}
	var mine []byte
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE((SELECT option_ids FROM voting_ballots WHERE poll_id=$1 AND user_id=$2),'[]'::jsonb) FROM voting_ballots WHERE poll_id=$1`, id, userID).Scan(&p.VoterCount, &mine)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(mine, &p.MyOptionIDs); err != nil {
		return p, err
	}
	p.HasVoted = len(p.MyOptionIDs) > 0
	rows, err := tx.QueryContext(ctx, `SELECT choice::int,COUNT(*) FROM voting_ballots b CROSS JOIN LATERAL jsonb_array_elements_text(b.option_ids) AS choice WHERE b.poll_id=$1 GROUP BY choice`, id)
	if err != nil {
		return p, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var optionID, count int
		if err = rows.Scan(&optionID, &count); err != nil {
			return p, err
		}
		if optionID > 0 && optionID <= len(p.Options) {
			p.Options[optionID-1].Votes = count
		}
	}
	if err = rows.Err(); err != nil {
		return p, err
	}
	return p, tx.Commit()
}

func (s *Store) Create(ctx context.Context, input Input) (int64, error) {
	if err := input.Validate(); err != nil {
		return 0, err
	}
	raw, err := json.Marshal(input.Options)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `INSERT INTO voting_polls(title,description,image,status,options) VALUES($1,$2,$3,$4,$5) RETURNING id`, input.Title, input.Description, input.Image, input.Status, raw).Scan(&id)
	return id, err
}

func (s *Store) Update(ctx context.Context, id int64, input Input) error {
	if err := input.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	p, err := readPoll(ctx, tx, id, "FOR UPDATE")
	if err != nil {
		return err
	}
	if input.Revision != p.Revision || (p.Status != "draft" && input.Status == "draft") {
		return ErrConflict
	}
	var started bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM voting_ballots WHERE poll_id=$1)`, id).Scan(&started); err != nil {
		return err
	}
	if started {
		old := make([]OptionInput, len(p.Options))
		for i, o := range p.Options {
			old[i] = OptionInput{Title: o.Title, Image: o.Image}
		}
		if !reflect.DeepEqual(old, input.Options) {
			return ErrConflict
		}
	}
	raw, err := json.Marshal(input.Options)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE voting_polls SET title=$2,description=$3,image=$4,status=$5,options=$6,revision=revision+1,updated_at=NOW() WHERE id=$1`, id, input.Title, input.Description, input.Image, input.Status, raw)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Delete(ctx context.Context, id, revision int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	p, err := readPoll(ctx, tx, id, "FOR UPDATE")
	if err != nil {
		return err
	}
	if p.Revision != revision {
		return ErrConflict
	}
	var started bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM voting_ballots WHERE poll_id=$1)`, id).Scan(&started); err != nil {
		return err
	}
	if started {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM voting_polls WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Vote(ctx context.Context, id, userID, revision int64, choices []int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = checkAccess(ctx, tx, userID, "FOR SHARE"); err != nil {
		return err
	}
	// Ballots and admin edits share this lock, so a choice cannot change while a vote is recorded.
	p, err := readPoll(ctx, tx, id, "FOR UPDATE")
	if err != nil {
		return err
	}
	if p.Status != "open" {
		return ErrClosed
	}
	if p.Revision != revision {
		return ErrConflict
	}
	if !validChoices(choices, len(p.Options)) {
		return ErrInvalid
	}
	raw, err := json.Marshal(choices)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO voting_ballots(poll_id,user_id,option_ids) VALUES($1,$2,$3) ON CONFLICT(poll_id,user_id) DO NOTHING`, id, userID, raw)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAlreadyVoted
	}
	return tx.Commit()
}
