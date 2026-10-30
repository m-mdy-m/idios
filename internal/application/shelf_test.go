package application_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"idios/internal/domain"
)

func TestShelfScansLocalBooksAndKeepsMetadataAcrossRescan(t *testing.T) {
	app, _ := newApp(t)
	ctx := context.Background()
	root := t.TempDir()
	bookDir := filepath.Join(root, "Digital-Design-and-Computer-Architecture-RISC-V-Edition")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "# Digital Design and Computer Architecture\n\nA practical introduction to digital systems and the RISC-V architecture.\n\n## Contents\n"
	if err := os.WriteFile(filepath.Join(bookDir, "README.md"), []byte(readme), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "Digital-Design.pdf"), []byte("%PDF-1.4 test fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := app.SetShelfRootPath(ctx, root); err != nil {
		t.Fatal(err)
	}
	books, err := app.ListShelfBooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("indexed %d books, want 1", len(books))
	}
	book := books[0]
	if book.ID != filepath.Base(bookDir) || book.Title != "Digital Design and Computer Architecture" || book.PDFFile != "Digital-Design.pdf" || book.ReadmeFile != "README.md" {
		t.Fatalf("unexpected indexed book: %+v", book)
	}
	if !strings.Contains(book.Summary, "RISC-V architecture") {
		t.Fatalf("summary not extracted from README: %q", book.Summary)
	}
	gotReadme, err := app.ShelfBookReadme(ctx, book.ID)
	if err != nil || gotReadme != readme {
		t.Fatalf("README = %q, err=%v", gotReadme, err)
	}
	pdfPath, pdfName, err := app.ShelfBookPDFPath(ctx, book.ID)
	if err != nil || pdfName != "Digital-Design.pdf" {
		t.Fatalf("PDF path = %q, name=%q, err=%v", pdfPath, pdfName, err)
	}

	if err := app.SetShelfBookTags(ctx, book.ID, []string{"architecture", "RISC-V", "architecture"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetShelfBookProgress(ctx, book.ID, "reading", 126, 800); err != nil {
		t.Fatal(err)
	}
	if err := app.SetShelfRootPath(ctx, root); err != nil {
		t.Fatalf("rescan failed: %v", err)
	}
	updated, err := app.GetShelfBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentPage != 126 || updated.TotalPages != 800 || updated.ReadingStatus != "reading" || len(updated.Tags) != 2 {
		t.Fatalf("rescan did not preserve reading metadata: %+v", updated)
	}
}

func TestInvalidShelfRootErrorDoesNotRevealFilesystemPath(t *testing.T) {
	app, _ := newApp(t)
	ctx := context.Background()
	privateRoot := filepath.Join(t.TempDir(), "private-user-folder", "books")
	err := app.SetShelfRootPath(ctx, privateRoot)
	if err == nil {
		t.Fatal("expected unavailable Shelf root to be rejected")
	}
	if strings.Contains(err.Error(), privateRoot) {
		t.Fatalf("error reveals filesystem path %q: %v", privateRoot, err)
	}
}

func TestShelfAsksNotesRelationsAndRepositoryIntegration(t *testing.T) {
	app, store := newApp(t)
	ctx := context.Background()
	root := t.TempDir()
	for _, folder := range []string{"Computer-Organization", "Build-a-PC"} {
		path := filepath.Join(root, folder)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("# "+strings.ReplaceAll(folder, "-", " ")+"\n\nNotes about computer engineering."), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.SetShelfRootPath(ctx, root); err != nil {
		t.Fatal(err)
	}
	books, err := app.ListShelfBooks(ctx)
	if err != nil || len(books) != 2 {
		t.Fatalf("books=%d err=%v", len(books), err)
	}
	first, second := books[0], books[1]

	note, err := app.CreateShelfNote(ctx, first.ID, "Pipeline notes", "The five-stage pipeline", []string{"cpu", "architecture"})
	if err != nil {
		t.Fatal(err)
	}
	if note.LinkedEntityType != "shelf_book" || note.LinkedEntityID != first.ID || !note.GraphVisible {
		t.Fatalf("note is not linked to Shelf book: %+v", note)
	}

	ask, err := app.CreateShelfAsk(ctx, first.ID, "Why is forwarding needed?", "Chapter 4, page 126", "The dependency behavior was unclear.")
	if err != nil {
		t.Fatal(err)
	}
	open, err := app.ListShelfAsks(ctx, first.ID, "open")
	if err != nil || len(open) != 1 || open[0].ID != ask.ID {
		t.Fatalf("open asks=%+v err=%v", open, err)
	}
	answered, err := app.AnswerShelfAsk(ctx, ask.ID, "Forwarding bypasses a register write-back delay.")
	if err != nil || answered.Status != "answered" || answered.AnsweredAt == nil {
		t.Fatalf("answered ask=%+v err=%v", answered, err)
	}
	open, err = app.ListShelfAsks(ctx, first.ID, "open")
	if err != nil || len(open) != 0 {
		t.Fatalf("open asks after answer=%+v err=%v", open, err)
	}
	archive, err := app.ListShelfAsks(ctx, first.ID, "answered")
	if err != nil || len(archive) != 1 || archive[0].Answer == "" {
		t.Fatalf("answered archive=%+v err=%v", archive, err)
	}
	if _, err := app.AnswerShelfAsk(ctx, ask.ID, ""); err != nil {
		t.Fatal(err)
	}
	open, err = app.ListShelfAsks(ctx, first.ID, "open")
	if err != nil || len(open) != 1 {
		t.Fatalf("reopened ask=%+v err=%v", open, err)
	}

	if _, err := app.CreateShelfBookRelation(ctx, first.ID, second.ID, "implements"); err != nil {
		t.Fatal(err)
	}
	repo, err := app.CreateShelfBookRepository(ctx, second.ID, "pc-build-lab", "Practical PC build experiments from the book")
	if err != nil {
		t.Fatal(err)
	}
	linked, err := app.GetShelfBook(ctx, second.ID)
	if err != nil || linked.RepositoryID != repo.ID {
		t.Fatalf("book/repository link=%+v err=%v", linked, err)
	}

	graph, err := app.ShelfGraph(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hasBook, hasRepository, hasRelation := false, false, false
	for _, node := range graph.Nodes {
		if node.ID == "shelf_book:"+first.ID {
			hasBook = true
		}
		if node.ID == "repository:"+repo.ID {
			hasRepository = true
		}
	}
	for _, edge := range graph.Edges {
		if edge.Source == "shelf_book:"+first.ID && edge.Target == "shelf_book:"+second.ID && edge.Type == "implements" {
			hasRelation = true
		}
	}
	if !hasBook || !hasRepository || !hasRelation {
		t.Fatalf("graph missing related nodes/edge: books=%v repo=%v relation=%v graph=%+v", hasBook, hasRepository, hasRelation, graph)
	}

	globalGraph, err := store.ListGraph(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundShelfBook := false
	for _, node := range globalGraph.Nodes {
		if node.ID == "shelf_book:"+first.ID {
			foundShelfBook = true
		}
	}
	if !foundShelfBook {
		t.Fatal("global IDIOS knowledge graph does not include Shelf books")
	}
}

func TestShelfRejectsInvalidProgress(t *testing.T) {
	app, _ := newApp(t)
	ctx := context.Background()
	root := t.TempDir()
	bookDir := filepath.Join(root, "safe-book")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "README.md"), []byte("# Safe book"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.SetShelfRootPath(ctx, root); err != nil {
		t.Fatal(err)
	}
	_, err := app.SetShelfBookProgress(ctx, "safe-book", "reading", 11, 10)
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("invalid progress error = %v", err)
	}
}

func TestShelfPDFAnnotationsPersistAcrossReadsAndValidatePageGeometry(t *testing.T) {
	app, _ := newApp(t)
	ctx := context.Background()
	root := t.TempDir()
	bookDir := filepath.Join(root, "Reader-Test")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "Reader-Test.pdf"), []byte("%PDF-1.4 test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.SetShelfRootPath(ctx, root); err != nil {
		t.Fatal(err)
	}

	item, err := app.CreateShelfAnnotation(ctx, domain.ShelfAnnotation{
		BookID: "Reader-Test", Page: 7, Quote: "A selected passage", Note: "Revisit this algorithm", Color: "green",
		Rectangles: []domain.ShelfAnnotationRect{{X: .12, Y: .31, Width: .4, Height: .02}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == "" || item.Page != 7 || len(item.Rectangles) != 1 || item.Color != "green" {
		t.Fatalf("unexpected annotation: %+v", item)
	}
	items, err := app.ListShelfAnnotations(ctx, "Reader-Test")
	if err != nil || len(items) != 1 || items[0].Quote != "A selected passage" || items[0].Rectangles[0].X != .12 {
		t.Fatalf("annotations=%+v err=%v", items, err)
	}
	updated, err := app.UpdateShelfAnnotation(ctx, item.ID, "Found the explanation", "blue")
	if err != nil || updated.Note != "Found the explanation" || updated.Color != "blue" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if _, err := app.CreateShelfAnnotation(ctx, domain.ShelfAnnotation{BookID: "Reader-Test", Page: 1, Quote: "bad", Color: "yellow", Rectangles: []domain.ShelfAnnotationRect{{X: .9, Y: .5, Width: .2, Height: .1}}}); err == nil {
		t.Fatal("expected out-of-page highlight geometry to be rejected")
	}
	if err := app.DeleteShelfAnnotation(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	items, err = app.ListShelfAnnotations(ctx, "Reader-Test")
	if err != nil || len(items) != 0 {
		t.Fatalf("annotations after delete=%+v err=%v", items, err)
	}
}
