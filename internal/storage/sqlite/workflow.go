package sqlite

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"idios/internal/domain"
)

func prSelect() string {
	return `SELECT p.id,p.number,p.repository_id,r.name AS repository_name,p.issue_id,i.number AS issue_number,i.title AS issue_title,p.title,p.description,p.status,p.target_branch_id,b.name AS target_branch_name,p.source_branch_id,p.created_at,p.updated_at,p.accepted_at
 FROM pull_requests p JOIN repositories r ON r.id=p.repository_id JOIN issues i ON i.id=p.issue_id JOIN branches b ON b.id=p.target_branch_id`
}

func prFrom(r Row) domain.PullRequest {
	return domain.PullRequest{
		ID:               str(r, "id"),
		Number:           i64(r["number"]),
		RepositoryID:     str(r, "repository_id"),
		RepositoryName:   str(r, "repository_name"),
		IssueID:          str(r, "issue_id"),
		IssueNumber:      i64(r["issue_number"]),
		IssueTitle:       str(r, "issue_title"),
		Title:            str(r, "title"),
		Description:      str(r, "description"),
		Status:           domain.PRStatus(str(r, "status")),
		TargetBranchID:   str(r, "target_branch_id"),
		TargetBranchName: str(r, "target_branch_name"),
		SourceBranchID:   ptrString(r["source_branch_id"]),
		CreatedAt:        parseTime(r["created_at"]),
		UpdatedAt:        parseTime(r["updated_at"]),
		AcceptedAt:       ptrTime(r["accepted_at"]),
	}
}

// CreatePullRequest opens the review record for an in-progress Issue that is not blocked.
func (s *Store) CreatePullRequest(ctx context.Context, issueRef, title, description string) (domain.PullRequest, error) {
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return domain.PullRequest{}, err
	}
	title, description = strings.TrimSpace(title), strings.TrimSpace(description)
	if title == "" {
		title = fmt.Sprintf("Issue #%d: %s", i.Number, i.Title)
	}
	if description == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: Pull Request description is required; put the work explanation and result here", domain.ErrInvalid)
	}
	if i.Status != domain.IssueInProgress {
		return domain.PullRequest{}, fmt.Errorf("%w: start the issue before creating a PR", domain.ErrInvalidTransition)
	}
	if err := requireUnblocked(ctx, s.p(), i); err != nil {
		return domain.PullRequest{}, err
	}
	now, id := nowText(), domain.NewID()
	var number int64
	err = s.run(ctx, func(p provider) error {
		freshIssue, e := issueByID(ctx, p, i.ID)
		if e != nil {
			return e
		}
		if freshIssue.Status != domain.IssueInProgress {
			return fmt.Errorf("%w: issue must be in_progress before creating a PR", domain.ErrInvalidTransition)
		}
		if e = requireUnblocked(ctx, p, freshIssue); e != nil {
			return e
		}
		repo, e := p.must(ctx, "repository", `SELECT default_branch_id FROM repositories WHERE id=?`, i.RepositoryID)
		if e != nil {
			return e
		}
		next, _, e := p.row(ctx, `SELECT COALESCE(MAX(number),0)+1 AS n FROM pull_requests WHERE repository_id=?`, i.RepositoryID)
		if e != nil {
			return e
		}
		number = i64(next["n"])
		if _, e = p.changes(ctx, `INSERT INTO pull_requests(id,number,repository_id,issue_id,title,description,status,target_branch_id,source_branch_id,created_at,updated_at,accepted_at)
 VALUES(?,?,?,?,?,?,'open',?,NULL,?,?,NULL)`,
			id, number, i.RepositoryID, i.ID, title, description, str(repo, "default_branch_id"), now, now); e != nil {
			return e
		}
		return activity(ctx, p, "pull_request", id, "pull_request.created", marshalPayload(map[string]any{"number": number, "issue_id": i.ID, "target": "main"}))
	})
	if err != nil {
		return domain.PullRequest{}, err
	}
	return s.GetPullRequest(ctx, id)
}

