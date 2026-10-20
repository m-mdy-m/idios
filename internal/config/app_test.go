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
