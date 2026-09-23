package service

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const monthCardOAuthConsentType = "month_card_oauth_consent"

// Issued only by authenticated CreateOrder after checking account read receipts.
// Public OAuth endpoints must not manufacture consent from their query values.
func (s *PaymentResumeService) CreateMonthCardOAuthConsent(claims WeChatPaymentResumeClaims) (string, error) {
	if err := s.ensureSigningKey(); err != nil {
		return "", err
	}
	if claims.OrderType != payment.OrderTypeMonthCard || claims.ConsentUserID <= 0 || !claims.RulesAccepted || claims.RulesPublication <= 0 {
		return "", infraerrors.BadRequest("MONTH_CARD_RULES_REQUIRED", "请先确认月卡购买规则")
	}
	claims.TokenType = monthCardOAuthConsentType
	claims.OpenID = ""
	claims.IssuedAt = time.Now().Unix()
	claims.ExpiresAt = time.Now().Add(wechatPaymentResumeTokenTTL).Unix()
	return s.createSignedToken(claims)
}
func (s *PaymentResumeService) ParseMonthCardOAuthConsent(token string) (*WeChatPaymentResumeClaims, error) {
	if err := s.ensureSigningKey(); err != nil {
		return nil, err
	}
	var claims WeChatPaymentResumeClaims
	if err := s.parseSignedToken(strings.TrimSpace(token), &claims); err != nil {
		return nil, infraerrors.BadRequest("MONTH_CARD_RULES_REQUIRED", "请重新选择月卡并确认购买规则")
	}
	if claims.TokenType != monthCardOAuthConsentType || claims.OrderType != payment.OrderTypeMonthCard || claims.ConsentUserID <= 0 || !claims.RulesAccepted || claims.RulesPublication <= 0 {
		return nil, infraerrors.BadRequest("MONTH_CARD_RULES_REQUIRED", "购买规则确认信息无效")
	}
	if err := validatePaymentResumeExpiry(claims.ExpiresAt, "MONTH_CARD_RULES_REQUIRED", "购买确认已过期，请重新勾选同意"); err != nil {
		return nil, err
	}
	return &claims, nil
}
