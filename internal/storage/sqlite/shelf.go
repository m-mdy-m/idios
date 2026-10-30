package sqlite

import (
	"context"
	"fmt"
	"strings"

	"idios/internal/domain"
)

const shelfBookSelect = `SELECT b.id,b.title,b.summary,b.directory_name,b.readme_file,b.pdf_file,b.available,
 b.reading_status,b.current_page,b.total_pages,COALESCE(b.repository_id,'') AS repository_id,
 COALESCE(r.name,'') AS repository_name,b.created_at,b.updated_at
 FROM shelf_books b LEFT JOIN repositories r ON r.id=b.repository_id`

func shelfBookFrom(r Row) domain.ShelfBook {
	return domain.ShelfBook{
		ID: str(r, "id"), Title: str(r, "title"), Summary: str(r, "summary"),
		DirectoryName: str(r, "directory_name"), ReadmeFile: str(r, "readme_file"), PDFFile: str(r, "pdf_file"),
		Available: boolRow(r, "available"), ReadingStatus: str(r, "reading_status"),
		CurrentPage: int(i64(r["current_page"])), TotalPages: int(i64(r["total_pages"])),
		RepositoryID: str(r, "repository_id"), RepositoryName: str(r, "repository_name"),
		CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"]), Tags: []string{},
	}
}

func (s *Store) ListShelfBooks(ctx context.Context) ([]domain.ShelfBook, error) {
	rows, err := s.p().all(ctx, shelfBookSelect+` ORDER BY b.available DESC,b.title COLLATE NOCASE,b.id`)
	if err != nil {
		return nil, err
	}
	books := collect(rows, shelfBookFrom)
	if len(books) == 0 {
		return books, nil
	}
	tags, err := s.p().all(ctx, `SELECT book_id,tag FROM shelf_book_tags ORDER BY normalized_tag`)
	if err != nil {
		return nil, err
	}
	byID := make(map[string][]string, len(books))
	for _, tag := range tags {
		byID[str(tag, "book_id")] = append(byID[str(tag, "book_id")], str(tag, "tag"))
	}
	for i := range books {
		books[i].Tags = byID[books[i].ID]
		if books[i].Tags == nil {
			books[i].Tags = []string{}
		}
	}
	return books, nil
}

func (s *Store) GetShelfBook(ctx context.Context, id string) (domain.ShelfBook, error) {
	r, err := s.p().must(ctx, fmt.Sprintf("Shelf book %q", id), shelfBookSelect+` WHERE b.id=?`, id)
	if err != nil {
		return domain.ShelfBook{}, err
	}
	book := shelfBookFrom(r)
	rows, err := s.p().all(ctx, `SELECT tag FROM shelf_book_tags WHERE book_id=? ORDER BY normalized_tag`, id)
	if err != nil {
		return domain.ShelfBook{}, err
	}
	book.Tags = make([]string, 0, len(rows))
	for _, tag := range rows {
		book.Tags = append(book.Tags, str(tag, "tag"))
	}
	return book, nil
}

// SyncShelfBooks marks previously indexed directories unavailable, then upserts the current scan.
// It deliberately preserves tags, reading progress, questions and repository links.
func (s *Store) SyncShelfBooks(ctx context.Context, books []domain.ShelfBook) error {
	return s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `UPDATE shelf_books SET available=0,updated_at=? WHERE available<>0`, nowText()); err != nil {
			return err
		}
		for _, book := range books {
			stamp := nowText()
			if _, err := p.changes(ctx, `INSERT INTO shelf_books(id,title,summary,directory_name,readme_file,pdf_file,available,reading_status,current_page,total_pages,last_scanned_at,created_at,updated_at)
VALUES(?,?,?,?,?,?,1,'unread',0,0,?,?,?)
ON CONFLICT(id) DO UPDATE SET title=excluded.title,summary=excluded.summary,directory_name=excluded.directory_name,
 readme_file=excluded.readme_file,pdf_file=excluded.pdf_file,available=1,last_scanned_at=excluded.last_scanned_at,updated_at=excluded.updated_at`,
				book.ID, book.Title, book.Summary, book.DirectoryName, book.ReadmeFile, book.PDFFile, stamp, stamp, stamp); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SetShelfBookTags(ctx context.Context, bookID string, tags []string) error {
	return s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `UPDATE shelf_books SET updated_at=? WHERE id=?`, nowText(), bookID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: Shelf book %q", domain.ErrNotFound, bookID)
		}
		if _, err := p.changes(ctx, `DELETE FROM shelf_book_tags WHERE book_id=?`, bookID); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, tag := range tags {
			name := strings.TrimSpace(tag)
			key := strings.ToLower(name)
			if name == "" || seen[key] {
				continue
			}
			seen[key] = true
			if _, err := p.changes(ctx, `INSERT INTO shelf_book_tags(book_id,tag,normalized_tag,created_at) VALUES(?,?,?,?)`, bookID, name, key, nowText()); err != nil {
				return err
			}
		}
		return activity(ctx, p, "shelf_book", bookID, "shelf.tags_updated", marshalPayload(map[string]any{"tags": tags}))
	})
}

