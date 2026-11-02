package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var errSeedanceImageStorageUnavailable = errors.New("reference image upload is unavailable; ask an administrator to configure image storage or use a public image URL")

func (s *OpenAIGatewayService) SetSeedanceImageStorage(resolve ImageStorageResolver) {
	s.seedanceImageStorage = resolve
}

const (
	SeedanceUpstreamProtocolCredentialKey = "seedance_upstream_protocol"
	SeedanceProtocolArk                   = "ark"
	SeedanceProtocolVideosJSON            = "videos_json"
)

func (a *Account) SeedanceUpstreamProtocol() string {
	if value := strings.TrimSpace(a.GetCredential(SeedanceUpstreamProtocolCredentialKey)); value != "" {
		return value
	}
	return SeedanceProtocolArk
}

// The Videos JSON contract has different content fields from Ark. Reject
// unsupported fields instead of silently changing the requested generation.
func seedanceVideosJSONBody(body []byte, model string) ([]byte, error) {
	out := map[string]any{"model": model}
	for key, value := range gjson.ParseBytes(body).Map() {
		switch key {
		case "model", "content":
		case "duration", "ratio", "resolution":
			out[key] = json.RawMessage(value.Raw)
		default:
			return nil, fmt.Errorf("videos JSON does not support the Ark field %q", key)
		}
	}
	texts := []string{}
	refs := map[string][]map[string]string{}
	for _, item := range gjson.GetBytes(body, "content").Array() {
		kind, role := item.Get("type").String(), item.Get("role").String()
		if kind == "text" {
			if item.Get("text").Type != gjson.String {
				return nil, fmt.Errorf("text content must be a string")
			}
			texts = append(texts, item.Get("text").String())
			continue
		}
		field := ""
		switch kind {
		case "image_url":
			field = "reference_images"
		case "video_url":
			field = "reference_videos"
		case "audio_url":
			field = "reference_audios"
		default:
			return nil, fmt.Errorf("Videos JSON does not support content type %q", kind)
		}
		raw := strings.TrimSpace(item.Get(kind + ".url").String())
		if kind != "image_url" || !strings.HasPrefix(raw, "data:") {
			if err := validateSeedanceMediaURL(raw); err != nil {
				return nil, err
			}
		}
		value := map[string]string{kind: raw}
		if kind == "image_url" && (role == "first_frame" || role == "last_frame") {
			if _, exists := out[role]; exists {
				return nil, fmt.Errorf("only one %s is supported", role)
			}
			out[role] = value
		} else {
			if role != "" && role != strings.TrimSuffix(field, "s") {
				return nil, fmt.Errorf("unsupported %s role %q", kind, role)
			}
			refs[field] = append(refs[field], value)
		}
	}
	if _, hasLast := out["last_frame"]; hasLast && out["first_frame"] == nil {
		return nil, fmt.Errorf("last_frame requires first_frame")
	}
	if out["first_frame"] != nil && len(refs) > 0 {
		return nil, fmt.Errorf("first/last frames cannot be combined with reference media")
	}
	for field, values := range refs {
		limit := 10
		if field == "reference_images" {
			limit = 30
		}
		if len(values) > limit {
			return nil, fmt.Errorf("%s supports at most %d items", field, limit)
		}
		out[field] = values
	}
	prompt := strings.TrimSpace(strings.Join(texts, "\n"))
	if prompt == "" && len(refs) == 0 && out["first_frame"] == nil {
		return nil, fmt.Errorf("a prompt or reference media is required")
	}
	if prompt != "" {
		out["prompt"] = prompt
	}
	return json.Marshal(out)
}

func validateSeedanceMediaURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || rejectPrivateImageHost(raw) != nil {
		return fmt.Errorf("Videos JSON requires public HTTP(S) media URLs")
	}
	return nil
}

