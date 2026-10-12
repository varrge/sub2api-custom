package service

import (
	"slices"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// APIKeyModelGroupRoutes restricts exact public model IDs to bound groups.
// An absent model keeps legacy routing; an explicit empty list denies it.
// These selections only narrow authority and never reorder the key's groups.
type APIKeyModelGroupRoutes map[string][]int64

func CloneAPIKeyModelGroupRoutes(routes APIKeyModelGroupRoutes) APIKeyModelGroupRoutes {
	out := make(APIKeyModelGroupRoutes, len(routes))
	for model, ids := range routes {
		out[model] = append([]int64{}, ids...)
	}
	return out
}

func (k *APIKey) HasModelRestrictions() bool {
	return k != nil && (k.ModelAllowlist.Enabled || len(k.ModelGroupRoutes) > 0)
}

func (k *APIKey) ModelGroupRestricted(model string) bool {
	if k == nil {
		return false
	}
	_, restricted := k.ModelGroupRoutes[strings.TrimSpace(model)]
	return restricted
}

func (k *APIKey) modelHasConfiguredGroup(model string) bool {
	ids, restricted := k.ModelGroupRoutes[strings.TrimSpace(model)]
	if !restricted {
		return true
	}
	for _, id := range ids {
		if k.HasGroupID(id) {
			return true
		}
	}
	return false
}

func (k *APIKey) AllowsModelInGroup(model string, groupID int64) bool {
	if !k.AllowsModel(model) {
		return false
	}
	ids, restricted := k.ModelGroupRoutes[strings.TrimSpace(model)]
	return !restricted || k.HasGroupID(groupID) && slices.Contains(ids, groupID)
}

// Used after routing, including subsequent turns pinned to the same upstream.
func (k *APIKey) AllowsModelInSelectedGroup(model string) bool {
	if k == nil {
		return false
	}
	id := int64(0)
	if k.GroupID != nil {
		id = *k.GroupID
	}
	return k.AllowsModelInGroup(model, id)
}

// SetAPIKeyModelGroups validates against the locked key row. It cannot bind a
// new group or widen its catalog/entitlement; those checks still run at admission.
func SetAPIKeyModelGroups(key *APIKey, model string, ids []int64) (APIKeyModelGroupRoutes, error) {
	model, err := NormalizeAPIKeyModelAccessModel(model)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] || !key.HasGroupID(id) {
			return nil, infraerrors.BadRequest("INVALID_MODEL_GROUPS", "Model groups must be unique groups already bound to this key")
		}
		seen[id] = true
	}
	next := CloneAPIKeyModelGroupRoutes(key.ModelGroupRoutes)
	if _, exists := next[model]; !exists && len(next) >= 512 {
		return nil, infraerrors.BadRequest("INVALID_MODEL_GROUPS", "A key cannot restrict groups for more than 512 models")
	}
	// Canonical ordering follows the key, never the checkbox click order.
	next[model] = []int64{}
	for _, id := range key.ConfiguredGroupIDs() {
		if seen[id] {
			next[model] = append(next[model], id)
		}
	}
	return next, nil
}
