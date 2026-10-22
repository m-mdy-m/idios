package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppConfigCreatesDefaultsAndMergesSmallOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	t.Setenv("IDIOS_CONFIG", path)

	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Appearance.Theme != "midnight" || len(cfg.Themes) < 3 || cfg.Keyboard["overview"] == "" {
		t.Fatalf("first-run config is missing defaults: %+v", cfg)
	}
	if !cfg.Notifications.QuoteReminders.Enabled || !cfg.Notifications.QuoteReminders.DesktopEnabled || cfg.Notifications.QuoteReminders.MinIntervalMinutes != 60 || cfg.Notifications.QuoteReminders.MaxIntervalMinutes != 240 {
		t.Fatalf("first-run config has unexpected quote reminder defaults: %+v", cfg.Notifications.QuoteReminders)
	}
	if !cfg.Notifications.PlanReminders.Enabled || cfg.Notifications.PlanReminders.MorningTime != "08:30" || cfg.Notifications.PlanReminders.MiddayTime != "13:00" || cfg.Notifications.PlanReminders.EveningTime != "20:30" {
		t.Fatalf("first-run config has unexpected daily plan reminder defaults: %+v", cfg.Notifications.PlanReminders)
	}
	if cfg.RepositoryTemplate.Description == "" || cfg.IssueTemplate.Templates["task"].Body == "" || cfg.IssueTemplate.Templates["bug"].Body == "" || cfg.PullRequestTemplate.Body == "" {
		t.Fatal("first-run config is missing Repository, Issue, or Pull Request templates")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"appearance":{"theme":"paper","colors":{"accent":"#123456"}},"ISSUE_TEMPLATE":{"default":"bug","templates":{"bug":{"title_prefix":"[DEFECT] ","body":"## Reproduction\n\nAdd steps here."},"design":{"name":"Design proposal","title_prefix":"[Design]: ","body":"## Goal\n\nDesign it."}}},"PULL_REQUEST_TEMPLATE":{"body":"## Summary\n\nCustom review checklist."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Appearance.Theme != "paper" {
		t.Fatalf("theme override = %q, want paper", cfg.Appearance.Theme)
	}
	if cfg.Themes["forest"] == nil || cfg.Keyboard["overview"] == "" {
		t.Fatal("partial config replacement accidentally removed packaged defaults")
	}
	if got := cfg.ResolvedColors()["accent"]; got != "#123456" {
		t.Fatalf("accent override = %q, want #123456", got)
	}
	if cfg.IssueTemplate.Default != "bug" || cfg.IssueTemplate.Templates["bug"].TitlePrefix != "[DEFECT] " {
		t.Fatalf("Issue template override was not applied: %+v", cfg.IssueTemplate)
	}
	if cfg.IssueTemplate.Templates["feature"].Body == "" {
		t.Fatal("overriding one Issue template should retain the other packaged templates")
	}
	if got := cfg.IssueTemplate.Templates["design"]; got.Name != "Design proposal" || got.TitlePrefix != "[Design]: " || got.Body != "## Goal\n\nDesign it." {
		t.Fatalf("custom Issue template was not merged: %+v", got)
	}
	if cfg.PullRequestTemplate.Body != "## Summary\n\nCustom review checklist." {
		t.Fatalf("Pull Request template override = %q", cfg.PullRequestTemplate.Body)
	}
	if cfg.RepositoryTemplate.Description == "" {
		t.Fatal("partial config unexpectedly removed the Repository template")
	}
}

func TestLoadAppConfigPreservesCustomWebThemes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("IDIOS_CONFIG", path)
	cfg := DefaultAppConfig()
	cfg.Web.Theme = "aurora-drift"
	cfg.Web.CustomThemes = map[string]CustomWebTheme{
		"aurora-drift": {
			Name: "Aurora Drift", Base: "dark", Background: "#0b1220", Surface: "#111d30",
			Text: "#e8f0ff", Muted: "#91a2be", Accent: "#8bd7f0", Border: "#2b3a55",
			Success: "#65d6a7", Warning: "#f3c66d", Danger: "#ff788e",
		},
	}
	if err := SaveAppConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Web.Theme != "aurora-drift" || loaded.Web.CustomThemes["aurora-drift"].Accent != "#8bd7f0" {
		t.Fatalf("custom web theme did not round-trip: %+v", loaded.Web)
	}
}

func TestPlanReminderTimesMustStayChronological(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("IDIOS_CONFIG", path)
	if err := os.WriteFile(path, []byte(`{"notifications":{"plan_reminders":{"enabled":true,"desktop_enabled":true,"morning_time":"19:00","midday_time":"13:00","evening_time":"08:30"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Notifications.PlanReminders
	if got.MorningTime != "08:30" || got.MiddayTime != "13:00" || got.EveningTime != "20:30" {
		t.Fatalf("invalid reminder order should restore packaged defaults; got %+v", got)
	}
}

func TestLoadAppConfigRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("IDIOS_CONFIG", path)
	if err := os.WriteFile(path, []byte(`{"appearance":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAppConfig(); err == nil {
		t.Fatal("expected invalid JSON to return an actionable error")
	}
}

func TestLoadAppConfigAppliesConfigFragmentsInLexicalOrder(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	fragmentDir := filepath.Join(root, "config.d")
	if err := os.MkdirAll(fragmentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IDIOS_CONFIG", path)
	if err := os.WriteFile(path, []byte(`{"web":{"theme":"light","compact":false},"server":{"port":43171}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fragmentDir, "20-workspace.json"), []byte(`{"web":{"compact":true},"server":{"port":43172}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fragmentDir, "10-theme.json"), []byte(`{"web":{"theme":"dark"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fragmentDir, "README.txt"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Theme != "dark" || !cfg.Web.Compact {
		t.Fatalf("config.d web overlay = %+v, want dark theme and compact mode", cfg.Web)
	}
	if cfg.Server.Port != 43172 {
		t.Fatalf("config.d server overlay port = %d, want 43172", cfg.Server.Port)
	}
	if cfg.Web.SidebarCollapsed {
		t.Fatal("unspecified fragment property should retain packaged/main config value")
	}
}
