//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const imageModerationJSON = `{"error":{"message":"Your request was rejected by the safety system. safety_violations=[sexual].","type":"image_generation_user_error","code":"moderation_blocked","moderation_details":{"moderation_stage":"output","categories":["sexual"]}}}`

func TestForwardImagesAPIKeyRejectsJSONErrorWithHTTP200(t *testing.T) {
	for _, keepalive := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_headers", true: "after_keepalive"}[keepalive], func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2.6-flare","prompt":"test"}`)
			c, rec := newOpenAIImagesTestContext(t, body)
			if keepalive {
				stop := StartOpenAIImagesJSONKeepalive(c, time.Hour)
				defer stop()
				require.True(t, openAIImagesJSONKeepaliveFromContext(c).beat())
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"req_moderation"}},
				Body:       io.NopCloser(strings.NewReader(imageModerationJSON)),
			}}
			svc := newOpenAIImagesTestService(upstream)
			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			result, err := svc.ForwardImages(context.Background(), c, newOpenAIImagesAPIKeyAccount(), body, parsed, "")
			var upstreamErr *OpenAIImagesUpstreamError
			require.ErrorAs(t, err, &upstreamErr)
			require.Nil(t, result, "a safety refusal must not produce billable image usage")
			require.Equal(t, http.StatusBadRequest, upstreamErr.StatusCode)
			require.Equal(t, "moderation_blocked", upstreamErr.Code)
			require.False(t, IsOpenAIImagesRetryableUpstreamError(upstreamErr))
			require.Equal(t, "moderation_blocked", gjson.Get(rec.Body.String(), "error.code").String())
			require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "safety_violations=[sexual]")
			if keepalive {
				require.Equal(t, http.StatusOK, rec.Code)
			} else {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestImageTaskServiceRejectsJSONErrorWithHTTP200(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)
	owner := ImageTaskOwner{UserID: 1, APIKeyID: 2}
	task, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	require.NoError(t, svc.Complete(context.Background(), task.ID, http.StatusOK, []byte(" \n"+imageModerationJSON)))
	got, err := svc.Get(context.Background(), owner, task.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, http.StatusBadRequest, got.HTTPStatus)
	require.Empty(t, got.Result)
	require.Equal(t, "moderation_blocked", gjson.GetBytes(got.Error, "code").String())
	require.Equal(t, "output", gjson.GetBytes(got.Error, "moderation_details.moderation_stage").String())
}
