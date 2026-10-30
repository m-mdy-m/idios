package sqlite

import (
	"context"
	"fmt"
	"sort"
	"time"
	_ "time/tzdata"

	"idios/internal/domain"
)

// RecordActivity appends an audit event. Call activity within the caller's transaction when
// the event must be atomic with a workflow change.
func (s *Store) RecordActivity(ctx context.Context, entityType, entityID, action, payload string) error {
	return activity(ctx, s.p(), entityType, entityID, action, payload)
}

// ListActivity returns newest events first. Limits are clamped to keep accidental unbounded
// reads from turning a local activity view into a full database scan.
func (s *Store) ListActivity(ctx context.Context, entityType, entityID string, limit int) ([]domain.ActivityEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 50
	}
	query := `SELECT id,entity_type,entity_id,action,payload,created_at FROM activity_events WHERE 1=1`
	// Daily presence markers power the calendar but are implementation details, not
	// user-facing activity-feed events. They remain queryable only when explicitly asked for.
	if entityType == "" {
		query += ` AND entity_type != 'app_day'`
	}
	args := make([]any, 0, 3)
	if entityType != "" {
		query += ` AND entity_type=?`
		args = append(args, entityType)
	}
	if entityID != "" {
		query += ` AND entity_id=?`
		args = append(args, entityID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, activityFrom), nil
}

// activity writes the event using the supplied queryer, which may be a transaction-bound
// provider. Keeping it here centralizes the event shape used by all domain operations.
func activity(ctx context.Context, p provider, entityType, entityID, action, payload string) error {
	_, err := p.changes(ctx, `INSERT INTO activity_events(id,entity_type,entity_id,action,payload,created_at) VALUES(?,?,?,?,?,?)`,
		domain.NewID(), entityType, entityID, action, payload, nowText())
	return err
}

func activityFrom(r Row) domain.ActivityEvent {
	return domain.ActivityEvent{
		ID:         str(r, "id"),
		EntityType: str(r, "entity_type"),
		EntityID:   str(r, "entity_id"),
		Action:     str(r, "action"),
		Payload:    str(r, "payload"),
		CreatedAt:  parseTime(r["created_at"]),
	}
}

// RecordDailyPresence inserts one durable app-use marker per local date. It stores the user's
// local YYYY-MM-DD in entity_id because created_at is UTC and can fall on the previous local date.
func (s *Store) RecordDailyPresence(ctx context.Context, localDate string) error {
	return s.run(ctx, func(p provider) error {
		_, err := p.changes(ctx, `INSERT INTO activity_events(id,entity_type,entity_id,action,payload,created_at)
			SELECT ?, 'app_day', ?, 'app.day_seen', ?, ?
			WHERE NOT EXISTS (SELECT 1 FROM activity_events WHERE entity_type='app_day' AND entity_id=? AND action='app.day_seen')`,
			domain.NewID(), localDate, localDate, nowText(), localDate)
		return err
	})
}

// ListActivityDays aggregates the complete retained event history into IANA time-zone calendar days.
// The browser supplies a time-zone name so daylight-saving transitions are handled historically.
func (s *Store) ListActivityDays(ctx context.Context, from, to, timezone string) ([]domain.ActivityDayCount, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load activity timezone %q: %w", timezone, err)
	}
	fromDate, err := time.ParseInLocation("2006-01-02", from, loc)
	if err != nil || fromDate.Format("2006-01-02") != from {
		return nil, fmt.Errorf("invalid activity start date %q", from)
	}
	toDate, err := time.ParseInLocation("2006-01-02", to, loc)
	if err != nil || toDate.Format("2006-01-02") != to {
		return nil, fmt.Errorf("invalid activity end date %q", to)
	}
	// Use local midnight boundaries converted to UTC, not a fixed UTC offset. This
	// keeps days correct across daylight-saving transitions and historical offset changes.
	startUTC := fromDate.UTC().Format(time.RFC3339Nano)
	endUTC := toDate.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano)
	rows, err := s.p().all(ctx, `SELECT created_at FROM activity_events
		WHERE entity_type != 'app_day' AND julianday(created_at) >= julianday(?) AND julianday(created_at) < julianday(?)`, startUTC, endUTC)
	if err != nil {
		return nil, fmt.Errorf("read activity events for local date range: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		stamp := parseTime(row["created_at"])
		if stamp.IsZero() {
			continue
		}
		day := stamp.In(loc).Format("2006-01-02")
		if day >= from && day <= to {
			counts[day]++
		}
	}
	presenceRows, err := s.p().all(ctx, `SELECT entity_id AS activity_date, COUNT(*) AS event_count
		FROM activity_events WHERE entity_type='app_day' AND action='app.day_seen' AND entity_id BETWEEN ? AND ?
		GROUP BY entity_id`, from, to)
	if err != nil {
		return nil, fmt.Errorf("aggregate daily application presence: %w", err)
	}
	for _, row := range presenceRows {
		day := str(row, "activity_date")
		if day != "" {
			counts[day] += int(i64(row["event_count"]))
		}
	}
	days := make([]string, 0, len(counts))
	for day := range counts {
		days = append(days, day)
	}
	sort.Strings(days)
	result := make([]domain.ActivityDayCount, 0, len(days))
	for _, day := range days {
		result = append(result, domain.ActivityDayCount{Date: day, EventCount: counts[day]})
	}
	return result, nil
}
