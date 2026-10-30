package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const shelfRootSetting = "shelf.root_path"
const maxShelfReadmeBytes = 4 << 20

var shelfRelationTypes = map[string]bool{
	"related_to":       true,
	"prerequisite_for": true,
	"contrasts_with":   true,
	"implements":       true,
	"references":       true,
	"uses":             true,
}

type ShelfService struct {
	store        shelfServiceStore
	repositories *RepositoryService
	notes        *NotesService
}

func expandShelfHomePath(path, home string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}
	remainder := strings.TrimLeft(strings.TrimPrefix(path, "~"), `/\`)
	remainder = strings.ReplaceAll(remainder, `\`, string(filepath.Separator))
	remainder = strings.ReplaceAll(remainder, "/", string(filepath.Separator))
	if remainder == "" {
		return home
	}
	return filepath.Join(home, remainder)
}

func (s *ShelfService) ShelfRootPath(ctx context.Context) (string, error) {
	return s.store.GetAppSetting(ctx, shelfRootSetting)
}

// SetShelfRootPath stores an absolute path only after confirming that it resolves to a directory.
// Files are never copied into IDIOS; the setting is a pointer to the user's existing library.

func (s *ShelfService) SetShelfRootPath(ctx context.Context, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("%w: Shelf folder path is required", domain.ErrInvalid)
	}
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		path = expandShelfHomePath(path, home)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%w: Shelf folder path could not be resolved", domain.ErrInvalid)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return fmt.Errorf("%w: the selected Shelf folder is unavailable or inaccessible", domain.ErrInvalid)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: the selected Shelf folder must be an accessible directory", domain.ErrInvalid)
	}
	if err := s.store.SetAppSetting(ctx, shelfRootSetting, resolved); err != nil {
		return err
	}
	_, err = s.ScanShelfBooks(ctx)
	return err
}

// ListShelfBooks returns the indexed catalogue without re-reading every README on every view.
// Use ScanShelfBooks when a user explicitly refreshes the filesystem index.

func (s *ShelfService) ListShelfBooks(ctx context.Context) ([]domain.ShelfBook, error) {
	return s.store.ListShelfBooks(ctx)
}

// CreateShelfReadingSession records a completed Focus Mode session in the durable workspace database.

func (s *ShelfService) CreateShelfReadingSession(ctx context.Context, item domain.ShelfReadingSession) (domain.ShelfReadingSession, error) {
	if strings.TrimSpace(item.BookID) == "" {
		return domain.ShelfReadingSession{}, fmt.Errorf("%w: book is required", domain.ErrInvalid)
	}
	book, err := s.GetShelfBook(ctx, item.BookID)
	if err != nil {
		return domain.ShelfReadingSession{}, err
	}
	item.BookTitle = book.Title
	if item.ID == "" {
		item.ID = domain.NewID()
	}
	if len(item.ID) > 128 {
		return domain.ShelfReadingSession{}, fmt.Errorf("%w: session ID is too long", domain.ErrInvalid)
	}
	for _, r := range item.ID {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return domain.ShelfReadingSession{}, fmt.Errorf("%w: session ID contains unsupported characters", domain.ErrInvalid)
		}
	}
	if item.StartedAt.IsZero() || item.EndedAt.IsZero() || !item.EndedAt.After(item.StartedAt) {
		return domain.ShelfReadingSession{}, fmt.Errorf("%w: a reading session must have a valid start and end time", domain.ErrInvalid)
	}
	duration := item.EndedAt.Sub(item.StartedAt)
	if duration > 24*time.Hour || item.StartedAt.After(time.Now().Add(5*time.Minute)) || item.EndedAt.After(time.Now().Add(5*time.Minute)) {
		return domain.ShelfReadingSession{}, fmt.Errorf("%w: reading session duration is invalid", domain.ErrInvalid)
	}
	if item.StartPage < 0 || item.EndPage < 0 {
		return domain.ShelfReadingSession{}, fmt.Errorf("%w: page numbers cannot be negative", domain.ErrInvalid)
	}
	item.DurationSeconds = int64(duration.Seconds())
	if item.DurationSeconds < 1 {
		return domain.ShelfReadingSession{}, fmt.Errorf("%w: session was too short to record", domain.ErrInvalid)
	}
	return s.store.CreateShelfReadingSession(ctx, item)
}

// ListShelfReadingSessions returns all completed sessions or those associated with a specific book.

func (s *ShelfService) ListShelfReadingSessions(ctx context.Context, bookID string) ([]domain.ShelfReadingSession, error) {
	if bookID != "" {
		if _, err := s.GetShelfBook(ctx, bookID); err != nil {
			return nil, err
		}
	}
	return s.store.ListShelfReadingSessions(ctx, bookID)
}

// ScanShelfBooks refreshes the catalog from first-level directories under the configured root.
// Folder names are stable IDs so an already linked note/book relationship survives a rescan.

func (s *ShelfService) ScanShelfBooks(ctx context.Context) ([]domain.ShelfBook, error) {
	root, err := s.ShelfRootPath(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(root) == "" {
		return s.store.ListShelfBooks(ctx)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("%w: the configured Shelf folder is no longer available", domain.ErrInvalid)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("%w: the configured Shelf folder cannot be read", domain.ErrInvalid)
	}
	books := make([]domain.ShelfBook, 0, len(entries))
	for _, entry := range entries {
		// Do not follow symbolic links out of the user's selected root.
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		bookDir := filepath.Join(root, entry.Name())
		files, err := os.ReadDir(bookDir)
		if err != nil {
			continue
		}
		readme, pdf := "", ""
		for _, file := range files {
			if file.IsDir() || file.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := file.Name()
			if readme == "" && (strings.EqualFold(name, "README.md") || strings.EqualFold(name, "README.markdown")) {
				readme = name
			}
			if strings.EqualFold(filepath.Ext(name), ".pdf") && (pdf == "" || strings.ToLower(name) < strings.ToLower(pdf)) {
				pdf = name
			}
		}
		title, summary := prettifyShelfTitle(entry.Name()), ""
		if readme != "" {
			content, readErr := readLimited(filepath.Join(bookDir, readme), maxShelfReadmeBytes)
			if readErr == nil {
				readTitle, readSummary := shelfReadmeMetadata(content)
				if readTitle != "" {
					title = readTitle
				}
				summary = readSummary
			}
		}
		books = append(books, domain.ShelfBook{ID: entry.Name(), Title: title, Summary: summary, DirectoryName: entry.Name(), ReadmeFile: readme, PDFFile: pdf, Available: true, ReadingStatus: "unread", Tags: []string{}})
	}
	sort.Slice(books, func(i, j int) bool { return strings.ToLower(books[i].Title) < strings.ToLower(books[j].Title) })
	if err := s.store.SyncShelfBooks(ctx, books); err != nil {
		return nil, err
	}
	return s.store.ListShelfBooks(ctx)
}

func (s *ShelfService) GetShelfBook(ctx context.Context, id string) (domain.ShelfBook, error) {
	book, err := s.store.GetShelfBook(ctx, id)
	if err == nil {
		return book, nil
	}
	// A direct deep link may be the first Shelf request after startup. Only scan on a cache miss;
	// routine note/question/file operations should not re-read every README in the whole library.
	root, rootErr := s.ShelfRootPath(ctx)
	if rootErr != nil {
		return domain.ShelfBook{}, rootErr
	}
	if strings.TrimSpace(root) == "" {
		return domain.ShelfBook{}, err
	}
	if _, scanErr := s.ScanShelfBooks(ctx); scanErr != nil {
		return domain.ShelfBook{}, scanErr
	}
	return s.store.GetShelfBook(ctx, id)
}

func (s *ShelfService) SetShelfBookTags(ctx context.Context, id string, tags []string) error {
	if len(tags) > 50 {
		return fmt.Errorf("%w: a book can have at most 50 tags", domain.ErrInvalid)
	}
	for _, tag := range tags {
		if err := invalidInputText("Shelf tag", tag, 80, false); err != nil {
			return err
		}
	}
	if _, err := s.GetShelfBook(ctx, id); err != nil {
		return err
	}
	return s.store.SetShelfBookTags(ctx, id, tags)
}

func (s *ShelfService) SetShelfBookProgress(ctx context.Context, id, status string, current, total int) (domain.ShelfBook, error) {
	if !map[string]bool{"unread": true, "reading": true, "finished": true}[status] {
		return domain.ShelfBook{}, fmt.Errorf("%w: reading status must be unread, reading, or finished", domain.ErrInvalid)
	}
	if current < 0 || total < 0 || (total > 0 && current > total) {
		return domain.ShelfBook{}, fmt.Errorf("%w: page counts must be non-negative and current page cannot exceed total pages", domain.ErrInvalid)
	}
	if status == "finished" && total > 0 {
		current = total
	}
	if _, err := s.GetShelfBook(ctx, id); err != nil {
		return domain.ShelfBook{}, err
	}
	return s.store.SetShelfBookProgress(ctx, id, status, current, total)
}

func (s *ShelfService) LinkShelfBookRepository(ctx context.Context, bookID, repositoryRef string) error {
	if _, err := s.GetShelfBook(ctx, bookID); err != nil {
		return err
	}
	if strings.TrimSpace(repositoryRef) == "" {
		return s.store.LinkShelfBookRepository(ctx, bookID, "")
	}
	repo, err := s.repositories.GetRepository(ctx, strings.TrimSpace(repositoryRef))
	if err != nil {
		return err
	}
	return s.store.LinkShelfBookRepository(ctx, bookID, repo.ID)
}

func (s *ShelfService) CreateShelfBookRepository(ctx context.Context, bookID, name, description string) (domain.Repository, error) {
	book, err := s.GetShelfBook(ctx, bookID)
	if err != nil {
		return domain.Repository{}, err
	}
	if book.RepositoryID != "" {
		return domain.Repository{}, fmt.Errorf("%w: this book is already linked to repository %q", domain.ErrConflict, book.RepositoryName)
	}
	repo, err := s.repositories.CreateRepositoryWithDefaultBranch(ctx, name, description, "main")
	if err != nil {
		return domain.Repository{}, err
	}
	if err := s.store.LinkShelfBookRepository(ctx, bookID, repo.ID); err != nil {
		_ = s.repositories.DeleteRepository(ctx, repo.ID)
		return domain.Repository{}, err
	}
	return repo, nil
}

func (s *ShelfService) ListShelfBookRelations(ctx context.Context, bookID string) ([]domain.ShelfBookRelation, error) {
	if bookID != "" {
		if _, err := s.GetShelfBook(ctx, bookID); err != nil {
			return nil, err
		}
	}
	return s.store.ListShelfBookRelations(ctx, bookID)
}

func (s *ShelfService) CreateShelfBookRelation(ctx context.Context, sourceID, targetID, relationType string) (domain.ShelfBookRelation, error) {
	if sourceID == targetID {
		return domain.ShelfBookRelation{}, fmt.Errorf("%w: a book cannot be related to itself", domain.ErrInvalid)
	}
	if !shelfRelationTypes[relationType] {
		return domain.ShelfBookRelation{}, fmt.Errorf("%w: unsupported Shelf relation type %q", domain.ErrInvalid, relationType)
	}
	if _, err := s.GetShelfBook(ctx, sourceID); err != nil {
		return domain.ShelfBookRelation{}, err
	}
	if _, err := s.GetShelfBook(ctx, targetID); err != nil {
		return domain.ShelfBookRelation{}, err
	}
	return s.store.CreateShelfBookRelation(ctx, sourceID, targetID, relationType)
}

func (s *ShelfService) DeleteShelfBookRelation(ctx context.Context, id string) error {
	return s.store.DeleteShelfBookRelation(ctx, id)
}

func (s *ShelfService) CreateShelfAsk(ctx context.Context, bookID, question, contextText, reason string) (domain.ShelfAsk, error) {
	if err := invalidInputText("question", question, 10000, true); err != nil {
		return domain.ShelfAsk{}, err
	}
	if err := invalidInputText("question context", contextText, 20000, false); err != nil {
		return domain.ShelfAsk{}, err
	}
	if err := invalidInputText("reason for asking", reason, 10000, false); err != nil {
		return domain.ShelfAsk{}, err
	}
	if _, err := s.GetShelfBook(ctx, bookID); err != nil {
		return domain.ShelfAsk{}, err
	}
	return s.store.CreateShelfAsk(ctx, domain.ShelfAsk{BookID: bookID, Question: question, Context: contextText, Reason: reason})
}

func (s *ShelfService) ListShelfAsks(ctx context.Context, bookID, status string) ([]domain.ShelfAsk, error) {
	if bookID != "" {
		if _, err := s.GetShelfBook(ctx, bookID); err != nil {
			return nil, err
		}
	}
	if status == "all" {
		status = ""
	}
	if status != "" && status != "open" && status != "answered" {
		return nil, fmt.Errorf("%w: ask status must be open, answered, or all", domain.ErrInvalid)
	}
	return s.store.ListShelfAsks(ctx, bookID, status)
}

func (s *ShelfService) AnswerShelfAsk(ctx context.Context, id, answer string) (domain.ShelfAsk, error) {
	if err := invalidInputText("answer", answer, 30000, false); err != nil {
		return domain.ShelfAsk{}, err
	}
	return s.store.AnswerShelfAsk(ctx, id, answer)
}

func (s *ShelfService) CreateShelfNote(ctx context.Context, bookID, title, content string, labels []string) (domain.Note, error) {
	if _, err := s.GetShelfBook(ctx, bookID); err != nil {
		return domain.Note{}, err
	}
	note, err := s.notes.CreateNoteWithLabels(ctx, title, content, labels)
	if err != nil {
		return domain.Note{}, err
	}
	if err := s.notes.LinkNote(ctx, note.ID, "shelf_book", bookID, true); err != nil {
		_ = s.notes.DeleteNote(ctx, note.ID)
		return domain.Note{}, err
	}
	items, err := s.notes.ListNotes(ctx)
	if err != nil {
		return domain.Note{}, err
	}
	for _, item := range items {
		if item.ID == note.ID {
			return item, nil
		}
	}
	return note, nil
}

// ListShelfAnnotations returns durable highlights and marginal notes for a scanned book.

func (s *ShelfService) ListShelfAnnotations(ctx context.Context, bookID string) ([]domain.ShelfAnnotation, error) {
	if _, err := s.GetShelfBook(ctx, bookID); err != nil {
		return nil, err
	}
	return s.store.ListShelfAnnotations(ctx, bookID)
}

// CreateShelfAnnotation stores a selected text range using page-relative geometry so it survives zoom changes.

func (s *ShelfService) CreateShelfAnnotation(ctx context.Context, item domain.ShelfAnnotation) (domain.ShelfAnnotation, error) {
	if err := invalidInputText("book reference", item.BookID, 256, true); err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if _, err := s.GetShelfBook(ctx, item.BookID); err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if item.Page < 1 {
		return domain.ShelfAnnotation{}, fmt.Errorf("%w: page must be greater than zero", domain.ErrInvalid)
	}
	if err := invalidInputText("selected text", item.Quote, 8000, true); err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if err := invalidInputText("annotation note", item.Note, 20000, false); err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if !map[string]bool{"yellow": true, "green": true, "blue": true, "pink": true, "purple": true}[item.Color] {
		return domain.ShelfAnnotation{}, fmt.Errorf("%w: unsupported highlight color", domain.ErrInvalid)
	}
	if len(item.Rectangles) == 0 || len(item.Rectangles) > 300 {
		return domain.ShelfAnnotation{}, fmt.Errorf("%w: highlight must contain between 1 and 300 rectangles", domain.ErrInvalid)
	}
	for _, rect := range item.Rectangles {
		values := []float64{rect.X, rect.Y, rect.Width, rect.Height}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return domain.ShelfAnnotation{}, fmt.Errorf("%w: invalid highlight position", domain.ErrInvalid)
			}
		}
		if rect.X < 0 || rect.Y < 0 || rect.Width <= 0 || rect.Height <= 0 || rect.X > 1 || rect.Y > 1 || rect.X+rect.Width > 1.02 || rect.Y+rect.Height > 1.02 {
			return domain.ShelfAnnotation{}, fmt.Errorf("%w: highlight position is outside the page", domain.ErrInvalid)
		}
	}
	item.Quote = strings.TrimSpace(item.Quote)
	item.Note = strings.TrimSpace(item.Note)
	return s.store.CreateShelfAnnotation(ctx, item)
}

func (s *ShelfService) UpdateShelfAnnotation(ctx context.Context, id, note, color string) (domain.ShelfAnnotation, error) {
	if err := invalidInputText("annotation reference", id, 256, true); err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if err := invalidInputText("annotation note", note, 20000, false); err != nil {
		return domain.ShelfAnnotation{}, err
	}
	if !map[string]bool{"yellow": true, "green": true, "blue": true, "pink": true, "purple": true}[color] {
		return domain.ShelfAnnotation{}, fmt.Errorf("%w: unsupported highlight color", domain.ErrInvalid)
	}
	return s.store.UpdateShelfAnnotation(ctx, id, note, color)
}

func (s *ShelfService) DeleteShelfAnnotation(ctx context.Context, id string) error {
	if err := invalidInputText("annotation reference", id, 256, true); err != nil {
		return err
	}
	return s.store.DeleteShelfAnnotation(ctx, id)
}

func (s *ShelfService) ShelfBookReadme(ctx context.Context, id string) (string, error) {
	book, err := s.GetShelfBook(ctx, id)
	if err != nil {
		return "", err
	}
	if book.ReadmeFile == "" {
		return "", fmt.Errorf("%w: this book does not contain a README.md", domain.ErrNotFound)
	}
	path, err := s.shelfFilePath(ctx, book, book.ReadmeFile)
	if err != nil {
		return "", err
	}
	return readLimited(path, maxShelfReadmeBytes)
}

// ShelfBookPDFPath returns a verified path for HTTP streaming. The server must never accept an
// arbitrary file path from a request; this method derives the filename from a scanned book record.

func (s *ShelfService) ShelfBookPDFPath(ctx context.Context, id string) (string, string, error) {
	book, err := s.GetShelfBook(ctx, id)
	if err != nil {
		return "", "", err
	}
	if book.PDFFile == "" {
		return "", "", fmt.Errorf("%w: this book does not contain a PDF", domain.ErrNotFound)
	}
	path, err := s.shelfFilePath(ctx, book, book.PDFFile)
	if err != nil {
		return "", "", err
	}
	return path, book.PDFFile, nil
}

func (s *ShelfService) shelfFilePath(ctx context.Context, book domain.ShelfBook, name string) (string, error) {
	if !book.Available || name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("%w: requested Shelf file is unavailable", domain.ErrNotFound)
	}
	root, err := s.ShelfRootPath(ctx)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("%w: the configured Shelf folder is unavailable", domain.ErrNotFound)
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, book.DirectoryName, name))
	if err != nil {
		return "", fmt.Errorf("%w: the requested Shelf document is unavailable", domain.ErrNotFound)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: Shelf file resolves outside the library root", domain.ErrInvalid)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: Shelf document is not a regular file", domain.ErrInvalid)
	}
	return path, nil
}

func readLimited(path string, maxBytes int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("file is not a regular file")
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("file exceeds the %d-byte preview limit", maxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func shelfReadmeMetadata(markdown string) (string, string) {
	var title, summary string
	paragraph := []string{}
	for _, raw := range strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			if len(paragraph) > 0 && summary == "" {
				summary = strings.Join(paragraph, " ")
				paragraph = nil
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			if title == "" && strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimLeft(line, "# "))
			}
			continue
		}
		if title != "" && summary != "" {
			break
		}
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "![") || strings.HasPrefix(line, "[") {
			continue
		}
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, ">") {
			continue
		}
		paragraph = append(paragraph, line)
	}
	if summary == "" && len(paragraph) > 0 {
		summary = strings.Join(paragraph, " ")
	}
	if len(summary) > 260 {
		summary = summary[:257] + "…"
	}
	return title, summary
}

func prettifyShelfTitle(folder string) string {
	text := strings.NewReplacer("-", " ", "_", " ").Replace(folder)
	return strings.Join(strings.Fields(text), " ")
}

func invalidInputText(label, value string, max int, required bool) error {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return fmt.Errorf("%w: %s is required", domain.ErrInvalid, label)
	}
	if len([]rune(value)) > max {
		return fmt.Errorf("%w: %s must be at most %d characters", domain.ErrInvalid, label, max)
	}
	return nil
}

func (s *ShelfService) ShelfGraph(ctx context.Context) (domain.Graph, error) {
	books, err := s.ListShelfBooks(ctx)
	if err != nil {
		return domain.Graph{}, err
	}
	relations, err := s.store.ListShelfBookRelations(ctx, "")
	if err != nil {
		return domain.Graph{}, err
	}
	g := domain.Graph{Nodes: []domain.GraphNode{}, Edges: []domain.GraphEdge{}}
	repositoryNodes := map[string]bool{}
	for _, book := range books {
		if !book.Available {
			continue
		}
		description := book.Summary
		if book.ReadingStatus != "" {
			description += " · " + book.ReadingStatus
		}
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: "shelf_book:" + book.ID, Type: "shelf_book", Label: book.Title, Description: description, Status: book.ReadingStatus, Reference: "shelf:" + book.ID})
		if book.RepositoryID != "" {
			if !repositoryNodes[book.RepositoryID] {
				g.Nodes = append(g.Nodes, domain.GraphNode{ID: "repository:" + book.RepositoryID, Type: "repository", Label: book.RepositoryName, Reference: "repo:" + book.RepositoryName})
				repositoryNodes[book.RepositoryID] = true
			}
			g.Edges = append(g.Edges, domain.GraphEdge{Source: "shelf_book:" + book.ID, Target: "repository:" + book.RepositoryID, Type: "has_companion_repository", Label: "companion repository"})
		}
	}
	available := make(map[string]bool, len(books))
	for _, book := range books {
		if book.Available {
			available[book.ID] = true
		}
	}
	for _, rel := range relations {
		if !available[rel.SourceBookID] || !available[rel.TargetBookID] {
			continue
		}
		g.Edges = append(g.Edges, domain.GraphEdge{Source: "shelf_book:" + rel.SourceBookID, Target: "shelf_book:" + rel.TargetBookID, Type: rel.RelationType, Label: rel.RelationType})
	}
	return g, nil
}
