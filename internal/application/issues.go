package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/validation"
	"strings"
)

type IssueService struct {
	store issueServiceStore
}

func (s *IssueService) CreateIssue(ctx context.Context, repo, title, description, parent, due, priority string) (domain.Issue, error) {
	return s.CreateIssueWithBranch(ctx, repo, title, description, parent, due, priority, "")
}

func (s *IssueService) CreateIssueWithBranch(ctx context.Context, repo, title, description, parent, due, priority, branchRef string) (domain.Issue, error) {
	return s.CreateIssueWithTypeAndBranch(ctx, repo, title, description, parent, due, priority, branchRef, string(domain.IssueTask))
}

func (s *IssueService) CreateIssueWithTypeAndBranch(ctx context.Context, repo, title, description, parent, due, priority, branchRef, issueType string) (domain.Issue, error) {
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

func (s *IssueService) DeleteIssue(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("issue reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteIssue(ctx, strings.TrimSpace(ref))
}

func (s *IssueService) GetIssue(ctx context.Context, ref string) (domain.Issue, error) {
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

func (s *IssueService) ListIssues(ctx context.Context, repo, status string) ([]domain.Issue, error) {
	items, err := s.store.ListIssues(ctx, repo, status)
	if err != nil {
		return nil, err
	}
	return s.withReadiness(ctx, items)
}

func (s *IssueService) withReadiness(ctx context.Context, items []domain.Issue) ([]domain.Issue, error) {
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

func (s *IssueService) StartIssue(ctx context.Context, ref string) (domain.Issue, error) {
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
	// independent callers from starting an Issue at the same moment a dependency is added.
	return s.store.StartIssue(ctx, i.ID)
}

func (s *IssueService) CancelIssue(ctx context.Context, ref string) (domain.Issue, error) {
	if err := invalidInput(validation.Text("issue reference", ref, true, 256)); err != nil {
		return domain.Issue{}, err
	}
	i, err := s.store.GetIssue(ctx, strings.TrimSpace(ref))
	if err != nil {
		return i, err
	}
	return s.store.SetIssueStatus(ctx, i.ID, domain.IssueCancelled)
}

func (s *IssueService) AddDependency(ctx context.Context, issueRef, prerequisiteRef string) error {
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

func (s *IssueService) RemoveDependency(ctx context.Context, issueRef, prerequisiteRef string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issueRef}, {"prerequisite reference", prerequisiteRef}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.RemoveDependency(ctx, strings.TrimSpace(issueRef), strings.TrimSpace(prerequisiteRef))
}

func (s *IssueService) ListDependencies(ctx context.Context, issue string) ([]domain.Issue, error) {
	return s.store.ListIssueDependencies(ctx, issue)
}
