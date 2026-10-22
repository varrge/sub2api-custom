package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyPrivilegeSettings = "privilege_settings"

type PrivilegeWelcomeSettings struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
	Effect  string `json:"effect"`
}

type PrivilegeSettings struct {
	EnterpriseWelcome PrivilegeWelcomeSettings `json:"enterprise_welcome"`
	PersonalRules     []PersonalPrivilegeRule  `json:"personal_rules"`
}

type PersonalPrivilegeRule struct {
	ID             string  `json:"id"`
	Enabled        bool    `json:"enabled"`
	Title          string  `json:"title"`
	Kind           string  `json:"kind"`
	Threshold      float64 `json:"threshold"`
	UserIDs        []int64 `json:"user_ids"`
	WelcomeEnabled bool    `json:"welcome_enabled"`
	WelcomeMessage string  `json:"welcome_message"`
	WelcomeEffect  string  `json:"welcome_effect"`
}

// UserPrivilegeRepository is a narrow, read-only capability of the user repository.
type UserPrivilegeRepository interface {
	GetPrivilegeMetric(ctx context.Context, userID int64, kind string) (float64, error)
	ValidatePrivilegeUserIDs(ctx context.Context, userIDs []int64) error
}

var privilegeRuleIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type PrivilegeWelcome struct {
	Message string `json:"message"`
	Effect  string `json:"effect"`
}

// UserPrivileges contains presentation benefits, never authorization or billing rules.
type UserPrivileges struct {
	Welcome *PrivilegeWelcome `json:"welcome"`
	Title   string            `json:"title,omitempty"`
}

func DefaultPrivilegeSettings() *PrivilegeSettings {
	return &PrivilegeSettings{PersonalRules: []PersonalPrivilegeRule{}, EnterpriseWelcome: PrivilegeWelcomeSettings{
		Enabled: true,
		Message: "欢迎尊贵的企业用户",
		Effect:  "capsule",
	}}
}

func validatePrivilegeSettings(settings *PrivilegeSettings) error {
	if settings == nil {
		return infraerrors.BadRequest("INVALID_PRIVILEGE_SETTINGS", "privilege settings are required")
	}
	if !validPrivilegeText(settings.EnterpriseWelcome.Message, 80, true) {
		return infraerrors.BadRequest("INVALID_PRIVILEGE_SETTINGS", "welcome message must contain 1–80 characters without control characters")
	}
	if !validWelcomeEffect(settings.EnterpriseWelcome.Effect) {
		return infraerrors.BadRequest("INVALID_PRIVILEGE_SETTINGS", "unknown enterprise welcome effect")
	}
	if len(settings.PersonalRules) > 50 {
		return infraerrors.BadRequest("INVALID_PRIVILEGE_SETTINGS", "at most 50 personal privilege rules are allowed")
	}
	ids := make(map[string]bool)
	for i, rule := range settings.PersonalRules {
		invalid := func(reason string) error {
			return infraerrors.BadRequest("INVALID_PRIVILEGE_SETTINGS", fmt.Sprintf("personal rule %d: %s", i+1, reason))
		}
		if !privilegeRuleIDPattern.MatchString(rule.ID) || ids[rule.ID] {
			return invalid("rule IDs must be unique, 1–64 ASCII letters, digits, hyphens or underscores")
		}
		ids[rule.ID] = true
		if !validWelcomeEffect(rule.WelcomeEffect) {
			return invalid("unknown welcome effect")
		}
		if !validPrivilegeText(rule.Title, 24, true) || !validPrivilegeText(rule.WelcomeMessage, 80, rule.WelcomeEnabled) {
			return invalid("title must contain 1–24 characters; welcome message must be single-line and at most 80 characters, and is required when enabled")
		}
		if math.IsNaN(rule.Threshold) || math.IsInf(rule.Threshold, 0) {
			return invalid("threshold must be finite")
		}
		switch rule.Kind {
		case "total_spend", "total_recharge", "account_age_days":
			if rule.Threshold <= 0 || rule.Threshold > 1e12 || len(rule.UserIDs) != 0 {
				return invalid("automatic rules require a positive threshold and no manually assigned users")
			}
			if rule.Kind == "account_age_days" && (math.Trunc(rule.Threshold) != rule.Threshold || rule.Threshold > 365000) {
				return invalid("account age must be a whole number of days from 1 to 365000")
			}
		case "manual":
			if rule.Threshold != 0 || len(rule.UserIDs) == 0 || len(rule.UserIDs) > 200 {
				return invalid("manual rules require 1–200 user IDs and a zero threshold")
			}
			users := make(map[int64]bool)
			for _, id := range rule.UserIDs {
				if id <= 0 || id > 9007199254740991 || users[id] {
					return invalid("user IDs must be unique positive safe integers")
				}
				users[id] = true
			}
		default:
			return invalid("unknown condition type")
		}
	}
	return nil
}

