package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Resource indexes have no current-group component: a key may be reordered or
// unbound while the resource still belongs to its original group. Encoding +1
// preserves the distinction between ungrouped (0) and a cache miss.
func resourceGroupCacheKey(kind, resourceID string, userID, apiKeyID int64) string {
	return openAIHTTPResponseOwnerCacheKey("resource-group:", fmt.Sprintf("%s:%d:%d:%s", kind, userID, apiKeyID, resourceID))
}

func (s *defaultOpenAIWSStateStore) BindResourceGroup(ctx context.Context, kind, resourceID string, userID, apiKeyID, groupID int64, ttl time.Duration) error {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" || userID <= 0 || groupID < 0 {
		return fmt.Errorf("invalid resource ownership")
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()
	key := resourceGroupCacheKey(kind, resourceID, userID, apiKeyID)
	s.responseOwnerMu.Lock()
	ensureBindingCapacity(s.resourceGroups, key, openAIWSStateStoreMaxEntriesPerMap)
	s.resourceGroups[key] = openAIWSAccountBinding{accountID: groupID + 1, expiresAt: time.Now().Add(ttl)}
	s.responseOwnerMu.Unlock()
	if s.cache == nil {
		return nil
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.SetSessionAccountID(cacheCtx, 0, key, groupID+1, ttl)
}

func (s *defaultOpenAIWSStateStore) GetResourceGroup(ctx context.Context, kind, resourceID string, userID, apiKeyID int64) (int64, bool, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" || userID <= 0 {
		return 0, false, nil
	}
	s.maybeCleanup()
	key := resourceGroupCacheKey(kind, resourceID, userID, apiKeyID)
	s.responseOwnerMu.RLock()
	binding, ok := s.resourceGroups[key]
	s.responseOwnerMu.RUnlock()
	if ok && time.Now().Before(binding.expiresAt) {
		return binding.accountID - 1, true, nil
	}
	if s.cache == nil {
		return 0, false, nil
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	value, err := s.cache.GetSessionAccountID(cacheCtx, 0, key)
	if errors.Is(err, ErrStickySessionNotFound) {
		return 0, false, nil
	}
	if err != nil || value <= 0 {
		return 0, false, err
	}
	return value - 1, true, nil
}

func (s *OpenAIGatewayService) ResolveResponseGroup(ctx context.Context, key *APIKey, responseID string) (*int64, error) {
	if s == nil || key == nil {
		return nil, fmt.Errorf("response ownership unavailable")
	}
	store := s.getOpenAIWSStateStore()
	groupID, found, err := store.GetResourceGroup(ctx, "response", responseID, key.UserID, 0)
	if err != nil {
		return nil, err
	}
	if found {
		return liveOptionalID(groupID), nil
	}
	// Rollout compatibility: old responses have only group-scoped ownership.
	ids := append([]int64(nil), key.GroupIDs...)
	if key.GroupID != nil {
		ids = append(ids, *key.GroupID)
	} else {
		ids = append(ids, 0)
	}
	for _, id := range ids {
		owned, err := s.ValidateOpenAIHTTPResponseOwner(ctx, id, responseID, key.UserID, key.ID)
		if errors.Is(err, ErrStickySessionNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if owned {
			return liveOptionalID(id), nil
		}
	}
	return nil, fmt.Errorf("previous_response_id is not available for this user")
}

// ResolveLiveCallGroup verifies downstream ownership before group admission.
func (s *OpenAIGatewayService) ResolveLiveCallGroup(ctx context.Context, key *APIKey, callID string) (*int64, error) {
	store, err := s.liveStore()
	if err != nil {
		return nil, err
	}
	record, err := store.GetLiveCall(ctx, hashLiveCallID(callID))
	if err != nil {
		return nil, err
	}
	if record == nil || record.CallID != callID || record.APIKeyID != key.ID || record.UserID != key.UserID || record.Controller == LiveControllerClosed {
		return nil, ErrLiveCallNotFound
	}
	return liveOptionalID(record.GroupID), nil
}

func (s *OpenAIGatewayService) ResolveGrokVideoGroup(ctx context.Context, key *APIKey, requestID string) (*int64, error) {
	pending, err := s.LoadGrokVideoPendingBilling(ctx, requestID, key.UserID, key.ID)
	if err != nil {
		return nil, err
	}
	if pending != nil && (pending.GroupID != nil || pending.AccountID > 0) {
		return pending.GroupID, nil
	}
	groupID, found, err := s.getOpenAIWSStateStore().GetResourceGroup(ctx, "grok_video", requestID, key.UserID, key.ID)
	if err != nil {
		return nil, err
	}
	if found {
		return liveOptionalID(groupID), nil
	}
	// Before group/account snapshots existed, the only authoritative binding
	// was keyed by group, user and API key. A nil legacy GroupID is unknown,
	// not proof the request was created without a group.
	ids := append([]int64{0}, key.ConfiguredGroupIDs()...)
	if key.GroupID != nil {
		ids = append(ids, *key.GroupID)
	}
	seen := make(map[int64]bool, len(ids))
	cacheKey := s.openAISessionCacheKey(GrokMediaVideoRequestSessionHash(requestID, key.UserID, key.ID))
	if s.cache != nil {
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			accountID, lookupErr := s.cache.GetSessionAccountID(ctx, id, cacheKey)
			if errors.Is(lookupErr, ErrStickySessionNotFound) {
				continue
			}
			if lookupErr != nil {
				return nil, lookupErr
			}
			if accountID > 0 {
				return liveOptionalID(id), nil
			}
		}
	}
	return nil, fmt.Errorf("video request ownership unavailable")
}

// GetGrokVideoAccount loads the original credential owner without scheduling a
// replacement account when group membership or scheduling eligibility changes.
func (s *OpenAIGatewayService) GetGrokVideoAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return nil, fmt.Errorf("video account unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil || account.Platform != PlatformGrok {
		return nil, fmt.Errorf("video account unavailable")
	}
	return account, nil
}

