package service

import (
	"strings"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// NormalizeAPIKeyModelAllowlist accepts concrete, case-sensitive model IDs only.
// A key's list narrows group permissions, so catalog membership is not required.
func NormalizeAPIKeyModelAllowlist(cfg GroupModelAllowlist) (GroupModelAllowlist, error) {
	out := GroupModelAllowlist{Enabled: cfg.Enabled, Models: []string{}, Mode: cfg.Mode}
	invalid := func(message string) (GroupModelAllowlist, error) {
		return GroupModelAllowlist{}, infraerrors.BadRequest("INVALID_MODEL_ALLOWLIST", message)
	}
	if cfg.Mode != "" && cfg.Mode != "allow" && cfg.Mode != "deny" {
		return invalid("model restriction mode must be allow or deny")
	}
	if len(cfg.Models) > 512 {
		return invalid("model allowlist cannot contain more than 512 models")
	}
	seen := make(map[string]bool, len(cfg.Models))
	for _, model := range cfg.Models {
		if strings.IndexFunc(model, unicode.IsControl) >= 0 {
			return invalid("model IDs cannot contain control characters")
		}
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if utf8.RuneCountInString(model) > 256 {
			return invalid("model IDs cannot exceed 256 characters")
		}
		if strings.ContainsAny(model, "*?[]") {
			return invalid("model allowlist requires concrete model IDs without wildcards")
		}
		if !seen[model] {
			seen[model] = true
			out.Models = append(out.Models, model)
		}
	}
	// An enabled empty allowlist intentionally denies every model. This is
	// distinct from disabling restrictions, and permits revoking the last grant.
	return out, nil
}

// AllowsModel compares concrete public model IDs. Routing can distinguish case,
// reasoning suffixes and Claude aliases, so those names must be selected
// separately. Protocol resource prefixes must be removed at that protocol boundary.
func (k *APIKey) AllowsModel(model string) bool {
	if k == nil {
		return false
	}
	if !k.ModelAllowlist.Enabled {
		return true
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	if mode := k.ModelAllowlist.Mode; mode != "" && mode != "allow" && mode != "deny" {
		return false
	}
	deny := k.ModelAllowlist.Mode == "deny"
	for _, allowed := range k.ModelAllowlist.Models {
		if strings.TrimSpace(allowed) == model {
			return !deny
		}
	}
	return deny
}

func cloneAPIKeyModelAllowlist(cfg GroupModelAllowlist) GroupModelAllowlist {
	return GroupModelAllowlist{Enabled: cfg.Enabled, Models: append([]string{}, cfg.Models...), Mode: cfg.Mode}
}

// Legacy editors omit mode and cannot safely edit an existing deny selection.
func validateAPIKeyModelModeUpdate(current GroupModelAllowlist, next *GroupModelAllowlist) error {
	if next != nil && current.Mode == "deny" && next.Mode == "" {
		return infraerrors.BadRequest("MODEL_MODE_REQUIRED", "refresh the page and explicitly select a model restriction mode")
	}
	return nil
}
