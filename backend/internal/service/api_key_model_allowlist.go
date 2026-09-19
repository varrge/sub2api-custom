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
	out := GroupModelAllowlist{Enabled: cfg.Enabled, Models: []string{}}
	invalid := func(message string) (GroupModelAllowlist, error) {
		return GroupModelAllowlist{}, infraerrors.BadRequest("INVALID_MODEL_ALLOWLIST", message)
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
	if out.Enabled && len(out.Models) == 0 {
		return invalid("model allowlist cannot be enabled with an empty model list")
	}
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
	for _, allowed := range k.ModelAllowlist.Models {
		if strings.TrimSpace(allowed) == model {
			return true
		}
	}
	return false
}

func cloneAPIKeyModelAllowlist(cfg GroupModelAllowlist) GroupModelAllowlist {
	return GroupModelAllowlist{Enabled: cfg.Enabled, Models: append([]string{}, cfg.Models...)}
}
