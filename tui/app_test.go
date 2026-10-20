package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/domain"
	"idios/internal/storage/sqlite"

	tea "charm.land/bubbletea/v2"
)

func TestWorkspaceViewLooksLikePersistentFullScreenUI(t *testing.T) {
	m := newModel(context.Background(), nil, "./idios.sqlite3", Options{NoColor: true})
	m.page = pageOverview
	m.width, m.height = 116, 38
	m.repositories = []domain.Repository{{Name: "CPU", Description: "Hardware and processor research"}, {Name: "ARLIZ", Description: "Writing and research"}}
	m.projects = []domain.Project{{Name: "Build a personal OS", Description: "Cross-repository project"}}
	m.issues = []domain.Issue{{Number: 4, RepositoryName: "CPU", Title: "Learn electricity", Status: domain.IssueOpen}}
	m.pullRequests = []domain.PullRequest{{Number: 2, RepositoryName: "CPU", Title: "Electricity notes", Status: domain.PRPending}}
	v := m.View()

	if !v.AltScreen {
		t.Fatal("TUI should occupy the alternate screen, not print a line-oriented prompt")
	}
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Fatal("mouse mode should be enabled for clickable controls")
	}
	for _, want := range []string{"IDIOS", "Overview", "Repositories", "Issues", "Jobs", "CPU", "ARLIZ"} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("rendered workspace does not contain %q", want)
		}
	}
	if strings.Contains(v.Content, "idios>") {
		t.Fatal("legacy prompt was rendered; UI should remain in a persistent workspace view")
	}
}

func TestMouseClickNavigatesUsingTheEventLoop(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageIssues // the workspace sidebar contains the Projects destination
	m.width, m.height = 110, 32
	v := m.View()

	var target *hitbox
	for i := range m.hits {
		if m.hits[i].action == "nav:projects" {
			target = &m.hits[i]
			break
		}
	}
	if target == nil {
		t.Fatal("expected a clickable Projects tab")
	}
	if v.OnMouse == nil {
		t.Fatal("Bubble Tea view should expose a mouse handler for the rendered hitboxes")
	}
	cmd := v.OnMouse(tea.MouseClickMsg{X: target.x, Y: target.y, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("clicking the Projects tab should dispatch an action")
	}
	msg := cmd()
	updated, _ := m.Update(msg)
	if got := updated.(*model).page; got != pageProjects {
		t.Fatalf("page after clicking Projects = %q, want %q", got, pageProjects)
	}
}

func TestNarrowTerminalWrapsTabsInsteadOfHidingThem(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.width, m.height = 64, 34
	view := m.View()
	for _, want := range []string{"Overview", "Repositories", "Issues", "Jobs", "Commands"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("tab %q disappeared at narrow terminal width", want)
		}
	}
}

func TestGraphViewShowsTypedBlockingRelationshipAndClickableNodes(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.width, m.height = 110, 38
	m.page = pageGraph
	m.graph = domain.Graph{
		Nodes: []domain.GraphNode{
			{ID: "cpu-issue", Type: "issue", Label: "Learn electricity fundamentals", Status: "open", Reference: "issue:CPU#4"},
			{ID: "arliz-issue", Type: "issue", Label: "Write ARLIZ chapter three", Status: "blocked", Reference: "issue:ARLIZ#3"},
			{ID: "electricity", Type: "concept", Label: "Electricity", Reference: "concept:Electricity"},
		},
		Edges: []domain.GraphEdge{
			{Source: "arliz-issue", Target: "cpu-issue", Type: "depends_on"},
			{Source: "arliz-issue", Target: "electricity", Type: "uses_concept"},
		},
	}
	view := m.View()
	for _, want := range []string{"What is blocked right now", "waits for", "[BLOCKED]", "uses concept", "CPU#4", "ARLIZ#3", "Electricity"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("graph view does not show %q", want)
		}
	}
	for _, h := range m.hits {
		if h.action == "graph-node:cpu-issue" {
			return
		}
	}
	t.Fatal("graph nodes should have click targets")
}

func TestFormSupportsCaretEditingAndPaste(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.beginForm(formRepository)
	m.handleFormKey("c")
	m.handleFormKey("p")
	m.handleFormKey("u")
	m.handleFormKey("left")
	m.Update(tea.PasteMsg{Content: "X"})
	if got, want := m.formFields[0].value, "cpXu"; got != want {
		t.Fatalf("field after inserting pasted text = %q, want %q", got, want)
	}
	m.handleFormKey("backspace")
	if got, want := m.formFields[0].value, "cpu"; got != want {
		t.Fatalf("field after backspace = %q, want %q", got, want)
	}
}

