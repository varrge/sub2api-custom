package service

import (
	"maps"
	"slices"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func ValidateAccountFallbackGroupsExtra(extra map[string]any, groupIDs []int64) error {
	raw, provided := extra[AccountFallbackGroupIDsExtraKey]
	if !provided {
		return nil
	}
	ids, err := ParseAccountFallbackGroupIDs(raw)
	if err != nil {
		return infraerrors.BadRequest("INVALID_ACCOUNT_FALLBACK_GROUPS", "fallback_group_ids must be an array of positive integer group IDs")
	}
	for _, id := range ids {
		if !slices.Contains(groupIDs, id) {
			return infraerrors.BadRequest("INVALID_ACCOUNT_FALLBACK_GROUPS", "fallback_group_ids must belong to the account's groups")
		}
	}
	return nil
}

func normalizeAccountFallbackUpdate(account *Account, extra map[string]any, groupIDs *[]int64) (map[string]any, error) {
	effectiveGroups := account.GroupIDs
	if groupIDs != nil {
		effectiveGroups = *groupIDs
	}
	if _, provided := extra[AccountFallbackGroupIDsExtraKey]; provided {
		if err := ValidateAccountFallbackGroupsExtra(extra, effectiveGroups); err != nil {
			return nil, err
		}
		return extra, nil
	}
	raw, exists := account.Extra[AccountFallbackGroupIDsExtraKey]
	if !exists || (extra == nil && groupIDs == nil) {
		return extra, nil
	}
	ids, err := ParseAccountFallbackGroupIDs(raw)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_FALLBACK_GROUPS", "stored fallback_group_ids must be corrected before editing the account")
	}
	if groupIDs != nil {
		ids = slices.DeleteFunc(ids, func(id int64) bool { return !slices.Contains(effectiveGroups, id) })
	}
	if extra == nil {
		extra = account.Extra
	}
	normalized := maps.Clone(extra)
	if normalized == nil {
		normalized = make(map[string]any)
	}
	normalized[AccountFallbackGroupIDsExtraKey] = ids
	return normalized, nil
}
