package application

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
)

type Service struct{ store ports.Store }

func New(store ports.Store) *Service              { return &Service{store: store} }
func (s *Service) Close() error                   { return s.store.Close() }
func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func invalidInput(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
}

func (s *Service) CreateRepository(ctx context.Context, name, description string) (domain.Repository, error) {
	if err := invalidInput(validation.Text("repository name", name, true, 100)); err != nil {
		return domain.Repository{}, err
	}
	if err := invalidInput(validation.Text("repository description", description, false, 5000)); err != nil {
		return domain.Repository{}, err
	}
	return s.store.CreateRepository(ctx, strings.TrimSpace(name), strings.TrimSpace(description))
}
func (s *Service) CreateRepositoryWithDefaultBranch(ctx context.Context, name, description, defaultBranch string) (domain.Repository, error) {
	if strings.TrimSpace(defaultBranch) == "" {
		defaultBranch = "main"
	}
	if err := invalidInput(validation.Text("repository name", name, true, 100)); err != nil {
		return domain.Repository{}, err
	}
	if err := invalidInput(validation.Text("repository description", description, false, 5000)); err != nil {
		return domain.Repository{}, err
	}
	if err := invalidInput(validation.Text("default branch name", defaultBranch, true, 200)); err != nil {
		return domain.Repository{}, err
	}
	return s.store.CreateRepositoryWithDefaultBranch(ctx, strings.TrimSpace(name), strings.TrimSpace(description), strings.TrimSpace(defaultBranch))
}
func (s *Service) UpdateRepositoryDescription(ctx context.Context, repository, description string) (domain.Repository, error) {
	if err := invalidInput(validation.Text("repository reference", repository, true, 256)); err != nil {
		return domain.Repository{}, err
	}
	if err := invalidInput(validation.Text("repository description", description, false, 5000)); err != nil {
		return domain.Repository{}, err
	}
	return s.store.UpdateRepositoryDescription(ctx, strings.TrimSpace(repository), strings.TrimSpace(description))
}
func (s *Service) DeleteRepository(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("repository reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteRepository(ctx, strings.TrimSpace(ref))
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
func (s *Service) CreateBranch(ctx context.Context, repo, name, base string) (domain.Branch, error) {
	if err := invalidInput(validation.Text("repository reference", repo, true, 256)); err != nil {
		return domain.Branch{}, err
	}
	if err := invalidInput(validation.Text("branch name", name, true, 200)); err != nil {
		return domain.Branch{}, err
	}
	if err := invalidInput(validation.Text("base branch", base, false, 200)); err != nil {
		return domain.Branch{}, err
	}
	return s.store.CreateBranch(ctx, strings.TrimSpace(repo), strings.TrimSpace(name), strings.TrimSpace(base))
}
func (s *Service) AssignIssueBranch(ctx context.Context, issue, branch string) error {
	if err := invalidInput(validation.Text("issue reference", issue, true, 256)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("branch reference", branch, false, 256)); err != nil {
		return err
	}
	return s.store.AssignIssueBranch(ctx, strings.TrimSpace(issue), strings.TrimSpace(branch))
}
func (s *Service) AssignIssueParent(ctx context.Context, issue, parent string) error {
	if err := invalidInput(validation.Text("issue reference", issue, true, 256)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("parent issue reference", parent, false, 256)); err != nil {
		return err
	}
	return s.store.AssignIssueParent(ctx, strings.TrimSpace(issue), strings.TrimSpace(parent))
}
func (s *Service) SetRepositorySetting(ctx context.Context, repo, key, value string) error {
	if err := invalidInput(validation.Text("repository reference", repo, true, 256)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("setting key", key, true, 128)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("setting value", value, false, 20000)); err != nil {
		return err
	}
	return s.store.SetRepositorySetting(ctx, strings.TrimSpace(repo), strings.TrimSpace(key), value)
}
func (s *Service) GetRepositorySettings(ctx context.Context, repo string) ([]domain.RepositorySetting, error) {
	return s.store.GetRepositorySettings(ctx, repo)
}
func (s *Service) CreateProject(ctx context.Context, name, description string) (domain.Project, error) {
	if err := invalidInput(validation.Text("project name", name, true, 160)); err != nil {
		return domain.Project{}, err
	}
	if err := invalidInput(validation.Text("project description", description, false, 10000)); err != nil {
		return domain.Project{}, err
	}
	return s.store.CreateProject(ctx, strings.TrimSpace(name), strings.TrimSpace(description))
}
func (s *Service) DeleteProject(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("project reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteProject(ctx, strings.TrimSpace(ref))
}
func (s *Service) GetProject(ctx context.Context, ref string) (domain.Project, error) {
	return s.store.GetProject(ctx, ref)
}
func (s *Service) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return s.store.ListProjects(ctx)
}
func (s *Service) ListProjectRepositories(ctx context.Context, project string) ([]domain.ProjectRepository, error) {
	return s.store.ListProjectRepositories(ctx, project)
}
func (s *Service) LinkRepositoryToProject(ctx context.Context, project, repository, relationship string, primary bool) error {
	for _, item := range []struct {
		name, value string
		required    bool
		max         int
	}{{"project reference", project, true, 256}, {"repository reference", repository, true, 256}, {"relationship", relationship, false, 1000}} {
		if err := invalidInput(validation.Text(item.name, item.value, item.required, item.max)); err != nil {
			return err
		}
	}
	return s.store.LinkRepositoryToProject(ctx, strings.TrimSpace(project), strings.TrimSpace(repository), strings.TrimSpace(relationship), primary)
}
func (s *Service) UnlinkRepositoryFromProject(ctx context.Context, project, repository string) error {
	for _, item := range []struct{ name, value string }{{"project reference", project}, {"repository reference", repository}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.UnlinkRepositoryFromProject(ctx, strings.TrimSpace(project), strings.TrimSpace(repository))
}
func (s *Service) SetPrimaryProjectRepository(ctx context.Context, project, repository string) error {
	for _, item := range []struct{ name, value string }{{"project reference", project}, {"repository reference", repository}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.SetPrimaryProjectRepository(ctx, strings.TrimSpace(project), strings.TrimSpace(repository))
}
func (s *Service) AddIssueToProject(ctx context.Context, project, issue string) error {
	for _, item := range []struct{ name, value string }{{"project reference", project}, {"issue reference", issue}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.AddIssueToProject(ctx, strings.TrimSpace(project), strings.TrimSpace(issue))
}
func (s *Service) ListProjectIssues(ctx context.Context, project string) ([]domain.Issue, error) {
	items, err := s.store.ListProjectIssues(ctx, project)
	if err != nil {
		return nil, err
	}
	return s.withReadiness(ctx, items)
}

func (s *Service) CreateIssue(ctx context.Context, repo, title, description, parent, due, priority string) (domain.Issue, error) {
	return s.CreateIssueWithBranch(ctx, repo, title, description, parent, due, priority, "")
}
func (s *Service) CreateIssueWithBranch(ctx context.Context, repo, title, description, parent, due, priority, branchRef string) (domain.Issue, error) {
	return s.CreateIssueWithTypeAndBranch(ctx, repo, title, description, parent, due, priority, branchRef, string(domain.IssueTask))
}
func (s *Service) CreateIssueWithTypeAndBranch(ctx context.Context, repo, title, description, parent, due, priority, branchRef, issueType string) (domain.Issue, error) {
	if err := invalidInput(validation.Text("repository reference", repo, true, 256)); err != nil {
		return domain.Issue{}, err
	}
	if err := invalidInput(validation.Text("issue title", title, true, 200)); err != nil {
		return domain.Issue{}, err
	}
	if err := invalidInput(validation.Text("issue description", description, false, 30000)); err != nil {
		return domain.Issue{}, err
	}
	if err := invalidInput(validation.Text("parent issue reference", parent, false, 256)); err != nil {
		return domain.Issue{}, err
	}
	if err := invalidInput(validation.Text("branch reference", branchRef, false, 256)); err != nil {
		return domain.Issue{}, err
	}
	if err := invalidInput(validation.ISODate("due date", due, false)); err != nil {
		return domain.Issue{}, err
	}
	if strings.TrimSpace(priority) == "" {
		priority = "normal"
	}
	if err := invalidInput(validation.OneOf("priority", priority, "low", "normal", "high", "urgent")); err != nil {
		return domain.Issue{}, err
	}
	if issueType == "" {
		issueType = string(domain.IssueTask)
	}
	if err := invalidInput(validation.OneOf("issue type", issueType, "task", "bug", "feature", "research")); err != nil {
		return domain.Issue{}, err
	}
	r, err := s.store.GetRepository(ctx, strings.TrimSpace(repo))
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
	var duePtr *string
	if strings.TrimSpace(due) != "" {
		cleanDue := strings.TrimSpace(due)
		duePtr = &cleanDue
	}
	var branchID *string
	if strings.TrimSpace(branchRef) != "" {
		branches, e := s.store.ListBranches(ctx, r.ID)
		if e != nil {
			return domain.Issue{}, e
		}
		found := false
		for _, candidate := range branches {
			if candidate.ID == branchRef || strings.EqualFold(candidate.Name, branchRef) {
				id := candidate.ID
				branchID = &id
				found = true
				break
			}
		}
		if !found {
			return domain.Issue{}, fmt.Errorf("%w: branch %q does not belong to repository %q", domain.ErrNotFound, branchRef, r.Name)
		}
	}
	return s.store.CreateIssue(ctx, domain.Issue{RepositoryID: r.ID, Title: title, Description: description, ParentIssueID: parentID, DueDate: duePtr, Priority: priority, Type: domain.IssueType(issueType), BranchID: branchID, Status: domain.IssueOpen})
}
func (s *Service) DeleteIssue(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("issue reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteIssue(ctx, strings.TrimSpace(ref))
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
	if err := invalidInput(validation.Text("issue reference", ref, true, 256)); err != nil {
		return domain.Issue{}, err
	}
	i, err := s.store.GetIssue(ctx, strings.TrimSpace(ref))
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
	if err := invalidInput(validation.Text("issue reference", ref, true, 256)); err != nil {
		return domain.Issue{}, err
	}
	i, err := s.store.GetIssue(ctx, strings.TrimSpace(ref))
	if err != nil {
		return i, err
	}
	return s.store.SetIssueStatus(ctx, i.ID, domain.IssueCancelled)
}
func (s *Service) AddDependency(ctx context.Context, issueRef, prerequisiteRef string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issueRef}, {"prerequisite reference", prerequisiteRef}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	i, err := s.store.GetIssue(ctx, strings.TrimSpace(issueRef))
	if err != nil {
		return err
	}
	p, err := s.store.GetIssue(ctx, strings.TrimSpace(prerequisiteRef))
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
	for _, item := range []struct{ name, value string }{{"issue reference", issueRef}, {"prerequisite reference", prerequisiteRef}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.RemoveDependency(ctx, strings.TrimSpace(issueRef), strings.TrimSpace(prerequisiteRef))
}

func (s *Service) ListDependencies(ctx context.Context, issue string) ([]domain.Issue, error) {
	return s.store.ListIssueDependencies(ctx, issue)
}

func (s *Service) CreatePullRequest(ctx context.Context, issue, title, description string) (domain.PullRequest, error) {
	return s.CreatePullRequestWithBranches(ctx, issue, title, description, "", "", false)
}
func (s *Service) CreatePullRequestWithBranches(ctx context.Context, issue, title, description, sourceBranch, targetBranch string, draft bool) (domain.PullRequest, error) {
	if err := invalidInput(validation.Text("issue reference", issue, true, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("pull request title", title, true, 200)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("pull request description", description, true, 30000)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("source branch", sourceBranch, false, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("target branch", targetBranch, false, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	i, err := s.store.GetIssue(ctx, strings.TrimSpace(issue))
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
	return s.store.CreatePullRequestWithBranches(ctx, i.ID, title, description, sourceBranch, targetBranch, draft)
}
func (s *Service) CreatePullRequestWithIssues(ctx context.Context, issueRefs []string, title, description, sourceBranch, targetBranch string, draft bool) (domain.PullRequest, error) {
	if len(issueRefs) == 0 {
		return domain.PullRequest{}, fmt.Errorf("%w: select at least one issue for the pull request", domain.ErrInvalid)
	}
	if err := invalidInput(validation.Text("pull request title", title, false, 200)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("pull request description", description, true, 30000)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("source branch", sourceBranch, false, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("target branch", targetBranch, false, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	return s.store.CreatePullRequestWithIssues(ctx, issueRefs, title, description, sourceBranch, targetBranch, draft)
}
func (s *Service) ListPullRequestIssues(ctx context.Context, ref string) ([]domain.Issue, error) {
	return s.store.ListPullRequestIssues(ctx, ref)
}
func (s *Service) ListRepositoryFiles(ctx context.Context, repo string) ([]domain.RepositoryFile, error) {
	return s.store.ListRepositoryFiles(ctx, repo)
}
func (s *Service) SaveRepositoryFile(ctx context.Context, repo, path, content string) (domain.RepositoryFile, error) {
	if err := invalidInput(validation.Text("repository reference", repo, true, 256)); err != nil {
		return domain.RepositoryFile{}, err
	}
	if err := invalidInput(validation.Text("file path", path, true, 512)); err != nil {
		return domain.RepositoryFile{}, err
	}
	if err := invalidInput(validation.Text("file content", content, false, 2000000)); err != nil {
		return domain.RepositoryFile{}, err
	}
	return s.store.SaveRepositoryFile(ctx, repo, path, content)
}
func (s *Service) DeleteRepositoryFile(ctx context.Context, repo, path string) error {
	return s.store.DeleteRepositoryFile(ctx, repo, path)
}
func (s *Service) ListRepositoryTags(ctx context.Context, repo string) ([]domain.RepositoryTag, error) {
	return s.store.ListRepositoryTags(ctx, repo)
}
func (s *Service) CreateRepositoryTag(ctx context.Context, repo, name, message string) (domain.RepositoryTag, error) {
	if err := invalidInput(validation.Text("repository reference", repo, true, 256)); err != nil {
		return domain.RepositoryTag{}, err
	}
	if err := invalidInput(validation.Text("tag name", name, true, 100)); err != nil {
		return domain.RepositoryTag{}, err
	}
	if err := invalidInput(validation.Text("tag message", message, false, 10000)); err != nil {
		return domain.RepositoryTag{}, err
	}
	return s.store.CreateRepositoryTag(ctx, repo, name, message)
}
func (s *Service) GetRepositoryTag(ctx context.Context, repo, tag string) (domain.RepositoryTag, error) {
	return s.store.GetRepositoryTag(ctx, repo, tag)
}
func (s *Service) GetRepositoryTagFiles(ctx context.Context, tagID string) ([]domain.RepositoryFile, error) {
	return s.store.GetRepositoryTagFiles(ctx, tagID)
}
func (s *Service) CompareRepositoryTags(ctx context.Context, repo, tag string) ([]domain.TagFileChange, error) {
	return s.store.CompareRepositoryTags(ctx, repo, tag)
}
func (s *Service) ListTagHistory(ctx context.Context, tagID string) ([]domain.CompletionRecord, error) {
	return s.store.ListTagHistory(ctx, tagID)
}
func (s *Service) MarkPullRequestReady(ctx context.Context, ref string) (domain.PullRequest, error) {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	return s.store.MarkPullRequestReady(ctx, strings.TrimSpace(ref))
}
func (s *Service) DeletePullRequest(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeletePullRequest(ctx, strings.TrimSpace(ref))
}
func (s *Service) GetPullRequest(ctx context.Context, ref string) (domain.PullRequest, error) {
	return s.store.GetPullRequest(ctx, ref)
}
func (s *Service) ListPullRequests(ctx context.Context, repo, status string) ([]domain.PullRequest, error) {
	return s.store.ListPullRequests(ctx, repo, status)
}

// UpdatePullRequest edits an open PR. A blank title or description keeps the current value.
func (s *Service) UpdatePullRequest(ctx context.Context, ref, title, description string) (domain.PullRequest, error) {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("pull request title", title, false, 200)); err != nil {
		return domain.PullRequest{}, err
	}
	if err := invalidInput(validation.Text("pull request description", description, false, 30000)); err != nil {
		return domain.PullRequest{}, err
	}
	if strings.TrimSpace(title) == "" && strings.TrimSpace(description) == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: give a new title or description", domain.ErrInvalid)
	}
	p, err := s.store.GetPullRequest(ctx, strings.TrimSpace(ref))
	if err != nil {
		return p, err
	}
	if strings.TrimSpace(title) == "" {
		title = p.Title
	}
	if strings.TrimSpace(description) == "" {
		description = p.Description
	}
	return s.store.UpdatePullRequest(ctx, ref, title, description)
}
func (s *Service) ClosePullRequest(ctx context.Context, ref string) (domain.PullRequest, error) {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	return s.store.ClosePullRequest(ctx, strings.TrimSpace(ref))
}
func (s *Service) AcceptPullRequest(ctx context.Context, ref, summary string) (domain.PullRequest, domain.CompletionRecord, domain.Issue, error) {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return domain.PullRequest{}, domain.CompletionRecord{}, domain.Issue{}, err
	}
	if err := invalidInput(validation.Text("completion summary", summary, false, 30000)); err != nil {
		return domain.PullRequest{}, domain.CompletionRecord{}, domain.Issue{}, err
	}
	p, err := s.store.GetPullRequest(ctx, strings.TrimSpace(ref))
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
	if err := invalidInput(validation.Text("concept name", name, true, 160)); err != nil {
		return domain.Concept{}, err
	}
	if err := invalidInput(validation.Text("concept description", description, false, 20000)); err != nil {
		return domain.Concept{}, err
	}
	return s.store.CreateConcept(ctx, strings.TrimSpace(name), strings.TrimSpace(description))
}
func (s *Service) DeleteConcept(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("concept reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteConcept(ctx, strings.TrimSpace(ref))
}
func (s *Service) GetConcept(ctx context.Context, ref string) (domain.Concept, error) {
	return s.store.GetConcept(ctx, ref)
}
func (s *Service) ListConcepts(ctx context.Context) ([]domain.Concept, error) {
	return s.store.ListConcepts(ctx)
}
func (s *Service) CreateRelation(ctx context.Context, sourceType, sourceID, targetType, targetID, kind string) (domain.Relation, error) {
	for _, item := range []struct{ label, value string }{{"source type", sourceType}, {"source reference", sourceID}, {"target type", targetType}, {"target reference", targetID}, {"relation type", kind}} {
		if err := invalidInput(validation.Text(item.label, item.value, true, 256)); err != nil {
			return domain.Relation{}, err
		}
	}
	entityTypes := []string{"repository", "branch", "project", "issue", "pull_request", "concept", "note", "label", "milestone", "completion"}
	if err := invalidInput(validation.OneOf("source type", sourceType, entityTypes...)); err != nil {
		return domain.Relation{}, err
	}
	if err := invalidInput(validation.OneOf("target type", targetType, entityTypes...)); err != nil {
		return domain.Relation{}, err
	}
	relationKinds := []string{"related_to", "uses", "contains", "part_of", "explains", "contrasts_with", "uses_concept", "references", "supports", "contradicts", "implements", "implemented_by"}
	if err := invalidInput(validation.OneOf("relation type", kind, relationKinds...)); err != nil {
		return domain.Relation{}, err
	}
	return s.store.CreateRelation(ctx, strings.TrimSpace(sourceType), strings.TrimSpace(sourceID), strings.TrimSpace(targetType), strings.TrimSpace(targetID), strings.TrimSpace(kind))
}
func (s *Service) LinkIssueConcept(ctx context.Context, issue, concept string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issue}, {"concept reference", concept}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.LinkIssueConcept(ctx, strings.TrimSpace(issue), strings.TrimSpace(concept))
}
func (s *Service) CreateLabel(ctx context.Context, name, scope, repo, description, color string) (domain.Label, error) {
	if err := invalidInput(validation.Text("label name", name, true, 100)); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.Text("label scope", scope, true, 32)); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.OneOf("label scope", scope, "global", "repository")); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.Text("repository reference", repo, false, 256)); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.Text("label description", description, false, 3000)); err != nil {
		return domain.Label{}, err
	}
	if strings.TrimSpace(color) == "" {
		color = "#7c9cff"
	}
	if err := invalidInput(validation.HexColor("label color", color)); err != nil {
		return domain.Label{}, err
	}
	if strings.TrimSpace(scope) == "repository" && strings.TrimSpace(repo) == "" {
		return domain.Label{}, invalidInput(fmt.Errorf("repository reference is required for repository-scoped labels"))
	}
	if strings.TrimSpace(scope) == "global" && strings.TrimSpace(repo) != "" {
		return domain.Label{}, invalidInput(fmt.Errorf("repository reference must be empty for global labels"))
	}
	return s.store.CreateLabel(ctx, strings.TrimSpace(name), strings.TrimSpace(scope), strings.TrimSpace(repo), strings.TrimSpace(description), strings.TrimSpace(color))
}
func (s *Service) DeleteLabel(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("label reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteLabel(ctx, strings.TrimSpace(ref))
}
func (s *Service) ListLabels(ctx context.Context, scope string) ([]domain.Label, error) {
	return s.store.ListLabels(ctx, scope)
}
func (s *Service) ListIssueLabels(ctx context.Context) (map[string][]domain.Label, error) {
	return s.store.ListIssueLabels(ctx)
}
func (s *Service) AssignLabel(ctx context.Context, issue, label string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issue}, {"label reference", label}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.AssignLabel(ctx, strings.TrimSpace(issue), strings.TrimSpace(label))
}
func (s *Service) CreateMilestone(ctx context.Context, scopeType, scopeRef, title, description, due string) (domain.Milestone, error) {
	if err := invalidInput(validation.Text("milestone scope", scopeType, true, 32)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.OneOf("milestone scope", scopeType, "repository", "project")); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.Text("milestone owner", scopeRef, true, 256)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.Text("milestone title", title, true, 200)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.Text("milestone description", description, false, 10000)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.ISODate("due date", due, false)); err != nil {
		return domain.Milestone{}, err
	}
	var ptr *string
	if strings.TrimSpace(due) != "" {
		cleanDue := strings.TrimSpace(due)
		ptr = &cleanDue
	}
	return s.store.CreateMilestone(ctx, strings.TrimSpace(scopeType), strings.TrimSpace(scopeRef), strings.TrimSpace(title), strings.TrimSpace(description), ptr)
}
func (s *Service) DeleteMilestone(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("milestone reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteMilestone(ctx, strings.TrimSpace(ref))
}
func (s *Service) ListMilestones(ctx context.Context, scopeType, scopeRef string) ([]domain.Milestone, error) {
	return s.store.ListMilestones(ctx, scopeType, scopeRef)
}
func (s *Service) ListIssueMilestones(ctx context.Context) (map[string][]domain.Milestone, error) {
	return s.store.ListIssueMilestones(ctx)
}
func (s *Service) AssignMilestone(ctx context.Context, issue, milestone string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issue}, {"milestone reference", milestone}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.AssignMilestone(ctx, strings.TrimSpace(issue), strings.TrimSpace(milestone))
}
func (s *Service) ListActivity(ctx context.Context, typ, id string, limit int) ([]domain.ActivityEvent, error) {
	return s.store.ListActivity(ctx, typ, id, limit)
}
func (s *Service) ImportLegacy(ctx context.Context, path string) (ports.LegacyImportResult, error) {
	return s.store.ImportLegacy(ctx, path)
}

// GraphLayout returns the workspace's durable graph presentation settings.
func (s *Service) GraphLayout(ctx context.Context) (domain.GraphLayout, error) {
	layout, err := s.store.GetGraphLayout(ctx)
	if err != nil {
		return domain.GraphLayout{}, err
	}
	return layout, nil
}

func finiteGraphNumber(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// SaveGraphLayout strictly validates mode, coordinates and viewport settings before persistence.
func (s *Service) SaveGraphLayout(ctx context.Context, layout domain.GraphLayout) (domain.GraphLayout, error) {
	if layout.Mode != "organic" && layout.Mode != "flowchart" {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph mode must be organic or flowchart"))
	}
	if len(layout.Positions) > 2 {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph layout contains unsupported layout modes"))
	}
	for mode, positions := range layout.Positions {
		if mode != "organic" && mode != "flowchart" {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("unsupported graph layout mode %q", mode))
		}
		if len(positions) > 5000 {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph layout for %s contains more than 5000 nodes", mode))
		}
		for id, point := range positions {
			if strings.TrimSpace(id) == "" || len(id) > 256 {
				return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph node id must contain between 1 and 256 characters"))
			}
			if !finiteGraphNumber(point.X) || !finiteGraphNumber(point.Y) || point.X < -120 || point.X > 1080 || point.Y < -120 || point.Y > 690 {
				return domain.GraphLayout{}, invalidInput(fmt.Errorf("position for graph node %q is outside the supported canvas bounds", id))
			}
		}
	}
	if len(layout.Views) > 2 {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph layout contains unsupported viewport modes"))
	}
	for mode, view := range layout.Views {
		if mode != "organic" && mode != "flowchart" {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("unsupported graph viewport mode %q", mode))
		}
		if !finiteGraphNumber(view.X) || !finiteGraphNumber(view.Y) || !finiteGraphNumber(view.Scale) || view.X < -10000 || view.X > 10000 || view.Y < -10000 || view.Y > 10000 || view.Scale < 0.35 || view.Scale > 3.2 {
			return domain.GraphLayout{}, invalidInput(fmt.Errorf("viewport for %s must use x/y within ±10000 and scale between 0.35 and 3.2", mode))
		}
	}
	if layout.Positions == nil {
		layout.Positions = map[string]map[string]domain.GraphPosition{}
	}
	if layout.Views == nil {
		layout.Views = map[string]domain.GraphViewport{}
	}
	for _, mode := range []string{"organic", "flowchart"} {
		if layout.Positions[mode] == nil {
			layout.Positions[mode] = map[string]domain.GraphPosition{}
		}
		if layout.Views[mode].Scale == 0 {
			layout.Views[mode] = domain.GraphViewport{X: 0, Y: 0, Scale: 1}
		}
	}
	if err := s.store.SaveGraphLayout(ctx, layout); err != nil {
		return domain.GraphLayout{}, err
	}
	return layout, nil
}

// ResetGraphLayout restores the generated arrangement for one mode, without disturbing the
// saved node positions in the other mode. The selected mode remains selected.
func (s *Service) ResetGraphLayout(ctx context.Context, mode string) (domain.GraphLayout, error) {
	if mode == "" {
		mode = "organic"
	}
	if mode != "organic" && mode != "flowchart" {
		return domain.GraphLayout{}, invalidInput(fmt.Errorf("graph mode must be organic or flowchart"))
	}
	layout, err := s.store.GetGraphLayout(ctx)
	if err != nil {
		return domain.GraphLayout{}, err
	}
	layout.Mode = mode
	if layout.Positions == nil {
		layout.Positions = map[string]map[string]domain.GraphPosition{}
	}
	layout.Positions[mode] = map[string]domain.GraphPosition{}
	if layout.Views == nil {
		layout.Views = map[string]domain.GraphViewport{}
	}
	layout.Views[mode] = domain.GraphViewport{X: 0, Y: 0, Scale: 1}
	if err := s.store.SaveGraphLayout(ctx, layout); err != nil {
		return domain.GraphLayout{}, err
	}
	return layout, nil
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
func (s *Service) ListNoteLabels(ctx context.Context) ([]domain.NoteLabel, error) {
	return s.store.ListNoteLabels(ctx)
}
func (s *Service) SetNoteLabelColor(ctx context.Context, name, color string) error {
	if err := invalidInput(validation.Text("note label name", name, true, 100)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("note label color", color, true, 16)); err != nil {
		return err
	}
	if err := invalidInput(validation.HexColor("note label color", color)); err != nil {
		return err
	}
	return s.store.SetNoteLabelColor(ctx, strings.TrimSpace(name), strings.TrimSpace(color))
}
func (s *Service) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	if err := invalidInput(validation.Text("note title", title, true, 200)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note content", content, false, 100000)); err != nil {
		return domain.Note{}, err
	}
	return s.store.CreateNote(ctx, strings.TrimSpace(title), content)
}
func (s *Service) CreateNoteWithLabels(ctx context.Context, title, content string, labels []string) (domain.Note, error) {
	if err := invalidInput(validation.Text("note title", title, true, 200)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note content", content, false, 100000)); err != nil {
		return domain.Note{}, err
	}
	if len(labels) > 50 {
		return domain.Note{}, fmt.Errorf("%w: a note can have at most 50 labels", domain.ErrInvalid)
	}
	for _, label := range labels {
		if err := invalidInput(validation.Text("note label", label, false, 80)); err != nil {
			return domain.Note{}, err
		}
	}
	return s.store.CreateNoteWithLabels(ctx, strings.TrimSpace(title), content, labels)
}
func (s *Service) UpdateNote(ctx context.Context, id, title, content string) (domain.Note, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(title) == "" {
		return domain.Note{}, fmt.Errorf("%w: note ID and title are required", domain.ErrInvalid)
	}
	return s.store.UpdateNote(ctx, id, title, content)
}
func (s *Service) UpdateNoteWithLabels(ctx context.Context, id, title, content string, labels []string) (domain.Note, error) {
	if err := invalidInput(validation.Text("note reference", id, true, 256)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note title", title, true, 200)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note content", content, false, 100000)); err != nil {
		return domain.Note{}, err
	}
	if len(labels) > 50 {
		return domain.Note{}, fmt.Errorf("%w: a note can have at most 50 labels", domain.ErrInvalid)
	}
	for _, label := range labels {
		if err := invalidInput(validation.Text("note label", label, false, 80)); err != nil {
			return domain.Note{}, err
		}
	}
	return s.store.UpdateNoteWithLabels(ctx, strings.TrimSpace(id), strings.TrimSpace(title), content, labels)
}
func (s *Service) DeleteNote(ctx context.Context, id string) error {
	if err := invalidInput(validation.Text("note reference", id, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteNote(ctx, strings.TrimSpace(id))
}
func (s *Service) LinkNote(ctx context.Context, id, entityType, entityRef string, graphVisible bool) error {
	for _, item := range []struct{ name, value string }{{"note reference", id}, {"target entity type", entityType}, {"target entity reference", entityRef}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	if err := invalidInput(validation.OneOf("target entity type", entityType, "issue", "repository", "pull_request", "concept", "project", "label", "milestone", "branch", "completion")); err != nil {
		return err
	}
	return s.store.LinkNote(ctx, strings.TrimSpace(id), strings.TrimSpace(entityType), strings.TrimSpace(entityRef), graphVisible)
}
func (s *Service) ListQuotes(ctx context.Context) ([]domain.Quote, error) {
	return s.store.ListQuotes(ctx)
}
func (s *Service) CreateQuote(ctx context.Context, quote, author string) (domain.Quote, error) {
	if err := invalidInput(validation.Text("quote text", quote, true, 10000)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote author", author, false, 500)); err != nil {
		return domain.Quote{}, err
	}
	return s.store.CreateQuote(ctx, strings.TrimSpace(quote), strings.TrimSpace(author))
}
func (s *Service) CreateQuoteWithSource(ctx context.Context, quote domain.Quote) (domain.Quote, error) {
	if err := invalidInput(validation.Text("quote text", quote.Text, true, 10000)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote author", quote.Author, false, 500)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote source", quote.Source, false, 1000)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.URL("quote source URL", quote.SourceURL)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote category", quote.Category, false, 100)); err != nil {
		return domain.Quote{}, err
	}
	return s.store.CreateQuoteWithSource(ctx, quote)
}
func (s *Service) DeleteQuote(ctx context.Context, id string) error {
	if err := invalidInput(validation.Text("quote reference", id, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteQuote(ctx, strings.TrimSpace(id))
}
func (s *Service) NextQuote(ctx context.Context) (domain.Quote, error) { return s.store.NextQuote(ctx) }
func (s *Service) GetWorkspaceProfile(ctx context.Context) (domain.WorkspaceProfile, error) {
	return s.store.GetWorkspaceProfile(ctx)
}
func (s *Service) UpdateWorkspaceProfile(ctx context.Context, profile domain.WorkspaceProfile) (domain.WorkspaceProfile, error) {
	if err := invalidInput(validation.Text("username", profile.Username, true, 80)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("display name", profile.FullName, true, 160)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("current focus", profile.Focus, false, 200)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("topics", profile.Topics, false, 1000)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.URL("profile website", profile.Website)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("profile bio", profile.Bio, false, 10000)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("workspace README", profile.Readme, false, 50000)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	return s.store.UpdateWorkspaceProfile(ctx, profile)
}
func (s *Service) ListWorkspacePins(ctx context.Context) ([]domain.WorkspacePin, error) {
	return s.store.ListWorkspacePins(ctx)
}
func (s *Service) ToggleWorkspacePin(ctx context.Context, entityType, entityRef, label, color string) (bool, error) {
	if err := invalidInput(validation.Text("pin entity type", entityType, true, 32)); err != nil {
		return false, err
	}
	if err := invalidInput(validation.OneOf("pin entity type", entityType, "issue", "repository", "pull_request", "project", "concept", "note", "milestone", "label")); err != nil {
		return false, err
	}
	if err := invalidInput(validation.Text("pinned entity reference", entityRef, true, 256)); err != nil {
		return false, err
	}
	if err := invalidInput(validation.Text("pin label", label, false, 100)); err != nil {
		return false, err
	}
	if strings.TrimSpace(color) == "" {
		color = "accent"
	}
	if err := invalidInput(validation.OneOf("pin color token", color, "accent", "warning", "success", "danger", "info", "muted", "blue", "violet", "amber", "green", "red", "yellow", "cyan", "neutral")); err != nil {
		return false, err
	}
	return s.store.ToggleWorkspacePin(ctx, strings.TrimSpace(entityType), strings.TrimSpace(entityRef), strings.TrimSpace(label), strings.TrimSpace(color))
}
func (s *Service) ListScheduledJobs(ctx context.Context) ([]domain.ScheduledJob, error) {
	return s.store.ListScheduledJobs(ctx)
}
func validateScheduledJobAction(action string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "custom-reminder"
	}
	switch action {
	case "daily-plan", "weekly-review", "blocked-issues", "open-issues", "custom-reminder":
		return nil
	default:
		return fmt.Errorf("%w: unsupported reminder kind %q", domain.ErrInvalid, action)
	}
}

func (s *Service) CreateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	if err := invalidInput(validation.Text("job name", job.Name, true, 200)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if err := invalidInput(validation.Text("job description", job.Description, false, 10000)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if err := validateScheduledJobAction(job.ActionType); err != nil {
		return domain.ScheduledJob{}, err
	}
	prepared, err := domain.PrepareScheduledJob(job, domain.Now().Local())
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	if prepared.ScheduleType == domain.ScheduleOnce && (prepared.NextRunAt == nil || !prepared.NextRunAt.After(domain.Now())) {
		return domain.ScheduledJob{}, fmt.Errorf("%w: one-time job date and time must be in the future", domain.ErrInvalid)
	}
	return s.store.CreateScheduledJob(ctx, prepared)
}
func (s *Service) UpdateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	if err := invalidInput(validation.Text("job name", job.Name, true, 200)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if err := invalidInput(validation.Text("job description", job.Description, false, 10000)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if strings.TrimSpace(job.ID) == "" {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	if err := validateScheduledJobAction(job.ActionType); err != nil {
		return domain.ScheduledJob{}, err
	}
	prepared, err := domain.PrepareScheduledJob(job, domain.Now().Local())
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	if prepared.ScheduleType == domain.ScheduleOnce && (prepared.NextRunAt == nil || !prepared.NextRunAt.After(domain.Now())) {
		return domain.ScheduledJob{}, fmt.Errorf("%w: one-time job date and time must be in the future", domain.ErrInvalid)
	}
	return s.store.UpdateScheduledJob(ctx, prepared)
}
func (s *Service) DeleteScheduledJob(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	return s.store.DeleteScheduledJob(ctx, id)
}
func (s *Service) ToggleScheduledJob(ctx context.Context, id string) (domain.ScheduledJob, error) {
	if strings.TrimSpace(id) == "" {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	return s.store.ToggleScheduledJob(ctx, id)
}
func (s *Service) RunScheduledJobNow(ctx context.Context, id string, now time.Time, notificationsEnabled bool) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	return s.store.RunScheduledJobNow(ctx, id, now, notificationsEnabled)
}
func (s *Service) ProcessDueJobs(ctx context.Context, now time.Time, notificationsEnabled bool) (int, error) {
	return s.store.ProcessDueJobs(ctx, now, notificationsEnabled)
}
func (s *Service) ListNotifications(ctx context.Context, limit int) ([]domain.Notification, error) {
	return s.store.ListNotifications(ctx, limit)
}
func (s *Service) CreateNotification(ctx context.Context, title, body, level string) (domain.Notification, error) {
	if err := invalidInput(validation.Text("notification title", title, true, 200)); err != nil {
		return domain.Notification{}, err
	}
	if err := invalidInput(validation.Text("notification body", body, false, 10000)); err != nil {
		return domain.Notification{}, err
	}
	if err := invalidInput(validation.Text("notification level", level, true, 32)); err != nil {
		return domain.Notification{}, err
	}
	if err := invalidInput(validation.OneOf("notification level", level, "info", "warning", "error")); err != nil {
		return domain.Notification{}, err
	}
	return s.store.CreateNotification(ctx, strings.TrimSpace(title), body, strings.TrimSpace(level))
}
func (s *Service) DeleteNotification(ctx context.Context, id string) error {
	if err := invalidInput(validation.Text("notification reference", id, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteNotification(ctx, strings.TrimSpace(id))
}
func (s *Service) MarkNotificationsRead(ctx context.Context) error {
	return s.store.MarkNotificationsRead(ctx)
}
func (s *Service) SetAppSetting(ctx context.Context, key, value string) error {
	if err := invalidInput(validation.Text("setting key", key, true, 128)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("setting value", value, false, 20000)); err != nil {
		return err
	}
	return s.store.SetAppSetting(ctx, strings.TrimSpace(key), value)
}
func (s *Service) GetAppSetting(ctx context.Context, key string) (string, error) {
	return s.store.GetAppSetting(ctx, key)
}
