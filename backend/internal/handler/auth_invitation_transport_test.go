package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestInvitationValidationAcceptsOptionalCodesAndRejectsExpiry(t *testing.T) {
	h, client := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		settingValues: map[string]string{service.SettingKeyInvitationCodeEnabled: "false"},
	})
	t.Cleanup(func() { _ = client.Close() })
	h.redeemService = service.NewRedeemService(&oauthPendingFlowRedeemCodeRepo{client: client}, nil, nil, nil, nil, client, nil, nil)
	for _, test := range []struct {
		code, kind, status string
		expiry             time.Time
		valid              bool
	}{
		{"OPTIONAL", "invitation", "unused", time.Now().Add(time.Hour), true},
		{"EXPIRED", "invitation", "unused", time.Now().Add(-time.Hour), false},
		{"USED", "invitation", "used", time.Now().Add(time.Hour), false},
		{"BALANCE", "balance", "unused", time.Now().Add(time.Hour), false},
	} {
		t.Run(test.code, func(t *testing.T) {
			_, err := client.RedeemCode.Create().SetCode(test.code).SetType(test.kind).SetStatus(test.status).SetExpiresAt(test.expiry).Save(context.Background())
			require.NoError(t, err)
			c, w := invitationHandlerContext(http.MethodPost, "/api/v1/auth/validate-invitation-code", `{"code":" `+test.code+` "}`, 0)
			h.ValidateInvitationCode(c)
			require.Equal(t, http.StatusOK, w.Code)
			var result struct {
				Data ValidateInvitationCodeResponse
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
			require.Equal(t, test.valid, result.Data.Valid, w.Body.String())
		})
	}
}

