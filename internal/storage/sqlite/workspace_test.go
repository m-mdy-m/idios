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

func TestNotesCanReuseMultipleLabelsAndReplaceThemOnEdit(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "note-labels.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	first, err := store.CreateNoteWithLabels(ctx, "Read about schedulers", "A note", []string{"research", "idea", "Research", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Labels) != 2 {
		t.Fatalf("first note labels = %+v, want 2 distinct labels", first.Labels)
	}
	second, err := store.CreateNoteWithLabels(ctx, "A second thought", "Another note", []string{"idea"})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Labels) != 1 || second.Labels[0].Name != "idea" {
		t.Fatalf("second note did not reuse label: %+v", second.Labels)
	}
	allLabels, err := store.ListNoteLabels(ctx)
	if err != nil || len(allLabels) != 2 {
		t.Fatalf("reusable labels = %+v, err=%v; want research and idea", allLabels, err)
	}

	updated, err := store.UpdateNoteWithLabels(ctx, first.ID, "Scheduler design", "Updated body", []string{"read-later"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Scheduler design" || updated.Content != "Updated body" || len(updated.Labels) != 1 || updated.Labels[0].Name != "read-later" {
		t.Fatalf("note edit did not replace title/body/labels: %+v", updated)
	}
	if len(second.Labels) != 1 || second.Labels[0].Name != "idea" {
		t.Fatalf("editing one note changed another note's labels: %+v", second.Labels)
	}
	labelsAfterEdit, err := store.ListNoteLabels(ctx)
	if err != nil || len(labelsAfterEdit) != 2 {
		t.Fatalf("unused labels were not cleaned after replacement: %+v err=%v", labelsAfterEdit, err)
	}
	if err := store.DeleteNote(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	labelsAfterDelete, err := store.ListNoteLabels(ctx)
	if err != nil || len(labelsAfterDelete) != 1 || labelsAfterDelete[0].Name != "read-later" {
		t.Fatalf("unused labels were not cleaned after note deletion: %+v err=%v", labelsAfterDelete, err)
	}
}

func TestScheduledJobsSupportEditOnceRunNowAndDelete(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "jobs.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().Local().Truncate(time.Minute)
	weekly, err := store.CreateScheduledJob(ctx, domain.ScheduledJob{
		Name: "Weekly planning", Description: "Plan", ActionType: "weekly-review", ScheduleType: domain.ScheduleWeekly,
		ScheduleWeekday: int(time.Weekday((int(now.Weekday()) + 1) % 7)), ScheduleTime: "09:30", Enabled: true, Notify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if weekly.ScheduleType != domain.ScheduleWeekly || weekly.NextRunAt == nil || weekly.NextRunAt.Before(now) {
		t.Fatalf("weekly schedule was not saved: %+v", weekly)
	}

	oneTimeAt := now.Add(2 * time.Hour)
	weekly.Name = "One-time planning"
	weekly.ScheduleType = domain.ScheduleOnce
	weekly.ScheduleTime = ""
	weekly.ScheduleWeekday = 0
	weekly.ScheduleDay = 0
	weekly.IntervalMinutes = 0
	weekly.NextRunAt = &oneTimeAt
	one, err := store.UpdateScheduledJob(ctx, weekly)
	if err != nil {
		t.Fatal(err)
	}
	if one.ScheduleType != domain.ScheduleOnce || one.NextRunAt == nil || !one.NextRunAt.Equal(oneTimeAt) || one.CompletedAt != nil {
		t.Fatalf("editing schedule to one-time did not persist: %+v", one)
	}

	if err := store.RunScheduledJobNow(ctx, one.ID, now, true); err != nil {
		t.Fatal(err)
	}
	completed, err := store.getScheduledJob(ctx, one.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Enabled || completed.CompletedAt == nil || completed.LastRunAt == nil {
		t.Fatalf("one-time job was not marked completed/disabled: %+v", completed)
	}
	notifications, err := store.ListNotifications(ctx, 20)
	if err != nil || len(notifications) != 1 || notifications[0].Title != "One-time planning" {
		t.Fatalf("run-now notification = %+v, err=%v", notifications, err)
	}
	if err := store.RunScheduledJobNow(ctx, one.ID, now.Add(time.Minute), true); err == nil {
		t.Fatal("completed one-time job unexpectedly ran again")
	}
	if _, err := store.ToggleScheduledJob(ctx, one.ID); err == nil {
		t.Fatal("completed one-time job unexpectedly re-enabled without rescheduling")
	}

	if err := store.DeleteScheduledJob(ctx, one.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.getScheduledJob(ctx, one.ID); err == nil {
		t.Fatal("deleted scheduled job is still retrievable")
	}
}

func TestDueOneTimeJobRunsExactlyOnceAndDoesNotRepeat(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "oneshot.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	due := time.Now().Add(-time.Minute)
	job, err := store.CreateScheduledJob(ctx, domain.ScheduledJob{
		Name: "One shot", ActionType: "custom-reminder", ScheduleType: domain.ScheduleOnce,
		NextRunAt: &due, Enabled: true, Notify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	count, err := store.ProcessDueJobs(ctx, time.Now(), true)
	if err != nil || count != 1 {
		t.Fatalf("first processing count = %d err=%v; want one", count, err)
	}
	count, err = store.ProcessDueJobs(ctx, time.Now().Add(24*time.Hour), true)
	if err != nil || count != 0 {
		t.Fatalf("second processing count = %d err=%v; one-time job must not repeat", count, err)
	}
	updated, err := store.getScheduledJob(ctx, job.ID)
	if err != nil || updated.CompletedAt == nil || updated.Enabled {
		t.Fatalf("one-time completion state = %+v err=%v", updated, err)
	}
	notifications, err := store.ListNotifications(ctx, 20)
	if err != nil || len(notifications) != 1 {
		t.Fatalf("one-time notifications = %d err=%v; want one", len(notifications), err)
	}
}
