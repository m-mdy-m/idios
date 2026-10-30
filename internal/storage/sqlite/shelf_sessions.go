package sqlite

import (
	"context"
	"fmt"

	"idios/internal/domain"
)

func shelfReadingSessionFrom(r Row) domain.ShelfReadingSession {
	return domain.ShelfReadingSession{
		ID: str(r, "id"), BookID: str(r, "book_id"), BookTitle: str(r, "book_title"),
		StartedAt: parseTime(r["started_at"]), EndedAt: parseTime(r["ended_at"]),
		DurationSeconds: i64(r["duration_seconds"]), StartPage: int(i64(r["start_page"])),
		EndPage: int(i64(r["end_page"])), CreatedAt: parseTime(r["created_at"]),
	}
}

const shelfReadingSessionSelect = `SELECT id,book_id,book_title,started_at,ended_at,duration_seconds,start_page,end_page,created_at FROM shelf_reading_sessions`

func (s *Store) CreateShelfReadingSession(ctx context.Context, item domain.ShelfReadingSession) (domain.ShelfReadingSession, error) {
	if item.ID == "" {
		item.ID = domain.NewID()
	}
	item.CreatedAt = domain.Now()
	stamp := timeText(item.CreatedAt)
	err := s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `INSERT INTO shelf_reading_sessions(id,book_id,book_title,started_at,ended_at,duration_seconds,start_page,end_page,created_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
			item.ID, item.BookID, item.BookTitle, timeText(item.StartedAt), timeText(item.EndedAt), item.DurationSeconds, item.StartPage, item.EndPage, stamp)
		if err != nil {
			return err
		}
		if changed == 0 {
			return nil
		} // Idempotent retry of a session queued while the server was unavailable.
		return activity(ctx, p, "shelf_book", item.BookID, "shelf.session_completed", marshalPayload(map[string]any{
			"session_id": item.ID, "book_id": item.BookID, "book_title": item.BookTitle,
			"duration_seconds": item.DurationSeconds, "start_page": item.StartPage, "end_page": item.EndPage,
		}))
	})
	if err != nil {
		return domain.ShelfReadingSession{}, err
	}
	row, err := s.p().must(ctx, fmt.Sprintf("Shelf reading session %q", item.ID), shelfReadingSessionSelect+` WHERE id=?`, item.ID)
	if err != nil {
		return domain.ShelfReadingSession{}, err
	}
	return shelfReadingSessionFrom(row), nil
}

func (s *Store) ListShelfReadingSessions(ctx context.Context, bookID string) ([]domain.ShelfReadingSession, error) {
	query := shelfReadingSessionSelect
	args := []any{}
	if bookID != "" {
		query += ` WHERE book_id=?`
		args = append(args, bookID)
	}
	query += ` ORDER BY started_at DESC,id DESC LIMIT 10000`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list Shelf reading sessions: %w", err)
	}
	return collect(rows, shelfReadingSessionFrom), nil
}
