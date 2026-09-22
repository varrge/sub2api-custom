package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func sidebarGroupsFixture() *SidebarGroupsConfig {
	return &SidebarGroupsConfig{Groups: []SidebarGroup{
		{ID: "user-tools", Label: " 常用工具 ", Visibility: "user", Items: []string{"/keys", "/custom/58fbdcec-4e96-4f58-b352-d15546cda63c"}},
		{ID: "admin-tools", Label: "管理", Visibility: "admin", Items: []string{"/admin/orders", "/keys"}},
	}}
}

func TestSidebarGroupsSettingsPersistence(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{}
	s := NewSettingService(repo, &config.Config{})
	updates := 0
	s.SetOnUpdateCallback(func() { updates++ })
	cfg, err := s.GetSidebarGroupsConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, DefaultSidebarGroupsConfig(), cfg)
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.JSONEq(t, `{"groups":[]}`, string(data))
	cfg = sidebarGroupsFixture()
	require.NoError(t, s.SetSidebarGroupsConfig(ctx, cfg))
	require.Equal(t, 1, updates, "successful saves must invalidate HTML settings cache")
	require.Equal(t, "常用工具", cfg.Groups[0].Label)
	restarted := NewSettingService(repo, &config.Config{})
	got, err := restarted.GetSidebarGroupsConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg, got)
	all, err := restarted.GetAllSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg, all.SidebarGroups)
	stored := repo.values[SettingKeySidebarGroups]
	cfg.Groups[0].Items = []string{"/admin/users"}
	require.Error(t, s.SetSidebarGroupsConfig(ctx, cfg))
	require.Equal(t, stored, repo.values[SettingKeySidebarGroups])
	require.Equal(t, 1, updates)

	// Generic settings writes must not erase a newer independently saved config.
	all.SidebarGroups = DefaultSidebarGroupsConfig()
	require.NoError(t, s.UpdateSettings(ctx, all))
	require.Equal(t, stored, repo.values[SettingKeySidebarGroups])
	repo.getValueErr = errors.New("db unavailable")
	_, err = restarted.GetSidebarGroupsConfig(ctx)
	require.Error(t, err)
}

func TestSidebarGroupsCorruptSettingsDefault(t *testing.T) {
	for _, raw := range []string{
		``, `{bad json`, `null`, `{}`, `{"groups":null}`, `{"groups":[]} {}`,
		`{"groups":[],"private":"secret"}`,
		`{"groups":[{"id":"x","label":"工具","visibility":"unknown","items":[]}]}`,
		`{"groups":[{"id":"x","label":"工具","visibility":"user","items":["/admin/users"]}]}`,
		`{"groups":[{"id":"x","label":"工具","visibility":"user","items":[],"private":"secret"}]}`,
	} {
		t.Run(raw, func(t *testing.T) {
			s := NewSettingService(&panelRateLimitSettingRepo{values: map[string]string{SettingKeySidebarGroups: raw}}, &config.Config{})
			got, err := s.GetSidebarGroupsConfig(context.Background())
			require.NoError(t, err)
			require.Equal(t, DefaultSidebarGroupsConfig(), got)
		})
	}
}

