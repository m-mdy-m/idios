package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"idios/internal/domain"
	"idios/internal/ports"
)

type Service struct{ store ports.Store }

func New(store ports.Store) *Service              { return &Service{store: store} }
func (s *Service) Close() error                   { return s.store.Close() }
func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) CreateRepository(ctx context.Context, name, description string) (domain.Repository, error) {
	return s.store.CreateRepository(ctx, name, description)
}
func (s *Service) GetRepository(ctx context.Context, ref string) (domain.Repository, error) {
	return s.store.GetRepository(ctx, ref)
}
func (s *Service) ListRepositories(ctx context.Context) ([]domain.Repository, error) {
	return s.store.ListRepositories(ctx)
}
func (s *Service) ListBranches(ctx context.Context, repo string) ([]domain.Branch, error) {
	return s.store.ListBranches(ctx, repo)
}
func (s *Service) SetRepositorySetting(ctx context.Context, repo, key, value string) error {
	return s.store.SetRepositorySetting(ctx, repo, key, value)
}
func (s *Service) GetRepositorySettings(ctx context.Context, repo string) ([]domain.RepositorySetting, error) {
	return s.store.GetRepositorySettings(ctx, repo)
}
func (s *Service) CreateProject(ctx context.Context, name, description string) (domain.Project, error) {
	return s.store.CreateProject(ctx, name, description)
}
func (s *Service) GetProject(ctx context.Context, ref string) (domain.Project, error) {
	return s.store.GetProject(ctx, ref)
}
func (s *Service) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return s.store.ListProjects(ctx)
}
func (s *Service) AddIssueToProject(ctx context.Context, project, issue string) error {
	return s.store.AddIssueToProject(ctx, project, issue)
}
func (s *Service) ListProjectIssues(ctx context.Context, project string) ([]domain.Issue, error) {
	items, err := s.store.ListProjectIssues(ctx, project)
	if err != nil {
		return nil, err
	}
	return s.withReadiness(ctx, items)
}

