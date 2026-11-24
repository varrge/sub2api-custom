package requestmodel

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/stretchr/testify/require"
)

func TestFromBodyJSONModel(t *testing.T) {
	if got := FromBody("application/json", []byte(`{"model":"gpt-5.4"}`)); got != "gpt-5.4" {
		t.Fatalf("got %q", got)
	}
}

func TestFromBodyJSONSessionModel(t *testing.T) {
	if got := FromBody("application/json", []byte(`{"session":{"model":"gpt-live"},"sdp":"v=0"}`)); got != "gpt-live" {
		t.Fatalf("got %q", got)
	}
}

func TestFromBodyJSONModelWinsOverSession(t *testing.T) {
	body := []byte(`{"model":"top-level","session":{"model":"nested"}}`)
	if got := FromBody("application/json", body); got != "top-level" {
		t.Fatalf("got %q", got)
	}
}

func TestFromBodyJSONBlankModelFallsThrough(t *testing.T) {
	if got := FromBody("application/json", []byte(`{"model":"  "}`)); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFromBodyMultipartModelField(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("prompt", "draw")
	_ = writer.WriteField("model", "gpt-image-1")
	_ = writer.Close()

	if got := FromBody(writer.FormDataContentType(), body.Bytes()); got != "gpt-image-1" {
		t.Fatalf("got %q", got)
	}
}

func TestFromBodyMultipartSessionField(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("sdp", "v=0")
	_ = writer.WriteField("session", `{"model":"live-alias"}`)
	_ = writer.WriteField("media", "file-part") // 字段名不匹配，应被跳过
	_ = writer.Close()

	if got := FromBody(writer.FormDataContentType(), body.Bytes()); got != "live-alias" {
		t.Fatalf("got %q", got)
	}
}

func TestFromBodySkipsFilePartsBeforeModelField(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "input.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write(bytes.Repeat([]byte{0}, 4096))
	_ = writer.WriteField("model", "gpt-image-1")
	_ = writer.Close()

	if got := FromBody(writer.FormDataContentType(), body.Bytes()); got != "gpt-image-1" {
		t.Fatalf("got %q", got)
	}
}

func TestFromBodyNoModelReturnsEmpty(t *testing.T) {
	if got := FromBody("application/json", []byte(`{"input":"hi"}`)); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := FromBody("text/plain", []byte("plain")); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestResetRequestBodyAllowsRereadWithoutCopy(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	ResetRequestBody(req, body)

	got, err := httputil.ReadRequestBodyWithPrealloc(req)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if &got[0] != &body[0] {
		t.Fatalf("expected zero-copy reread of the same slice")
	}
	if req.ContentLength != int64(len(body)) {
		t.Fatalf("content length mismatch: %d", req.ContentLength)
	}
	if req.Header.Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Fatalf("content-length header mismatch: %q", req.Header.Get("Content-Length"))
	}
}

func TestResetRequestBodyNilRequestIsNoop(t *testing.T) {
	ResetRequestBody(nil, []byte("x"))
}

func TestFromBodyForRouteLivePrefersSessionModel(t *testing.T) {
	body := []byte(`{"model":"top-level","session":{"model":"session-model"},"sdp":"v=0"}`)

	if got := FromBodyForRoute("/v1/live", "application/json", body); got != "session-model" {
		t.Fatalf("live route should prefer session.model, got %q", got)
	}
	if got := FromBodyForRoute("/backend-api/codex/realtime/calls", "application/json", body); got != "session-model" {
		t.Fatalf("codex realtime route should prefer session.model, got %q", got)
	}
	if got := FromBodyForRoute("/live", "application/json", body); got != "session-model" {
		t.Fatalf("root live alias should prefer session.model, got %q", got)
	}
}

func TestFromBodyForRouteNonLivePrefersTopLevelModel(t *testing.T) {
	body := []byte(`{"model":"top-level","session":{"model":"session-model"}}`)

	if got := FromBodyForRoute("/v1/messages", "application/json", body); got != "top-level" {
		t.Fatalf("non-live route should prefer top-level model, got %q", got)
	}
	if got := FromBodyForRoute("/v1/responses", "application/json", body); got != "top-level" {
		t.Fatalf("responses route should prefer top-level model, got %q", got)
	}
}

func TestFromBodyForRouteFallsBackWhenPreferredFieldMissing(t *testing.T) {
	if got := FromBodyForRoute("/v1/live", "application/json", []byte(`{"model":"top-level"}`)); got != DefaultLiveModel {
		t.Fatalf("live without session.model should use its effective default, got %q", got)
	}
	if got := FromBodyForRoute("/v1/messages", "application/json", []byte(`{"session":{"model":"session-model"}}`)); got != "session-model" {
		t.Fatalf("non-live without model should fall back to session.model, got %q", got)
	}
}

func TestFromBodyForRouteMultipartSessionFirstForLive(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "top-level")
	_ = writer.WriteField("session", `{"model":"session-model"}`)
	_ = writer.Close()
	contentType := writer.FormDataContentType()

	if got := FromBodyForRoute("/v1/live", contentType, body.Bytes()); got != "session-model" {
		t.Fatalf("live multipart should prefer session field, got %q", got)
	}
	if got := FromBodyForRoute("/v1/messages", contentType, body.Bytes()); got != "top-level" {
		t.Fatalf("non-live multipart should prefer model field, got %q", got)
	}
}

