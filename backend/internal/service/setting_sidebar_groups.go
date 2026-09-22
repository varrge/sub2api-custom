package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const SettingKeySidebarGroups = "sidebar_groups"

const (
	maxSidebarGroups     = 32
	maxSidebarGroupItems = 64
	maxSidebarItems      = 256
)

var (
	sidebarGroupIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	sidebarItemPattern    = regexp.MustCompile(`^/[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`)
)

// SidebarGroup references existing top-level navigation entries. It does not
// grant access to a page or change the permissions of an existing entry.
type SidebarGroup struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Visibility string   `json:"visibility"`
	Items      []string `json:"items"`
}

type SidebarGroupsConfig struct {
	Groups []SidebarGroup `json:"groups"`
}

func DefaultSidebarGroupsConfig() *SidebarGroupsConfig {
	return &SidebarGroupsConfig{Groups: []SidebarGroup{}}
}

func validSidebarGroupLabel(label string) bool {
	if label == "" || !utf8.ValidString(label) || utf8.RuneCountInString(label) > 64 {
		return false
	}
	for _, r := range label {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validSidebarItem(item, visibility string) bool {
	if len(item) > 256 || !sidebarItemPattern.MatchString(item) {
		return false
	}
	path := strings.ToLower(item)
	return visibility != "user" || (path != "/admin" && !strings.HasPrefix(path, "/admin/"))
}

// Validate also normalizes labels and the empty groups array before persistence.
func (c *SidebarGroupsConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("sidebar groups configuration is required")
	}
	if len(c.Groups) > maxSidebarGroups {
		return fmt.Errorf("sidebar groups must contain at most %d groups", maxSidebarGroups)
	}
	if c.Groups == nil {
		c.Groups = []SidebarGroup{}
	}
	ids := make(map[string]bool, len(c.Groups))
	paths := make(map[string]bool)
	total := 0
	for i := range c.Groups {
		group := &c.Groups[i]
		if !sidebarGroupIDPattern.MatchString(group.ID) || ids[group.ID] {
			return fmt.Errorf("group IDs must be unique and contain 1 to 64 letters, digits, underscores or hyphens")
		}
		ids[group.ID] = true
		group.Label = strings.TrimSpace(group.Label)
		if !validSidebarGroupLabel(group.Label) {
			return fmt.Errorf("group labels must contain 1 to 64 characters without control characters")
		}
		if group.Visibility != "user" && group.Visibility != "admin" {
			return fmt.Errorf("group visibility must be user or admin")
		}
		if group.Items == nil || len(group.Items) > maxSidebarGroupItems {
			return fmt.Errorf("group items must be an array of at most %d paths", maxSidebarGroupItems)
		}
		total += len(group.Items)
		if total > maxSidebarItems {
			return fmt.Errorf("sidebar groups must contain at most %d paths in total", maxSidebarItems)
		}
		for _, item := range group.Items {
			if !validSidebarItem(item, group.Visibility) {
				return fmt.Errorf("group items must be canonical internal paths allowed for their visibility")
			}
			key := group.Visibility + ":" + item
			if paths[key] {
				return fmt.Errorf("a path may only appear once within the same visibility")
			}
			paths[key] = true
		}
	}
	return nil
}

// ParseSidebarGroupsConfig safely restores the original sidebar for corrupt or
// absent settings, including settings written by older application versions.
func ParseSidebarGroupsConfig(raw string) *SidebarGroupsConfig {
	var cfg SidebarGroupsConfig
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil || cfg.Groups == nil {
		return DefaultSidebarGroupsConfig()
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return DefaultSidebarGroupsConfig()
	}
	if err := cfg.Validate(); err != nil {
		return DefaultSidebarGroupsConfig()
	}
	return &cfg
}

// ParsePublicSidebarGroupsConfig projects only known, safe user fields. Legacy
// records can contain admin paths or unknown fields even within user groups.
func ParsePublicSidebarGroupsConfig(raw string) *SidebarGroupsConfig {
	result := DefaultSidebarGroupsConfig()
	var cfg SidebarGroupsConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return result
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	total := 0
	for _, group := range cfg.Groups {
		group.Label = strings.TrimSpace(group.Label)
		if group.Visibility != "user" || !sidebarGroupIDPattern.MatchString(group.ID) || ids[group.ID] || !validSidebarGroupLabel(group.Label) || group.Items == nil {
			continue
		}
		items := make([]string, 0, min(len(group.Items), maxSidebarGroupItems))
		for _, item := range group.Items {
			if !validSidebarItem(item, "user") || paths[item] {
				continue
			}
			if len(items) == maxSidebarGroupItems || total == maxSidebarItems {
				break
			}
			items = append(items, item)
			paths[item] = true
			total++
		}
		group.Items = items
		result.Groups = append(result.Groups, group)
		ids[group.ID] = true
		if len(result.Groups) == maxSidebarGroups || total == maxSidebarItems {
			break
		}
	}
	return result
}

func (s *SettingService) GetSidebarGroupsConfig(ctx context.Context) (*SidebarGroupsConfig, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeySidebarGroups)
	if errors.Is(err, ErrSettingNotFound) {
		return DefaultSidebarGroupsConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read sidebar groups config: %w", err)
	}
	return ParseSidebarGroupsConfig(raw), nil
}

func (s *SettingService) SetSidebarGroupsConfig(ctx context.Context, cfg *SidebarGroupsConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := s.settingRepo.Set(ctx, SettingKeySidebarGroups, string(data)); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
