package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

const AccountFallbackGroupIDsExtraKey = "fallback_group_ids"

// ParseAccountFallbackGroupIDs accepts the in-memory and JSON forms of a group ID list.
func ParseAccountFallbackGroupIDs(raw any) ([]int64, error) {
	var values []any
	switch ids := raw.(type) {
	case nil:
		return nil, nil
	case []any:
		values = ids
	case []int64:
		for _, id := range ids {
			values = append(values, id)
		}
	case []int:
		for _, id := range ids {
			values = append(values, id)
		}
	case []float64:
		for _, id := range ids {
			values = append(values, id)
		}
	default:
		return nil, fmt.Errorf("fallback_group_ids must be an array of positive integer group IDs")
	}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		var id int64
		switch v := value.(type) {
		case int64:
			id = v
		case int:
			id = int64(v)
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v >= 1<<63 {
				return nil, fmt.Errorf("fallback_group_ids must contain positive integer group IDs")
			}
			id = int64(v)
		case json.Number:
			var err error
			id, err = v.Int64()
			if err != nil {
				return nil, fmt.Errorf("fallback_group_ids must contain positive integer group IDs")
			}
		default:
			return nil, fmt.Errorf("fallback_group_ids must contain positive integer group IDs")
		}
		if id <= 0 {
			return nil, fmt.Errorf("fallback_group_ids must contain positive integer group IDs")
		}
		if !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	return result, nil
}

func (a *Account) IsFallbackForGroup(groupID *int64) bool {
	if a == nil || groupID == nil || *groupID <= 0 || a.Extra == nil {
		return false
	}
	ids, err := ParseAccountFallbackGroupIDs(a.Extra[AccountFallbackGroupIDsExtraKey])
	return err == nil && slices.Contains(ids, *groupID)
}

type accountFallbackPoolKey struct{}

type accountFallbackPool struct {
	groupID  int64
	fallback bool
}

func withAccountFallbackPool(ctx context.Context, groupID *int64, fallback bool) context.Context {
	return context.WithValue(ctx, accountFallbackPoolKey{}, accountFallbackPool{groupID: derefGroupID(groupID), fallback: fallback})
}

func accountFallbackPoolSet(ctx context.Context) bool {
	_, ok := ctx.Value(accountFallbackPoolKey{}).(accountFallbackPool)
	return ok
}

func accountMatchesFallbackPool(ctx context.Context, account *Account) bool {
	pool, ok := ctx.Value(accountFallbackPoolKey{}).(accountFallbackPool)
	return !ok || account.IsFallbackForGroup(&pool.groupID) == pool.fallback
}
