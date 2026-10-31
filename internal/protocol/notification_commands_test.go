package protocol

import (
	"context"
	"path/filepath"
	"testing"

	"idios/internal/application"
	"idios/internal/storage/sqlite"
)

func TestQuoteReminderTestCreatesSupportedNotificationLevel(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "protocol-quote-reminder.sqlite3"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app := application.New(store)
	defer app.Close()

	if _, err := app.CreateQuote(ctx, "A useful test quote for the shared inbox.", "IDIOS"); err != nil {
		t.Fatalf("create quote fixture: %v", err)
	}
	result, err := Execute(ctx, app, "quote.reminder.test", nil)
	if err != nil {
		t.Fatalf("quote.reminder.test returned error: %v", err)
	}
	if result == nil {
		t.Fatal("quote.reminder.test returned an empty result")
	}

	notifications, err := app.ListNotifications(ctx, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}
	if notifications[0].Level != "info" {
		t.Fatalf("notification level = %q, want info", notifications[0].Level)
	}
}