func TestIsLiveRequestRoute(t *testing.T) {
	for _, path := range []string{"/v1/live", "/live", "/backend-api/codex/realtime/calls"} {
		if !IsLiveRequestRoute(path) {
			t.Fatalf("%s should be a live route", path)
		}
	}
	for _, path := range []string{"/v1/messages", "/v1/responses", "/realtime", ""} {
		if IsLiveRequestRoute(path) {
			t.Fatalf("%s should not be a live route", path)
		}
	}
}

// gjson 的部分扫描会在 multipart 字节流里误匹配 session JSON 内的 model；
// 声明为 multipart 的请求体必须走 multipart 解析。
func TestFromBodyMultipartContentTypeSkipsJSONScan(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "top-level")
	_ = writer.WriteField("session", `{"model":"session-model"}`)
	_ = writer.Close()

	if got := FromBody(writer.FormDataContentType(), body.Bytes()); got != "top-level" {
		t.Fatalf("multipart body should use the model field, got %q", got)
	}
	if got := FromBodyForRoute("/v1/live", writer.FormDataContentType(), body.Bytes()); got != "session-model" {
		t.Fatalf("live multipart should use the session field, got %q", got)
	}
}

func TestFromBodyCandidatesIncludesCaseVariantsAndDuplicates(t *testing.T) {
	// encoding/json 绑定对键名大小写不敏感、重复键取末值；gjson 取首个。
	// 候选集必须包含全部出现。
	got := FromBodyCandidates("", "application/json", []byte(`{"Model":"caps-model"}`))
	if len(got) != 1 || got[0] != "caps-model" {
		t.Fatalf("case-variant key must be a candidate, got %#v", got)
	}

	got = FromBodyCandidates("", "application/json", []byte(`{"model":"first","model":"last"}`))
	if len(got) != 2 || got[0] != "first" || got[1] != "last" {
		t.Fatalf("duplicate keys must both be candidates, got %#v", got)
	}
}

func TestFromBodyCandidatesSessionVariants(t *testing.T) {
	body := []byte(`{"Session":{"Model":"s-caps"},"session":{"model":"s-lower"},"model":"top"}`)

	// 非 Live 入口：顶层候选。
	got := FromBodyCandidates("", "application/json", body)
	if len(got) != 1 || got[0] != "top" {
		t.Fatalf("non-live should use top-level candidates, got %#v", got)
	}

	// Live 入口：session 候选（含大小写变体与重复）。
	got = FromBodyCandidates("/v1/live", "application/json", body)
	if len(got) != 2 || got[0] != "s-caps" || got[1] != "s-lower" {
		t.Fatalf("live should collect all session candidates, got %#v", got)
	}
}

func TestFromBodyCandidatesMultipartDuplicateFields(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "gpt-image-1")
	_ = writer.WriteField("model", "gpt-image-1.5")
	_ = writer.WriteField("session", `{"model":"live-a"}`)
	_ = writer.WriteField("session", `{"model":"live-b"}`)
	_ = writer.Close()

	models, sessions := multipartModelCandidates(writer.FormDataContentType(), body.Bytes())
	if len(models) != 2 || models[0] != "gpt-image-1" || models[1] != "gpt-image-1.5" {
		t.Fatalf("duplicate model fields must all be candidates, got %#v", models)
	}
	if len(sessions) != 2 {
		t.Fatalf("duplicate session fields must all be captured, got %#v", sessions)
	}

	got := FromBodyCandidates("", writer.FormDataContentType(), body.Bytes())
	if len(got) != 2 || got[0] != "gpt-image-1" || got[1] != "gpt-image-1.5" {
		t.Fatalf("non-live multipart candidates mismatch, got %#v", got)
	}

	got = FromBodyCandidates("/v1/live", writer.FormDataContentType(), body.Bytes())
	if len(got) != 2 || got[0] != "live-a" || got[1] != "live-b" {
		t.Fatalf("live multipart session candidates mismatch, got %#v", got)
	}
}