// Convert uploaded first/reference frames only after validating the full request.
// Public URLs and native Ark uploads do not need object storage.
func (s *OpenAIGatewayService) storeSeedanceReferenceImages(ctx context.Context, body []byte) ([]byte, error) {
	paths := []string{"first_frame.image_url", "last_frame.image_url"}
	for i := range gjson.GetBytes(body, "reference_images").Array() {
		paths = append(paths, fmt.Sprintf("reference_images.%d.image_url", i))
	}
	type uploadedImage struct {
		path, contentType string
		data              []byte
	}
	var images []uploadedImage
	decoder := ImageResultUploader{maxDownloadBytes: 10 << 20}
	for _, path := range paths {
		raw := gjson.GetBytes(body, path).String()
		if !strings.HasPrefix(raw, "data:") {
			continue
		}
		data, _, err := decoder.decodeImageDataURL(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid reference image (PNG, JPG or WebP, up to 10 MB)")
		}
		info, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || (format != "png" && format != "jpeg" && format != "webp") || info.Width <= 0 || info.Height <= 0 || int64(info.Width) > 25_000_000/int64(info.Height) {
			return nil, fmt.Errorf("invalid reference image (PNG, JPG or WebP, up to 25 megapixels)")
		}
		images = append(images, uploadedImage{path: path, contentType: "image/" + format, data: data})
	}
	if len(images) == 0 {
		return body, nil
	}
	if s.seedanceImageStorage == nil {
		return nil, errSeedanceImageStorageUnavailable
	}
	uploader, enabled := s.seedanceImageStorage()
	if !enabled || uploader == nil || uploader.storage == nil {
		return nil, errSeedanceImageStorageUnavailable
	}
	for _, item := range images {
		if int64(len(item.data)) > uploader.maxDownloadBytes {
			return nil, fmt.Errorf("reference image exceeds the configured image storage size limit")
		}
	}
	id := "references/seedance-" + uuid.NewString()
	for i, item := range images {
		imageURL, err := uploader.storage.Save(ctx, uploader.buildKey(id, i, item.contentType), item.contentType, item.data)
		if err != nil || validateSeedanceMediaURL(imageURL) != nil {
			return nil, errSeedanceImageStorageUnavailable
		}
		body, err = sjson.SetBytes(body, item.path, imageURL)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func seedanceVideosJSONURL(base string, endpoint GrokMediaEndpoint, id string) (string, error) {
	target := buildOpenAIEndpointURL(base, "/v1/videos")
	if endpoint != SeedanceEndpointCreate {
		if id == "" || validateUpstreamPathSegment("video task ID", id) != nil {
			return "", fmt.Errorf("invalid video task ID")
		}
		target += "/" + url.PathEscape(id)
	}
	if endpoint == SeedanceEndpointContent {
		target += "/content"
	}
	return target, nil
}

func (s *OpenAIGatewayService) forwardSeedanceVideosJSON(ctx context.Context, c *gin.Context, account *Account, endpoint GrokMediaEndpoint, taskID string, body []byte) (*OpenAIForwardResult, error) {
	if endpoint == SeedanceEndpointDelete {
		c.Header("Allow", "GET")
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": gin.H{"type": "unsupported_operation", "message": "Videos JSON does not support deleting tasks"}})
		return nil, fmt.Errorf("Videos JSON task deletion is unsupported")
	}
	base, err := s.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return nil, err
	}
	id := strings.TrimPrefix(taskID, "seedance:")
	requestEndpoint := endpoint
	if endpoint == SeedanceEndpointContent {
		requestEndpoint = SeedanceEndpointStatus
	}
	target, err := seedanceVideosJSONURL(base, requestEndpoint, id)
	if err != nil {
		return nil, err
	}
	info := GrokMediaRequestInfo{}
	method, upstreamModel := http.MethodGet, ""
	if endpoint == SeedanceEndpointCreate {
		info, err = ParseSeedanceRequest(body)
		if err == nil {
			upstreamModel = account.GetMappedModel(info.Model)
			body, err = seedanceVideosJSONBody(body, upstreamModel)
		}
		if err == nil {
			body, err = s.storeSeedanceReferenceImages(ctx, body)
		}
		if err != nil {
			status, code := http.StatusBadRequest, "invalid_request_error"
			if errors.Is(err, errSeedanceImageStorageUnavailable) {
				status, code = http.StatusServiceUnavailable, "media_storage_unavailable"
			}
			c.JSON(status, gin.H{"error": gin.H{"type": code, "message": err.Error()}})
			return nil, err
		}
		method = http.MethodPost
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, fmt.Errorf("Seedance account missing API key")
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(ctx), method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, err
	}
	responseBody, readErr := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	// Never replay a potentially accepted asynchronous create.
	if resp.StatusCode >= 300 {
		if resp.StatusCode >= 400 {
			writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
		}
		return nil, fmt.Errorf("Videos JSON upstream status %d", resp.StatusCode)
	}
	invalid := func(message string) (*OpenAIForwardResult, error) {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "invalid_upstream_response", "message": message}})
		return nil, fmt.Errorf("%s", message)
	}
	if !gjson.ValidBytes(responseBody) || !gjson.ParseBytes(responseBody).IsObject() || (gjson.GetBytes(responseBody, "error").Type != gjson.Null && gjson.GetBytes(responseBody, "status").String() != "failed") {
		return invalid("Videos JSON upstream returned an invalid task response")
	}
	responseID := gjson.GetBytes(responseBody, "id")
	if endpoint == SeedanceEndpointCreate {
		if responseID.Type != gjson.String || validateUpstreamPathSegment("video task ID", responseID.String()) != nil || strings.TrimSpace(responseID.String()) == "" {
			return invalid("Videos JSON create response is missing a valid task ID")
		}
		id = responseID.String()
	} else if responseID.Exists() && (responseID.Type != gjson.String || responseID.String() != id) {
		return invalid("Videos JSON upstream returned a different task ID")
	}
	status := gjson.GetBytes(responseBody, "status").String()
	switch status {
	case "queued":
	case "processing":
		status = "running"
	case "completed":
		status = "succeeded"
	case "failed":
	case "":
		if endpoint == SeedanceEndpointCreate {
			status = "queued"
		} else {
			return invalid("Videos JSON response is missing task status")
		}
	default:
		return invalid("Videos JSON upstream returned an unsupported task status")
	}
	result := &OpenAIForwardResult{Model: info.Model, BillingModel: info.Model, UpstreamModel: upstreamModel,
		ResponseID: SeedanceTaskKey(id), Duration: time.Since(started), ResponseHeaders: resp.Header.Clone(),
		VideoResolution: info.Resolution, VideoDurationSeconds: info.DurationSeconds}
	// Creation only records the task. Polling and downloading share completion billing.
	if endpoint != SeedanceEndpointCreate && status == "succeeded" {
		result.VideoCount = 1
		result.UpstreamModel = gjson.GetBytes(responseBody, "model").String()
		result.VideoDurationSeconds = max(0, int(gjson.GetBytes(responseBody, "duration").Int()))
		result.VideoResolution = strings.TrimSpace(gjson.GetBytes(responseBody, "resolution").String())
	}
	if endpoint == SeedanceEndpointContent {
		if status != "succeeded" {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"type": "video_not_ready", "message": "Video is not completed"}})
			return nil, fmt.Errorf("video is not completed")
		}
		target, err = seedanceVideosJSONURL(base, SeedanceEndpointContent, id)
		if err != nil {
			return nil, err
		}
		if err := s.streamSeedanceVideosJSONContent(ctx, c, account, target, token, proxy); err != nil {
			return result, err
		}
		return result, nil
	}
	var normalized map[string]json.RawMessage
	_ = json.Unmarshal(responseBody, &normalized)
	normalized["id"], _ = json.Marshal(id)
	normalized["status"], _ = json.Marshal(status)
	if status == "succeeded" {
		normalized["content"], _ = json.Marshal(map[string]string{"video_url": "/api/v3/contents/generations/tasks/" + url.PathEscape(id) + "/content"})
	}
	normalizedBody, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	writeGrokMediaResponse(c, resp, normalizedBody, s.responseHeaderFilter)
	return result, nil
}