func TestOAuthInvitationTransportStartsAndPersistsForEveryProvider(t *testing.T) {
	for _, provider := range []string{"linuxdo", "wechat", "oidc", "github", "google", "dingtalk"} {
		t.Run(provider, func(t *testing.T) {
			h, client := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
				settingValues: map[string]string{
					service.SettingKeyWeChatConnectEnabled:     "true",
					service.SettingKeyWeChatConnectAppID:       "wechat-app",
					service.SettingKeyWeChatConnectAppSecret:   "wechat-secret",
					service.SettingKeyWeChatConnectMode:        "open",
					service.SettingKeyWeChatConnectRedirectURL: "https://app.example/api/v1/auth/oauth/wechat/callback",
					service.SettingKeyGitHubOAuthEnabled:       "true",
					service.SettingKeyGitHubOAuthClientID:      "github-client",
					service.SettingKeyGitHubOAuthClientSecret:  "github-secret",
					service.SettingKeyGitHubOAuthRedirectURL:   "https://app.example/api/v1/auth/oauth/github/callback",
					service.SettingKeyGoogleOAuthEnabled:       "true",
					service.SettingKeyGoogleOAuthClientID:      "google-client",
					service.SettingKeyGoogleOAuthClientSecret:  "google-secret",
					service.SettingKeyGoogleOAuthRedirectURL:   "https://app.example/api/v1/auth/oauth/google/callback",
				},
			})
			t.Cleanup(func() { _ = client.Close() })
			h.cfg = &config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}
			start := h.GitHubOAuthStart
			switch provider {
			case "linuxdo":
				h.settingSvc = nil
				h.cfg.LinuxDo = config.LinuxDoConnectConfig{Enabled: true, AuthorizeURL: "https://provider.example/authorize", RedirectURL: "https://app.example/callback"}
				start = h.LinuxDoOAuthStart
			case "oidc":
				h.settingSvc = nil
				h.cfg.OIDC = config.OIDCConnectConfig{Enabled: true, AuthorizeURL: "https://provider.example/authorize", RedirectURL: "https://app.example/callback"}
				start = h.OIDCOAuthStart
			case "dingtalk":
				h.settingSvc = nil
				h.cfg.DingTalk = config.DingTalkConnectConfig{Enabled: true, AuthorizeURL: "https://provider.example/authorize", RedirectURL: "https://app.example/callback"}
				start = h.DingTalkOAuthStart
			case "wechat":
				start = h.WeChatOAuthStart
			case "google":
				start = h.GoogleOAuthStart
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "https://app.example/api/v1/auth/oauth/"+provider+"/start?invitation_code=%20OPTIONAL123%20", nil)
			start(c)
			require.Equal(t, http.StatusFound, recorder.Code, recorder.Body.String())
			location, err := url.Parse(recorder.Header().Get("Location"))
			require.NoError(t, err)
			state := location.Query().Get("state")
			require.NotEmpty(t, state)
			require.Empty(t, location.Query().Get("invitation_code"))
			cookie := findCookie(recorder.Result().Cookies(), oauthInvitationCookieName(provider))
			require.NotNil(t, cookie)
			require.True(t, cookie.HttpOnly)
			require.True(t, cookie.Secure)
			require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)

			callbackRecorder := httptest.NewRecorder()
			callback, _ := gin.CreateTestContext(callbackRecorder)
			callback.Request = httptest.NewRequest(http.MethodGet, "/callback?invitation_code=UNTRUSTED", nil)
			callback.Request.AddCookie(cookie)
			require.NoError(t, h.restoreOAuthInvitationCode(callback, provider, state))
			require.Equal(t, "OPTIONAL123", readOAuthInvitationCode(callback))
			require.NoError(t, h.createOAuthPendingSession(callback, oauthPendingSessionPayload{
				Intent: oauthIntentLogin,
				Identity: service.PendingAuthIdentityKey{
					ProviderType: provider, ProviderKey: provider, ProviderSubject: "subject",
				},
				ResolvedEmail:          "new@example.com",
				BrowserSessionKey:      "browser-session",
				CompletionResponse:     map[string]any{"redirect": "/dashboard"},
				UpstreamIdentityClaims: map[string]any{"username": "new"},
			}))
			session, err := client.PendingAuthSession.Query().Only(context.Background())
			require.NoError(t, err)
			require.Equal(t, "OPTIONAL123", pendingOAuthInvitationCode(session, nil))
			replacement := " REPLACEMENT "
			require.Equal(t, "REPLACEMENT", pendingOAuthInvitationCode(session, &replacement))
			removed := ""
			require.Empty(t, pendingOAuthInvitationCode(session, &removed))
		})
	}
}

func TestOAuthInvitationTransportRejectsChangedContext(t *testing.T) {
	h := &AuthHandler{cfg: &config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}}
	for _, scenario := range []string{"valid", "state", "provider", "signature", "expired", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			claims := oauthInvitationClaims{
				Code: "INVITE123",
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer: "oauth-registration-invitation", ID: "state",
					Audience: jwt.ClaimStrings{"linuxdo"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
				},
			}
			provider, state, secret := "linuxdo", "state", "test-secret"
			switch scenario {
			case "state":
				state = "other-state"
			case "provider":
				provider = "google"
			case "signature":
				secret = "wrong-secret"
			case "expired":
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
			}
			value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
			require.NoError(t, err)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/callback?invitation_code=UNTRUSTED", nil)
			if scenario != "missing" {
				c.Request.AddCookie(&http.Cookie{Name: oauthInvitationCookieName(provider), Value: value})
			}
			err = h.restoreOAuthInvitationCode(c, provider, state)
			switch scenario {
			case "valid":
				require.NoError(t, err)
				require.Equal(t, "INVITE123", readOAuthInvitationCode(c))
			case "missing":
				require.NoError(t, err)
				require.Empty(t, readOAuthInvitationCode(c))
			default:
				require.Error(t, err)
				require.Empty(t, readOAuthInvitationCode(c))
			}
		})
	}
}