func (s *Store) GetPullRequest(ctx context.Context, ref string) (domain.PullRequest, error) {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "pr:"))
	if at := strings.LastIndex(ref, "!"); at > 0 {
		number, err := strconv.ParseInt(ref[at+1:], 10, 64)
		if err != nil || number < 1 {
			return domain.PullRequest{}, fmt.Errorf("%w: PR reference must look like Repository!1", domain.ErrInvalid)
		}
		row, err := s.p().must(ctx, fmt.Sprintf("Pull Request %q (use Repository!number or stable ID)", ref),
			prSelect()+` WHERE r.name=? COLLATE NOCASE AND p.number=? LIMIT 1`, ref[:at], number)
		if err != nil {
			return domain.PullRequest{}, err
		}
		return prFrom(row), nil
	}
	row, err := s.p().must(ctx, fmt.Sprintf("Pull Request %q (use Repository!number or stable ID)", ref), prSelect()+` WHERE p.id=? LIMIT 1`, ref)
	if err != nil {
		return domain.PullRequest{}, err
	}
	return prFrom(row), nil
}

func (s *Store) ListPullRequests(ctx context.Context, repoRef, status string) ([]domain.PullRequest, error) {
	query := prSelect() + ` WHERE 1=1`
	args := []any{}
	if repoRef != "" {
		repo, err := s.GetRepository(ctx, repoRef)
		if err != nil {
			return nil, err
		}
		query += ` AND p.repository_id=?`
		args = append(args, repo.ID)
	}
	if status != "" {
		query += ` AND p.status=?`
		args = append(args, status)
	}
	query += ` ORDER BY r.name,p.number`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, prFrom), nil
}

// UpdatePullRequest rewrites the title and description of a PR that is still open, so the PR can
// grow with the work. Accepted and closed PRs are records and cannot be edited.
func (s *Store) UpdatePullRequest(ctx context.Context, prRef, title, description string) (domain.PullRequest, error) {
	p, err := s.GetPullRequest(ctx, prRef)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if p.Status != domain.PRPending {
		return domain.PullRequest{}, fmt.Errorf("%w: only open PRs can be edited", domain.ErrInvalidTransition)
	}
	title, description = strings.TrimSpace(title), strings.TrimSpace(description)
	if title == "" || description == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: PR title and description cannot be empty", domain.ErrInvalid)
	}
	changed, err := s.p().changes(ctx, `UPDATE pull_requests SET title=?,description=?,updated_at=? WHERE id=? AND status='open'`, title, description, nowText(), p.ID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if changed == 0 {
		return domain.PullRequest{}, fmt.Errorf("%w: PR state changed concurrently", domain.ErrInvalidTransition)
	}
	_ = s.RecordActivity(ctx, "pull_request", p.ID, "pull_request.updated", "")
	return s.GetPullRequest(ctx, p.ID)
}