func (s *Service) CreateIssue(ctx context.Context, repo, title, description, parent, due, priority string) (domain.Issue, error) {
	if strings.TrimSpace(title) == "" {
		return domain.Issue{}, fmt.Errorf("%w: issue title is required", domain.ErrInvalid)
	}
	r, err := s.store.GetRepository(ctx, repo)
	if err != nil {
		return domain.Issue{}, err
	}
	var parentID *string
	if strings.TrimSpace(parent) != "" {
		p, e := s.store.GetIssue(ctx, parent)
		if e != nil {
			return domain.Issue{}, e
		}
		if p.RepositoryID != r.ID {
			return domain.Issue{}, fmt.Errorf("%w: a Sub-issue must have its parent in the same Repository; use depends_on for cross-repository work", domain.ErrInvalid)
		}
		if p.Status != domain.IssueOpen {
			return domain.Issue{}, fmt.Errorf("%w: the parent Issue must remain open while its Sub-issues are defined", domain.ErrInvalidTransition)
		}
		parentID = &p.ID
	}
	if due != "" {
		if _, err := time.Parse("2006-01-02", due); err != nil {
			return domain.Issue{}, fmt.Errorf("%w: due date must use YYYY-MM-DD", domain.ErrInvalid)
		}
	}
	var duePtr *string
	if due != "" {
		duePtr = &due
	}
	return s.store.CreateIssue(ctx, domain.Issue{RepositoryID: r.ID, Title: title, Description: description, ParentIssueID: parentID, DueDate: duePtr, Priority: priority, Status: domain.IssueOpen})
}
func (s *Service) GetIssue(ctx context.Context, ref string) (domain.Issue, error) {
	i, err := s.store.GetIssue(ctx, ref)
	if err != nil {
		return i, err
	}
	bs, err := s.store.ListBlockers(ctx, i.ID)
	if err != nil {
		return i, err
	}
	i.Blockers = bs
	i.Blocked = len(bs) > 0 && i.Status != domain.IssueDone && i.Status != domain.IssueCancelled
	return i, nil
}
func (s *Service) ListIssues(ctx context.Context, repo, status string) ([]domain.Issue, error) {
	items, err := s.store.ListIssues(ctx, repo, status)
	if err != nil {
		return nil, err
	}
	return s.withReadiness(ctx, items)
}
func (s *Service) withReadiness(ctx context.Context, items []domain.Issue) ([]domain.Issue, error) {
	for n := range items {
		bs, err := s.store.ListBlockers(ctx, items[n].ID)
		if err != nil {
			return nil, err
		}
		items[n].Blockers = bs
		items[n].Blocked = len(bs) > 0 && items[n].Status != domain.IssueDone && items[n].Status != domain.IssueCancelled
	}
	return items, nil
}
func (s *Service) StartIssue(ctx context.Context, ref string) (domain.Issue, error) {
	i, err := s.store.GetIssue(ctx, ref)
	if err != nil {
		return i, err
	}
	if i.Status == domain.IssueDone || i.Status == domain.IssueCancelled {
		return i, fmt.Errorf("%w: cannot start an issue in status %s", domain.ErrInvalidTransition, i.Status)
	}
	blockers, err := s.store.ListBlockers(ctx, i.ID)
	if err != nil {
		return i, err
	}
	if len(blockers) > 0 {
		return i, blockedError(i, blockers)
	}
	// Storage repeats the precondition check within a write transaction to keep concurrent
	// CLI/TUI processes from starting an Issue at the same moment a dependency is added.
	return s.store.StartIssue(ctx, i.ID)
}
func (s *Service) CancelIssue(ctx context.Context, ref string) (domain.Issue, error) {
	i, err := s.store.GetIssue(ctx, ref)
	if err != nil {
		return i, err
	}
	return s.store.SetIssueStatus(ctx, i.ID, domain.IssueCancelled)
}
func (s *Service) AddDependency(ctx context.Context, issueRef, prerequisiteRef string) error {
	i, err := s.store.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	p, err := s.store.GetIssue(ctx, prerequisiteRef)
	if err != nil {
		return err
	}
	if i.Status != domain.IssueOpen {
		return fmt.Errorf("%w: dependencies must be defined while the dependent Issue is open", domain.ErrInvalidTransition)
	}
	if i.ID == p.ID {
		return fmt.Errorf("%w: an Issue cannot depend on itself", domain.ErrInvalid)
	}
	edges, err := s.store.ListDependencyEdges(ctx)
	if err != nil {
		return err
	}
	adj := map[string][]string{}
	for _, e := range edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
	}
	seen := map[string]bool{}
	var reaches func(string) bool
	reaches = func(id string) bool {
		if id == i.ID {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id] = true
		for _, next := range adj[id] {
			if reaches(next) {
				return true
			}
		}
		return false
	}
	if reaches(p.ID) {
		return fmt.Errorf("%w: adding %s depends_on %s would create a dependency cycle", domain.ErrInvalid, i.RepositoryName+refSuffix(i), p.RepositoryName+refSuffix(p))
	}
	return s.store.AddDependency(ctx, i.ID, p.ID)
}
func refSuffix(i domain.Issue) string { return fmt.Sprintf("#%d", i.Number) }
func blockedError(i domain.Issue, bs []domain.Blocker) error {
	parts := make([]string, 0, len(bs))
	for _, b := range bs {
		parts = append(parts, fmt.Sprintf("%s#%d %q (%s)", b.RepositoryName, b.Number, b.Title, b.Status))
	}
	return fmt.Errorf("%w: %s#%d is waiting for %s", domain.ErrBlocked, i.RepositoryName, i.Number, strings.Join(parts, "; "))
}
func (s *Service) RemoveDependency(ctx context.Context, issueRef, prerequisiteRef string) error {
	return s.store.RemoveDependency(ctx, issueRef, prerequisiteRef)
}

