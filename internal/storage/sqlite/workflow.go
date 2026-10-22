package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"idios/internal/domain"
)

func prSelect() string {
	return `SELECT p.id,p.number,p.repository_id,r.name AS repository_name,p.issue_id,i.number AS issue_number,i.title AS issue_title,p.title,p.description,p.status,p.target_branch_id,b.name AS target_branch_name,p.source_branch_id,source_branch.name AS source_branch_name,p.is_draft,p.created_at,p.updated_at,p.accepted_at
 FROM pull_requests p JOIN repositories r ON r.id=p.repository_id JOIN issues i ON i.id=p.issue_id JOIN branches b ON b.id=p.target_branch_id LEFT JOIN branches source_branch ON source_branch.id=p.source_branch_id`
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
		SourceBranchName: str(r, "source_branch_name"),
		IsDraft:          boolRow(r, "is_draft"),
		CreatedAt:        parseTime(r["created_at"]),
		UpdatedAt:        parseTime(r["updated_at"]),
		AcceptedAt:       ptrTime(r["accepted_at"]),
	}
}

// CreatePullRequest opens the review record for an in-progress Issue that is not blocked.
func (s *Store) CreatePullRequest(ctx context.Context, issueRef, title, description string) (domain.PullRequest, error) {
	return s.CreatePullRequestWithBranches(ctx, issueRef, title, description, "", "", false)
}

// CreatePullRequestWithBranches records a review between two workflow branches. It does not
// execute Git commands or merge filesystem changes; it governs the IDIOS issue lifecycle.
func (s *Store) CreatePullRequestWithBranches(ctx context.Context, issueRef, title, description, sourceRef, targetRef string, draft bool) (domain.PullRequest, error) {
	return s.CreatePullRequestWithIssues(ctx, []string{issueRef}, title, description, sourceRef, targetRef, draft)
}

// CreatePullRequestWithIssues creates one simulated review record covering one or more issues.
// It does not invoke Git or alter files on disk; accepting the PR appends an immutable IDIOS completion.
func (s *Store) CreatePullRequestWithIssues(ctx context.Context, issueRefs []string, title, description, sourceRef, targetRef string, draft bool) (domain.PullRequest, error) {
	if len(issueRefs) == 0 {
		return domain.PullRequest{}, fmt.Errorf("%w: a pull request must include at least one issue", domain.ErrInvalid)
	}
	unique := map[string]bool{}
	issues := make([]domain.Issue, 0, len(issueRefs))
	for _, ref := range issueRefs {
		issue, err := s.GetIssue(ctx, strings.TrimSpace(ref))
		if err != nil {
			return domain.PullRequest{}, err
		}
		if unique[issue.ID] {
			continue
		}
		unique[issue.ID] = true
		if issue.Status != domain.IssueInProgress {
			return domain.PullRequest{}, fmt.Errorf("%w: issue %s#%d must be in progress before creating a PR", domain.ErrInvalidTransition, issue.RepositoryName, issue.Number)
		}
		if err := requireUnblocked(ctx, s.p(), issue); err != nil {
			return domain.PullRequest{}, err
		}
		issues = append(issues, issue)
	}
	i := issues[0]
	for _, issue := range issues[1:] {
		if issue.RepositoryID != i.RepositoryID {
			return domain.PullRequest{}, fmt.Errorf("%w: all issues in one simulated PR must belong to the same repository", domain.ErrInvalid)
		}
	}
	title, description = strings.TrimSpace(title), strings.TrimSpace(description)
	if title == "" {
		if len(issues) == 1 {
			title = fmt.Sprintf("Issue #%d: %s", i.Number, i.Title)
		} else {
			title = fmt.Sprintf("Batch completion for %d issues", len(issues))
		}
	}
	if description == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: Pull Request description is required; explain what was implemented and verified", domain.ErrInvalid)
	}
	repo, err := s.GetRepository(ctx, i.RepositoryID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	branches, err := s.ListBranches(ctx, repo.ID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	var targetID, sourceID string
	for _, branch := range branches {
		if targetRef == "" && branch.IsDefault {
			targetID = branch.ID
		}
		if targetRef != "" && (branch.ID == targetRef || strings.EqualFold(branch.Name, targetRef)) {
			targetID = branch.ID
		}
		if sourceRef != "" && (branch.ID == sourceRef || strings.EqualFold(branch.Name, sourceRef)) {
			sourceID = branch.ID
		}
	}
	if targetRef != "" && targetID == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: target branch %q does not belong to repository %q", domain.ErrNotFound, targetRef, repo.Name)
	}
	if targetID == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: repository has no default branch", domain.ErrInvalid)
	}
	if sourceRef != "" && sourceID == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: source branch %q does not belong to repository %q", domain.ErrNotFound, sourceRef, repo.Name)
	}
	if sourceID != "" && sourceID == targetID {
		return domain.PullRequest{}, fmt.Errorf("%w: source and target branch must differ", domain.ErrInvalid)
	}
	now, id := nowText(), domain.NewID()
	var number int64
	err = s.run(ctx, func(p provider) error {
		for _, issue := range issues {
			fresh, e := issueByID(ctx, p, issue.ID)
			if e != nil {
				return e
			}
			if fresh.Status != domain.IssueInProgress {
				return fmt.Errorf("%w: issue %s#%d is no longer in progress", domain.ErrInvalidTransition, issue.RepositoryName, issue.Number)
			}
			if e = requireUnblocked(ctx, p, fresh); e != nil {
				return e
			}
			existingPR, found, e := p.row(ctx, `SELECT p.id,p.status FROM pull_requests p LEFT JOIN pull_request_issues pri ON pri.pull_request_id=p.id WHERE (p.issue_id=? OR pri.issue_id=?) AND p.status IN ('open','accepted') LIMIT 1`, fresh.ID, fresh.ID)
			if e != nil {
				return e
			}
			if found {
				return fmt.Errorf("%w: issue %s#%d is already included in pull request %s (%s)", domain.ErrInvalidTransition, fresh.RepositoryName, fresh.Number, str(existingPR, "id"), str(existingPR, "status"))
			}
		}
		next, _, e := p.row(ctx, `SELECT COALESCE(MAX(number),0)+1 AS n FROM pull_requests WHERE repository_id=?`, i.RepositoryID)
		if e != nil {
			return e
		}
		number = i64(next["n"])
		var source any
		if sourceID != "" {
			source = sourceID
		}
		if _, e = p.changes(ctx, `INSERT INTO pull_requests(id,number,repository_id,issue_id,title,description,status,target_branch_id,source_branch_id,is_draft,created_at,updated_at,accepted_at) VALUES(?,?,?,?,?,?,'open',?,?,?,?,?,NULL)`, id, number, i.RepositoryID, i.ID, title, description, targetID, source, boolInt(draft), now, now); e != nil {
			return e
		}
		for pos, issue := range issues {
			if _, e = p.changes(ctx, `INSERT INTO pull_request_issues(pull_request_id,issue_id,position,created_at) VALUES(?,?,?,?)`, id, issue.ID, pos, now); e != nil {
				return e
			}
		}
		ids := make([]string, 0, len(issues))
		for _, issue := range issues {
			ids = append(ids, issue.ID)
		}
		return activity(ctx, p, "pull_request", id, "pull_request.created", marshalPayload(map[string]any{"number": number, "issue_ids": ids, "source_branch_id": sourceID, "target_branch_id": targetID, "draft": draft}))
	})
	if err != nil {
		return domain.PullRequest{}, err
	}
	return s.GetPullRequest(ctx, id)
}

