package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type resourceGroupTestCache struct{ stubGatewayCache }

func (c *resourceGroupTestCache) GetSessionAccountID(_ context.Context, _ int64, key string) (int64, error) {
	if value, ok := c.sessionBindings[key]; ok {
		return value, nil
	}
	return 0, ErrStickySessionNotFound
}

type legacyVideoGroupCache struct {
	stubGatewayCache
	bindings map[string]int64
	pending  []byte
}

func (c *legacyVideoGroupCache) GetSessionAccountID(_ context.Context, groupID int64, key string) (int64, error) {
	if id, ok := c.bindings[fmt.Sprintf("%d:%s", groupID, key)]; ok {
		return id, nil
	}
	return 0, ErrStickySessionNotFound
}
func (c *legacyVideoGroupCache) GetGrokVideoPendingBilling(context.Context, string) ([]byte, error) {
	return c.pending, nil
}

func TestLegacyVideoSnapshotRecoversGroupScopedOwnership(t *testing.T) {
	ctx := context.Background()
	payload, err := json.Marshal(GrokVideoPendingBilling{Model: "grok-imagine-video"})
	require.NoError(t, err)
	cache := &legacyVideoGroupCache{pending: payload, bindings: map[string]int64{}}
	svc := &OpenAIGatewayService{cache: cache}
	key := &APIKey{ID: 2, UserID: 1, GroupID: liveOptionalID(9), GroupIDs: []int64{9, 42}}
	cacheKey := svc.openAISessionCacheKey(GrokMediaVideoRequestSessionHash("legacy-video", 1, 2))
	cache.bindings[fmt.Sprintf("42:%s", cacheKey)] = 77
	group, err := svc.ResolveGrokVideoGroup(ctx, key, "legacy-video")
	require.NoError(t, err)
	require.Equal(t, int64(42), *group)
	account, err := svc.ResolveGrokMediaVideoRequestAccount(ctx, group, "legacy-video", 1, 2)
	require.NoError(t, err)
	require.Equal(t, int64(77), account)
	delete(cache.bindings, fmt.Sprintf("42:%s", cacheKey))
	_, err = svc.ResolveGrokVideoGroup(ctx, key, "legacy-video")
	require.Error(t, err, "missing old ownership must not imply ungrouped")
	cache.bindings[fmt.Sprintf("0:%s", cacheKey)] = 78
	group, err = svc.ResolveGrokVideoGroup(ctx, key, "legacy-video")
	require.NoError(t, err)
	require.Nil(t, group, "only an actual ungrouped binding authorizes group zero")
}

func TestResponseGroupSurvivesKeyReorderAndPreservesTenantOwnership(t *testing.T) {
	ctx := context.Background()
	cache := &resourceGroupTestCache{}
	writer := NewOpenAIWSStateStore(cache)
	require.NoError(t, writer.BindHTTPResponseOwner(ctx, 17, "resp_original", 21, 31, time.Hour))
	reader := &OpenAIGatewayService{cache: cache}
	key := &APIKey{UserID: 21, ID: 31, GroupIDs: []int64{99, 17}, MultiGroupEnabled: true}
	id, err := reader.ResolveResponseGroup(ctx, key, "resp_original")
	require.NoError(t, err)
	require.Equal(t, int64(17), *id)
	// Existing Responses ownership allows another key of the same user.
	key.ID = 32
	key.GroupIDs = []int64{99}
	id, err = reader.ResolveResponseGroup(ctx, key, "resp_original")
	require.NoError(t, err)
	require.Equal(t, int64(17), *id)
	key.UserID = 22
	_, err = reader.ResolveResponseGroup(ctx, key, "resp_original")
	require.Error(t, err)
}

func TestResourceGroupKeepsUngroupedAndEnforcesExactKeyForVideo(t *testing.T) {
	ctx := context.Background()
	cache := &resourceGroupTestCache{}
	writer := NewOpenAIWSStateStore(cache)
	require.NoError(t, writer.BindResourceGroup(ctx, "grok_video", "request-1", 1, 2, 0, time.Hour))
	reader := NewOpenAIWSStateStore(cache)
	id, found, err := reader.GetResourceGroup(ctx, "grok_video", "request-1", 1, 2)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, id)
	_, found, err = reader.GetResourceGroup(ctx, "grok_video", "request-1", 1, 3)
	require.NoError(t, err)
	require.False(t, found)
}