func TestFormValidationErrorKeepsTheEnteredValues(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.beginForm(formRepository)
	m.formFields[0].value = "CPU"
	m.formCaret = 3
	updated, _ := m.Update(operationMsg{err: domain.ErrInvalid})
	got := updated.(*model)
	if got.form != formRepository || got.formFields[0].value != "CPU" || got.formError == "" {
		t.Fatal("validation errors should preserve the form and all user-entered data")
	}
}

func TestClickingInputAreaMovesFormFocus(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.width, m.height = 110, 36
	m.beginForm(formRepository)
	m.formFocus = 0
	v := m.View()

	var target *hitbox
	for i := range m.hits {
		if m.hits[i].action == "form-field:1" {
			target = &m.hits[i]
			break
		}
	}
	if target == nil {
		t.Fatal("the whole description input should be clickable")
	}
	cmd := v.OnMouse(tea.MouseClickMsg{X: target.x + target.w/2, Y: target.y, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("clicking the input field should dispatch a focus action")
	}
	msg := cmd()
	updated, _ := m.Update(msg)
	got := updated.(*model)
	if got.formFocus != 1 {
		t.Fatalf("focused field = %d, want 1", got.formFocus)
	}
}

func TestCompactTerminalDoesNotRenderPastItsBounds(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.width, m.height = 35, 10
	v := m.View()
	if !v.AltScreen {
		t.Fatal("compact mode should still use the full-screen terminal buffer")
	}
	lines := strings.Split(v.Content, "\n")
	if len(lines) != m.height {
		t.Fatalf("compact view has %d lines, expected %d", len(lines), m.height)
	}
	for i, line := range lines {
		if len([]rune(line)) > m.width {
			t.Errorf("line %d exceeds terminal width: %d > %d", i, len([]rune(line)), m.width)
		}
	}
	if !strings.Contains(v.Content, "too small") {
		t.Fatal("compact view should explain how to restore the full UI")
	}
}

func TestRepositoryFormLoadsConfiguredOverviewTemplate(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.RepositoryTemplate.Description = "# {{repository}}\n\n## Purpose\n\nDescribe the outcome."
	m := newModel(context.Background(), nil, "", Options{NoColor: true, Config: cfg})
	m.beginForm(formRepository)
	if len(m.formFields) != 2 || !m.formFields[1].multiline {
		t.Fatalf("unexpected Repository form shape: %#v", m.formFields)
	}
	if got := m.formFields[1].value; got != cfg.RepositoryTemplate.Description {
		t.Fatalf("Repository overview template = %q, want configured template", got)
	}
}

func TestIssueTemplateSelectionLoadsConfiguredMultilineBody(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := newModel(context.Background(), nil, "", Options{NoColor: true, Config: cfg})
	m.repositories = []domain.Repository{{Name: "IDIOS"}}
	m.beginForm(formIssue)
	if len(m.formFields) != 6 || !m.formFields[3].multiline {
		t.Fatalf("unexpected Issue form shape before selection: %#v", m.formFields)
	}
	m.formFields[0].value = "bug"
	if !m.applyIssueTemplateSelection() {
		t.Fatalf("could not apply bug template: %s", m.formError)
	}
	if m.formFields[2].value != "[Bug]: " {
		t.Fatalf("bug template title prefix = %q", m.formFields[2].value)
	}
	if !m.formFields[3].multiline || !strings.Contains(m.formFields[3].value, "## Expected behavior") || !strings.Contains(m.formFields[3].value, "## Steps to reproduce") {
		t.Fatal("bug template should provide a structured, editable, multiline body")
	}
	if m.formTitle != "Create Issue · Bug report" {
		t.Fatalf("form title = %q", m.formTitle)
	}
}

func TestPullRequestFormUsesConfigurableMultilineTemplate(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.PullRequestTemplate.TitlePrefix = "Review: "
	cfg.PullRequestTemplate.Body = "## Summary\n\n## Verification\n\n- [ ] Tests pass"
	m := newModel(context.Background(), nil, "", Options{NoColor: true, Config: cfg})
	m.selectedIssueRef = "CPU#4"
	m.issues = []domain.Issue{{Number: 4, RepositoryName: "CPU", Title: "Implement graph view", Status: domain.IssueInProgress}}
	m.beginForm(formPullRequest)
	if m.formFields[0].value != "CPU#4" {
		t.Fatalf("Issue reference prefill = %q", m.formFields[0].value)
	}
	if m.formFields[1].value != "Review: Implement graph view" {
		t.Fatalf("PR title prefill = %q", m.formFields[1].value)
	}
	if !m.formFields[2].multiline || m.formFields[2].value != cfg.PullRequestTemplate.Body {
		t.Fatalf("PR form did not load configured body: %#v", m.formFields[2])
	}
}

func TestMultilineFormSupportsEnterAndMultilinePaste(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true, Config: config.DefaultAppConfig()})
	m.beginForm(formRepository)
	m.formFocus = 1
	m.formFields[1].value = "alpha"
	m.formCaret = len([]rune(m.formFields[1].value))
	m.handleFormKey("enter")
	m.Update(tea.PasteMsg{Content: "beta\r\ngamma"})
	if got, want := m.formFields[1].value, "alpha\nbeta\ngamma"; got != want {
		t.Fatalf("multiline editing result = %q, want %q", got, want)
	}
}

