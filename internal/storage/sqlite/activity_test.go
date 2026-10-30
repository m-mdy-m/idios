package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestActivityCalendarAggregatesOldHistoryAndDeduplicatesDailyPresence(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "activity.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	older := today.AddDate(0, 0, -4)
	olderKey := older.Format("2006-01-02")
	todayKey := today.Format("2006-01-02")
	// Insert more than the visible activity-feed cap on the current day, plus an
	// older event that would not survive ListActivity's newest-100 pagination.
	for i := 0; i < 125; i++ {
		created := today.Add(12*time.Hour + time.Duration(i)*time.Millisecond).Format(time.RFC3339Nano)
		if _, err := store.p().changes(ctx, `INSERT INTO activity_events(id,entity_type,entity_id,action,payload,created_at) VALUES(?,?,?,?,?,?)`, fmt.Sprintf("recent-%03d", i), "test", fmt.Sprintf("entity-%03d", i), "test.updated", "", created); err != nil {
			t.Fatalf("insert recent event %d: %v", i, err)
		}
	}
	if _, err := store.p().changes(ctx, `INSERT INTO activity_events(id,entity_type,entity_id,action,payload,created_at) VALUES(?,?,?,?,?,?)`, "old-event", "test", "old-entity", "test.created", "", older.Add(12*time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDailyPresence(ctx, olderKey); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDailyPresence(ctx, olderKey); err != nil {
		t.Fatal(err)
	}

	feed, err := store.ListActivity(ctx, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range feed {
		if event.EntityType == "app_day" {
			t.Fatal("internal presence marker leaked into the user-facing activity feed")
		}
		if event.ID == "old-event" {
			t.Fatal("old event unexpectedly appeared in the newest-100 feed")
		}
	}

	days, err := store.ListActivityDays(ctx, olderKey, todayKey, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, day := range days {
		counts[day.Date] = day.EventCount
	}
	if counts[olderKey] != 2 {
		t.Fatalf("historical day count = %d, want old event + one deduplicated presence marker", counts[olderKey])
	}
	if counts[todayKey] != 125 {
		t.Fatalf("today's aggregate = %d, want all 125 events despite the feed limit", counts[todayKey])
	}
}

func TestActivityCalendarUsesIANATimezoneAcrossDST(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "activity-dst.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Berlin is UTC+1 in January 2026. A UTC timestamp at 22:30 belongs to
	// January 21 locally; a fixed summer offset would incorrectly place it on
	// January 22. IANA-zone conversion must respect the date's actual offset.
	if _, err := store.p().changes(ctx, `INSERT INTO activity_events(id,entity_type,entity_id,action,payload,created_at) VALUES(?,?,?,?,?,?)`,
		"winter-late-utc", "test", "dst", "test.created", "", "2026-01-21T22:30:00Z"); err != nil {
		t.Fatal(err)
	}
	days, err := store.ListActivityDays(ctx, "2026-01-21", "2026-01-22", "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, day := range days {
		counts[day.Date] = day.EventCount
	}
	if counts["2026-01-21"] != 1 {
		t.Fatalf("Berlin winter date count = %d, want 1", counts["2026-01-21"])
	}
	if counts["2026-01-22"] != 0 {
		t.Fatalf("Berlin next-day count = %d, want 0", counts["2026-01-22"])
	}
}
