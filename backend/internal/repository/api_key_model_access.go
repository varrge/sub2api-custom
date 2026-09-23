package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.APIKeyModelAccessRepository = (*apiKeyRepository)(nil)

func (r *apiKeyRepository) updateWithModelAccessRevision(ctx context.Context, key *service.APIKey, fields service.APIKeyUpdateFields) error {
	checkAndUpdate := func(ctx context.Context, client *dbent.Client) error {
		current, err := client.APIKey.Query().Where(apikey.IDEQ(key.ID), apikey.DeletedAtIsNil()).ForUpdate().Only(ctx)
		if err != nil {
			return translatePersistenceError(err, service.ErrAPIKeyNotFound, nil)
		}
		if service.APIKeyModelAccessRevision(apiKeyEntityToService(current)) != fields.ModelAllowlistRevision {
			return service.ErrAPIKeyModelAccessConflict
		}
		return r.updateFields(ctx, key, fields)
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return checkAndUpdate(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := checkAndUpdate(dbent.NewTxContext(ctx, tx), tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *apiKeyRepository) ListModelAccessKeys(ctx context.Context, userID int64) ([]service.APIKey, error) {
	entities, err := r.activeQuery().Where(apikey.UserIDEQ(userID)).
		Select(apikey.FieldID, apikey.FieldUserID, apikey.FieldName, apikey.FieldStatus,
			apikey.FieldExpiresAt, apikey.FieldGroupID, apikey.FieldGroupIds, apikey.FieldModelAllowlist).
		Order(dbent.Asc(apikey.FieldID)).Limit(service.MaxAPIKeyModelAccessKeys + 1).All(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]service.APIKey, 0, len(entities))
	for _, entity := range entities {
		keys = append(keys, *apiKeyEntityToService(entity))
	}
	if err := r.attachConfiguredGroupValues(ctx, keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *apiKeyRepository) UpdateModelAccess(ctx context.Context, userID int64, model string, entries []service.APIKeyModelAccessEntry) ([]service.APIKey, []string, error) {
	model, err := service.NormalizeAPIKeyModelAccessModel(model)
	if err != nil {
		return nil, nil, err
	}
	if err := service.ValidateAPIKeyModelAccessEntries(entries); err != nil {
		return nil, nil, err
	}
	ids := make([]int64, 0, len(entries))
	byID := make(map[int64]service.APIKeyModelAccessEntry, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
		byID[entry.ID] = entry
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Lock in ID order. Ownership, revisions, validation and writes all use
	// these locked rows; neither a single-key editor nor a second batch can
	// slip an update between the check and the write.
	entities, err := tx.APIKey.Query().Where(apikey.UserIDEQ(userID), apikey.DeletedAtIsNil(), apikey.IDIn(ids...)).
		Order(dbent.Asc(apikey.FieldID)).ForUpdate().All(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(entities) != len(entries) {
		return nil, nil, service.ErrAPIKeyModelAccessConflict
	}
	keys := make([]service.APIKey, 0, len(entities))
	changed := make([]string, 0)
	for _, entity := range entities {
		key := apiKeyEntityToService(entity)
		entry := byID[key.ID]
		if service.APIKeyModelAccessRevision(key) != entry.Revision {
			return nil, nil, service.ErrAPIKeyModelAccessConflict
		}
		next, err := service.SetAPIKeyModelAccess(key.ModelAllowlist, model, entry.Allowed)
		if err != nil {
			return nil, nil, err
		}
		if key.AllowsModel(model) != entry.Allowed {
			if err := tx.APIKey.UpdateOneID(key.ID).
				SetModelAllowlist(service.DomainGroupModelAllowlist(next)).SetUpdatedAt(time.Now()).Exec(ctx); err != nil {
				return nil, nil, err
			}
			changed = append(changed, key.Key)
			key.ModelAllowlist = next
		}
		keys = append(keys, *key)
	}
	// Group metadata is intentionally not returned by this mutation. Callers
	// retain their display metadata, applying only the committed policies.
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return keys, changed, nil
}
