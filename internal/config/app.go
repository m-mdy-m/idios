package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	Server              ServerConfig                 `json:"server"`
	Web                 WebConfig                    `json:"web"`
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
	PlanReminders       PlanReminderConfig  `json:"plan_reminders"`
}

// PlanReminderConfig schedules three date-scoped check-ins for plans. Times use local 24-hour HH:MM.
type PlanReminderConfig struct {
	Enabled        bool   `json:"enabled"`
	DesktopEnabled bool   `json:"desktop_enabled"`
	MorningTime    string `json:"morning_time"`
	MiddayTime     string `json:"midday_time"`
	EveningTime    string `json:"evening_time"`
}

// ServerConfig controls the local-only HTTP service. The listener is restricted to loopback.
type ServerConfig struct {
	Host                   string `json:"host"`
	Port                   int    `json:"port"`
	OpenBrowser            bool   `json:"open_browser"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
}

// WebConfig stores presentation preferences shared by every browser session.
type CustomWebTheme struct {
	Name       string `json:"name"`
	Base       string `json:"base"`
	Background string `json:"background"`
	Surface    string `json:"surface"`
	Text       string `json:"text"`
	Muted      string `json:"muted"`
	Accent     string `json:"accent"`
	Border     string `json:"border"`
	Success    string `json:"success"`
	Warning    string `json:"warning"`
	Danger     string `json:"danger"`
}

type WebConfig struct {
	Theme            string                    `json:"theme"`
	CustomThemes     map[string]CustomWebTheme `json:"custom_themes"`
	SidebarCollapsed bool                      `json:"sidebar_collapsed"`
	Compact          bool                      `json:"compact"`
}

// QuoteReminderConfig controls random quote reminders shared by the Go service.
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

// AppConfigPath is the user-editable configuration file path. IDIOS_CONFIG wins, then the
// XDG config directory, with compatibility for an existing ~/.idios/config.json.
func AppConfigPath() string {
	if value := strings.TrimSpace(os.Getenv("IDIOS_CONFIG")); value != "" {
		return value
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "idios", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "idios", "config.json")
	}
	modern := filepath.Join(home, ".config", "idios", "config.json")
	legacy := filepath.Join(home, ".idios", "config.json")
	if _, err := os.Stat(modern); os.IsNotExist(err) {
		if _, legacyErr := os.Stat(legacy); legacyErr == nil {
			return legacy
		}
	}
	return modern
}

// ConfigDir returns the directory containing the active config file and its optional config.d
// fragments. Every adapter uses this same resolved location.
func ConfigDir() string { return filepath.Dir(AppConfigPath()) }

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
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return cfg, fmt.Errorf("read IDIOS config %q: %w", path, err)
	}
	// Merge the main file over packaged defaults, then apply config.d fragments in lexical
	// order. The same resolved config is consumed by CLI and the local server.
	var base, override map[string]any
	if err := json.Unmarshal(defaultConfigJSON, &base); err != nil {
		return cfg, fmt.Errorf("parse embedded IDIOS defaults: %w", err)
	}
	if err := json.Unmarshal(data, &override); err != nil {
		return cfg, fmt.Errorf("parse IDIOS config %q: %w", path, err)
	}
	mergeConfig(base, override)
	fragmentDir := filepath.Join(filepath.Dir(path), "config.d")
	entries, err := os.ReadDir(fragmentDir)
	if err != nil && !os.IsNotExist(err) {
		return cfg, fmt.Errorf("read IDIOS config fragments %q: %w", fragmentDir, err)
	}
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
				continue
			}
			fragmentPath := filepath.Join(fragmentDir, entry.Name())
			fragmentData, readErr := os.ReadFile(fragmentPath)
			if readErr != nil {
				return cfg, fmt.Errorf("read IDIOS config fragment %q: %w", fragmentPath, readErr)
			}
			var fragment map[string]any
			if decodeErr := json.Unmarshal(fragmentData, &fragment); decodeErr != nil {
				return cfg, fmt.Errorf("parse IDIOS config fragment %q: %w", fragmentPath, decodeErr)
			}
			mergeConfig(base, fragment)
		}
	}
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
	if cfg.Web.CustomThemes == nil {
		cfg.Web.CustomThemes = map[string]CustomWebTheme{}
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
	if cfg.Notifications.PollIntervalMinutes > 1440 {
		cfg.Notifications.PollIntervalMinutes = 1440
	}
	if cfg.Notifications.PlanReminders.MorningTime == "" {
		cfg.Notifications.PlanReminders.MorningTime = defaults.Notifications.PlanReminders.MorningTime
	}
	if cfg.Notifications.PlanReminders.MiddayTime == "" {
		cfg.Notifications.PlanReminders.MiddayTime = defaults.Notifications.PlanReminders.MiddayTime
	}
	if cfg.Notifications.PlanReminders.EveningTime == "" {
		cfg.Notifications.PlanReminders.EveningTime = defaults.Notifications.PlanReminders.EveningTime
	}
	if !validLocalReminderTime(cfg.Notifications.PlanReminders.MorningTime) {
		cfg.Notifications.PlanReminders.MorningTime = defaults.Notifications.PlanReminders.MorningTime
	}
	if !validLocalReminderTime(cfg.Notifications.PlanReminders.MiddayTime) {
		cfg.Notifications.PlanReminders.MiddayTime = defaults.Notifications.PlanReminders.MiddayTime
	}
	if !validLocalReminderTime(cfg.Notifications.PlanReminders.EveningTime) {
		cfg.Notifications.PlanReminders.EveningTime = defaults.Notifications.PlanReminders.EveningTime
	}
	if !validPlanReminderOrder(cfg.Notifications.PlanReminders) {
		cfg.Notifications.PlanReminders.MorningTime = defaults.Notifications.PlanReminders.MorningTime
		cfg.Notifications.PlanReminders.MiddayTime = defaults.Notifications.PlanReminders.MiddayTime
		cfg.Notifications.PlanReminders.EveningTime = defaults.Notifications.PlanReminders.EveningTime
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
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		cfg.Server.Port = 43170
	}
	if cfg.Server.RefreshIntervalSeconds < 10 {
		cfg.Server.RefreshIntervalSeconds = 30
	}
	if cfg.Web.Theme == "" {
		cfg.Web.Theme = "dark"
	}
	return cfg, nil
}

func validLocalReminderTime(value string) bool {
	parsed, err := time.Parse("15:04", value)
	return err == nil && parsed.Format("15:04") == value
}

func validPlanReminderOrder(settings PlanReminderConfig) bool {
	morning, morningErr := time.Parse("15:04", settings.MorningTime)
	midday, middayErr := time.Parse("15:04", settings.MiddayTime)
	evening, eveningErr := time.Parse("15:04", settings.EveningTime)
	return morningErr == nil && middayErr == nil && eveningErr == nil && morning.Before(midday) && midday.Before(evening)
}

// SaveAppConfig writes config atomically with private file permissions.
func SaveAppConfig(cfg AppConfig) error {
	if cfg.Web.CustomThemes == nil {
		cfg.Web.CustomThemes = map[string]CustomWebTheme{}
	}
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