func TestOAuthInvitationTransportClearsStaleCodeForNewStartAndBind(t *testing.T) {
	h := &AuthHandler{cfg: &config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}}
	for _, query := range []string{"", "?invitation_code=INVITE123&intent=bind_current_user"} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/start"+query, nil)
		require.NoError(t, h.captureOAuthInvitationCode(c, "linuxdo", "state", false))
		requireCookieCleared(t, recorder, oauthInvitationCookieName("linuxdo"))
	}
}

func TestOAuthInvitationTransportCompletionUsesStoredOptionalCode(t *testing.T) {
	for _, provider := range []string{"linuxdo", "wechat", "oidc", "github", "google", "dingtalk", "pending"} {
		t.Run(provider, func(t *testing.T) {
			var h *AuthHandler
			var client *dbent.Client
			if provider == "pending" {
				h, client = newOAuthPendingFlowTestHandlerWithEmailVerification(t, false, "new@example.com", "246810")
			} else {
				h, client = newOAuthPendingFlowTestHandler(t, false)
			}
			t.Cleanup(func() { _ = client.Close() })
			ctx := context.Background()
			invite, err := client.RedeemCode.Create().SetCode("OPTIONAL123").
				SetType(service.RedeemTypeInvitation).SetStatus(service.StatusUnused).SetValue(0).Save(ctx)
			require.NoError(t, err)
			sessionProvider := provider
			if provider == "pending" {
				sessionProvider = "oidc"
			}
			session, err := client.PendingAuthSession.Create().
				SetSessionToken("session").SetBrowserSessionKey("browser").
				SetIntent(oauthIntentLogin).SetProviderType(sessionProvider).SetProviderKey(sessionProvider).
				SetProviderSubject("new-subject").SetResolvedEmail("new@example.com").
				SetUpstreamIdentityClaims(map[string]any{"username": "new"}).
				SetLocalFlowState(map[string]any{
					oauthInvitationCodeStateKey: "OPTIONAL123",
					oauthCompletionResponseKey:  map[string]any{"error": "invitation_required"},
				}).
				SetExpiresAt(time.Now().Add(time.Minute)).Save(ctx)
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/complete-registration", strings.NewReader(`{"password":"secret-123","email":"new@example.com","verify_code":"246810"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.AddCookie(encodedCookie(oauthPendingSessionCookieName, session.SessionToken))
			c.Request.AddCookie(encodedCookie(oauthPendingBrowserCookieName, "browser"))
			switch provider {
			case "linuxdo":
				h.CompleteLinuxDoOAuthRegistration(c)
			case "wechat":
				h.CompleteWeChatOAuthRegistration(c)
			case "oidc":
				h.CompleteOIDCOAuthRegistration(c)
			case "dingtalk":
				h.CompleteDingTalkOAuthRegistration(c)
			case "pending":
				h.createPendingOAuthAccount(c, "oidc")
			default:
				h.completeEmailOAuthRegistration(c, provider)
			}
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			user, err := client.User.Query().Where(dbuser.EmailEQ("new@example.com")).Only(ctx)
			require.NoError(t, err)
			invite, err = client.RedeemCode.Get(ctx, invite.ID)
			require.NoError(t, err)
			require.Equal(t, service.StatusUsed, invite.Status)
			require.Equal(t, user.ID, *invite.UsedBy)
		})
	}
}

func TestOAuthInvitationTransportExistingEmailLoginIgnoresInvalidCode(t *testing.T) {
	h, client := newOAuthPendingFlowTestHandler(t, false)
	t.Cleanup(func() { _ = client.Close() })
	_, err := client.User.Create().SetEmail("existing@example.com").SetPasswordHash("hash").
		SetRole(service.RoleUser).SetStatus(service.StatusActive).Save(context.Background())
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/callback", nil)
	c.Set(oauthInvitationCodeStateKey, "INVALID")
	h.emailOAuthCallbackWithProfile(c, "google", config.EmailOAuthProviderConfig{},
		"/auth/oauth/callback", "/dashboard", &emailOAuthProfile{
			Subject: "existing-subject", Email: "existing@example.com", EmailVerified: true,
		})
	require.Equal(t, http.StatusFound, recorder.Code)
	require.NotEmpty(t, parseOAuthRedirectFragment(t, recorder.Header().Get("Location")).Get("access_token"))
}