func (s *OpenAIGatewayService) streamSeedanceVideosJSONContent(ctx context.Context, c *gin.Context, account *Account, target, token, proxy string) error {
	origin, _ := url.Parse(target)
	withCredentials := true
	for redirects := 0; redirects <= 3; redirects++ {
		requestCtx := WithHTTPUpstreamRedirectsDisabled(ctx)
		if !withCredentials {
			requestCtx = WithHTTPUpstreamPublicHostsOnly(requestCtx)
		}
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "video/*, application/octet-stream")
		for _, name := range []string{"Range", "If-Range"} {
			if value := c.GetHeader(name); value != "" {
				req.Header.Set(name, value)
			}
		}
		if withCredentials {
			req.Header.Set("Authorization", "Bearer "+token)
			account.ApplyHeaderOverrides(req.Header)
		}
		resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location, err := resp.Location()
			_ = resp.Body.Close()
			if err != nil {
				return fmt.Errorf("video content redirect has no valid location")
			}
			target = location.String()
			// Validate without normalizing signed paths or query strings.
			_, err = s.validateOutboundURL(target)
			if err != nil {
				return fmt.Errorf("video content redirect is not allowed")
			}
			if location.User != nil || location.Scheme != "https" || rejectPrivateImageHost(target) != nil {
				return fmt.Errorf("video content redirect is not a public HTTPS URL")
			}
			withCredentials = withCredentials && strings.EqualFold(location.Host, origin.Host) && location.Scheme == origin.Scheme
			continue
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 400 && resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
			body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
			if err != nil {
				return err
			}
			writeGrokMediaResponse(c, resp, body, s.responseHeaderFilter)
			return fmt.Errorf("video content upstream status %d", resp.StatusCode)
		}
		return writeGrokMediaContentResponse(c, resp)
	}
	return fmt.Errorf("too many video content redirects")
}
