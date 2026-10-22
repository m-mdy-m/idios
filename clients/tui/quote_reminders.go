package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/desktopnotify"
	"idios/internal/domain"
	builtinquotes "idios/internal/quotes"
	"idios/internal/reminders"
	"idios/clients/server"

	tea "charm.land/bubbletea/v2"
)

// ensureBuiltinQuotes adds source-attributed starter quotations exactly once by quote text.
// Existing user-authored quotes are never rewritten or deleted.
func ensureBuiltinQuotes(ctx context.Context, app *application.Service) error {
	if app == nil {
		return fmt.Errorf("quote library requires an application service")
	}
	existing, err := app.ListQuotes(ctx)
	if err != nil {
		return fmt.Errorf("list quotes before adding the starter collection: %w", err)
	}
	seen := make(map[string]bool, len(existing))
	for _, item := range existing {
		seen[quoteDedupKey(item.Text)] = true
	}
	for _, item := range builtinquotes.Builtin() {
		key := quoteDedupKey(item.Text)
		if seen[key] {
			continue
		}
		if _, err := app.CreateQuoteWithSource(ctx, item); err != nil {
			return fmt.Errorf("add sourced quote by %s: %w", item.Author, err)
		}
		seen[key] = true
	}
	return nil
}

func quoteDedupKey(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func normalizeQuoteReminderRange(cfg config.QuoteReminderConfig) (int, int) {
	return reminders.NormalizeRange(cfg)
}

func randomQuoteIntervalMinutes(cfg config.QuoteReminderConfig) int {
	return reminders.RandomIntervalMinutes(cfg)
}

func nextQuoteReminderAt(now time.Time, cfg config.QuoteReminderConfig) time.Time {
	return reminders.NextAt(now, cfg)
}

func quoteReminderDue(cfg config.QuoteReminderConfig, at time.Time) bool {
	return reminders.Due(cfg, at)
}

func scheduleQuoteReminderIfNeeded(cfg *config.AppConfig, now time.Time) bool {
	return reminders.EnsureSchedule(cfg, now)
}

type quoteReminderMsg struct {
	skipped      bool
	quote        domain.Quote
	notification domain.Notification
	nextAt       time.Time
	err          error
	desktopErr   error
}

type quoteTestMsg struct {
	quote      domain.Quote
	desktopErr error
	err        error
}

func quoteReminderCmd(ctx context.Context, app *application.Service, cfg config.QuoteReminderConfig, at time.Time) tea.Cmd {
	return func() tea.Msg {
		result := quoteReminderMsg{nextAt: nextQuoteReminderAt(at, cfg)}
		if !cfg.Enabled {
			return result
		}
		// The always-on Go service owns random quote reminders whenever it is healthy.
		// Keep this TUI fallback for standalone use without the local server.
		if server.ProbeActive(ctx) {
			result.skipped = true
			return result
		}
		items, err := app.ListQuotes(ctx)
		if err != nil {
			result.err = fmt.Errorf("load quotes for reminder: %w", err)
			return result
		}
		if len(items) == 0 {
			result.err = fmt.Errorf("quote library is empty; add at least one quote")
			return result
		}
		item := chooseRandomQuote(items, cfg.LastQuoteID)
		result.quote = item
		body := reminders.FormatText(item)
		stored, storeErr := app.CreateNotification(ctx, "Quote of the moment", body, "quote")
		if storeErr != nil {
			result.err = fmt.Errorf("save quote reminder to IDIOS inbox: %w", storeErr)
		} else {
			result.notification = stored
		}
		if cfg.DesktopEnabled {
			result.desktopErr = desktopnotify.Send(ctx, "IDIOS · Quote of the moment", body)
		}
		return result
	}
}

func sendTestQuoteCmd(ctx context.Context, app *application.Service, lastID string) tea.Cmd {
	return func() tea.Msg {
		items, err := app.ListQuotes(ctx)
		if err != nil {
			return quoteTestMsg{err: fmt.Errorf("load quotes: %w", err)}
		}
		if len(items) == 0 {
			return quoteTestMsg{err: fmt.Errorf("the quote library is empty")}
		}
		item := chooseRandomQuote(items, lastID)
		body := reminders.FormatText(item)
		return quoteTestMsg{quote: item, desktopErr: desktopnotify.Send(ctx, "IDIOS · Test quote notification", body)}
	}
}

func chooseRandomQuote(items []domain.Quote, lastID string) domain.Quote {
	return reminders.Choose(items, lastID)
}
