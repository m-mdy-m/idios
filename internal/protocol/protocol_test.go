package protocol

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"idios/internal/application"
	"idios/internal/domain"
	"idios/internal/storage/sqlite"
)

func TestCatalogHasUniqueIDsAndUsableSchemas(t *testing.T) {
	t.Parallel()
	catalog := Catalog()
	if len(catalog) < 30 {
		t.Fatalf("catalog has %d commands; expected a substantial shared action surface", len(catalog))
	}
	seen := make(map[string]bool, len(catalog))
	for _, command := range catalog {
		if command.ID == "" || command.Title == "" || command.Category == "" {
			t.Errorf("command metadata is incomplete: %+v", command)
		}
		if seen[command.ID] {
			t.Errorf("duplicate command ID %q", command.ID)
		}
		seen[command.ID] = true
		for _, field := range command.Fields {
			if field.Name == "" || field.Label == "" || field.Type == "" {
				t.Errorf("command %q has incomplete field metadata: %+v", command.ID, field)
			}
			if field.Type == "select" && len(field.Options) == 0 && field.Source == "" {
				t.Errorf("select field %q in %q has neither static options nor a dynamic source", field.Name, command.ID)
			}
		}
		if _, ok := Find(command.ID); !ok {
			t.Errorf("Find cannot locate listed command %q", command.ID)
		}
	}
}

func TestCatalogReturnsIndependentFieldSlices(t *testing.T) {
	first := Catalog()
	second := Catalog()
	if len(first) == 0 || len(first[0].Fields) == 0 {
		t.Fatal("catalog unexpectedly empty")
	}
	first[0].Fields[0].Label = "corrupted"
	if second[0].Fields[0].Label == "corrupted" {
		t.Fatal("catalog caller can mutate the shared command schema")
	}
}

func TestExecuteValidatesRequestsBeforeRequiringApplication(t *testing.T) {
	t.Parallel()
	if _, err := Execute(context.Background(), nil, "does.not.exist", nil); err == nil || !strings.Contains(err.Error(), "unknown protocol command") {
		t.Fatalf("unknown command error = %v", err)
	}
	if _, err := Execute(context.Background(), nil, "issue.create", map[string]any{}); err == nil || !strings.Contains(err.Error(), "Repository is required") {
		t.Fatalf("missing required field error = %v", err)
	}
	if _, err := Execute(context.Background(), nil, "issue.create", map[string]any{"repo": "X", "title": "Y", "priority": "impossible"}); err == nil || !strings.Contains(err.Error(), "Priority must be one of") {
		t.Fatalf("invalid select error = %v", err)
	}
	if _, err := Execute(context.Background(), nil, "issue.create", map[string]any{"repo": "X", "title": "Y", "priority": "normal", "due": "2026-14-70"}); err == nil || !strings.Contains(err.Error(), "Due date must use YYYY-MM-DD") {
		t.Fatalf("invalid date error = %v", err)
	}
	if _, err := Execute(context.Background(), nil, "issue.create", map[string]any{"repo": "X", "title": "Y", "priority": "normal", "due": "2026-10-22"}); err == nil || !strings.Contains(err.Error(), "Application is unavailable") {
		t.Fatalf("valid request without app error = %v", err)
	}
	if _, err := Execute(context.Background(), nil, "issue.create", map[string]any{"repo": "X", "title": "Y", "unexpected_field": "extra"}); err == nil || !strings.Contains(err.Error(), "unexpected_field") {
		t.Fatalf("unknown input field error = %v, want the field name before app availability is checked", err)
	}
}

func TestBulkLineParserBoundsInput(t *testing.T) {
	got, err := lines("  first\r\n\n second \rthird", 3)
	if err != nil {
		t.Fatalf("lines returned error: %v", err)
	}
	if len(got) != 3 || got[0] != "first" || got[1] != "second" || got[2] != "third" {
		t.Fatalf("unexpected line parsing: %#v", got)
	}
	if _, err := lines("1\n2\n3", 2); err == nil {
		t.Fatal("expected max-items error")
	}
}

func TestCatalogDeclaresDynamicEntityOptions(t *testing.T) {
	catalog := Catalog()
	findSource := func(commandID, fieldName string) string {
		t.Helper()
		command, ok := Find(commandID)
		if !ok {
			t.Fatalf("missing command %q", commandID)
		}
		for _, field := range command.Fields {
			if field.Name == fieldName {
				return field.Source
			}
		}
		t.Fatalf("command %q has no field %q", commandID, fieldName)
		return ""
	}
	if got := findSource("issue.create", "repo"); got != "repositories" {
		t.Fatalf("issue.create repo source = %q", got)
	}
	if got := findSource("relation.create", "source"); got != "source_entities" {
		t.Fatalf("relation.create source source = %q", got)
	}
	if got := findSource("milestone.create", "owner"); got != "owners" {
		t.Fatalf("milestone.create owner source = %q", got)
	}
	if got := findSource("label.create", "repository"); got != "repositories" {
		t.Fatalf("label.create repository source = %q", got)
	}
	if len(catalog) == 0 {
		t.Fatal("protocol catalog unexpectedly empty")
	}
}