func (s *Service) ListDependencies(ctx context.Context, issue string) ([]domain.Issue, error) {
	return s.store.ListIssueDependencies(ctx, issue)
}

func (s *Service) CreatePullRequest(ctx context.Context, issue, title, description string) (domain.PullRequest, error) {
	i, err := s.store.GetIssue(ctx, issue)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if i.Status != domain.IssueInProgress {
		return domain.PullRequest{}, fmt.Errorf("%w: start the Issue before creating its PR", domain.ErrInvalidTransition)
	}
	bs, err := s.store.ListBlockers(ctx, i.ID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if len(bs) > 0 {
		return domain.PullRequest{}, blockedError(i, bs)
	}
	if strings.TrimSpace(description) == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: PR description is mandatory; describe the work and its outcome", domain.ErrInvalid)
	}
	return s.store.CreatePullRequest(ctx, i.ID, title, description)
}
func (s *Service) GetPullRequest(ctx context.Context, ref string) (domain.PullRequest, error) {
	return s.store.GetPullRequest(ctx, ref)
}
func (s *Service) ListPullRequests(ctx context.Context, repo, status string) ([]domain.PullRequest, error) {
	return s.store.ListPullRequests(ctx, repo, status)
}
func (s *Service) ClosePullRequest(ctx context.Context, ref string) (domain.PullRequest, error) {
	return s.store.ClosePullRequest(ctx, ref)
}
func (s *Service) AcceptPullRequest(ctx context.Context, ref, summary string) (domain.PullRequest, domain.CompletionRecord, domain.Issue, error) {
	p, err := s.store.GetPullRequest(ctx, ref)
	if err != nil {
		return p, domain.CompletionRecord{}, domain.Issue{}, err
	}
	i, err := s.store.GetIssue(ctx, p.IssueID)
	if err != nil {
		return p, domain.CompletionRecord{}, domain.Issue{}, err
	}
	bs, err := s.store.ListBlockers(ctx, i.ID)
	if err != nil {
		return p, domain.CompletionRecord{}, domain.Issue{}, err
	}
	if len(bs) > 0 {
		return p, domain.CompletionRecord{}, i, blockedError(i, bs)
	}
	return s.store.AcceptPullRequest(ctx, p.ID, summary)
}
func (s *Service) ListCompletions(ctx context.Context, repo string) ([]domain.CompletionRecord, error) {
	return s.store.ListCompletions(ctx, repo)
}

