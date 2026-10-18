package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"idios/internal/domain"
)

func TestDemoSeedCreatesConnectedWorkspaceAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "demo.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	report, err := store.SeedDemo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Repositories != 3 || report.Issues != 8 || report.Projects != 1 || report.Concepts != 2 || report.Notes != 3 || report.Quotes != 4 || report.Jobs != 3 {
		t.Fatalf("unexpected demo seed report: %+v", report)
	}
	repos, err := store.ListRepositories(ctx)
	if err != nil || len(repos) != 3 {
		t.Fatalf("seeded repositories = %d, err=%v; want 3", len(repos), err)
	}
	issues, err := store.ListIssues(ctx, "", "")
	if err != nil || len(issues) != 8 {
		t.Fatalf("seeded issues = %d, err=%v; want 8", len(issues), err)
	}
	graph, err := store.ListGraph(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantEdges := map[string]bool{"has_sub_issue": false, "depends_on": false, "produces_completion": false, "note_about": false, "supports": false}
	for _, edge := range graph.Edges {
		if _, ok := wantEdges[edge.Type]; ok {
			wantEdges[edge.Type] = true
		}
	}
	for edge, found := range wantEdges {
		if !found {
			t.Errorf("demo graph is missing a %q edge", edge)
		}
	}
	pins, err := store.ListWorkspacePins(ctx)
	if err != nil || len(pins) < 2 {
		t.Fatalf("seeded important pins = %d, err=%v; want at least 2", len(pins), err)
	}

	repeat, err := store.SeedDemo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !repeat.AlreadySeeded {
		t.Fatalf("second seed did not report AlreadySeeded: %+v", repeat)
	}
	repos, err = store.ListRepositories(ctx)
	if err != nil || len(repos) != 3 {
		t.Fatalf("repeat seed duplicated repositories: len=%d err=%v", len(repos), err)
	}
}

func TestNotesQuotesAndScheduledNotifications(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "workspace.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.CreateRepository(ctx, "NotesRepo", "notes test"); err != nil {
		t.Fatal(err)
	}
	note, err := store.CreateNote(ctx, "Graph idea", "A note that becomes a graph node.")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkNote(ctx, note.ID, "repository", "NotesRepo", true); err != nil {
		t.Fatal(err)
	}
	notes, err := store.ListNotes(ctx)
	if err != nil || len(notes) != 1 || notes[0].LinkedEntityLabel != "NotesRepo" || !notes[0].GraphVisible {
		t.Fatalf("linked note not persisted: notes=%+v err=%v", notes, err)
	}
	graph, err := store.ListGraph(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundNoteEdge := false
	for _, edge := range graph.Edges {
		if edge.Type == "note_about" {
			foundNoteEdge = true
			break
		}
	}
	if !foundNoteEdge {
		t.Fatal("graph-visible note did not produce a note_about edge")
	}

	first, err := store.CreateQuote(ctx, "One useful line", "author one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateQuote(ctx, "Another useful line", "author two")
	if err != nil {
		t.Fatal(err)
	}
	q1, err := store.NextQuote(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q2, err := store.NextQuote(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if q1.ID == q2.ID || !((q1.ID == first.ID && q2.ID == second.ID) || (q1.ID == second.ID && q2.ID == first.ID)) {
		t.Fatalf("quotes did not rotate across calls: %s, %s", q1.ID, q2.ID)
	}

	due := time.Now().Add(-time.Minute)
	job, err := store.CreateScheduledJob(ctx, domain.ScheduledJob{Name: "Test reminder", Description: "Review the graph", ActionType: "blocked-issues", IntervalMinutes: 60, Enabled: true, Notify: true, NextRunAt: &due})
	if err != nil {
		t.Fatal(err)
	}
	count, err := store.ProcessDueJobs(ctx, time.Now(), true)
	if err != nil || count != 1 {
		t.Fatalf("due jobs processed = %d, err=%v; want 1", count, err)
	}
	notifications, err := store.ListNotifications(ctx, 20)
	if err != nil || len(notifications) != 1 || notifications[0].Title != job.Name {
		t.Fatalf("notification not recorded: %+v err=%v", notifications, err)
	}
	updated, err := store.getScheduledJob(ctx, job.ID)
	if err != nil || updated.LastRunAt == nil || updated.NextRunAt == nil || !updated.NextRunAt.After(*updated.LastRunAt) {
		t.Fatalf("job schedule was not advanced after running: %+v err=%v", updated, err)
	}
}
