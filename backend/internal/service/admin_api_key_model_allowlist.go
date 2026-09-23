package service

import (
	"context"
	"fmt"
)

// AdminAPIKeyModelLimits extends management without widening AdminService.
type AdminAPIKeyModelLimits interface {
	AdminUpdateAPIKeyModelLimits(context.Context, int64, AdminUpdateAPIKeyModelLimitsRequest) (*AdminUpdateAPIKeyGroupIDResult, error)
}

type AdminUpdateAPIKeyModelLimitsRequest struct {
	ModelAllowlistRevision string
	GroupID                *int64
	GroupIDs               *[]int64
	ModelAllowlist         *GroupModelAllowlist
	ResetRateLimitUsage    bool
}

// AdminUpdateAPIKeyModelLimits validates the whole selection before writing
// groups, model restrictions and optional usage resets in a single update.
func (s *adminServiceImpl) AdminUpdateAPIKeyModelLimits(ctx context.Context, keyID int64, req AdminUpdateAPIKeyModelLimitsRequest) (*AdminUpdateAPIKeyGroupIDResult, error) {
	var cfg GroupModelAllowlist
	if req.ModelAllowlist != nil {
		var err error
		cfg, err = NormalizeAPIKeyModelAllowlist(*req.ModelAllowlist)
		if err != nil {
			return nil, err
		}
	}
	ids, selected, err := apiKeyRequestedGroups(req.GroupID, false, req.GroupIDs, true)
	if err != nil {
		return nil, err
	}
	key, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if err := validateAPIKeyModelModeUpdate(key.ModelAllowlist, req.ModelAllowlist); err != nil {
		return nil, err
	}
	var groups []*Group
	if selected {
		var user *User
		if len(ids) > 0 {
			user, err = s.userRepo.GetByID(ctx, key.UserID)
			if err != nil {
				return nil, err
			}
		}
		groups, err = s.apiKeyGroupService().validateGroupSelection(ctx, user, key.ConfiguredGroupIDs(), ids)
		if err != nil {
			return nil, err
		}
	}
	fields := APIKeyUpdateFields{GroupIDs: selected, ModelAllowlist: req.ModelAllowlist != nil, RateLimitUsage: req.ResetRateLimitUsage}
	if selected {
		setAPIKeyGroups(key, ids, groups)
	}
	if req.ModelAllowlist != nil {
		if req.ModelAllowlistRevision != "" && req.ModelAllowlistRevision != APIKeyModelAccessRevision(key) {
			return nil, ErrAPIKeyModelAccessConflict
		}
		fields.ModelAllowlistRevision = req.ModelAllowlistRevision
		key.ModelAllowlist = cfg
	}
	if req.ResetRateLimitUsage {
		key.Usage5h, key.Usage1d, key.Usage7d = 0, 0, 0
		key.Window5hStart, key.Window1dStart, key.Window7dStart = nil, nil, nil
	}
	if err := s.apiKeyRepo.Update(ctx, key, fields); err != nil {
		return nil, fmt.Errorf("update api key: %w", err)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, key.Key)
	}
	if req.ResetRateLimitUsage && s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateAPIKeyRateLimit(ctx, key.ID)
	}
	return &AdminUpdateAPIKeyGroupIDResult{APIKey: key}, nil
}
