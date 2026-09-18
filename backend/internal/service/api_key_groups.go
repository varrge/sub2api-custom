package service

import (
	"context"
	"encoding/json"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ValidateAPIKeyGroupJSON preserves field presence, including explicit null.
func ValidateAPIKeyGroupJSON(data []byte) (bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return false, err
	}
	_, single := fields["group_id"]
	many, multiple := fields["group_ids"]
	if single && multiple {
		return single, infraerrors.BadRequest("API_KEY_GROUP_FIELDS_CONFLICT", "group_id and group_ids cannot be supplied together")
	}
	if multiple && string(many) == "null" {
		return single, infraerrors.BadRequest("API_KEY_GROUPS_INVALID", "group_ids must be an array")
	}
	return single, nil
}

func (r *CreateAPIKeyRequest) UnmarshalJSON(data []byte) error {
	type plain CreateAPIKeyRequest
	if err := json.Unmarshal(data, (*plain)(r)); err != nil {
		return err
	}
	present, err := ValidateAPIKeyGroupJSON(data)
	r.GroupIDPresent = present
	return err
}
func (r *UpdateAPIKeyRequest) UnmarshalJSON(data []byte) error {
	type plain UpdateAPIKeyRequest
	if err := json.Unmarshal(data, (*plain)(r)); err != nil {
		return err
	}
	present, err := ValidateAPIKeyGroupJSON(data)
	r.GroupIDPresent = present
	return err
}

func apiKeyRequestedGroups(single *int64, singlePresent bool, multiple *[]int64, allowEmpty bool) ([]int64, bool, error) {
	singlePresent = singlePresent || single != nil
	if singlePresent && multiple != nil {
		return nil, false, infraerrors.BadRequest("API_KEY_GROUP_FIELDS_CONFLICT", "group_id and group_ids cannot be supplied together")
	}
	if !singlePresent && multiple == nil {
		return nil, false, nil
	}
	ids := []int64{}
	if multiple != nil {
		ids = append(ids, (*multiple)...)
	} else if single != nil && *single != 0 {
		ids = append(ids, *single)
	}
	if len(ids) == 0 && !allowEmpty {
		return nil, true, infraerrors.BadRequest("API_KEY_GROUP_REQUIRED", "select at least one group")
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return nil, true, infraerrors.BadRequest("API_KEY_GROUPS_INVALID", "group IDs must be positive and unique")
		}
		seen[id] = true
	}
	return ids, true, nil
}

func firstAPIKeyGroupID(ids []int64) *int64 {
	if len(ids) == 0 {
		return nil
	}
	id := ids[0]
	return &id
}

func setAPIKeyGroups(key *APIKey, ids []int64, groups []*Group) {
	key.GroupIDs = append([]int64{}, ids...)
	key.Groups = groups
	key.GroupID = firstAPIKeyGroupID(ids)
	key.Group = nil
	if len(groups) > 0 {
		key.Group = groups[0]
	}
	key.MultiGroupEnabled = key.MultiGroupEnabled || len(ids) > 1
}

// Existing bindings survive temporary ineligibility. Only newly selected groups
// require current entitlement; reordering never grants new access.
func (s *APIKeyService) validateGroupSelection(ctx context.Context, user *User, retained, ids []int64) ([]*Group, error) {
	old := map[int64]bool{}
	for _, id := range retained {
		old[id] = true
	}
	groups := make([]*Group, 0, len(ids))
	for _, id := range ids {
		group, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get group: %w", err)
		}
		if !old[id] && !s.canUserBindGroup(ctx, user, group) {
			return nil, ErrGroupNotAllowed
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// AdminAPIKeyGroups is an additive management interface for ordered selections.
type AdminAPIKeyGroups interface {
	AdminUpdateAPIKeyGroups(context.Context, int64, []int64) (*AdminUpdateAPIKeyGroupIDResult, error)
	GetAPIKeyAvailableGroups(context.Context, int64) ([]Group, error)
}

func (s *adminServiceImpl) apiKeyGroupService() *APIKeyService {
	// The configured invalidator is the shared API key service, including month-card eligibility.
	if svc, ok := s.authCacheInvalidator.(*APIKeyService); ok {
		return svc
	}
	return &APIKeyService{userRepo: s.userRepo, groupRepo: s.groupRepo, userSubRepo: s.userSubRepo}
}

func (s *adminServiceImpl) GetAPIKeyAvailableGroups(ctx context.Context, userID int64) ([]Group, error) {
	return s.apiKeyGroupService().GetAvailableGroups(ctx, userID)
}

func (s *adminServiceImpl) AdminUpdateAPIKeyGroups(ctx context.Context, keyID int64, ids []int64) (*AdminUpdateAPIKeyGroupIDResult, error) {
	ids, _, err := apiKeyRequestedGroups(nil, false, &ids, true)
	if err != nil {
		return nil, err
	}
	key, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	var user *User
	if len(ids) > 0 {
		user, err = s.userRepo.GetByID(ctx, key.UserID)
		if err != nil {
			return nil, err
		}
	}
	groups, err := s.apiKeyGroupService().validateGroupSelection(ctx, user, key.ConfiguredGroupIDs(), ids)
	if err != nil {
		return nil, err
	}
	setAPIKeyGroups(key, ids, groups)
	if err := s.apiKeyRepo.Update(ctx, key, APIKeyUpdateFields{GroupIDs: true}); err != nil {
		return nil, fmt.Errorf("update api key: %w", err)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, key.Key)
	}
	return &AdminUpdateAPIKeyGroupIDResult{APIKey: key}, nil
}

// GetGroupForRequest loads the original group for an existing task or session.
func (s *APIKeyService) GetGroupForRequest(ctx context.Context, id int64) (*Group, error) {
	return s.groupRepo.GetByID(ctx, id)
}