func validPrivilegeText(text string, max int, required bool) bool {
	return utf8.ValidString(text) && (!required || strings.TrimSpace(text) != "") && utf8.RuneCountInString(text) <= max &&
		!strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' })
}

func validWelcomeEffect(effect string) bool {
	switch effect {
	case "", "capsule", "slide", "halo", "ribbon":
		return true
	default:
		return false
	}
}

func defaultWelcomeEffect(effect string) string {
	if effect == "" {
		return "capsule"
	}
	return effect
}

func getPrivilegeSettings(ctx context.Context, repo SettingRepository) (*PrivilegeSettings, error) {
	raw, err := repo.GetValue(ctx, SettingKeyPrivilegeSettings)
	if errors.Is(err, ErrSettingNotFound) {
		return DefaultPrivilegeSettings(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get privilege settings: %w", err)
	}
	settings := DefaultPrivilegeSettings()
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, fmt.Errorf("decode privilege settings: %w", err)
	}
	if err := validatePrivilegeSettings(settings); err != nil {
		return nil, fmt.Errorf("invalid stored privilege settings: %w", err)
	}
	if settings.PersonalRules == nil {
		settings.PersonalRules = []PersonalPrivilegeRule{}
	}
	settings.EnterpriseWelcome.Effect = defaultWelcomeEffect(settings.EnterpriseWelcome.Effect)
	for i := range settings.PersonalRules {
		settings.PersonalRules[i].WelcomeEffect = defaultWelcomeEffect(settings.PersonalRules[i].WelcomeEffect)
	}
	return settings, nil
}

func (s *SettingService) GetPrivilegeSettings(ctx context.Context) (*PrivilegeSettings, error) {
	return getPrivilegeSettings(ctx, s.settingRepo)
}

func (s *SettingService) SetPrivilegeSettings(ctx context.Context, settings *PrivilegeSettings) (*PrivilegeSettings, error) {
	if err := validatePrivilegeSettings(settings); err != nil {
		return nil, err
	}
	normalized := *settings
	normalized.EnterpriseWelcome.Message = strings.TrimSpace(normalized.EnterpriseWelcome.Message)
	// Cached clients omit new fields; preserve effects by stable rule ID instead of resetting them.
	needsCurrent := settings.PersonalRules == nil || settings.EnterpriseWelcome.Effect == ""
	for _, rule := range settings.PersonalRules {
		needsCurrent = needsCurrent || rule.WelcomeEffect == ""
	}
	current := DefaultPrivilegeSettings()
	if needsCurrent {
		var err error
		current, err = s.GetPrivilegeSettings(ctx)
		if err != nil {
			return nil, err
		}
	}
	if normalized.EnterpriseWelcome.Effect == "" {
		normalized.EnterpriseWelcome.Effect = current.EnterpriseWelcome.Effect
	}
	if settings.PersonalRules == nil {
		normalized.PersonalRules = current.PersonalRules
	} else {
		previousEffects := make(map[string]string, len(current.PersonalRules))
		for _, rule := range current.PersonalRules {
			previousEffects[rule.ID] = rule.WelcomeEffect
		}
		normalized.PersonalRules = make([]PersonalPrivilegeRule, len(settings.PersonalRules))
		for i, rule := range settings.PersonalRules {
			rule.Title = strings.TrimSpace(rule.Title)
			rule.WelcomeMessage = strings.TrimSpace(rule.WelcomeMessage)
			rule.UserIDs = append([]int64{}, rule.UserIDs...)
			if rule.WelcomeEffect == "" {
				rule.WelcomeEffect = defaultWelcomeEffect(previousEffects[rule.ID])
			}
			normalized.PersonalRules[i] = rule
		}
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode privilege settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyPrivilegeSettings, string(data)); err != nil {
		return nil, fmt.Errorf("save privilege settings: %w", err)
	}
	return &normalized, nil
}

