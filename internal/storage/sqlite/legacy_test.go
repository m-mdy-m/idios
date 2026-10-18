package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"idios/internal/domain"
)

func TestLegacyImportMapsTasksToIssuesAndPreservesDoneInvariant(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "legacy.sqlite3")
	enginePath := filepath.Join(dir, "engine.sqlite3")
	old, err := openDatabase(legacyPath, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE concepts(seq INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT NOT NULL UNIQUE,name TEXT NOT NULL,description TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`,
		`CREATE TABLE tasks(seq INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT NOT NULL UNIQUE,text TEXT NOT NULL,due_date TEXT NOT NULL,status TEXT NOT NULL,done_at TEXT,goal_id TEXT,source_id TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`,
		`CREATE TABLE relations(id TEXT PRIMARY KEY,source_type TEXT,source_id TEXT,target_type TEXT,target_id TEXT,type TEXT,created_at TEXT,updated_at TEXT)`,
	} {
		if _, err := old.ExecContext(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	now := nowText()
	if _, err := old.ExecContext(ctx, `INSERT INTO concepts(id,name,description,created_at,updated_at) VALUES('legacy-concept-1','CPU','Processor concept',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := old.ExecContext(ctx, `INSERT INTO tasks(id,text,due_date,status,done_at,goal_id,source_id,created_at,updated_at) VALUES('legacy-task-done','Learn CPU','2024-01-03','done','2024-01-04T10:00:00Z',NULL,NULL,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := old.ExecContext(ctx, `INSERT INTO tasks(id,text,due_date,status,done_at,goal_id,source_id,created_at,updated_at) VALUES('legacy-task-open','Learn paging','2024-01-04','open',NULL,NULL,NULL,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	engine, err := OpenStore(enginePath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	result, err := engine.ImportLegacy(ctx, legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.ImportedTasks != 2 || result.ImportedConcepts != 1 || result.ImportedDoneTasks != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	done, err := engine.GetIssue(ctx, "legacy-task-done")
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != domain.IssueDone {
		t.Fatalf("migrated done task has status %s", done.Status)
	}
	prs, err := engine.ListPullRequests(ctx, "Legacy Tasks", "accepted")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected a synthetic accepted PR, got %d", len(prs))
	}
	if !contains(prs[0].Description, "Synthetic migration record") {
		t.Fatalf("migration provenance missing: %s", prs[0].Description)
	}
	concept, err := engine.GetConcept(ctx, "CPU")
	if err != nil {
		t.Fatal(err)
	}
	if concept.Name != "CPU" {
		t.Fatalf("concept not imported: %+v", concept)
	}
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