func (s *Store) ClosePullRequest(ctx context.Context, prRef string) (domain.PullRequest, error) {
	p, err := s.GetPullRequest(ctx, prRef)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if p.Status != domain.PRPending {
		return domain.PullRequest{}, fmt.Errorf("%w: only open PRs can be closed", domain.ErrInvalidTransition)
	}
	changed, err := s.p().changes(ctx, `UPDATE pull_requests SET status='closed',updated_at=? WHERE id=? AND status='open'`, nowText(), p.ID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if changed == 0 {
		return domain.PullRequest{}, fmt.Errorf("%w: PR state changed concurrently", domain.ErrInvalidTransition)
	}
	_ = s.RecordActivity(ctx, "pull_request", p.ID, "pull_request.closed", "")
	return s.GetPullRequest(ctx, p.ID)
}

// AcceptPullRequest is the only v1 path to `done`. Accepting the PR, appending the
// CompletionRecord, closing the Issue and writing activity events share one transaction.
func (s *Store) AcceptPullRequest(ctx context.Context, prRef, summary string) (domain.PullRequest, domain.CompletionRecord, domain.Issue, error) {
	p, err := s.GetPullRequest(ctx, prRef)
	if err != nil {
		return domain.PullRequest{}, domain.CompletionRecord{}, domain.Issue{}, err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "Accepted PR: " + p.Title
	}
	now := nowText()
	var completionID string
	err = s.run(ctx, func(q provider) error {
		pr, e := q.must(ctx, "PR", prSelect()+` WHERE p.id=?`, p.ID)
		if e != nil {
			return e
		}
		freshPR := prFrom(pr)
		if freshPR.Status != domain.PRPending {
			return fmt.Errorf("%w: PR is not open", domain.ErrInvalidTransition)
		}
		issue, e := issueByID(ctx, q, freshPR.IssueID)
		if e != nil {
			return fmt.Errorf("Issue: %w", e)
		}
		if issue.Status != domain.IssueInProgress {
			return fmt.Errorf("%w: issue must be in_progress before its PR can be accepted", domain.ErrInvalidTransition)
		}
		if e := requireUnblocked(ctx, q, issue); e != nil {
			return e
		}
		// V1 records a workflow completion, not a Git commit or file merge.
		if _, e = q.changes(ctx, `UPDATE pull_requests SET status='accepted',accepted_at=?,updated_at=? WHERE id=? AND status='open'`, now, now, freshPR.ID); e != nil {
			return e
		}
		sequence, parent, e := nextCompletion(ctx, q, freshPR.TargetBranchID)
		if e != nil {
			return e
		}
		completionID = domain.NewID()
		if _, e = q.changes(ctx, `INSERT INTO completions(id,repository_id,branch_id,issue_id,pull_request_id,summary,sequence,parent_completion_id,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
			completionID, issue.RepositoryID, freshPR.TargetBranchID, issue.ID, freshPR.ID, summary, sequence, parent, now); e != nil {
			return e
		}
		if _, e = q.changes(ctx, `UPDATE issues SET status='done',closed_at=?,updated_at=? WHERE id=? AND status='in_progress'`, now, now, issue.ID); e != nil {
			return e
		}
		if e = activity(ctx, q, "pull_request", freshPR.ID, "pull_request.accepted", marshalPayload(map[string]any{"issue_id": issue.ID, "completion_id": completionID})); e != nil {
			return e
		}
		return activity(ctx, q, "issue", issue.ID, "issue.completed", marshalPayload(map[string]any{"pull_request_id": freshPR.ID, "completion_id": completionID}))
	})
	if err != nil {
		return domain.PullRequest{}, domain.CompletionRecord{}, domain.Issue{}, err
	}
	updatedPR, err := s.GetPullRequest(ctx, p.ID)
	if err != nil {
		return domain.PullRequest{}, domain.CompletionRecord{}, domain.Issue{}, err
	}
	issue, err := s.GetIssue(ctx, p.IssueID)
	if err != nil {
		return domain.PullRequest{}, domain.CompletionRecord{}, domain.Issue{}, err
	}
	completion, err := s.completionByID(ctx, completionID)
	return updatedPR, completion, issue, err
}

// nextCompletion appends to the branch-local sequence and links the previous record.
func nextCompletion(ctx context.Context, p provider, branchID string) (int64, any, error) {
	latest, ok, err := p.row(ctx, `SELECT id,sequence FROM completions WHERE branch_id=? ORDER BY sequence DESC LIMIT 1`, branchID)
	if err != nil {
		return 0, nil, err
	}
	if !ok {
		return 1, nil, nil
	}
	return i64(latest["sequence"]) + 1, str(latest, "id"), nil
}

func completionFrom(r Row) domain.CompletionRecord {
	return domain.CompletionRecord{
		ID:                 str(r, "id"),
		RepositoryID:       str(r, "repository_id"),
		RepositoryName:     str(r, "repository_name"),
		BranchID:           str(r, "branch_id"),
		BranchName:         str(r, "branch_name"),
		IssueID:            str(r, "issue_id"),
		IssueNumber:        i64(r["issue_number"]),
		PullRequestID:      str(r, "pull_request_id"),
		Summary:            str(r, "summary"),
		Sequence:           i64(r["sequence"]),
		ParentCompletionID: ptrString(r["parent_completion_id"]),
		CreatedAt:          parseTime(r["created_at"]),
	}
}

func completionSelect() string {
	return `SELECT c.id,c.repository_id,r.name AS repository_name,c.branch_id,b.name AS branch_name,c.issue_id,i.number AS issue_number,c.pull_request_id,c.summary,c.sequence,c.parent_completion_id,c.created_at
 FROM completions c JOIN branches b ON b.id=c.branch_id JOIN repositories r ON r.id=c.repository_id JOIN issues i ON i.id=c.issue_id`
}

func (s *Store) completionByID(ctx context.Context, id string) (domain.CompletionRecord, error) {
	row, err := s.p().must(ctx, "completion", completionSelect()+` WHERE c.id=?`, id)
	if err != nil {
		return domain.CompletionRecord{}, err
	}
	return completionFrom(row), nil
}

func (s *Store) ListCompletions(ctx context.Context, repoRef string) ([]domain.CompletionRecord, error) {
	query := completionSelect() + ` WHERE 1=1`
	args := []any{}
	if repoRef != "" {
		repo, err := s.GetRepository(ctx, repoRef)
		if err != nil {
			return nil, err
		}
		query += ` AND c.repository_id=?`
		args = append(args, repo.ID)
	}
	query += ` ORDER BY r.name,c.sequence DESC`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, completionFrom), nil
}