func (s *Store) ListPullRequestIssues(ctx context.Context, prRef string) ([]domain.Issue, error) {
	pr, err := s.GetPullRequest(ctx, prRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, issueSelect()+` JOIN pull_request_issues pri ON pri.issue_id=i.id WHERE pri.pull_request_id=? ORDER BY pri.position,i.number`, pr.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, issueFrom), nil
}

// MarkPullRequestReady turns a draft review record into a normal pull request.
func (s *Store) MarkPullRequestReady(ctx context.Context, ref string) (domain.PullRequest, error) {
	p, err := s.GetPullRequest(ctx, ref)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if p.Status != domain.PRPending {
		return domain.PullRequest{}, fmt.Errorf("%w: only an open pull request can be marked ready", domain.ErrInvalidTransition)
	}
	if !p.IsDraft {
		return p, nil
	}
	err = s.run(ctx, func(q provider) error {
		fresh, e := q.must(ctx, "pull request", prSelect()+` WHERE p.id=?`, p.ID)
		if e != nil {
			return e
		}
		current := prFrom(fresh)
		if current.Status != domain.PRPending {
			return fmt.Errorf("%w: only an open pull request can be marked ready", domain.ErrInvalidTransition)
		}
		if !current.IsDraft {
			return nil
		}
		changed, e := q.changes(ctx, `UPDATE pull_requests SET is_draft=0,updated_at=? WHERE id=? AND status='open' AND is_draft=1`, nowText(), p.ID)
		if e != nil {
			return e
		}
		if changed == 0 {
			return fmt.Errorf("%w: pull request changed before it could be marked ready", domain.ErrInvalidTransition)
		}
		return activity(ctx, q, "pull_request", p.ID, "pull_request.ready_for_review", marshalPayload(map[string]any{"number": p.Number, "title": p.Title}))
	})
	if err != nil {
		return domain.PullRequest{}, err
	}
	return s.GetPullRequest(ctx, p.ID)
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
		return s.withPRIssueIDs(ctx, prFrom(row))
	}
	row, err := s.p().must(ctx, fmt.Sprintf("Pull Request %q (use Repository!number or stable ID)", ref), prSelect()+` WHERE p.id=? LIMIT 1`, ref)
	if err != nil {
		return domain.PullRequest{}, err
	}
	return s.withPRIssueIDs(ctx, prFrom(row))
}