func (s *Service) CreateConcept(ctx context.Context, name, description string) (domain.Concept, error) {
	return s.store.CreateConcept(ctx, name, description)
}
func (s *Service) GetConcept(ctx context.Context, ref string) (domain.Concept, error) {
	return s.store.GetConcept(ctx, ref)
}
func (s *Service) ListConcepts(ctx context.Context) ([]domain.Concept, error) {
	return s.store.ListConcepts(ctx)
}
func (s *Service) CreateRelation(ctx context.Context, sourceType, sourceID, targetType, targetID, kind string) (domain.Relation, error) {
	return s.store.CreateRelation(ctx, sourceType, sourceID, targetType, targetID, kind)
}
func (s *Service) LinkIssueConcept(ctx context.Context, issue, concept string) error {
	return s.store.LinkIssueConcept(ctx, issue, concept)
}
func (s *Service) CreateLabel(ctx context.Context, name, scope, repo, description, color string) (domain.Label, error) {
	return s.store.CreateLabel(ctx, name, scope, repo, description, color)
}
func (s *Service) ListLabels(ctx context.Context, scope string) ([]domain.Label, error) {
	return s.store.ListLabels(ctx, scope)
}
func (s *Service) ListIssueLabels(ctx context.Context) (map[string][]domain.Label, error) {
	return s.store.ListIssueLabels(ctx)
}
func (s *Service) AssignLabel(ctx context.Context, issue, label string) error {
	return s.store.AssignLabel(ctx, issue, label)
}
func (s *Service) CreateMilestone(ctx context.Context, scopeType, scopeRef, title, description, due string) (domain.Milestone, error) {
	var ptr *string
	if due != "" {
		if _, err := time.Parse("2006-01-02", due); err != nil {
			return domain.Milestone{}, fmt.Errorf("%w: due date must use YYYY-MM-DD", domain.ErrInvalid)
		}
		ptr = &due
	}
	return s.store.CreateMilestone(ctx, scopeType, scopeRef, title, description, ptr)
}
func (s *Service) ListMilestones(ctx context.Context, scopeType, scopeRef string) ([]domain.Milestone, error) {
	return s.store.ListMilestones(ctx, scopeType, scopeRef)
}
func (s *Service) ListIssueMilestones(ctx context.Context) (map[string][]domain.Milestone, error) {
	return s.store.ListIssueMilestones(ctx)
}
func (s *Service) AssignMilestone(ctx context.Context, issue, milestone string) error {
	return s.store.AssignMilestone(ctx, issue, milestone)
}
func (s *Service) ListActivity(ctx context.Context, typ, id string, limit int) ([]domain.ActivityEvent, error) {
	return s.store.ListActivity(ctx, typ, id, limit)
}
func (s *Service) ImportLegacy(ctx context.Context, path string) (ports.LegacyImportResult, error) {
	return s.store.ImportLegacy(ctx, path)
}

func (s *Service) Graph(ctx context.Context, from string, depth int) (domain.Graph, error) {
	g, err := s.store.ListGraph(ctx)
	if err != nil {
		return g, err
	}
	if strings.TrimSpace(from) == "" {
		return g, nil
	}
	if depth < 0 {
		depth = 0
	}
	start := -1
	for idx, n := range g.Nodes {
		if strings.EqualFold(n.Reference, from) || strings.EqualFold(n.ID, from) || strings.EqualFold(n.Type+":"+n.Label, from) {
			start = idx
			break
		}
	}
	if start < 0 {
		return domain.Graph{}, fmt.Errorf("%w: graph start node %q (try issue:ARLIZ#12 or concept:CPU)", domain.ErrNotFound, from)
	}
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
		adj[e.Target] = append(adj[e.Target], e.Source)
	}
	dist := map[string]int{g.Nodes[start].ID: 0}
	queue := []string{g.Nodes[start].ID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if dist[cur] >= depth {
			continue
		}
		for _, next := range adj[cur] {
			if _, ok := dist[next]; ok {
				continue
			}
			dist[next] = dist[cur] + 1
			queue = append(queue, next)
		}
	}
	ids := map[string]bool{}
	for id := range dist {
		ids[id] = true
	}
	filtered := domain.Graph{Nodes: []domain.GraphNode{}, Edges: []domain.GraphEdge{}}
	for _, n := range g.Nodes {
		if ids[n.ID] {
			filtered.Nodes = append(filtered.Nodes, n)
		}
	}
	for _, e := range g.Edges {
		if ids[e.Source] && ids[e.Target] {
			filtered.Edges = append(filtered.Edges, e)
		}
	}
	return filtered, nil
}

