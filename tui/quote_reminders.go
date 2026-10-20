package tui

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/desktopnotify"
	"idios/internal/domain"
	builtinquotes "idios/internal/quotes"

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
	const maxMinutesAllowed = 525600 // one year
	minMinutes, maxMinutes := cfg.MinIntervalMinutes, cfg.MaxIntervalMinutes
	if minMinutes < 1 {
		minMinutes = 60
	}
	if minMinutes > maxMinutesAllowed {
		minMinutes = maxMinutesAllowed
	}
	if maxMinutes < minMinutes {
		maxMinutes = minMinutes
	}
	if maxMinutes > maxMinutesAllowed {
		maxMinutes = maxMinutesAllowed
	}
	return minMinutes, maxMinutes
}

func randomQuoteIntervalMinutes(cfg config.QuoteReminderConfig) int {
	minMinutes, maxMinutes := normalizeQuoteReminderRange(cfg)
	if minMinutes == maxMinutes {
		return minMinutes
	}
	span := int64(maxMinutes - minMinutes + 1)
	n, err := crand.Int(crand.Reader, big.NewInt(span))
	if err != nil {
		return minMinutes
	}
	return minMinutes + int(n.Int64())
}

func nextQuoteReminderAt(now time.Time, cfg config.QuoteReminderConfig) time.Time {
	return now.Add(time.Duration(randomQuoteIntervalMinutes(cfg)) * time.Minute)
}

func quoteReminderDue(cfg config.QuoteReminderConfig, at time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	if strings.TrimSpace(cfg.NextAt) == "" {
		return true
	}
	next, err := time.Parse(time.RFC3339Nano, cfg.NextAt)
	return err != nil || !at.Before(next)
}

func scheduleQuoteReminderIfNeeded(cfg *config.AppConfig, now time.Time) bool {
	if cfg == nil || !cfg.Notifications.QuoteReminders.Enabled {
		return false
	}
	r := &cfg.Notifications.QuoteReminders
	if r.NextAt != "" {
		if next, err := time.Parse(time.RFC3339Nano, r.NextAt); err == nil && next.After(now) {
			return false
		}
	}
	r.NextAt = nextQuoteReminderAt(now, *r).Format(time.RFC3339Nano)
	return true
}

type quoteReminderMsg struct {
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
		body := "“" + item.Text + "”"
		if item.Author != "" {
			body += "\n— " + item.Author
		}
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
		body := "“" + item.Text + "”"
		if item.Author != "" {
			body += "\n— " + item.Author
		}
		return quoteTestMsg{quote: item, desktopErr: desktopnotify.Send(ctx, "IDIOS · Test quote notification", body)}
	}
}

func chooseRandomQuote(items []domain.Quote, lastID string) domain.Quote {
	if len(items) == 0 {
		return domain.Quote{}
	}
	candidates := items
	if len(items) > 1 && lastID != "" {
		filtered := make([]domain.Quote, 0, len(items)-1)
		for _, item := range items {
			if item.ID != lastID {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
	}
	index, err := crand.Int(crand.Reader, big.NewInt(int64(len(candidates))))
	if err != nil {
		return candidates[0]
	}
	return candidates[index.Int64()]
}