func TestFromBodyForRouteReturnsFirstCandidate(t *testing.T) {
	got := FromBodyForRoute("", "application/json", []byte(`{"model":"first","model":"last"}`))
	if got != "first" {
		t.Fatalf("dispatch helper keeps gjson-first semantics, got %q", got)
	}
}

func TestValidateBodyRejectsAmbiguousRequestFields(t *testing.T) {
	for _, tc := range []struct{ name, route, body, want string }{
		{"duplicate", "/v1/responses", `{"model":"a","model":"b"}`, "model is specified more than once"},
		{"same value", "/v1/responses", `{"model":"a","model":"a"}`, "model is specified more than once"},
		{"case and null", "/v1/responses", `{"model":"a","MODEL":null}`, "model is specified more than once"},
		{"escaped", "/v1/responses", `{"model":"a","\u006dodel":123}`, "model is specified more than once"},
		{"nested", "/v1/responses", `{"model":"a","input":[{"model":"b","model":"c"}],"stream":true,"stream":false}`, "input[0].model is specified more than once"},
		{"live session", "/v1/live", `{"session":{"model":"a","Model":"b"}}`, "session.model is specified more than once"},
		{"live carriers", "/backend-api/codex/realtime/calls", `{"session":{"model":"a"},"Session":null}`, "session is specified more than once"},
		{"independent session", "/v1/responses", `{"model":"a","session":{"model":"b","model":"c"}}`, "session.model is specified more than once"},
		{"malformed left to endpoint", "/v1/responses", `{"model":`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBody(tc.route, "application/json", []byte(tc.body))
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.want)
			}
		})
	}
}

func TestValidateBodyMultipartModelAndSession(t *testing.T) {
	for _, tc := range []struct {
		name, route string
		fields      [][2]string
		want        string
	}{
		{"duplicate model", "/v1/images/edits", [][2]string{{"model", "a"}, {"Model", "b"}}, "model is specified more than once"},
		{"duplicate session", "/v1/live", [][2]string{{"session", `{"model":"a"}`}, {"session", `{"model":"a"}`}}, "session is specified more than once"},
		{"session model", "/v1/live", [][2]string{{"session", `{"model":"a","model":"b"}`}}, "session.model is specified more than once"},
		{"multiple images", "/v1/images/edits", [][2]string{{"model", "a"}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			w := multipart.NewWriter(&body)
			for _, field := range tc.fields {
				require.NoError(t, w.WriteField(field[0], field[1]))
			}
			for range 2 {
				p, err := w.CreateFormFile("image", "a.png")
				require.NoError(t, err)
				_, err = p.Write([]byte(`{"model":"file data"}`))
				require.NoError(t, err)
			}
			require.NoError(t, w.Close())
			err := ValidateBody(tc.route, w.FormDataContentType(), body.Bytes())
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.want)
			}
		})
	}
}

func TestValidateStatefulFrameUsesSharedConflictRules(t *testing.T) {
	for _, body := range []string{
		`{"type":"response.create","response":{"model":"a","Model":null}}`,
		`{"type":"session.update","session":{"model":"a"},"Session":{}}`,
		`{"type":"response.create","response":{},"Response":null}`,
		`{"model":"a","\u006dodel":null}`,
	} {
		require.ErrorContains(t, ValidateStatefulFrame([]byte(body)), "specified more than once")
	}
	require.EqualError(t, ValidateStatefulFrame([]byte(`{"model":"a","input":[{"model":"b","model":"c"}]}`)), "input[0].model is specified more than once")
	require.NoError(t, ValidateStatefulFrame([]byte(`{"model":"a","arguments":{"id":"b","ID":"c"}}`)))
	require.EqualError(t, ValidateStatefulFrame([]byte(`{"model":"a","arguments":{"id":"b","id":"c"}}`)), "arguments.id is specified more than once")
}