func (s *Store) withPRIssueIDs(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
	rows, err := s.p().all(ctx, `SELECT issue_id FROM pull_request_issues WHERE pull_request_id=? ORDER BY position`, pr.ID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	for _, row := range rows {
		pr.IssueIDs = append(pr.IssueIDs, str(row, "issue_id"))
	}
	return pr, nil
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
	items := collect(rows, prFrom)
	for i := range items {
		var e error
		items[i], e = s.withPRIssueIDs(ctx, items[i])
		if e != nil {
			return nil, e
		}
	}
	return items, nil
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
		if freshPR.IsDraft {
			return fmt.Errorf("%w: mark the draft PR ready for review before accepting it", domain.ErrInvalidTransition)
		}
		issue, e := issueByID(ctx, q, freshPR.IssueID)
		if e != nil {
			return fmt.Errorf("Issue: %w", e)
		}
		includedRows, e := q.all(ctx, `SELECT issue_id FROM pull_request_issues WHERE pull_request_id=? ORDER BY position`, freshPR.ID)
		if e != nil {
			return e
		}
		if len(includedRows) == 0 {
			return fmt.Errorf("%w: PR has no linked issues", domain.ErrInvalid)
		}
		included := make([]domain.Issue, 0, len(includedRows))
		for _, linked := range includedRows {
			member, memberErr := issueByID(ctx, q, str(linked, "issue_id"))
			if memberErr != nil {
				return memberErr
			}
			if member.Status != domain.IssueInProgress {
				return fmt.Errorf("%w: issue %s#%d must be in_progress before merge", domain.ErrInvalidTransition, member.RepositoryName, member.Number)
			}
			if memberErr = requireUnblocked(ctx, q, member); memberErr != nil {
				return memberErr
			}
			included = append(included, member)
		}
		// The IDIOS engine appends a simulated commit record; no Git operation or disk merge occurs.
		if _, e = q.changes(ctx, `UPDATE pull_requests SET status='accepted',accepted_at=?,updated_at=? WHERE id=? AND status='open'`, now, now, freshPR.ID); e != nil {
			return e
		}
		sequence, parent, e := nextCompletion(ctx, q, freshPR.TargetBranchID)
		if e != nil {
			return e
		}
		completionID = domain.NewID()
		commitHash := simulatedCommitHash(freshPR.ID, sequence, summary, now)
		if _, e = q.changes(ctx, `INSERT INTO completions(id,repository_id,branch_id,issue_id,pull_request_id,summary,sequence,parent_completion_id,created_at,commit_hash,file_snapshot_ready) VALUES(?,?,?,?,?,?,?,?,?,?,1)`, completionID, issue.RepositoryID, freshPR.TargetBranchID, issue.ID, freshPR.ID, summary, sequence, parent, now, commitHash); e != nil {
			return e
		}
		for _, member := range included {
			if _, e = q.changes(ctx, `UPDATE issues SET status='done',closed_at=?,updated_at=? WHERE id=? AND status='in_progress'`, now, now, member.ID); e != nil {
				return e
			}
			if e = activity(ctx, q, "issue", member.ID, "issue.completed", marshalPayload(map[string]any{"pull_request_id": freshPR.ID, "completion_id": completionID, "commit_hash": commitHash})); e != nil {
				return e
			}
		}
		if _, e = q.changes(ctx, `INSERT INTO completion_files(completion_id,path,content) SELECT ?,path,content FROM repository_files WHERE repository_id=?`, completionID, issue.RepositoryID); e != nil {
			return e
		}
		linkedIDs := make([]string, 0, len(included))
		for _, member := range included {
			linkedIDs = append(linkedIDs, member.ID)
		}
		return activity(ctx, q, "pull_request", freshPR.ID, "pull_request.merged", marshalPayload(map[string]any{"issue_ids": linkedIDs, "completion_id": completionID, "commit_hash": commitHash, "sequence": sequence}))
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

func simulatedCommitHash(prID string, sequence int64, summary, createdAt string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("idios-simulated-commit:%s:%d:%s:%s", prID, sequence, summary, createdAt)))
	return hex.EncodeToString(sum[:])[:12]
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
		CommitHash:         str(r, "commit_hash"),
	}
}

func completionSelect() string {
	return `SELECT c.id,c.repository_id,r.name AS repository_name,c.branch_id,b.name AS branch_name,c.issue_id,i.number AS issue_number,c.pull_request_id,c.summary,c.sequence,c.parent_completion_id,c.created_at,c.commit_hash
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
