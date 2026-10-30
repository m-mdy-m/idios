package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/validation"
	"strings"
)

type PullRequestService struct {
	store pullRequestServiceStore
}

func (s *PullRequestService) CreatePullRequest(ctx context.Context, issue, title, description string) (domain.PullRequest, error) {
	return s.CreatePullRequestWithBranches(ctx, issue, title, description, "", "", false)
}

func (s *PullRequestService) CreatePullRequestWithBranches(ctx context.Context, issue, title, description, sourceBranch, targetBranch string, draft bool) (domain.PullRequest, error) {
	if err := invalidInput(validation.Text("issue reference", issue, true, 256)); err != nil {
		return domain.PullRequest{}, err
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

func (s *PullRequestService) CreatePullRequestWithIssues(ctx context.Context, issueRefs []string, title, description, sourceBranch, targetBranch string, draft bool) (domain.PullRequest, error) {
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

func (s *PullRequestService) ListPullRequestIssues(ctx context.Context, ref string) ([]domain.Issue, error) {
	return s.store.ListPullRequestIssues(ctx, ref)
}

func (s *PullRequestService) MarkPullRequestReady(ctx context.Context, ref string) (domain.PullRequest, error) {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	return s.store.MarkPullRequestReady(ctx, strings.TrimSpace(ref))
}

func (s *PullRequestService) DeletePullRequest(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeletePullRequest(ctx, strings.TrimSpace(ref))
}

func (s *PullRequestService) GetPullRequest(ctx context.Context, ref string) (domain.PullRequest, error) {
	return s.store.GetPullRequest(ctx, ref)
}

func (s *PullRequestService) ListPullRequests(ctx context.Context, repo, status string) ([]domain.PullRequest, error) {
	return s.store.ListPullRequests(ctx, repo, status)
}

// UpdatePullRequest edits an open PR. A blank title or description keeps the current value.

func (s *PullRequestService) UpdatePullRequest(ctx context.Context, ref, title, description string) (domain.PullRequest, error) {
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

func (s *PullRequestService) ClosePullRequest(ctx context.Context, ref string) (domain.PullRequest, error) {
	if err := invalidInput(validation.Text("pull request reference", ref, true, 256)); err != nil {
		return domain.PullRequest{}, err
	}
	return s.store.ClosePullRequest(ctx, strings.TrimSpace(ref))
}

func (s *PullRequestService) AcceptPullRequest(ctx context.Context, ref, summary string) (domain.PullRequest, domain.CompletionRecord, domain.Issue, error) {
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

func (s *PullRequestService) ListCompletions(ctx context.Context, repo string) ([]domain.CompletionRecord, error) {
	return s.store.ListCompletions(ctx, repo)
}