func TestValidateBodyLenientControlsBeforeRouting(t *testing.T) {
	for _, body := range [][]byte{
		append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"model":"a","Model":"b"}`)...),
		[]byte("{\"model\":\"a\",\"input\":\"line\nline\",\"model\":\"b\"}"),
	} {
		require.ErrorContains(t, ValidateBody("/v1/responses", "application/json", body), "specified more than once")
	}
	require.NoError(t, ValidateBody("/v1/responses", "application/json", []byte("{\"model\":\"a\",\"input\":\"line\nline\"}")))
}

func TestGeminiCacheModelBoundaryPreservesAmbiguityChecks(t *testing.T) {
	body := []byte(`{"model":"models/gemini-2.5-pro"}`)
	require.Equal(t, []string{"gemini-2.5-pro"}, FromBodyCandidates("/v1beta/cachedContents", "application/json", body))
	require.Equal(t, []string{"models/gemini-2.5-pro"}, FromBodyCandidates("/v1/chat/completions", "application/json", body))
	for _, body := range []string{
		`{"model":"models/allowed","model":"models/denied"}`,
		`{"model":"models/allowed","Model":"models/denied"}`,
		`{"model":"models/allowed","model":null}`,
	} {
		require.Error(t, ValidateBody("/v1beta/cachedContents", "application/json", []byte(body)))
	}
}

// 回归：JSON handler 不看 Content-Type、宽容读取会剥掉 BOM，准入候选必须与之
// 一致，否则把请求体标成 multipart 或加 BOM 就能让白名单拿不到任何模型而放行。
func TestFromBodyCandidatesFollowsWhatJSONHandlersParse(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-8","messages":[]}`)
	cases := []struct {
		name        string
		contentType string
		body        []byte
	}{
		{name: "multipart 标签", contentType: "multipart/form-data; boundary=zzz", body: body},
		{name: "无 boundary 的 multipart 标签", contentType: "multipart/form-data", body: body},
		{name: "UTF-8 BOM", contentType: "application/json", body: append([]byte{0xEF, 0xBB, 0xBF}, body...)},
		{name: "BOM 加 multipart 标签", contentType: "multipart/form-data", body: append([]byte{0xEF, 0xBB, 0xBF}, body...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FromBodyCandidates("/v1/messages", tc.contentType, tc.body)
			if len(got) != 1 || got[0] != "claude-opus-4-8" {
				t.Fatalf("candidates = %v, want [claude-opus-4-8]", got)
			}
			if route := FromBodyForRoute("/v1/messages", tc.contentType, tc.body); route != "claude-opus-4-8" {
				t.Fatalf("route model = %q", route)
			}
		})
	}
}

// 回归：multipart handler 跳过首个 boundary 之前的前导内容，前导内容是 JSON 时
// 白名单必须同时看到表单里的 model，不能只取 JSON 解读。
func TestFromBodyCandidatesMultipartWithJSONPreamble(t *testing.T) {
	body := []byte("{}\r\n--B\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\ngpt-image-2\r\n--B--\r\n")
	got := FromBodyCandidates("/v1/images/edits", "multipart/form-data; boundary=B", body)
	if len(got) != 1 || got[0] != "gpt-image-2" {
		t.Fatalf("candidates = %v, want [gpt-image-2]", got)
	}
}

func TestFromBodyCandidatesDefaultsSurviveBOMAndContentTypeVariants(t *testing.T) {
	for _, tc := range []struct {
		route, body string
		want        []string
	}{
		{"/v1/live", `{"model":"hint"}`, []string{DefaultLiveModel}},
		{"/v1/live", `{"session":{"Model":"hint"}}`, []string{DefaultLiveModel, "hint"}},
		{"/v1/live", `{"Session":{"model":"explicit"}}`, []string{"explicit"}},
		{"/v1/images/generations", `{"Model":"hint"}`, []string{DefaultOpenAIImagesModel, "hint"}},
		{"/v1/images/edits", `{"model":"explicit"}`, []string{"explicit"}},
		{"/v1/images/generations/async", `{}`, []string{DefaultOpenAIImagesModel}},
	} {
		for _, contentType := range []string{"application/json", "multipart/form-data", "multipart/form-data; boundary=none"} {
			for _, prefix := range []string{"", "\xef\xbb\xbf"} {
				require.Equal(t, tc.want, FromBodyCandidates(tc.route, contentType, []byte(prefix+tc.body)), "%s %s %q", tc.route, contentType, prefix)
			}
		}
	}
}