func (s *Store) SetShelfBookProgress(ctx context.Context, bookID, status string, current, total int) (domain.ShelfBook, error) {
	err := s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `UPDATE shelf_books SET reading_status=?,current_page=?,total_pages=?,updated_at=? WHERE id=?`, status, current, total, nowText(), bookID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: Shelf book %q", domain.ErrNotFound, bookID)
		}
		return activity(ctx, p, "shelf_book", bookID, "shelf.progress_updated", marshalPayload(map[string]any{"status": status, "current_page": current, "total_pages": total}))
	})
	if err != nil {
		return domain.ShelfBook{}, err
	}
	return s.GetShelfBook(ctx, bookID)
}

func (s *Store) LinkShelfBookRepository(ctx context.Context, bookID, repositoryID string) error {
	return s.run(ctx, func(p provider) error {
		var repoValue any
		if strings.TrimSpace(repositoryID) != "" {
			repoValue = repositoryID
		}
		changed, err := p.changes(ctx, `UPDATE shelf_books SET repository_id=?,updated_at=? WHERE id=?`, repoValue, nowText(), bookID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: Shelf book %q", domain.ErrNotFound, bookID)
		}
		return activity(ctx, p, "shelf_book", bookID, "shelf.repository_linked", marshalPayload(map[string]string{"repository_id": repositoryID}))
	})
}

func (s *Store) ListShelfBookRelations(ctx context.Context, bookID string) ([]domain.ShelfBookRelation, error) {
	query := `SELECT rel.id,rel.source_book_id,s.title AS source_book_title,rel.target_book_id,t.title AS target_book_title,rel.relation_type,rel.created_at
FROM shelf_book_relations rel JOIN shelf_books s ON s.id=rel.source_book_id JOIN shelf_books t ON t.id=rel.target_book_id`
	args := []any{}
	if bookID != "" {
		query += ` WHERE rel.source_book_id=? OR rel.target_book_id=?`
		args = append(args, bookID, bookID)
	}
	query += ` ORDER BY rel.created_at,rel.id`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r Row) domain.ShelfBookRelation {
		return domain.ShelfBookRelation{
			ID: str(r, "id"), SourceBookID: str(r, "source_book_id"), SourceBookTitle: str(r, "source_book_title"),
			TargetBookID: str(r, "target_book_id"), TargetBookTitle: str(r, "target_book_title"), RelationType: str(r, "relation_type"), CreatedAt: parseTime(r["created_at"]),
		}
	}), nil
}

func (s *Store) CreateShelfBookRelation(ctx context.Context, sourceID, targetID, relationType string) (domain.ShelfBookRelation, error) {
	id, stamp := domain.NewID(), nowText()
	err := s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `INSERT INTO shelf_book_relations(id,source_book_id,target_book_id,relation_type,created_at) VALUES(?,?,?,?,?)`, id, sourceID, targetID, relationType, stamp); err != nil {
			return err
		}
		return activity(ctx, p, "shelf_book", sourceID, "shelf.relation_created", marshalPayload(map[string]string{"relation_id": id, "target_book_id": targetID, "relation_type": relationType}))
	})
	if err != nil {
		return domain.ShelfBookRelation{}, err
	}
	items, err := s.ListShelfBookRelations(ctx, sourceID)
	if err != nil {
		return domain.ShelfBookRelation{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.ShelfBookRelation{}, fmt.Errorf("%w: Shelf relation %q", domain.ErrNotFound, id)
}

func (s *Store) DeleteShelfBookRelation(ctx context.Context, id string) error {
	return s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `DELETE FROM shelf_book_relations WHERE id=?`, id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: Shelf relation %q", domain.ErrNotFound, id)
		}
		return activity(ctx, p, "shelf_relation", id, "shelf.relation_deleted", "")
	})
}