func TestLiveGroupLookupIgnoresCurrentGroupButRejectsOtherKey(t *testing.T) {
	ctx := context.Background()
	store := &liveTestStore{}
	record := &LiveCallRecord{CallID: "call_original", CallHash: hashLiveCallID("call_original"), APIKeyID: 3, UserID: 4, GroupID: 5, Controller: LiveControllerPending}
	require.NoError(t, store.SaveLiveCall(ctx, record, time.Hour))
	svc := &OpenAIGatewayService{cache: store}
	key := &APIKey{ID: 3, UserID: 4, GroupID: liveOptionalID(9), GroupIDs: []int64{9, 5}}
	id, err := svc.ResolveLiveCallGroup(ctx, key, record.CallID)
	require.NoError(t, err)
	require.Equal(t, int64(5), *id)
	key.ID++
	_, err = svc.ResolveLiveCallGroup(ctx, key, record.CallID)
	require.ErrorIs(t, err, ErrLiveCallNotFound)
}

func TestVideoGroupLookupRestoresOriginalGroupAfterUnbinding(t *testing.T) {
	ctx := context.Background()
	cache := &resourceGroupTestCache{}
	writer := &OpenAIGatewayService{cache: cache}
	originalGroup := int64(42)
	require.NoError(t, writer.BindGrokMediaVideoRequestAccount(ctx, &originalGroup, "video-original", 1, 2, 9))
	reader := &OpenAIGatewayService{cache: cache}
	key := &APIKey{ID: 2, UserID: 1, GroupIDs: []int64{99}, MultiGroupEnabled: true}
	group, err := reader.ResolveGrokVideoGroup(ctx, key, "video-original")
	require.NoError(t, err)
	require.Equal(t, originalGroup, *group)
	account, err := reader.ResolveGrokMediaVideoRequestAccount(ctx, group, "video-original", 1, 2)
	require.NoError(t, err)
	require.Equal(t, int64(9), account)
	key.ID++
	_, err = reader.ResolveGrokVideoGroup(ctx, key, "video-original")
	require.Error(t, err)
}

func TestVoiceResourcePinsGroupAndAccountAcrossInstances(t *testing.T) {
	ctx := context.Background()
	cache := &resourceGroupTestCache{}
	writer := &OpenAIGatewayService{cache: cache}
	key := &APIKey{ID: 1, UserID: 2, GroupID: liveOptionalID(3)}
	require.NoError(t, writer.BindGrokVoiceResource(ctx, key, "voice-custom", 44))
	reader := &OpenAIGatewayService{cache: cache}
	key.GroupID = liveOptionalID(9)
	for _, resourceID := range []string{"collection", "voice-custom"} {
		group, account, found, err := reader.ResolveGrokVoiceResource(ctx, key, resourceID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, int64(3), *group)
		require.Equal(t, int64(44), account)
	}
	key.ID++
	_, _, found, err := reader.ResolveGrokVoiceResource(ctx, key, "voice-custom")
	require.NoError(t, err)
	require.False(t, found)
}

func TestStatefulAdmissionRejectsBillableEventsAfterRevocation(t *testing.T) {
	denied := errors.New("original group revoked")
	calls := 0
	ctx := WithStatefulAdmission(context.Background(), func(context.Context) error { calls++; return denied })
	for _, event := range []string{"response.create", "input_audio_buffer.append", "input_audio_buffer.commit", "conversation.item.create"} {
		require.ErrorIs(t, checkStatefulEventAdmission(ctx, []byte(`{"type":"`+event+`"}`)), denied)
	}
	require.NoError(t, checkStatefulEventAdmission(ctx, []byte(`{"type":"response.cancel"}`)))
	require.Equal(t, 4, calls)
}
