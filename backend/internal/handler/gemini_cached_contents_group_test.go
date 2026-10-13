//go:build unit

package handler

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGeminiCachePinnedGroupLifecycle(t *testing.T) {
	repo := &geminiCachedContentHandlerRepoStub{}
	record := seedGeminiCachedContent(repo, "owned", 7, time.Now().Add(time.Hour))
	record.ChannelUsage.OriginalModel = "public-alias"
	h := newGeminiCachedContentHandlerForTest(repo)
	key := geminiCachedContentTestAPIKey(service.PlatformGemini)
	key.Group.Status = service.StatusActive
	original := key.Group
	other := &service.Group{ID: 99, Platform: service.PlatformGemini, Status: service.StatusActive}
	key.Groups = []*service.Group{other, original}
	key.GroupIDs = []int64{other.ID, original.ID}
	key.Group, key.GroupID = other, &other.ID
	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodPatch, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			path := "/v1beta/cachedContents/owned"
			if method == http.MethodPost {
				path = "/v1beta/models/public-alias:generateContent"
			}
			c, _ := newGeminiCachedContentTestContext(method, path, key)
			c.Request.Body = io.NopCloser(strings.NewReader(`{"cachedContent":"cachedContents/owned"}`))
			selected, historical, err := h.ResolveAPIKeyPinnedGroup(c, key)
			require.NoError(t, err)
			require.Equal(t, int64(42), *selected.GroupID)
			require.Equal(t, method == http.MethodGet || method == http.MethodDelete, historical)
			require.Equal(t, int64(99), *key.GroupID)
			if !historical {
				require.Equal(t, "public-alias", c.GetString("api_key_resource_model"))
				require.Equal(t, record.AccountID, c.GetInt64("api_key_cached_content_account_id"))
			}
		})
	}

	key.GroupIDs = []int64{other.ID}
	key.Groups = []*service.Group{other}
	key.ModelGroupRoutes = service.APIKeyModelGroupRoutes{"public-alias": {}}
	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodPatch} {
		c, _ := newGeminiCachedContentTestContext(method, "/v1beta/cachedContents/owned", key)
		selected, historical, err := h.ResolveAPIKeyPinnedGroup(c, key)
		if method == http.MethodPatch {
			require.ErrorContains(t, err, "no longer selected")
			require.Nil(t, selected)
		} else {
			require.NoError(t, err)
			require.True(t, historical)
			require.Equal(t, int64(42), *selected.GroupID)
		}
	}
	key.ID = 999
	c, _ := newGeminiCachedContentTestContext(http.MethodGet, "/v1beta/cachedContents/owned", key)
	_, _, err := h.ResolveAPIKeyPinnedGroup(c, key)
	require.ErrorContains(t, err, service.GeminiCachedContentNotFoundMessage)
}

func TestGeminiCacheRechecksOriginalPublicModelForPatchAndGeneration(t *testing.T) {
	for _, policy := range []string{"key", "model-groups", "group"} {
		t.Run(policy, func(t *testing.T) {
			repo := &geminiCachedContentHandlerRepoStub{}
			record := seedGeminiCachedContent(repo, "owned", 7, time.Now().Add(time.Hour))
			record.ChannelUsage.OriginalModel = "public-alias"
			h := newGeminiCachedContentHandlerForTest(repo)
			key := geminiCachedContentTestAPIKey(service.PlatformGemini)
			key.Group.Status = service.StatusActive
			deny := service.GroupModelAllowlist{Enabled: true, Mode: "deny", Models: []string{"public-alias"}}
			switch policy {
			case "key":
				key.ModelAllowlist = deny
			case "model-groups":
				key.ModelGroupRoutes = service.APIKeyModelGroupRoutes{"public-alias": {}}
			case "group":
				key.Group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{record.Model}}
			}
			c, w := newGeminiCachedContentTestContext(http.MethodPatch, "/v1beta/cachedContents/owned", key)
			c.Params = gin.Params{{Key: "cacheID", Value: "owned"}}
			c.Request.Body = io.NopCloser(strings.NewReader(`{"ttl":"7200s"}`))
			h.GeminiCachedContentsPatch(c)
			require.Equal(t, 404, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "public-alias")
			c, w = newGeminiCachedContentTestContext(http.MethodPost, "/v1beta/models/public-alias:generateContent", key)
			_, _, ok := h.resolveGeminiCachedContentBinding(c, zap.NewNop(), key, record.Model, []byte(`{"cachedContent":"cachedContents/owned"}`))
			require.False(t, ok)
			require.Equal(t, 404, w.Code)
		})
	}
}

func TestGeminiCacheListMergesBoundGroupsWithStablePagination(t *testing.T) {
	repo := &geminiCachedContentHandlerRepoStub{}
	seedGeminiCachedContent(repo, "first", 7, time.Now().Add(time.Hour))
	second := seedGeminiCachedContent(repo, "second", 7, time.Now().Add(time.Hour))
	second.GroupID = 99
	third := seedGeminiCachedContent(repo, "third", 7, time.Now().Add(time.Hour))
	third.GroupID = 123
	h := newGeminiCachedContentHandlerForTest(repo)
	key := geminiCachedContentTestAPIKey(service.PlatformGemini)
	key.GroupIDs = []int64{42, 99}
	key.Groups = []*service.Group{key.Group, {ID: 99, Platform: service.PlatformGemini}, {ID: 123, Platform: service.PlatformGemini}}
	c, w := newGeminiCachedContentTestContext(http.MethodGet, "/v1beta/cachedContents?pageSize=1", key)
	h.GeminiCachedContentsList(c)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "cachedContents/second")
	require.NotContains(t, w.Body.String(), "third")
	require.Contains(t, w.Body.String(), encodeGeminiCachedContentPageToken(second.ID))
	c, w = newGeminiCachedContentTestContext(http.MethodGet, "/v1beta/cachedContents?pageSize=1&pageToken="+encodeGeminiCachedContentPageToken(second.ID), key)
	h.GeminiCachedContentsList(c)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "cachedContents/first")
	require.NotContains(t, w.Body.String(), "nextPageToken")
}
