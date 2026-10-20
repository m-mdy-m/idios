package tui

import (
	"testing"
	"time"

	"idios/internal/config"
	"idios/internal/domain"
)

func TestRandomQuoteIntervalIsInclusiveWithinConfiguredRange(t *testing.T) {
	cfg := config.QuoteReminderConfig{MinIntervalMinutes: 1, MaxIntervalMinutes: 5}
	seen := map[int]bool{}
	for i := 0; i < 500; i++ {
		got := randomQuoteIntervalMinutes(cfg)
		if got < 1 || got > 5 {
			t.Fatalf("random interval = %d, want inclusive range 1..5", got)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Fatalf("random interval did not vary across samples: %v", seen)
	}
}

func TestQuoteReminderDueRespectsEnabledAndNextAt(t *testing.T) {
	now := time.Date(2026, time.October, 20, 12, 0, 0, 0, time.UTC)
	cfg := config.QuoteReminderConfig{Enabled: false}
	if quoteReminderDue(cfg, now) {
		t.Fatal("disabled reminder must not be due")
	}
	cfg.Enabled = true
	if !quoteReminderDue(cfg, now) {
		t.Fatal("enabled reminder without a next time should be scheduled immediately")
	}
	cfg.NextAt = now.Add(time.Minute).Format(time.RFC3339Nano)
	if quoteReminderDue(cfg, now) {
		t.Fatal("future reminder should not be due")
	}
	if !quoteReminderDue(cfg, now.Add(time.Minute)) {
		t.Fatal("reminder should be due at its scheduled time")
	}
	cfg.NextAt = "invalid timestamp"
	if !quoteReminderDue(cfg, now) {
		t.Fatal("invalid persisted timestamp should be treated as due so it can be repaired")
	}
}

func TestScheduleQuoteReminderKeepsFutureScheduleAndRepairsExpiredSchedule(t *testing.T) {
	now := time.Date(2026, time.October, 20, 12, 0, 0, 0, time.UTC)
	cfg := config.DefaultAppConfig()
	cfg.Notifications.QuoteReminders.Enabled = true
	cfg.Notifications.QuoteReminders.MinIntervalMinutes = 30
	cfg.Notifications.QuoteReminders.MaxIntervalMinutes = 30
	cfg.Notifications.QuoteReminders.NextAt = now.Add(time.Hour).Format(time.RFC3339Nano)
	if scheduleQuoteReminderIfNeeded(&cfg, now) {
		t.Fatal("a valid future schedule should not be replaced")
	}
	cfg.Notifications.QuoteReminders.NextAt = now.Add(-time.Minute).Format(time.RFC3339Nano)
	if !scheduleQuoteReminderIfNeeded(&cfg, now) {
		t.Fatal("expired schedule should be repaired")
	}
	next, err := time.Parse(time.RFC3339Nano, cfg.Notifications.QuoteReminders.NextAt)
	if err != nil || !next.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("repaired next time = %q (%v), want %v", cfg.Notifications.QuoteReminders.NextAt, err, now.Add(30*time.Minute))
	}
}

func TestChooseRandomQuoteAvoidsRepeatingPreviousWhenPossible(t *testing.T) {
	items := []domain.Quote{{ID: "first", Text: "first"}, {ID: "second", Text: "second"}}
	for i := 0; i < 100; i++ {
		got := chooseRandomQuote(items, "first")
		if got.ID != "second" {
			t.Fatalf("selected quote %q, want not to repeat first", got.ID)
		}
	}
}