func (s *UserService) GetPrivileges(ctx context.Context, userID int64) (*UserPrivileges, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := &UserPrivileges{}
	if user.IsAdmin() {
		return result, nil
	}
	settings, err := getPrivilegeSettings(ctx, s.settingRepo)
	if err != nil {
		return nil, err
	}
	if user.IsEnterprise() {
		if settings.EnterpriseWelcome.Enabled {
			result.Welcome = &PrivilegeWelcome{Message: settings.EnterpriseWelcome.Message, Effect: settings.EnterpriseWelcome.Effect}
		}
		return result, nil
	}
	metrics := make(map[string]float64)
	for _, rule := range settings.PersonalRules {
		if !rule.Enabled {
			continue
		}
		matched := false
		switch rule.Kind {
		case "manual":
			for _, id := range rule.UserIDs {
				if user.ID == id {
					matched = true
					break
				}
			}
		case "account_age_days":
			matched = !user.CreatedAt.IsZero() && math.Floor(time.Since(user.CreatedAt).Hours()/24) >= rule.Threshold
		case "total_spend", "total_recharge":
			value, loaded := metrics[rule.Kind]
			if !loaded {
				repo, ok := s.userRepo.(UserPrivilegeRepository)
				if !ok {
					return nil, fmt.Errorf("user privilege metrics repository is unavailable")
				}
				value, err = repo.GetPrivilegeMetric(ctx, userID, rule.Kind)
				if err != nil {
					return nil, fmt.Errorf("get privilege metric: %w", err)
				}
				if math.IsNaN(value) || math.IsInf(value, 0) {
					return nil, fmt.Errorf("invalid privilege metric")
				}
				metrics[rule.Kind] = value
			}
			matched = value >= rule.Threshold
		}
		if matched {
			result.Title = rule.Title
			if rule.WelcomeEnabled {
				result.Welcome = &PrivilegeWelcome{Message: rule.WelcomeMessage, Effect: rule.WelcomeEffect}
			}
			break // Admin-defined list order is the priority across different condition types.
		}
	}
	return result, nil
}

func (s *UserService) ValidatePrivilegeTargets(ctx context.Context, rules []PersonalPrivilegeRule) error {
	seen := make(map[int64]bool)
	var ids []int64
	for _, rule := range rules {
		if rule.Kind == "manual" {
			for _, id := range rule.UserIDs {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if len(ids) > 1000 {
		return infraerrors.BadRequest("INVALID_PRIVILEGE_SETTINGS", "at most 1000 distinct manually assigned users are allowed")
	}
	if s == nil {
		return fmt.Errorf("user service is unavailable")
	}
	repo, ok := s.userRepo.(UserPrivilegeRepository)
	if !ok {
		return fmt.Errorf("user privilege repository is unavailable")
	}
	return repo.ValidatePrivilegeUserIDs(ctx, ids)
}