func TestMultilineFieldNavigationPreservesColumn(t *testing.T) {
	value := "first\nsecond\nthird"
	caret := len([]rune("first\nsec"))
	if got, ok := moveMultilineCaret(value, caret, 1); !ok || got != len([]rune("first\nsecond\nthi")) {
		t.Fatalf("moving caret down = (%d, %t)", got, ok)
	}
	if got := multilineLineBoundary(value, caret, false); got != len([]rune("first\n")) {
		t.Fatalf("Home boundary = %d", got)
	}
	lines := multilineFieldLines(value, caret, true, "placeholder", 2)
	rendered := strings.Join(lines, "\n")
	contentWithoutCaret := strings.ReplaceAll(rendered, "▏", "")
	if !strings.Contains(contentWithoutCaret, "second") || !strings.Contains(rendered, "sec▏ond") {
		t.Fatalf("multiline viewport omitted the caret line or caret marker: %#v", lines)
	}
}

func TestCleanAndDemoTUIAreVisiblySeparatedWithoutInAppSeedAction(t *testing.T) {
	clean := newModel(context.Background(), nil, "clean.sqlite3", Options{NoColor: true, Config: config.DefaultAppConfig()})
	clean.page = pageIntro
	clean.width, clean.height = 116, 38
	cleanView := clean.View()
	if strings.Contains(strings.ToLower(cleanView.Content), "load demo workspace") {
		t.Fatal("standard TUI must not offer an in-app demo-loading action")
	}
	if strings.Contains(cleanView.Content, "DEMO WORKSPACE") || strings.Contains(cleanView.Content, "· DEMO") {
		t.Fatal("standard TUI must not be labeled as the demo workspace")
	}

	demo := newModel(context.Background(), nil, "demo.sqlite3", Options{NoColor: true, DemoMode: true, Config: config.DefaultAppConfig()})
	demo.page = pageIntro
	demo.width, demo.height = 116, 38
	demoView := demo.View()
	if !strings.Contains(demoView.Content, "DEMO") || !strings.Contains(demoView.Content, "DEMO WORKSPACE") {
		t.Fatal("demo TUI should be explicitly marked as the isolated demo workspace")
	}
	if strings.Contains(strings.ToLower(demoView.Content), "load demo workspace") {
		t.Fatal("demo TUI must not offer a redundant in-app demo-loading action")
	}
}