func (s *OpenAIGatewayService) GetGrokVoiceAccount(ctx context.Context, key *APIKey, accountID int64) (*Account, error) {
	account, err := s.GetGrokVideoAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !account.IsSchedulable() || !openAIStickyAccountMatchesGroup(account, key.GroupID) {
		return nil, fmt.Errorf("custom voice's original account is unavailable")
	}
	return account, nil
}

const grokVoiceOwnershipTTL = 365 * 24 * time.Hour

// A custom voice and its collection share one original credential owner. Redis
// ownership is renewed on resource access; misses fail closed for resource IDs.
func (s *OpenAIGatewayService) BindGrokVoiceResource(ctx context.Context, key *APIKey, voiceID string, accountID int64) error {
	store := s.getOpenAIWSStateStore()
	for _, id := range []string{"collection", voiceID} {
		if id == "" {
			continue
		}
		if err := store.BindResourceGroup(ctx, "grok_voice_account", id, key.UserID, key.ID, accountID, grokVoiceOwnershipTTL); err != nil {
			return err
		}
		if err := store.BindResourceGroup(ctx, "grok_voice", id, key.UserID, key.ID, derefGroupID(key.GroupID), grokVoiceOwnershipTTL); err != nil {
			return err
		}
	}
	return nil
}

func (s *OpenAIGatewayService) ResolveGrokVoiceResource(ctx context.Context, key *APIKey, voiceID string) (*int64, int64, bool, error) {
	store := s.getOpenAIWSStateStore()
	groupID, found, err := store.GetResourceGroup(ctx, "grok_voice", voiceID, key.UserID, key.ID)
	if err != nil || !found {
		return nil, 0, false, err
	}
	accountID, found, err := store.GetResourceGroup(ctx, "grok_voice_account", voiceID, key.UserID, key.ID)
	if err != nil || !found || accountID <= 0 {
		return nil, 0, false, err
	}
	return liveOptionalID(groupID), accountID, true, nil
}
