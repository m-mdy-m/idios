package sqlite

import (
	"context"

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