func TestScheduledJobFormParsesFlexibleSchedules(t *testing.T) {
	interval, err := parseScheduledJobFields("interval", "90m")
	if err != nil || interval.IntervalMinutes != 90 {
		t.Fatalf("parsed 90m interval = %+v err=%v", interval, err)
	}

	weekly, err := parseScheduledJobFields("weekly", "Monday 09:30")
	if err != nil || weekly.ScheduleWeekday != int(time.Monday) || weekly.ScheduleTime != "09:30" {
		t.Fatalf("parsed weekly schedule = %+v err=%v", weekly, err)
	}

	monthly, err := parseScheduledJobFields("monthly", "31 17:45")
	if err != nil || monthly.ScheduleDay != 31 || monthly.ScheduleTime != "17:45" {
		t.Fatalf("parsed monthly schedule = %+v err=%v", monthly, err)
	}

	future := time.Now().Local().Add(48 * time.Hour).Truncate(time.Minute)
	once, err := parseScheduledJobFields("once", future.Format("2006-01-02 15:04"))
	if err != nil || once.NextRunAt == nil || !once.NextRunAt.Equal(future) {
		t.Fatalf("parsed one-time schedule = %+v err=%v, want %s", once, err, future)
	}

	if _, err := parseScheduledJobFields("daily", "tomorrow morning"); err == nil {
		t.Fatal("invalid daily time unexpectedly parsed")
	}
}

func TestNotesFilterUsesSharedLabelsAndSearch(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.notes = []domain.Note{
		{ID: "one", Title: "Scheduler idea", Content: "Interval jobs", Labels: []domain.NoteLabel{{Name: "research"}, {Name: "idea"}}},
		{ID: "two", Title: "Book notes", Content: "Read later", Labels: []domain.NoteLabel{{Name: "reading"}}},
	}
	m.activeNoteLabel = "idea"
	if got := m.filteredNotes(); len(got) != 1 || got[0].ID != "one" {
		t.Fatalf("label filter returned %+v, want only note one", got)
	}
	m.search = "book"
	if got := m.filteredNotes(); len(got) != 0 {
		t.Fatalf("combined label + text filter returned %+v, want empty", got)
	}
	m.activeNoteLabel, m.search = "", "#reading"
	if got := m.filteredNotes(); len(got) != 1 || got[0].ID != "two" {
		t.Fatalf("search did not match note labels: %+v", got)
	}
}

func TestWorkspacePagesExposeNoteTagsAndJobControls(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.width, m.height, m.page = 120, 45, pageNotes
	m.selectedNoteID = "note-a"
	m.notes = []domain.Note{{ID: "note-a", Title: "Design thoughts", Content: "Keep labels reusable", Labels: []domain.NoteLabel{{ID: "tag-a", Name: "idea"}, {ID: "tag-b", Name: "research"}}}}
	m.noteLabels = []domain.NoteLabel{{Name: "idea"}, {Name: "research"}}
	view := m.View()
	for _, want := range []string{"#idea", "#research", "Labels", "Design thoughts"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("Notes view does not include %q", want)
		}
	}

	m.page = pageJobs
	future := time.Now().Add(time.Hour)
	m.jobs = []domain.ScheduledJob{{ID: "job-a", Name: "Hourly review", ScheduleType: domain.ScheduleInterval, IntervalMinutes: 60, Enabled: true, Notify: true, NextRunAt: &future}}
	view = m.View()
	for _, want := range []string{"Hourly review", "Edit schedule", "Run now", "Delete", "Pause"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("Jobs view does not include %q", want)
		}
	}
}

func TestStartupProcessesOverdueJobsAndRefreshesState(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "startup-jobs.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	due := time.Now().Local().Add(-time.Minute)
	job, err := store.CreateScheduledJob(ctx, domain.ScheduledJob{
		Name: "Overdue one-time reminder", ActionType: "custom-reminder", ScheduleType: domain.ScheduleOnce,
		NextRunAt: &due, Enabled: true, Notify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultAppConfig()
	cfg.Notifications.Enabled = true
	m := newModel(ctx, application.New(store), "startup-jobs.sqlite3", Options{NoColor: true, Config: cfg})

	_, cmd := m.Update(snapshotMsg(snapshot{jobs: []domain.ScheduledJob{job}}))
	if cmd == nil {
		t.Fatal("first successful snapshot did not schedule startup catch-up")
	}
	processed, ok := cmd().(schedulerProcessedMsg)
	if !ok || processed.err != nil || processed.count != 1 {
		t.Fatalf("startup catch-up = %#v; want one processed job without error", processed)
	}
	m.Update(processed)
	if len(m.jobs) != 1 || m.jobs[0].CompletedAt == nil || m.jobs[0].Enabled {
		t.Fatalf("startup catch-up did not refresh one-time job state: %+v", m.jobs)
	}
	if len(m.notifications) != 1 || m.notifications[0].Title != job.Name {
		t.Fatalf("startup catch-up did not refresh notification inbox: %+v", m.notifications)
	}
}