// Workspace experience APIs keep notes, quotes, reminders, notifications and profile data behind
// the same Store port as the core workflow entities.
func (s *Service) SeedDemo(ctx context.Context) (domain.SeedReport, error) {
	return s.store.SeedDemo(ctx)
}
func (s *Service) ListNotes(ctx context.Context) ([]domain.Note, error) {
	return s.store.ListNotes(ctx)
}
func (s *Service) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	if strings.TrimSpace(title) == "" {
		return domain.Note{}, fmt.Errorf("%w: note title is required", domain.ErrInvalid)
	}
	return s.store.CreateNote(ctx, title, content)
}
func (s *Service) UpdateNote(ctx context.Context, id, title, content string) (domain.Note, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(title) == "" {
		return domain.Note{}, fmt.Errorf("%w: note ID and title are required", domain.ErrInvalid)
	}
	return s.store.UpdateNote(ctx, id, title, content)
}
func (s *Service) DeleteNote(ctx context.Context, id string) error {
	return s.store.DeleteNote(ctx, id)
}
func (s *Service) LinkNote(ctx context.Context, id, entityType, entityRef string, graphVisible bool) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(entityType) == "" || strings.TrimSpace(entityRef) == "" {
		return fmt.Errorf("%w: note and target entity are required", domain.ErrInvalid)
	}
	return s.store.LinkNote(ctx, id, entityType, entityRef, graphVisible)
}
func (s *Service) ListQuotes(ctx context.Context) ([]domain.Quote, error) {
	return s.store.ListQuotes(ctx)
}
func (s *Service) CreateQuote(ctx context.Context, quote, author string) (domain.Quote, error) {
	if strings.TrimSpace(quote) == "" {
		return domain.Quote{}, fmt.Errorf("%w: quote text is required", domain.ErrInvalid)
	}
	return s.store.CreateQuote(ctx, quote, author)
}
func (s *Service) NextQuote(ctx context.Context) (domain.Quote, error) { return s.store.NextQuote(ctx) }
func (s *Service) GetWorkspaceProfile(ctx context.Context) (domain.WorkspaceProfile, error) {
	return s.store.GetWorkspaceProfile(ctx)
}
func (s *Service) UpdateWorkspaceProfile(ctx context.Context, profile domain.WorkspaceProfile) (domain.WorkspaceProfile, error) {
	if strings.TrimSpace(profile.Username) == "" || strings.TrimSpace(profile.FullName) == "" {
		return domain.WorkspaceProfile{}, fmt.Errorf("%w: username and display name are required", domain.ErrInvalid)
	}
	return s.store.UpdateWorkspaceProfile(ctx, profile)
}
func (s *Service) ListWorkspacePins(ctx context.Context) ([]domain.WorkspacePin, error) {
	return s.store.ListWorkspacePins(ctx)
}
func (s *Service) ToggleWorkspacePin(ctx context.Context, entityType, entityRef, label, color string) (bool, error) {
	return s.store.ToggleWorkspacePin(ctx, entityType, entityRef, label, color)
}
func (s *Service) ListScheduledJobs(ctx context.Context) ([]domain.ScheduledJob, error) {
	return s.store.ListScheduledJobs(ctx)
}
func (s *Service) CreateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	if strings.TrimSpace(job.Name) == "" {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job name is required", domain.ErrInvalid)
	}
	if job.IntervalMinutes < 1 {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job interval must be at least one minute", domain.ErrInvalid)
	}
	return s.store.CreateScheduledJob(ctx, job)
}
func (s *Service) ToggleScheduledJob(ctx context.Context, id string) (domain.ScheduledJob, error) {
	return s.store.ToggleScheduledJob(ctx, id)
}
func (s *Service) ProcessDueJobs(ctx context.Context, now time.Time, notificationsEnabled bool) (int, error) {
	return s.store.ProcessDueJobs(ctx, now, notificationsEnabled)
}
func (s *Service) ListNotifications(ctx context.Context, limit int) ([]domain.Notification, error) {
	return s.store.ListNotifications(ctx, limit)
}
func (s *Service) MarkNotificationsRead(ctx context.Context) error {
	return s.store.MarkNotificationsRead(ctx)
}
func (s *Service) SetAppSetting(ctx context.Context, key, value string) error {
	return s.store.SetAppSetting(ctx, key, value)
}
func (s *Service) GetAppSetting(ctx context.Context, key string) (string, error) {
	return s.store.GetAppSetting(ctx, key)
}
