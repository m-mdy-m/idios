package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/ports"
	"strings"
	"time"
)

type ActivityService struct{ store ports.ActivityStore }

func (s *ActivityService) ListActivity(ctx context.Context, typ, id string, limit int) ([]domain.ActivityEvent, error) {
	return s.store.ListActivity(ctx, typ, id, limit)
}

// RecordDailyPresence records that the local application was actively opened on a local calendar day.
// The storage implementation is idempotent, so React StrictMode and route changes cannot inflate it.

func (s *ActivityService) RecordDailyPresence(ctx context.Context, localDate string) error {
	if _, err := time.Parse("2006-01-02", localDate); err != nil || len(localDate) != len("2006-01-02") {
		return fmt.Errorf("local_date must be a valid YYYY-MM-DD date")
	}
	return s.store.RecordDailyPresence(ctx, localDate)
}

// ListActivityDays returns calendar aggregates for all retained event history, not only the newest feed page.

func (s *ActivityService) ListActivityDays(ctx context.Context, from, to, timezone string) ([]domain.ActivityDayCount, error) {
	fromDate, fromErr := time.Parse("2006-01-02", from)
	toDate, toErr := time.Parse("2006-01-02", to)
	if fromErr != nil || toErr != nil || len(from) != 10 || len(to) != 10 || fromDate.After(toDate) {
		return nil, fmt.Errorf("from and to must be valid YYYY-MM-DD dates, with from not after to")
	}
	timezone = strings.TrimSpace(timezone)
	if timezone == "" || len(timezone) > 128 {
		return nil, fmt.Errorf("timezone must be a valid IANA time-zone name")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("timezone must be a valid IANA time-zone name")
	}
	return s.store.ListActivityDays(ctx, from, to, timezone)
}
