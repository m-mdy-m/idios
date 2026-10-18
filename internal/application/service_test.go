package application_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"idios/internal/application"
	"idios/internal/domain"
	"idios/internal/storage/sqlite"
)

func newApp(t *testing.T) (*application.Service, *sqlite.Store) {
	t.Helper()
	st, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "idios.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return application.New(st), st
}
func createRepo(t *testing.T, a *application.Service, c context.Context, name, desc string) domain.Repository {
	t.Helper()
	v, e := a.CreateRepository(c, name, desc)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func createIssue(t *testing.T, a *application.Service, c context.Context, repo, title, desc, parent string) domain.Issue {
	t.Helper()
	v, e := a.CreateIssue(c, repo, title, desc, parent, "", "")
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func start(t *testing.T, a *application.Service, c context.Context, ref string) domain.Issue {
	t.Helper()
	v, e := a.StartIssue(c, ref)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func createPR(t *testing.T, a *application.Service, c context.Context, issue, title, description string) domain.PullRequest {
	t.Helper()
	v, e := a.CreatePullRequest(c, issue, title, description)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func acceptPR(t *testing.T, a *application.Service, c context.Context, ref, summary string) (domain.PullRequest, domain.CompletionRecord, domain.Issue) {
	t.Helper()
	p, co, i, e := a.AcceptPullRequest(c, ref, summary)
	if e != nil {
		t.Fatal(e)
	}
	return p, co, i
}
func getIssue(t *testing.T, a *application.Service, c context.Context, ref string) domain.Issue {
	t.Helper()
	v, e := a.GetIssue(c, ref)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func listCompletions(t *testing.T, a *application.Service, c context.Context, repo string) []domain.CompletionRecord {
	t.Helper()
	v, e := a.ListCompletions(c, repo)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func mustProject(t *testing.T, a *application.Service, c context.Context, name string) domain.Project {
	t.Helper()
	v, e := a.CreateProject(c, name, "")
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func mustLabel(t *testing.T, a *application.Service, c context.Context, name, scope, repo string) domain.Label {
	t.Helper()
	v, e := a.CreateLabel(c, name, scope, repo, "test label", "")
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func mustMilestone(t *testing.T, a *application.Service, c context.Context, scope, owner, title string) domain.Milestone {
	t.Helper()
	v, e := a.CreateMilestone(c, scope, owner, title, "", "")
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func mustGraph(t *testing.T, a *application.Service, c context.Context, from string, depth int) domain.Graph {
	t.Helper()
	v, e := a.Graph(c, from, depth)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestCrossRepositoryDependencyBlocksUntilAcceptedPR(t *testing.T) {
	ctx := context.Background()
	app, st := newApp(t)
	cpu := createRepo(t, app, ctx, "CPU", "learning")
	arliz := createRepo(t, app, ctx, "ARLIZ", "writing")
	createIssue(t, app, ctx, cpu.Name, "Learn basic electricity", "voltage and current", "")
	chapter := createIssue(t, app, ctx, arliz.Name, "Write chapter three", "", "")
	if err := app.AddDependency(ctx, "ARLIZ#1", "CPU#1"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.StartIssue(ctx, "ARLIZ#1"); !errors.Is(err, domain.ErrBlocked) {
		t.Fatalf("expected blocked error, got %v", err)
	}
	if _, err := app.CreatePullRequest(ctx, "CPU#1", "", "finished research"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("PR must follow start, got %v", err)
	}
	start(t, app, ctx, "CPU#1")
	createPR(t, app, ctx, "CPU#1", "", "Reviewed voltage, current and resistance")
	_, completion, doneIssue := acceptPR(t, app, ctx, "CPU!1", "Electricity basics learned")
	if doneIssue.Status != domain.IssueDone {
		t.Fatalf("issue status = %s, want done", doneIssue.Status)
	}
	if completion.BranchName != "main" || completion.Sequence != 1 {
		t.Fatalf("unexpected completion: %+v", completion)
	}
	started := start(t, app, ctx, "ARLIZ#1")
	if started.Status != domain.IssueInProgress {
		t.Fatalf("chapter status = %s", started.Status)
	}
	createPR(t, app, ctx, "ARLIZ#1", "", "Drafted chapter after completing prerequisite")
	_, _, doneChapter := acceptPR(t, app, ctx, "ARLIZ!1", "Chapter three draft ready")
	if doneChapter.Status != domain.IssueDone {
		t.Fatalf("chapter status = %s", doneChapter.Status)
	}
	if _, err := st.SetIssueStatus(ctx, chapter.ID, domain.IssueDone); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("direct done bypass must fail, got %v", err)
	}
	comps := listCompletions(t, app, ctx, "")
	if len(comps) != 2 {
		t.Fatalf("got %d completions, want 2", len(comps))
	}
	graph := mustGraph(t, app, ctx, "issue:ARLIZ#1", 2)
	found := false
	for _, n := range graph.Nodes {
		if n.Reference == "completion:CPU/main/1" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cross-repository prerequisite completion in graph neighborhood")
	}
}

func TestDependencyCyclesAreRejected(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "R", "")
	createIssue(t, app, ctx, "R", "A", "", "")
	createIssue(t, app, ctx, "R", "B", "", "")
	createIssue(t, app, ctx, "R", "C", "", "")
	if err := app.AddDependency(ctx, "R#1", "R#2"); err != nil {
		t.Fatal(err)
	}
	if err := app.AddDependency(ctx, "R#2", "R#3"); err != nil {
		t.Fatal(err)
	}
	if err := app.AddDependency(ctx, "R#3", "R#1"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected cycle rejection, got %v", err)
	}
}

func TestParentIssueCannotStartBeforeAllSubIssuesDone(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	createIssue(t, app, ctx, "CPU", "Implement CPU model", "", "")
	createIssue(t, app, ctx, "CPU", "Study registers", "", "CPU#1")
	if _, err := app.StartIssue(ctx, "CPU#1"); !errors.Is(err, domain.ErrBlocked) {
		t.Fatalf("expected parent to be blocked, got %v", err)
	}
	start(t, app, ctx, "CPU#2")
	createPR(t, app, ctx, "CPU#2", "", "Reviewed register model")
	acceptPR(t, app, ctx, "CPU!1", "Registers covered")
	if got := start(t, app, ctx, "CPU#1"); got.Status != domain.IssueInProgress {
		t.Fatal(got.Status)
	}
}

func TestEachRepositoryStartsWithMainOnly(t *testing.T) {
	ctx := context.Background()
	app, st := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	branches, err := st.ListBranches(ctx, "CPU")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 || branches[0].Name != "main" || !branches[0].IsDefault {
		t.Fatalf("unexpected branches: %+v", branches)
	}
}

func TestScopedLabelsAndProjects(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	p := mustProject(t, app, ctx, "Learning")
	createIssue(t, app, ctx, "CPU", "Read docs", "", "")
	mustLabel(t, app, ctx, "lang", "global", "")
	mustLabel(t, app, ctx, "lang", "repository", "CPU")
	if err := app.AssignLabel(ctx, "CPU#1", "lang"); err != nil {
		t.Fatal(err)
	}
	if err := app.AddIssueToProject(ctx, p.Name, "CPU#1"); err != nil {
		t.Fatal(err)
	}
	milestone := mustMilestone(t, app, ctx, "project", p.Name, "First pass")
	if err := app.AssignMilestone(ctx, "CPU#1", milestone.ID); err != nil {
		t.Fatal(err)
	}
	g := mustGraph(t, app, ctx, "issue:CPU#1", 1)
	edges := map[string]bool{}
	for _, e := range g.Edges {
		edges[e.Type] = true
	}
	if !edges["labelled_with"] || !edges["contains_issue"] || !edges["in_milestone"] {
		t.Fatalf("expected label/project/milestone graph edges, got %#v", edges)
	}
}

func TestIssueReadinessIsDerivedNotStored(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	createIssue(t, app, ctx, "CPU", "Prerequisite", "", "")
	createIssue(t, app, ctx, "CPU", "Main work", "", "")
	if err := app.AddDependency(ctx, "CPU#2", "CPU#1"); err != nil {
		t.Fatal(err)
	}
	i := getIssue(t, app, ctx, "CPU#2")
	if !i.Blocked || len(i.Blockers) != 1 {
		t.Fatalf("expected derived blocked state, got %+v", i)
	}
}

func TestCancelledPrerequisiteCanBeRemovedFromOpenDependent(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	createRepo(t, app, ctx, "ARLIZ", "")
	createIssue(t, app, ctx, "CPU", "Optional old prerequisite", "", "")
	createIssue(t, app, ctx, "ARLIZ", "Write chapter", "", "")
	if err := app.AddDependency(ctx, "ARLIZ#1", "CPU#1"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CancelIssue(ctx, "CPU#1"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.StartIssue(ctx, "ARLIZ#1"); !errors.Is(err, domain.ErrBlocked) {
		t.Fatalf("cancelled prerequisite must not silently count as completed, got %v", err)
	}
	if err := app.RemoveDependency(ctx, "ARLIZ#1", "CPU#1"); err != nil {
		t.Fatal(err)
	}
	if got := start(t, app, ctx, "ARLIZ#1"); got.Status != domain.IssueInProgress {
		t.Fatalf("dependent status = %s, want in_progress", got.Status)
	}
}

func TestCancelledSubIssueNoLongerBlocksParent(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	createIssue(t, app, ctx, "CPU", "Implement model", "", "")
	createIssue(t, app, ctx, "CPU", "Optional sub-issue", "", "CPU#1")
	if _, err := app.CancelIssue(ctx, "CPU#2"); err != nil {
		t.Fatal(err)
	}
	if got := start(t, app, ctx, "CPU#1"); got.Status != domain.IssueInProgress {
		t.Fatalf("parent status = %s, want in_progress", got.Status)
	}
}

func TestSubIssueCannotBeAddedAfterParentStarts(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	createIssue(t, app, ctx, "CPU", "Implement CPU model", "", "")
	start(t, app, ctx, "CPU#1")
	if _, err := app.CreateIssue(ctx, "CPU", "Late sub-issue", "", "CPU#1", "", ""); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("expected parent lifecycle guard, got %v", err)
	}
}

func TestSubIssueIncludesParentReference(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	createIssue(t, app, ctx, "CPU", "Parent", "", "")
	child := createIssue(t, app, ctx, "CPU", "Child", "", "CPU#1")
	if child.ParentReference != "CPU#1" {
		t.Fatalf("ParentReference = %q, want CPU#1", child.ParentReference)
	}
}

func TestGraphDistinguishesGlobalAndRepositoryLabelsWithSameName(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	createIssue(t, app, ctx, "CPU", "Read language docs", "", "")
	createConceptForTest(t, app, ctx, "Programming language")
	mustLabel(t, app, ctx, "lang", "global", "")
	mustLabel(t, app, ctx, "lang", "repository", "CPU")

	g := mustGraph(t, app, ctx, "", 1)
	refs := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Type == "label" {
			refs[n.Reference] = true
		}
	}
	if !refs["label:global/lang"] || !refs["label:repository/CPU/lang"] {
		t.Fatalf("label graph refs must be scope-qualified; got %#v", refs)
	}
	if _, err := app.CreateRelation(ctx, "label", "label:repository/CPU/lang", "concept", "Programming language", "related_to"); err != nil {
		t.Fatalf("qualified label relation should resolve: %v", err)
	}
	localGraph := mustGraph(t, app, ctx, "label:repository/CPU/lang", 1)
	foundConcept := false
	for _, n := range localGraph.Nodes {
		if n.Type == "concept" && n.Label == "Programming language" {
			foundConcept = true
		}
	}
	if !foundConcept {
		t.Fatal("expected qualified local label graph to include the related Concept")
	}
}

func TestGraphDistinguishesSameTitleMilestonesByScope(t *testing.T) {
	ctx := context.Background()
	app, _ := newApp(t)
	createRepo(t, app, ctx, "CPU", "")
	mustProject(t, app, ctx, "Learning")
	mustMilestone(t, app, ctx, "repository", "CPU", "Release")
	mustMilestone(t, app, ctx, "project", "Learning", "Release")
	createConceptForTest(t, app, ctx, "Versioning")

	g := mustGraph(t, app, ctx, "", 1)
	refs := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Type == "milestone" {
			refs[n.Reference] = true
		}
	}
	if !refs["milestone:repository/CPU/Release"] || !refs["milestone:project/Learning/Release"] {
		t.Fatalf("milestone graph refs must be scope-qualified; got %#v", refs)
	}
	if _, err := app.CreateRelation(ctx, "milestone", "milestone:project/Learning/Release", "concept", "Versioning", "related_to"); err != nil {
		t.Fatalf("qualified milestone relation should resolve: %v", err)
	}
}

func createConceptForTest(t *testing.T, app *application.Service, ctx context.Context, name string) domain.Concept {
	t.Helper()
	c, err := app.CreateConcept(ctx, name, "")
	if err != nil {
		t.Fatal(err)
	}
	return c
}
