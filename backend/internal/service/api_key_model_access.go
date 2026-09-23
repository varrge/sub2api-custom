package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrAPIKeyModelAccessConflict = infraerrors.Conflict("API_KEY_MODEL_ACCESS_CONFLICT", "Model permissions changed; reload and review before saving")

const MaxAPIKeyModelAccessKeys = 10000

// APIKeyModelAccessRepository owns the transaction. Its update returns only
// after commit so authentication caches cannot be evicted before persistence.
type APIKeyModelAccessRepository interface {
	ListModelAccessKeys(context.Context, int64) ([]APIKey, error)
	UpdateModelAccess(context.Context, int64, string, []APIKeyModelAccessEntry) ([]APIKey, []string, error)
}

type APIKeyModelAccessEntry struct {
	ID       int64  `json:"id"`
	Revision string `json:"revision"`
	Allowed  bool   `json:"allowed"`
}

type APIKeyModelAccessGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

// Deliberately excludes the credential, IP history and billing data.
type APIKeyModelAccessKey struct {
	ID             int64                    `json:"id"`
	Name           string                   `json:"name"`
	Status         string                   `json:"status"`
	ExpiresAt      *time.Time               `json:"expires_at"`
	GroupIDs       []int64                  `json:"group_ids"`
	Groups         []APIKeyModelAccessGroup `json:"groups"`
	ModelAllowlist GroupModelAllowlist      `json:"model_allowlist"`
	Revision       string                   `json:"revision"`
}

type APIKeyModelAccessSnapshot struct {
	Keys []APIKeyModelAccessKey `json:"keys"`
}

type APIKeyModelAccessResult struct {
	Keys         []APIKeyModelAccessKey `json:"keys"`
	UpdatedCount int                    `json:"updated_count"`
}

// A policy revision excludes volatile usage and last_used_at: billing should
// never cause spurious edit conflicts. IDs bind the revision to this key.
func APIKeyModelAccessRevision(key *APIKey) string {
	policy := cloneAPIKeyModelAllowlist(key.ModelAllowlist)
	data, _ := json.Marshal(struct {
		ID, UserID int64
		Policy     GroupModelAllowlist
	}{key.ID, key.UserID, policy})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func NormalizeAPIKeyModelAccessModel(model string) (string, error) {
	cfg, err := NormalizeAPIKeyModelAllowlist(GroupModelAllowlist{Models: []string{model}})
	if err != nil {
		return "", err
	}
	if len(cfg.Models) != 1 {
		return "", infraerrors.BadRequest("INVALID_MODEL_ALLOWLIST", "A concrete model ID is required")
	}
	return cfg.Models[0], nil
}

// SetAPIKeyModelAccess edits one exact public ID, preserving permissions for
// every other current or future model. A disabled policy's dormant selections
// must not become active when this operation first restricts the key.
func SetAPIKeyModelAccess(current GroupModelAllowlist, model string, allowed bool) (GroupModelAllowlist, error) {
	model, err := NormalizeAPIKeyModelAccessModel(model)
	if err != nil {
		return GroupModelAllowlist{}, err
	}
	if (&APIKey{ModelAllowlist: current}).AllowsModel(model) == allowed {
		return cloneAPIKeyModelAllowlist(current), nil
	}
	if !current.Enabled {
		return GroupModelAllowlist{Enabled: true, Mode: "deny", Models: []string{model}}, nil
	}
	next, err := NormalizeAPIKeyModelAllowlist(current)
	if err != nil {
		return GroupModelAllowlist{}, err
	}
	add := allowed != (next.Mode == "deny")
	if add {
		next.Models = append(next.Models, model)
	} else {
		models := make([]string, 0, len(next.Models))
		for _, existing := range next.Models {
			if existing != model {
				models = append(models, existing)
			}
		}
		next.Models = models
	}
	return NormalizeAPIKeyModelAllowlist(next)
}

func ValidateAPIKeyModelAccessEntries(entries []APIKeyModelAccessEntry) error {
	if len(entries) == 0 || len(entries) > MaxAPIKeyModelAccessKeys {
		return infraerrors.BadRequest("INVALID_MODEL_ACCESS_ENTRIES", "Provide between 1 and 10000 keys")
	}
	seen := make(map[int64]bool, len(entries))
	for _, entry := range entries {
		revision, err := hex.DecodeString(entry.Revision)
		if entry.ID <= 0 || seen[entry.ID] || err != nil || len(revision) != sha256.Size || entry.Revision != strings.ToLower(entry.Revision) {
			return infraerrors.BadRequest("INVALID_MODEL_ACCESS_ENTRIES", "Each key requires a unique positive ID and a valid revision")
		}
		seen[entry.ID] = true
	}
	return nil
}

func modelAccessKeys(keys []APIKey) []APIKeyModelAccessKey {
	out := make([]APIKeyModelAccessKey, 0, len(keys))
	for i := range keys {
		key := &keys[i]
		groups := make([]APIKeyModelAccessGroup, 0, len(key.Groups))
		for _, group := range key.Groups {
			if group != nil {
				groups = append(groups, APIKeyModelAccessGroup{ID: group.ID, Name: group.Name, Platform: group.Platform})
			}
		}
		out = append(out, APIKeyModelAccessKey{
			ID: key.ID, Name: key.Name, Status: key.Status, ExpiresAt: key.ExpiresAt,
			GroupIDs: append([]int64{}, key.ConfiguredGroupIDs()...), Groups: groups,
			ModelAllowlist: cloneAPIKeyModelAllowlist(key.ModelAllowlist), Revision: APIKeyModelAccessRevision(key),
		})
	}
	return out
}

func (s *APIKeyService) ModelAccess(ctx context.Context, userID int64) (*APIKeyModelAccessSnapshot, error) {
	repo, ok := s.apiKeyRepo.(APIKeyModelAccessRepository)
	if !ok {
		return nil, fmt.Errorf("model access repository is unavailable")
	}
	keys, err := repo.ListModelAccessKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(keys) > MaxAPIKeyModelAccessKeys {
		return nil, infraerrors.BadRequest("MODEL_ACCESS_TOO_MANY_KEYS", "Too many keys to edit together")
	}
	return &APIKeyModelAccessSnapshot{Keys: modelAccessKeys(keys)}, nil
}

func (s *APIKeyService) UpdateModelAccess(ctx context.Context, userID int64, model string, entries []APIKeyModelAccessEntry) (*APIKeyModelAccessResult, error) {
	model, err := NormalizeAPIKeyModelAccessModel(model)
	if err != nil {
		return nil, err
	}
	if err := ValidateAPIKeyModelAccessEntries(entries); err != nil {
		return nil, err
	}
	repo, ok := s.apiKeyRepo.(APIKeyModelAccessRepository)
	if !ok {
		return nil, fmt.Errorf("model access repository is unavailable")
	}
	keys, changedCredentials, err := repo.UpdateModelAccess(ctx, userID, model, entries)
	if err != nil {
		return nil, err
	}
	// A client may disconnect just after commit. Finish cache invalidation
	// independently, with bounded concurrency and a bounded overall duration.
	invalidationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	concurrency := min(16, len(changedCredentials))
	for offset := 0; offset < concurrency; offset++ {
		workers.Add(1)
		go func(offset int) {
			defer workers.Done()
			for i := offset; i < len(changedCredentials); i += concurrency {
				s.InvalidateAuthCacheByKey(invalidationCtx, changedCredentials[i])
			}
		}(offset)
	}
	workers.Wait()
	return &APIKeyModelAccessResult{Keys: modelAccessKeys(keys), UpdatedCount: len(changedCredentials)}, nil
}
