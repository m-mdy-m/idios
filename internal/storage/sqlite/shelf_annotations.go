package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"idios/internal/domain"
)

func shelfAnnotationFrom(r Row) domain.ShelfAnnotation {
	item := domain.ShelfAnnotation{
		ID: str(r, "id"), BookID: str(r, "book_id"), Page: int(i64(r["page"])),
		Quote: str(r, "quote"), Note: str(r, "note"), Color: str(r, "color"),
		CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"]),
		Rectangles: []domain.ShelfAnnotationRect{},
	}
	if raw := str(r, "rectangles_json"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &item.Rectangles)
	}
	if item.Rectangles == nil {
		item.Rectangles = []domain.ShelfAnnotationRect{}
	}
	return item
}

const shelfAnnotationSelect = `SELECT id,book_id,page,quote,note,color,rectangles_json,created_at,updated_at FROM shelf_annotations`

func (s *Store) ListShelfAnnotations(ctx context.Context, bookID string) ([]domain.ShelfAnnotation, error) {
	rows, err := s.p().all(ctx, shelfAnnotationSelect+` WHERE book_id=? ORDER BY page,created_at,id`, bookID)
	if err != nil {
		return nil, err
	}
	return collect(rows, shelfAnnotationFrom), nil
}

func (s *Store) CreateShelfAnnotation(ctx context.Context, item domain.ShelfAnnotation) (domain.ShelfAnnotation, error) {
	item.ID = domain.NewID()
	stamp := nowText()
	rects, err := json.Marshal(item.Rectangles)
	if err != nil {
		return domain.ShelfAnnotation{}, fmt.Errorf("encode Shelf annotation rectangles: %w", err)
	}
	err = s.run(ctx, func(p provider) error {
		_, err := p.changes(ctx, `INSERT INTO shelf_annotations(id,book_id,page,quote,note,color,rectangles_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
			item.ID, item.BookID, item.Page, strings.TrimSpace(item.Quote), strings.TrimSpace(item.Note), item.Color, string(rects), stamp, stamp)
		return err
	})
	if err != nil {
		return domain.ShelfAnnotation{}, err
	}
	rows, err := s.p().all(ctx, shelfAnnotationSelect+` WHERE id=?`, item.ID)
	if err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if len(rows) == 0 {
		return domain.ShelfAnnotation{}, fmt.Errorf("%w: Shelf annotation %q", domain.ErrNotFound, item.ID)
	}
	return shelfAnnotationFrom(rows[0]), nil
}

func (s *Store) UpdateShelfAnnotation(ctx context.Context, id, note, color string) (domain.ShelfAnnotation, error) {
	err := s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `UPDATE shelf_annotations SET note=?,color=?,updated_at=? WHERE id=?`, strings.TrimSpace(note), color, nowText(), id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: Shelf annotation %q", domain.ErrNotFound, id)
		}
		return nil
	})
	if err != nil {
		return domain.ShelfAnnotation{}, err
	}
	rows, err := s.p().all(ctx, shelfAnnotationSelect+` WHERE id=?`, id)
	if err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if len(rows) == 0 {
		return domain.ShelfAnnotation{}, fmt.Errorf("%w: Shelf annotation %q", domain.ErrNotFound, id)
	}
	return shelfAnnotationFrom(rows[0]), nil
}

func (s *Store) DeleteShelfAnnotation(ctx context.Context, id string) error {
	return s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `DELETE FROM shelf_annotations WHERE id=?`, id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: Shelf annotation %q", domain.ErrNotFound, id)
		}
		return nil
	})
}
