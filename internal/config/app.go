package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed defaults.json
var defaultConfigJSON []byte

// AppConfig controls presentation and workspace behavior without changing the Engine rules.
type AppConfig struct {
	Version             int                          `json:"version"`
	Startup             StartupConfig                `json:"startup"`
	Debug               bool                         `json:"debug"`
	Appearance          AppearanceConfig             `json:"appearance"`
	Themes              map[string]map[string]string `json:"themes"`
	Notifications       NotificationConfig           `json:"notifications"`
	Keyboard            map[string]string            `json:"keyboard"`
	RepositoryTemplate  RepositoryTemplateConfig     `json:"REPOSITORY_TEMPLATE"`
	IssueTemplate       IssueTemplateConfig          `json:"ISSUE_TEMPLATE"`
	PullRequestTemplate PullRequestTemplateConfig    `json:"PULL_REQUEST_TEMPLATE"`
}

type RepositoryTemplateConfig struct {
	Description string `json:"description"`
}

type IssueTemplateConfig struct {
	Default   string                             `json:"default"`
	Templates map[string]IssueTemplateDefinition `json:"templates"`
}

type IssueTemplateDefinition struct {
	Name        string `json:"name"`
	TitlePrefix string `json:"title_prefix"`
	Body        string `json:"body"`
}

type PullRequestTemplateConfig struct {
	TitlePrefix string `json:"title_prefix"`
	Body        string `json:"body"`
}

type StartupConfig struct {
	ShowIntro bool `json:"show_intro"`
}

type AppearanceConfig struct {
	Theme   string            `json:"theme"`
	Density string            `json:"density"`
	Colors  map[string]string `json:"colors"`
}

type NotificationConfig struct {
	Enabled             bool                `json:"enabled"`
	PollIntervalMinutes int                 `json:"poll_interval_minutes"`
	QuoteReminders      QuoteReminderConfig `json:"quote_reminders"`
}

// QuoteReminderConfig controls random quote reminders while the interactive TUI is running.
// NextAt and LastQuoteID are persisted scheduler state, not user-facing preferences.
type QuoteReminderConfig struct {
	Enabled            bool   `json:"enabled"`
	DesktopEnabled     bool   `json:"desktop_enabled"`
	MinIntervalMinutes int    `json:"min_interval_minutes"`
	MaxIntervalMinutes int    `json:"max_interval_minutes"`
	NextAt             string `json:"next_at,omitempty"`
	LastQuoteID        string `json:"last_quote_id,omitempty"`
}

// DefaultAppConfig returns a fresh copy of the packaged defaults.
func DefaultAppConfig() AppConfig {
	var cfg AppConfig
	if err := json.Unmarshal(defaultConfigJSON, &cfg); err != nil {
		panic("invalid embedded IDIOS defaults: " + err.Error())
	}
	return cfg
}

// AppConfigPath is the user-editable configuration file path.
func AppConfigPath() string {
	if value := strings.TrimSpace(os.Getenv("IDIOS_CONFIG")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".idios", "config.json")
	}
	return filepath.Join(home, ".idios", "config.json")
}

