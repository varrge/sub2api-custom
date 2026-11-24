package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const oauthInvitationCodeStateKey = "invitation_code"

type oauthInvitationClaims struct {
	Code string `json:"code"`
	jwt.RegisteredClaims
}

func oauthInvitationCookieName(provider string) string {
	return "oauth_invitation_" + provider
}

func clearOAuthInvitationCodeCookie(c *gin.Context, provider string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthInvitationCookieName(provider),
		Path:     oauthPendingBrowserCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) captureOAuthInvitationCode(c *gin.Context, provider, state string, secure bool) error {
	code := strings.TrimSpace(c.Query("invitation_code"))
	if code == "" || normalizeOAuthIntent(c.Query("intent")) == oauthIntentBindCurrentUser {
		clearOAuthInvitationCodeCookie(c, provider, secure)
		return nil
	}
	if len(code) > 128 {
		return infraerrors.BadRequest("INVALID_INVITATION_CODE", "invalid invitation code")
	}
	secret := h.oauthBindCookieSecret()
	if secret == "" {
		return infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth cookie signing key is not configured")
	}
	claims := oauthInvitationClaims{
		Code: code,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "oauth-registration-invitation",
			Audience:  jwt.ClaimStrings{provider},
			ID:        state,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(oauthPendingCookieMaxAgeSec) * time.Second)),
		},
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return err
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthInvitationCookieName(provider),
		Value:    value,
		Path:     oauthPendingBrowserCookiePath,
		MaxAge:   oauthPendingCookieMaxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Restore only after the provider's OAuth state has been checked.
func (h *AuthHandler) restoreOAuthInvitationCode(c *gin.Context, provider, state string) error {
	cookie, err := c.Request.Cookie(oauthInvitationCookieName(provider))
	if errors.Is(err, http.ErrNoCookie) {
		return nil
	}
	if err != nil {
		return err
	}
	claims := &oauthInvitationClaims{}
	_, err = jwt.ParseWithClaims(cookie.Value, claims, func(*jwt.Token) (any, error) {
		secret := h.oauthBindCookieSecret()
		if secret == "" {
			return nil, errors.New("missing oauth cookie signing key")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("oauth-registration-invitation"), jwt.WithAudience(provider), jwt.WithExpirationRequired())
	if err != nil {
		return err
	}
	if claims.ID != state || state == "" || strings.TrimSpace(claims.Code) == "" {
		return errors.New("invalid oauth invitation state")
	}
	c.Set(oauthInvitationCodeStateKey, strings.TrimSpace(claims.Code))
	return nil
}

func readOAuthInvitationCode(c *gin.Context) string {
	return c.GetString(oauthInvitationCodeStateKey)
}

func pendingOAuthInvitationCode(session *dbent.PendingAuthSession, requestedCode *string) string {
	if requestedCode != nil {
		return strings.TrimSpace(*requestedCode)
	}
	if session == nil {
		return ""
	}
	return pendingSessionStringValue(session.LocalFlowState, oauthInvitationCodeStateKey)
}
