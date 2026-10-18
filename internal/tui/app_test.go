package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"idios/internal/domain"
)

func TestWorkspaceViewLooksLikePersistentFullScreenUI(t *testing.T) {
	m := newModel(context.Background(), nil, "./idios.sqlite3", Options{NoColor: true})
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
	for _, want := range []string{"IDIOS", "Overview", "Repositories", "Projects", "Issues", "Pull requests", "Graph", "Concepts", "Settings", "CPU", "ARLIZ"} {
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
	for _, want := range []string{"Repositories", "Projects", "Pull requests", "Graph", "Concepts", "Settings"} {
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
	for _, want := range []string{"Relationship map", "depends_on · BLOCKING", "uses_concept", "CPU#4", "ARLIZ#3", "Electricity"} {
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
