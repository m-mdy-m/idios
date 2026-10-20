package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"idios/internal/domain"
)

func openDeleteTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "delete-tests.sqlite3"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return store
}

func TestDeleteRepositoryCascadesWorkAndDetachesNotes(t *testing.T) {
	ctx := context.Background()
	store := openDeleteTestStore(t)
	repo, err := store.CreateRepository(ctx, "CPU", "CPU work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Keep this consistent"}); err != nil {
		t.Fatal(err)
	}
	note, err := store.CreateNote(ctx, "Design note", "Keep this note even if the repository is removed.")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkNote(ctx, note.ID, "repository", repo.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRepositorySetting(ctx, repo.ID, "purpose", "tests"); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteRepository(ctx, repo.ID); err != nil {
		t.Fatalf("DeleteRepository: %v", err)
	}
	repositories, err := store.ListRepositories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 0 {
		t.Fatalf("repositories after deletion = %d, want 0", len(repositories))
	}
	issues, err := store.ListIssues(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues after repository deletion = %d, want 0", len(issues))
	}
	notes, err := store.ListNotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("notes after repository deletion = %d, want 1 preserved note", len(notes))
	}
	if notes[0].LinkedEntityID != "" || notes[0].LinkedEntityType != "" || notes[0].GraphVisible {
		t.Fatalf("note still contains a dead repository link: %#v", notes[0])
	}
	if _, err := store.p().must(ctx, "repository settings", `SELECT repository_id FROM repository_settings WHERE repository_id=?`, repo.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("repository settings still exist, err=%v", err)
	}
}

func TestDeleteIssueRefusesIncomingDependencyAndAllowsCleanup(t *testing.T) {
	ctx := context.Background()
	store := openDeleteTestStore(t)
	repo, err := store.CreateRepository(ctx, "CORE", "")
	if err != nil {
		t.Fatal(err)
	}
	prerequisite, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Prerequisite"})
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Blocked work"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddDependency(ctx, fmt.Sprintf("CORE#%d", blocked.Number), fmt.Sprintf("CORE#%d", prerequisite.Number)); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteIssue(ctx, prerequisite.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("DeleteIssue(prerequisite) error = %v, want ErrConflict", err)
	}
	if _, err := store.GetIssue(ctx, prerequisite.ID); err != nil {
		t.Fatalf("protected Issue was deleted: %v", err)
	}
	if err := store.DeleteIssue(ctx, blocked.ID); err != nil {
		t.Fatalf("delete dependent Issue: %v", err)
	}
	if err := store.DeleteIssue(ctx, prerequisite.ID); err != nil {
		t.Fatalf("delete prerequisite after dependency cleanup: %v", err)
	}
}

func TestDeletePullRequestProtectsAcceptedCompletionHistory(t *testing.T) {
	ctx := context.Background()
	store := openDeleteTestStore(t)
	repo, err := store.CreateRepository(ctx, "HISTORY", "")
	if err != nil {
		t.Fatal(err)
	}
	issue, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Document the result"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartIssue(ctx, issue.ID); err != nil {
		t.Fatal(err)
	}
	pr, err := store.CreatePullRequest(ctx, fmt.Sprintf("HISTORY#%d", issue.Number), "Document result", "Verified result")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := store.AcceptPullRequest(ctx, fmt.Sprintf("HISTORY!%d", pr.Number), "Tested"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePullRequest(ctx, pr.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("DeletePullRequest(accepted) error = %v, want ErrConflict", err)
	}
	if _, err := store.GetPullRequest(ctx, pr.ID); err != nil {
		t.Fatalf("accepted Pull Request was deleted: %v", err)
	}
}

func TestDeleteQuoteRemovesSavedQuote(t *testing.T) {
	ctx := context.Background()
	store := openDeleteTestStore(t)
	quote, err := store.CreateQuote(ctx, "Delete test", "IDIOS")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteQuote(ctx, quote.ID); err != nil {
		t.Fatal(err)
	}
	quotes, err := store.ListQuotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 0 {
		t.Fatalf("quotes after deletion = %d, want 0", len(quotes))
	}
}