func shelfAskFrom(r Row) domain.ShelfAsk {
	ask := domain.ShelfAsk{ID: str(r, "id"), BookID: str(r, "book_id"), BookTitle: str(r, "book_title"), Question: str(r, "question"), Context: str(r, "context"), Reason: str(r, "reason"), Answer: str(r, "answer"), Status: str(r, "status"), CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"])}
	if answered := str(r, "answered_at"); answered != "" {
		stamp := parseTime(answered)
		if !stamp.IsZero() {
			ask.AnsweredAt = &stamp
		}
	}
	return ask
}

const shelfAskSelect = `SELECT a.id,a.book_id,b.title AS book_title,a.question,a.context,a.reason,a.answer,a.status,a.created_at,a.updated_at,COALESCE(a.answered_at,'') AS answered_at FROM shelf_asks a JOIN shelf_books b ON b.id=a.book_id`

func (s *Store) CreateShelfAsk(ctx context.Context, ask domain.ShelfAsk) (domain.ShelfAsk, error) {
	ask.ID = domain.NewID()
	stamp := nowText()
	err := s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `INSERT INTO shelf_asks(id,book_id,question,context,reason,answer,status,created_at,updated_at) VALUES(?,?,?,?,?,'','open',?,?)`, ask.ID, ask.BookID, strings.TrimSpace(ask.Question), strings.TrimSpace(ask.Context), strings.TrimSpace(ask.Reason), stamp, stamp); err != nil {
			return err
		}
		return activity(ctx, p, "shelf_ask", ask.ID, "shelf.ask_created", marshalPayload(map[string]string{"book_id": ask.BookID, "question": strings.TrimSpace(ask.Question)}))
	})
	if err != nil {
		return domain.ShelfAsk{}, err
	}
	rows, err := s.p().all(ctx, shelfAskSelect+` WHERE a.id=?`, ask.ID)
	if err != nil {
		return domain.ShelfAsk{}, err
	}
	if len(rows) == 0 {
		return domain.ShelfAsk{}, fmt.Errorf("%w: Shelf ask %q", domain.ErrNotFound, ask.ID)
	}
	return shelfAskFrom(rows[0]), nil
}

func (s *Store) ListShelfAsks(ctx context.Context, bookID, status string) ([]domain.ShelfAsk, error) {
	query := shelfAskSelect
	where := []string{}
	args := []any{}
	if bookID != "" {
		where = append(where, "a.book_id=?")
		args = append(args, bookID)
	}
	if status != "" {
		where = append(where, "a.status=?")
		args = append(args, status)
	}
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, " AND ")
	}
	query += ` ORDER BY CASE a.status WHEN 'open' THEN 0 ELSE 1 END,a.updated_at DESC,a.created_at DESC`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, shelfAskFrom), nil
}

func (s *Store) AnswerShelfAsk(ctx context.Context, id, answer string) (domain.ShelfAsk, error) {
	answer = strings.TrimSpace(answer)
	err := s.run(ctx, func(p provider) error {
		status := "answered"
		var answeredAt any = nowText()
		if answer == "" {
			status = "open"
			answeredAt = nil
		}
		changed, err := p.changes(ctx, `UPDATE shelf_asks SET answer=?,status=?,answered_at=?,updated_at=? WHERE id=?`, answer, status, answeredAt, nowText(), id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: Shelf ask %q", domain.ErrNotFound, id)
		}
		return activity(ctx, p, "shelf_ask", id, "shelf.ask_answered", marshalPayload(map[string]string{"status": status}))
	})
	if err != nil {
		return domain.ShelfAsk{}, err
	}
	rows, err := s.p().all(ctx, shelfAskSelect+` WHERE a.id=?`, id)
	if err != nil {
		return domain.ShelfAsk{}, err
	}
	if len(rows) == 0 {
		return domain.ShelfAsk{}, fmt.Errorf("%w: Shelf ask %q", domain.ErrNotFound, id)
	}
	return shelfAskFrom(rows[0]), nil
}