func TestOptionsResolveCurrentEngineEntities(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "protocol-options.sqlite3"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app := application.New(store)
	defer app.Close()

	repo, err := app.CreateRepository(ctx, "ProtocolRepo", "protocol options fixture")
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	issue, err := app.CreateIssue(ctx, repo.ID, "Shared issue", "", "", "", "normal")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	project, err := app.CreateProject(ctx, "Shared project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	cases := []struct {
		source     string
		entityType string
		wantValue  string
	}{
		{source: "repositories", wantValue: repo.ID},
		{source: "issues", wantValue: issue.ID},
		{source: "owners", entityType: "repository", wantValue: repo.ID},
		{source: "owners", entityType: "project", wantValue: project.ID},
		{source: "source_entities", entityType: "issue", wantValue: issue.ID},
	}
	for _, tc := range cases {
		t.Run(tc.source+"/"+tc.entityType, func(t *testing.T) {
			items, err := Options(ctx, app, tc.source, tc.entityType)
			if err != nil {
				t.Fatalf("Options returned error: %v", err)
			}
			for _, item := range items {
				if item.Value == tc.wantValue {
					return
				}
			}
			t.Fatalf("options %#v do not contain %q", items, tc.wantValue)
		})
	}
}

func TestExecuteAcceptsDynamicIssueAndBranchSelectors(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "protocol-dynamic-selects.sqlite3"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app := application.New(store)
	defer app.Close()

	repo, err := app.CreateRepositoryWithDefaultBranch(ctx, "SelectorRepo", "dynamic selector fixture", "main")
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	branches, err := app.ListBranches(ctx, repo.ID)
	if err != nil || len(branches) == 0 {
		t.Fatalf("list default branch: branches=%#v err=%v", branches, err)
	}
	defaultBranchID := branches[0].ID
	featureBranch, err := app.CreateBranch(ctx, repo.ID, "feature/protocol-selectors", defaultBranchID)
	if err != nil {
		t.Fatalf("create feature branch: %v", err)
	}

	// The web form submits stable IDs behind the labels, not the display text.
	createdValue, err := Execute(ctx, app, "issue.create", map[string]any{
		"repo": repo.ID, "title": "Issue created from repository", "priority": "normal", "branch": featureBranch.ID,
	})
	if err != nil {
		t.Fatalf("create issue with dynamic branch selector: %v", err)
	}
	created, ok := createdValue.(domain.Issue)
	if !ok {
		t.Fatalf("issue.create returned %T, want domain.Issue", createdValue)
	}
	if created.BranchID == nil || *created.BranchID != featureBranch.ID {
		t.Fatalf("created issue branch = %v, want %q", created.BranchID, featureBranch.ID)
	}

	// Assigning an issue to a branch uses dynamic issue and branch selectors too.
	second, err := app.CreateIssue(ctx, repo.ID, "Issue to assign", "", "", "", "normal")
	if err != nil {
		t.Fatalf("create issue to assign: %v", err)
	}
	if _, err := Execute(ctx, app, "issue.branch.assign", map[string]any{"issue": second.ID, "branch": defaultBranchID}); err != nil {
		t.Fatalf("assign issue to dynamic branch selector: %v", err)
	}

	if _, err := app.StartIssue(ctx, created.ID); err != nil {
		t.Fatalf("start issue before PR: %v", err)
	}
	if _, err := Execute(ctx, app, "pull_request.create", map[string]any{
		"issue": created.ID, "title": "Review dynamic selector handling", "description": "Validated issue and branch references.",
		"source_branch": featureBranch.ID, "target_branch": defaultBranchID,
	}); err != nil {
		t.Fatalf("create pull request with dynamic issue/branch selectors: %v", err)
	}
}

func TestBulkIssueCreateReturnsPerBatchResult(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "protocol-bulk.sqlite3"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app := application.New(store)
	defer app.Close()
	repo, err := app.CreateRepository(ctx, "BatchRepo", "bulk result fixture")
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	result, err := Execute(ctx, app, "issue.bulk_create", map[string]any{
		"repo": repo.ID, "titles": "First batch issue\nSecond batch issue", "priority": "normal",
	})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	report, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("bulk result type = %T, want map[string]any", result)
	}
	if got := report["created"]; got != 2 {
		t.Fatalf("created = %v, want 2", got)
	}
	if got := report["total"]; got != 2 {
		t.Fatalf("total = %v, want 2", got)
	}
	failed, ok := report["failed"].([]map[string]string)
	if !ok || len(failed) != 0 {
		t.Fatalf("failed = %#v, want empty per-item failure list", report["failed"])
	}
}

func TestSharedCatalogCoversCoreTUIAndWebWorkflows(t *testing.T) {
	commands := make(map[string]Command)
	for _, command := range Catalog() {
		commands[command.ID] = command
	}
	for _, id := range []string{
		"repository.create", "repository.delete", "repository.description.update", "branch.create",
		"issue.create", "issue.start", "issue.cancel", "issue.delete", "issue.branch.assign", "issue.parent.assign", "issue.dependency.add", "issue.dependency.remove",
		"issue.concept.link", "issue.label.assign", "issue.milestone.assign",
		"pull_request.create", "pull_request.edit", "pull_request.mark_ready", "pull_request.accept", "pull_request.close", "pull_request.delete",
		"project.create", "project.delete", "project.add_issue", "concept.create", "concept.delete", "relation.create",
		"note.create", "note.update", "note.label.color", "note.link", "note.delete", "quote.create", "quote.delete", "quote.reminder.test",
		"job.create", "job.update", "job.toggle", "job.run", "job.delete", "workspace.profile.update", "workspace.pin.toggle",
		"notification.read_all", "notification.create", "notification.delete", "issue.bulk_create", "quote.bulk_create", "job.bulk_create",
	} {
		if _, ok := commands[id]; !ok {
			t.Errorf("shared catalog is missing workflow command %q", id)
		}
	}

	jobRun, ok := commands["job.run"]
	if !ok {
		t.Fatal("job.run is missing")
	}
	foundNotify := false
	for _, field := range jobRun.Fields {
		if field.Name == "notify" && field.Type == "boolean" {
			foundNotify = true
			break
		}
	}
	if !foundNotify {
		t.Error("job.run must expose its notification preference so all clients can preserve shared configuration")
	}
}