func TestSidebarGroupsValidation(t *testing.T) {
	for name, change := range map[string]func(*SidebarGroupsConfig){
		"empty id":             func(c *SidebarGroupsConfig) { c.Groups[0].ID = "" },
		"unsafe id":            func(c *SidebarGroupsConfig) { c.Groups[0].ID = "x/y" },
		"long id":              func(c *SidebarGroupsConfig) { c.Groups[0].ID = strings.Repeat("x", 65) },
		"duplicate id":         func(c *SidebarGroupsConfig) { c.Groups[1].ID = c.Groups[0].ID },
		"empty label":          func(c *SidebarGroupsConfig) { c.Groups[0].Label = " \t " },
		"long label":           func(c *SidebarGroupsConfig) { c.Groups[0].Label = strings.Repeat("月", 65) },
		"control label":        func(c *SidebarGroupsConfig) { c.Groups[0].Label = "工具\n分组" },
		"invalid utf8 label":   func(c *SidebarGroupsConfig) { c.Groups[0].Label = "\xff" },
		"unknown scope":        func(c *SidebarGroupsConfig) { c.Groups[0].Visibility = "User" },
		"missing items":        func(c *SidebarGroupsConfig) { c.Groups[0].Items = nil },
		"duplicate item":       func(c *SidebarGroupsConfig) { c.Groups[0].Items = []string{"/keys", "/keys"} },
		"same scope duplicate": func(c *SidebarGroupsConfig) { c.Groups[1].Visibility = "user"; c.Groups[1].Items = []string{"/keys"} },
		"too many groups":      func(c *SidebarGroupsConfig) { c.Groups = make([]SidebarGroup, 33) },
		"too many items":       func(c *SidebarGroupsConfig) { c.Groups[0].Items = make([]string, 65) },
		"too many total items": func(c *SidebarGroupsConfig) {
			c.Groups = nil
			for i := 0; i < 5; i++ {
				g := SidebarGroup{ID: fmt.Sprintf("group-%d", i), Label: "工具", Visibility: "user", Items: []string{}}
				for j := 0; j < 64; j++ {
					g.Items = append(g.Items, fmt.Sprintf("/page-%d-%d", i, j))
				}
				c.Groups = append(c.Groups, g)
			}
		},
	} {
		t.Run(name, func(t *testing.T) { c := sidebarGroupsFixture(); change(c); require.Error(t, c.Validate()) })
	}
	for _, path := range []string{
		"/admin", "/admin/users", "/Admin/users", "https://example.com", "//example.com", "keys", "/keys?x=1", "/keys#x",
		"/keys/../admin", "/keys/./usage", "/keys/", "/keys//usage", "/custom/%2E%2E", "/custom/x y", "/custom/x\\y", "/custom/<script>", "/keys\n", "/" + strings.Repeat("a", 256),
	} {
		t.Run(path, func(t *testing.T) {
			c := sidebarGroupsFixture()
			c.Groups[0].Items = []string{path}
			require.Error(t, c.Validate())
		})
	}
	c := sidebarGroupsFixture()
	c.Groups[0].Label = strings.Repeat("月", 64)
	require.NoError(t, c.Validate(), "same path in different scopes is allowed")
	require.NoError(t, DefaultSidebarGroupsConfig().Validate())
	require.Error(t, (*SidebarGroupsConfig)(nil).Validate())
}

func TestSidebarGroupsPublicProjectionAndBootstrap(t *testing.T) {
	// Simulate legacy data with private fields and admin items inside a user group.
	raw := `{"private":"secret","groups":[
		{"id":"admin","label":"admin-secret","visibility":"admin","items":["/admin/users"]},
		{"id":"tools","label":" 工具 ","visibility":"user","private":"secret","items":["/keys","/admin/orders","/admin","//external","/keys","/custom/abc"]},
		{"id":"more","label":"更多","visibility":"user","items":["/keys","/usage"]},
		{"id":"tools","label":"重复","visibility":"user","items":["/profile"]},
		{"id":"unknown","label":"unknown-secret","visibility":"unknown","items":["/profile"]}
	]}`
	want := &SidebarGroupsConfig{Groups: []SidebarGroup{
		{ID: "tools", Label: "工具", Visibility: "user", Items: []string{"/keys", "/custom/abc"}},
		{ID: "more", Label: "更多", Visibility: "user", Items: []string{"/usage"}},
	}}
	s := NewSettingService(&panelRateLimitSettingRepo{values: map[string]string{SettingKeySidebarGroups: raw}}, &config.Config{})
	settings, err := s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, want, settings.SidebarGroups)
	payload, err := s.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	publicPayload, ok := payload.(*PublicSettingsInjectionPayload)
	require.True(t, ok, "expected public settings injection payload")
	require.Equal(t, want, publicPayload.SidebarGroups)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	require.NotContains(t, string(encoded), "/admin/")
	for _, raw := range []string{``, `null`, `{"groups":null}`, `{invalid`, `{"groups":[{"items":1}]}`} {
		require.Equal(t, DefaultSidebarGroupsConfig(), ParsePublicSidebarGroupsConfig(raw))
	}
}