func TestOAuthInvitationTransportOIDCFastPathConsumesOptionalCode(t *testing.T) {
	h, client := newOAuthPendingFlowTestHandler(t, false)
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	invite, err := client.RedeemCode.Create().SetCode("OPTIONAL123").
		SetType(service.RedeemTypeInvitation).SetStatus(service.StatusUnused).SetValue(0).Save(ctx)
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/callback", nil)
	c.Set(oauthInvitationCodeStateKey, "OPTIONAL123")
	require.True(t, h.tryOIDCVerifiedEmailFastPath(c, "/auth/oidc/callback", "/dashboard",
		service.PendingAuthIdentityKey{ProviderType: "oidc", ProviderKey: "oidc", ProviderSubject: "new-subject"},
		"new@example.com", "new", nil))
	invite, err = client.RedeemCode.Get(ctx, invite.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUsed, invite.Status)
	require.NotNil(t, invite.UsedBy)
}

func TestOAuthInvitationTransportLinuxDoCallbackConsumesCodeOrRejectsRegistration(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"provider-token","token_type":"Bearer"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"123","username":"new"}`))
	}))
	defer upstream.Close()
	for _, valid := range []bool{true, false} {
		name := "valid"
		if !valid {
			name = "invalid"
		}
		t.Run(name, func(t *testing.T) {
			h, client := newLinuxDoOAuthHandlerAndClient(t, false, config.LinuxDoConnectConfig{
				Enabled: true, AuthorizeURL: upstream.URL + "/authorize",
				TokenURL: upstream.URL + "/token", UserInfoURL: upstream.URL + "/userinfo",
				RedirectURL: "https://app.example/callback",
			})
			t.Cleanup(func() { _ = client.Close() })
			ctx := context.Background()
			if valid {
				_, err := client.RedeemCode.Create().SetCode("OPTIONAL123").
					SetType(service.RedeemTypeInvitation).SetStatus(service.StatusUnused).SetValue(0).Save(ctx)
				require.NoError(t, err)
			}
			startRecorder := httptest.NewRecorder()
			start, _ := gin.CreateTestContext(startRecorder)
			start.Request = httptest.NewRequest(http.MethodGet, "/start?invitation_code=OPTIONAL123", nil)
			h.LinuxDoOAuthStart(start)
			require.Equal(t, http.StatusFound, startRecorder.Code)
			authorize, err := url.Parse(startRecorder.Header().Get("Location"))
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			callback, _ := gin.CreateTestContext(recorder)
			callback.Request = httptest.NewRequest(http.MethodGet, "/callback?code=provider-code&state="+url.QueryEscape(authorize.Query().Get("state")), nil)
			for _, cookie := range startRecorder.Result().Cookies() {
				if cookie.MaxAge > 0 {
					callback.Request.AddCookie(cookie)
				}
			}
			h.LinuxDoOAuthCallback(callback)
			require.Equal(t, http.StatusFound, recorder.Code)
			requireCookieCleared(t, recorder, oauthInvitationCookieName("linuxdo"))
			count, err := client.User.Query().Count(ctx)
			require.NoError(t, err)
			fragment := parseOAuthRedirectFragment(t, recorder.Header().Get("Location"))
			if valid {
				require.Equal(t, 1, count)
				require.NotEmpty(t, fragment.Get("access_token"))
				invite, err := client.RedeemCode.Query().Only(ctx)
				require.NoError(t, err)
				require.Equal(t, service.StatusUsed, invite.Status)
			} else {
				require.Zero(t, count)
				require.Empty(t, fragment.Get("access_token"))
				require.NotEmpty(t, fragment.Get("error"))
			}
		})
	}
}
