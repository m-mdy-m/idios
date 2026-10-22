package reminders

import (
	"testing"
	"time"

	"idios/internal/config"
	"idios/internal/domain"
)

func TestRandomIntervalStaysInsideInclusiveRange(t *testing.T) {
	cfg := config.QuoteReminderConfig{MinIntervalMinutes: 1, MaxIntervalMinutes: 5}
	for range 250 {
		got := RandomIntervalMinutes(cfg)
		if got < 1 || got > 5 {
			t.Fatalf("random interval = %d, want range 1..5", got)
		}
	}
}

func TestDueAndEnsureSchedule(t *testing.T) {
	now := time.Date(2026, time.October, 22, 12, 0, 0, 0, time.UTC)
	cfg := config.QuoteReminderConfig{Enabled: true, MinIntervalMinutes: 15, MaxIntervalMinutes: 15}
	if !Due(cfg, now) {
		t.Fatal("missing schedule should be due")
	}
	appCfg := config.DefaultAppConfig()
	appCfg.Notifications.QuoteReminders = cfg
	if !EnsureSchedule(&appCfg, now) {
		t.Fatal("missing schedule should be initialized")
	}
	got, err := time.Parse(time.RFC3339Nano, appCfg.Notifications.QuoteReminders.NextAt)
	if err != nil || !got.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("nextAt = %q (%v), want %v", appCfg.Notifications.QuoteReminders.NextAt, err, now.Add(15*time.Minute))
	}
	if Due(appCfg.Notifications.QuoteReminders, now) {
		t.Fatal("future schedule must not be due")
	}
}

func TestChooseAvoidsPreviousWhenPossible(t *testing.T) {
	items := []domain.Quote{{ID: "a"}, {ID: "b"}}
	for range 50 {
		if got := Choose(items, "a"); got.ID != "b" {
			t.Fatalf("got %q, want b", got.ID)
		}
	}
}
