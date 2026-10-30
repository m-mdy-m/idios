package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"idios/internal/diagnostics"
	"idios/internal/domain"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Shelf is a local-first catalog: source documents stay in their original folders while IDIOS
// stores reading metadata, tags, typed relations and the question archive in its SQLite database.
func (s *Server) getShelf(w http.ResponseWriter, r *http.Request) {
	root, err := s.app.ShelfRootPath(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	books, err := s.app.ListShelfBooks(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"root_path": root, "configured": strings.TrimSpace(root) != "", "books": books})
}

func (s *Server) getShelfReadingSessions(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListShelfReadingSessions(requestContext(r), r.URL.Query().Get("book_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createShelfReadingSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID        string    `json:"id"`
		BookID    string    `json:"book_id"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`
		StartPage int       `json:"start_page"`
		EndPage   int       `json:"end_page"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateShelfReadingSession(requestContext(r), domain.ShelfReadingSession{
		ID: input.ID, BookID: input.BookID, StartedAt: input.StartedAt, EndedAt: input.EndedAt,
		StartPage: input.StartPage, EndPage: input.EndPage,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateShelfSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RootPath string `json:"root_path"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.SetShelfRootPath(requestContext(r), input.RootPath); err != nil {
		writeError(w, err)
		return
	}
	root, err := s.app.ShelfRootPath(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	books, err := s.app.ListShelfBooks(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"root_path": root, "configured": root != "", "books": books})
}

func (s *Server) scanShelf(w http.ResponseWriter, r *http.Request) {
	books, err := s.app.ScanShelfBooks(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	root, err := s.app.ShelfRootPath(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"root_path": root, "configured": strings.TrimSpace(root) != "", "books": books})
}

func (s *Server) getShelfBooks(w http.ResponseWriter, r *http.Request) {
	books, err := s.app.ListShelfBooks(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, books)
}

func (s *Server) getShelfBook(w http.ResponseWriter, r *http.Request) {
	book, err := s.app.GetShelfBook(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) getShelfAnnotations(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListShelfAnnotations(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createShelfAnnotation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Page       int                          `json:"page"`
		Quote      string                       `json:"quote"`
		Note       string                       `json:"note"`
		Color      string                       `json:"color"`
		Rectangles []domain.ShelfAnnotationRect `json:"rectangles"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Color == "" {
		input.Color = "yellow"
	}
	item, err := s.app.CreateShelfAnnotation(requestContext(r), domain.ShelfAnnotation{BookID: r.PathValue("ref"), Page: input.Page, Quote: input.Quote, Note: input.Note, Color: input.Color, Rectangles: input.Rectangles})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateShelfAnnotation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Note  string `json:"note"`
		Color string `json:"color"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.UpdateShelfAnnotation(requestContext(r), r.PathValue("ref"), input.Note, input.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteShelfAnnotation(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteShelfAnnotation(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getShelfBookReadme(w http.ResponseWriter, r *http.Request) {
	content, err := s.app.ShelfBookReadme(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}

func (s *Server) getShelfBookPDF(w http.ResponseWriter, r *http.Request) {
	// This response marker lets the embedded Reader detect a stale, separately installed
	// IDIOS server instead of surfacing PDF.js's confusing generic 204 response.
	w.Header().Set("X-IDIOS-PDF-Protocol", "shelf-pdf-bytes-v1")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// The server-wide 30s WriteTimeout can cut off a large PDF on a slow disk; give this one route more time.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
	path, name, err := s.app.ShelfBookPDFPath(requestContext(r), r.PathValue("ref"))
	if err != nil {
		// A cancelled browser request is not a server error. The client may have navigated
		// away while the book record was being resolved.
		if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
			return
		}
		writeError(w, err)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, err)
		return
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		writeError(w, fmt.Errorf("%w: the PDF file is empty or unavailable", domain.ErrInvalid))
		return
	}
	var prefix [1024]byte
	read, _ := file.ReadAt(prefix[:], 0)
	if !bytes.Contains(prefix[:read], []byte("%PDF-")) {
		writeError(w, fmt.Errorf("%w: the selected file does not contain a valid PDF header", domain.ErrInvalid))
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": name}))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))

	// PDF.js in Shelf requests the complete file as one no-cache GET. Return an explicit
	// 200 and stream the file bytes for that path; do not allow validators or a stale cache
	// entry to turn the Reader response into 204/304 with no body. Keep native/browser
	// Range requests supported for the optional external PDF viewer.
	r.Header.Del("If-Modified-Since")
	r.Header.Del("If-None-Match")
	if strings.TrimSpace(r.Header.Get("Range")) != "" {
		http.ServeContent(w, r, name, info.ModTime(), file)
		return
	}
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	if _, copyErr := io.Copy(w, file); copyErr != nil && r.Context().Err() == nil {
		diagnostics.Write("ERROR", "failed to stream a Shelf PDF: "+copyErr.Error(), s.debugEnabled())
	}
}

// getShelfBookPDFData returns the same verified PDF as base64 inside an ordinary JSON response.
// Some browser extensions, download managers and local proxies intercept responses that look like
// PDF downloads and replace them with an empty one; a JSON API response is never treated that way.
func (s *Server) getShelfBookPDFData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-IDIOS-PDF-Protocol", "shelf-pdf-bytes-v1")
	w.Header().Set("Cache-Control", "no-store")
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
	path, name, err := s.app.ShelfBookPDFPath(requestContext(r), r.PathValue("ref"))
	if err != nil {
		if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
			return
		}
		writeError(w, err)
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeError(w, err)
		return
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		writeError(w, fmt.Errorf("%w: the PDF file is empty or unavailable", domain.ErrInvalid))
		return
	}
	if info.Size() > 256<<20 {
		writeError(w, fmt.Errorf("%w: the PDF is larger than the 256 MB in-app Reader limit", domain.ErrInvalid))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, err)
		return
	}
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	if !bytes.Contains(head, []byte("%PDF-")) {
		writeError(w, fmt.Errorf("%w: the selected file does not contain a valid PDF header", domain.ErrInvalid))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "size": len(data), "data": base64.StdEncoding.EncodeToString(data)})
}

func (s *Server) updateShelfBookTags(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Tags []string `json:"tags"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.SetShelfBookTags(requestContext(r), r.PathValue("ref"), input.Tags); err != nil {
		writeError(w, err)
		return
	}
	book, err := s.app.GetShelfBook(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) updateShelfBookProgress(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status      string `json:"status"`
		CurrentPage int    `json:"current_page"`
		TotalPages  int    `json:"total_pages"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	book, err := s.app.SetShelfBookProgress(requestContext(r), r.PathValue("ref"), input.Status, input.CurrentPage, input.TotalPages)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) getShelfBookNotes(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("ref")
	if _, err := s.app.GetShelfBook(requestContext(r), bookID); err != nil {
		writeError(w, err)
		return
	}
	notes, err := s.app.ListNotes(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]domain.Note, 0)
	for _, note := range notes {
		if note.LinkedEntityType == "shelf_book" && note.LinkedEntityID == bookID {
			result = append(result, note)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createShelfBookNote(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Labels  []string `json:"labels"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	note, err := s.app.CreateShelfNote(requestContext(r), r.PathValue("ref"), input.Title, input.Content, input.Labels)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) getShelfAsksForBook(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListShelfAsks(requestContext(r), r.PathValue("ref"), "all")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createShelfAsk(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Question string `json:"question"`
		Context  string `json:"context"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateShelfAsk(requestContext(r), r.PathValue("ref"), input.Question, input.Context, input.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) createShelfBookRepository(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	repo, err := s.app.CreateShelfBookRepository(requestContext(r), r.PathValue("ref"), input.Name, input.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (s *Server) linkShelfBookRepository(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Repository string `json:"repository"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LinkShelfBookRepository(requestContext(r), r.PathValue("ref"), input.Repository); err != nil {
		writeError(w, err)
		return
	}
	book, err := s.app.GetShelfBook(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) getShelfAsks(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListShelfAsks(requestContext(r), r.URL.Query().Get("book_id"), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) answerShelfAsk(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Answer string `json:"answer"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.AnswerShelfAsk(requestContext(r), r.PathValue("ref"), input.Answer)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getShelfRelations(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListShelfBookRelations(requestContext(r), r.URL.Query().Get("book_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createShelfRelation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourceBookID string `json:"source_book_id"`
		TargetBookID string `json:"target_book_id"`
		RelationType string `json:"relation_type"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateShelfBookRelation(requestContext(r), input.SourceBookID, input.TargetBookID, input.RelationType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) deleteShelfRelation(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteShelfBookRelation(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getShelfGraph(w http.ResponseWriter, r *http.Request) {
	graph, err := s.app.ShelfGraph(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, graph)
}