// LoadAppConfig creates a default config for new installations and overlays any supplied values
// onto packaged defaults. Unknown fields are tolerated to allow forward-compatible local edits.
func LoadAppConfig() (AppConfig, error) {
	cfg := DefaultAppConfig()
	path := AppConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return cfg, fmt.Errorf("create IDIOS config directory: %w", err)
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := SaveAppConfig(cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read IDIOS config %q: %w", path, err)
	}
	// Merge user JSON over packaged defaults instead of replacing whole nested structs.
	// This lets users keep a tiny config containing just the options they want to change.
	var base, override map[string]any
	if err := json.Unmarshal(defaultConfigJSON, &base); err != nil {
		return cfg, fmt.Errorf("parse embedded IDIOS defaults: %w", err)
	}
	if err := json.Unmarshal(data, &override); err != nil {
		return cfg, fmt.Errorf("parse IDIOS config %q: %w", path, err)
	}
	mergeConfig(base, override)
	merged, err := json.Marshal(base)
	if err != nil {
		return cfg, fmt.Errorf("merge IDIOS config: %w", err)
	}
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return cfg, fmt.Errorf("decode merged IDIOS config: %w", err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Appearance.Theme == "" {
		cfg.Appearance.Theme = "midnight"
	}
	if cfg.Appearance.Colors == nil {
		cfg.Appearance.Colors = map[string]string{}
	}
	if cfg.Appearance.Density == "" {
		cfg.Appearance.Density = "comfortable"
	}
	defaults := DefaultAppConfig()
	// Older releases copied the packaged themes into config.json, which froze the old colors.
	// Packaged themes are therefore authoritative; personal tweaks belong in appearance.colors,
	// and themes with other names are kept untouched.
	if cfg.Themes == nil {
		cfg.Themes = map[string]map[string]string{}
	}
	for name, theme := range defaults.Themes {
		cfg.Themes[name] = theme
	}
	if cfg.Keyboard == nil {
		cfg.Keyboard = defaults.Keyboard
	}
	if cfg.IssueTemplate.Templates == nil || len(cfg.IssueTemplate.Templates) == 0 {
		cfg.IssueTemplate = defaults.IssueTemplate
	}
	if _, ok := cfg.IssueTemplate.Templates[cfg.IssueTemplate.Default]; !ok {
		cfg.IssueTemplate.Default = defaults.IssueTemplate.Default
	}
	if cfg.RepositoryTemplate.Description == "" {
		cfg.RepositoryTemplate.Description = defaults.RepositoryTemplate.Description
	}
	if cfg.PullRequestTemplate.Body == "" {
		cfg.PullRequestTemplate.Body = defaults.PullRequestTemplate.Body
	}
	if cfg.Notifications.PollIntervalMinutes < 1 {
		cfg.Notifications.PollIntervalMinutes = 1
	}
	const maxQuoteReminderMinutes = 525600 // one year; also bounds time.Duration arithmetic.
	if cfg.Notifications.QuoteReminders.MinIntervalMinutes < 1 {
		cfg.Notifications.QuoteReminders.MinIntervalMinutes = defaults.Notifications.QuoteReminders.MinIntervalMinutes
	}
	if cfg.Notifications.QuoteReminders.MinIntervalMinutes > maxQuoteReminderMinutes {
		cfg.Notifications.QuoteReminders.MinIntervalMinutes = maxQuoteReminderMinutes
	}
	if cfg.Notifications.QuoteReminders.MaxIntervalMinutes < cfg.Notifications.QuoteReminders.MinIntervalMinutes {
		cfg.Notifications.QuoteReminders.MaxIntervalMinutes = defaults.Notifications.QuoteReminders.MaxIntervalMinutes
	}
	if cfg.Notifications.QuoteReminders.MaxIntervalMinutes < cfg.Notifications.QuoteReminders.MinIntervalMinutes {
		cfg.Notifications.QuoteReminders.MaxIntervalMinutes = cfg.Notifications.QuoteReminders.MinIntervalMinutes
	}
	if cfg.Notifications.QuoteReminders.MaxIntervalMinutes > maxQuoteReminderMinutes {
		cfg.Notifications.QuoteReminders.MaxIntervalMinutes = maxQuoteReminderMinutes
	}
	return cfg, nil
}

// SaveAppConfig writes config atomically with private file permissions.
func SaveAppConfig(cfg AppConfig) error {
	path := AppConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create IDIOS config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode IDIOS config: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace IDIOS config: %w", err)
	}
	return nil
}

// mergeConfig recursively overlays user-provided JSON values on top of defaults.
func mergeConfig(base, override map[string]any) {
	for key, value := range override {
		if next, ok := value.(map[string]any); ok {
			if existing, ok := base[key].(map[string]any); ok {
				mergeConfig(existing, next)
				continue
			}
		}
		base[key] = value
	}
}

// ResolvedColors returns the active preset merged with any explicit custom overrides.
func (c AppConfig) ResolvedColors() map[string]string {
	active := c.Themes[c.Appearance.Theme]
	if active == nil {
		active = c.Themes["midnight"]
	}
	result := make(map[string]string, len(active)+len(c.Appearance.Colors))
	for key, value := range active {
		result[key] = value
	}
	for key, value := range c.Appearance.Colors {
		if strings.TrimSpace(value) != "" {
			result[key] = value
		}
	}
	return result
}
