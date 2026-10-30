// Package reminders contains quote-reminder policy shared by the background server and Web inbox.
package reminders

import (
	crand "crypto/rand"
	"math/big"
	"strings"
	"time"

	"idios/internal/config"
	"idios/internal/domain"
)

const maxMinutesAllowed = 525600 // one year

// NormalizeRange clamps user-configurable random reminder intervals to a safe inclusive range.
func NormalizeRange(cfg config.QuoteReminderConfig) (int, int) {
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

// RandomIntervalMinutes returns an inclusive random interval. If secure randomness fails,
// it uses the minimum interval so a reminder can still be scheduled predictably.
func RandomIntervalMinutes(cfg config.QuoteReminderConfig) int {
	minMinutes, maxMinutes := NormalizeRange(cfg)
	if minMinutes == maxMinutes {
		return minMinutes
	}
	n, err := crand.Int(crand.Reader, big.NewInt(int64(maxMinutes-minMinutes+1)))
	if err != nil {
		return minMinutes
	}
	return minMinutes + int(n.Int64())
}

// NextAt calculates the next reminder time using the shared interval policy.
func NextAt(now time.Time, cfg config.QuoteReminderConfig) time.Time {
	return now.Add(time.Duration(RandomIntervalMinutes(cfg)) * time.Minute)
}

// Due reports whether an enabled reminder has reached its persisted due time.
// A missing or malformed timestamp is due so it can be repaired by the caller.
func Due(cfg config.QuoteReminderConfig, at time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	if strings.TrimSpace(cfg.NextAt) == "" {
		return true
	}
	next, err := time.Parse(time.RFC3339Nano, cfg.NextAt)
	return err != nil || !at.Before(next)
}

// EnsureSchedule preserves a valid future reminder and otherwise chooses the next interval.
func EnsureSchedule(cfg *config.AppConfig, now time.Time) bool {
	if cfg == nil || !cfg.Notifications.QuoteReminders.Enabled {
		return false
	}
	r := &cfg.Notifications.QuoteReminders
	if strings.TrimSpace(r.NextAt) != "" {
		if next, err := time.Parse(time.RFC3339Nano, r.NextAt); err == nil && next.After(now) {
			return false
		}
	}
	r.NextAt = NextAt(now, *r).Format(time.RFC3339Nano)
	return true
}

// Choose returns a random quote and avoids immediately repeating the previous quote when possible.
func Choose(items []domain.Quote, lastID string) domain.Quote {
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
	n, err := crand.Int(crand.Reader, big.NewInt(int64(len(candidates))))
	if err != nil {
		return candidates[0]
	}
	return candidates[n.Int64()]
}

// FormatText formats a quote consistently for notifications and the Web inbox.
func FormatText(item domain.Quote) string {
	body := "“" + item.Text + "”"
	if item.Author != "" {
		body += "\n— " + item.Author
	}
	if item.Source != "" {
		body += "\n" + item.Source
	}
	return body
}
