package sqlite

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"idios/internal/domain"
)

func issueSelect() string {
	return `SELECT i.id,i.number,i.repository_id,r.name AS repository_name,i.title,i.description,i.status,i.issue_type AS issue_type,i.priority,i.due_date,i.parent_issue_id,parent_repo.name AS parent_repository_name,parent_issue.number AS parent_issue_number,i.branch_id,branch.name AS branch_name,i.created_at,i.updated_at,i.closed_at
 FROM issues i JOIN repositories r ON r.id=i.repository_id
 LEFT JOIN branches branch ON branch.id=i.branch_id
 LEFT JOIN issues parent_issue ON parent_issue.id=i.parent_issue_id
 LEFT JOIN repositories parent_repo ON parent_repo.id=parent_issue.repository_id`
}

func issueFrom(r Row) domain.Issue {
	parentID := ptrString(r["parent_issue_id"])
	parentRef := ""
	if parentID != nil && i64(r["parent_issue_number"]) > 0 {
		parentRef = fmt.Sprintf("%s#%d", str(r, "parent_repository_name"), i64(r["parent_issue_number"]))
	}
	return domain.Issue{
		ID:              str(r, "id"),
		Number:          i64(r["number"]),
		RepositoryID:    str(r, "repository_id"),
		RepositoryName:  str(r, "repository_name"),
		Title:           str(r, "title"),
		Description:     str(r, "description"),
		Status:          domain.IssueStatus(str(r, "status")),
		Type:            domain.IssueType(str(r, "issue_type")),
		Priority:        str(r, "priority"),
		DueDate:         ptrString(r["due_date"]),
		ParentIssueID:   parentID,
		ParentReference: parentRef,
		BranchID:        ptrString(r["branch_id"]),
		BranchName:      str(r, "branch_name"),
		CreatedAt:       parseTime(r["created_at"]),
		UpdatedAt:       parseTime(r["updated_at"]),
		ClosedAt:        ptrTime(r["closed_at"]),
	}
}

func (s *Store) CreateIssue(ctx context.Context, in domain.Issue) (domain.Issue, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return domain.Issue{}, fmt.Errorf("%w: issue title is required", domain.ErrInvalid)
	}
	repo, err := s.GetRepository(ctx, in.RepositoryID)
	if err != nil {
		return domain.Issue{}, err
	}
	in.RepositoryID = repo.ID
	if in.BranchID != nil && strings.TrimSpace(*in.BranchID) != "" {
		branch, e := s.p().must(ctx, "branch", `SELECT id FROM branches WHERE id=? AND repository_id=?`, *in.BranchID, repo.ID)
		if e != nil {
			return domain.Issue{}, fmt.Errorf("branch must belong to repository %q: %w", repo.Name, e)
		}
		branchID := str(branch, "id")
		in.BranchID = &branchID
	} else {
		in.BranchID = nil
	}
	if in.ParentIssueID != nil && *in.ParentIssueID != "" {
		parent, err := s.GetIssue(ctx, *in.ParentIssueID)
		if err != nil {
			return domain.Issue{}, fmt.Errorf("parent issue: %w", err)
		}
		if parent.RepositoryID != repo.ID {
			return domain.Issue{}, fmt.Errorf("%w: parent and child issues must belong to the same repository", domain.ErrInvalid)
		}
		if parent.Status != domain.IssueOpen {
			return domain.Issue{}, fmt.Errorf("%w: the parent Issue must be open while defining its Sub-issues", domain.ErrInvalidTransition)
		}
		in.ParentIssueID = &parent.ID
	}
	if in.Type == "" {
		in.Type = domain.IssueTask
	}
	if in.Type != domain.IssueTask && in.Type != domain.IssueBug && in.Type != domain.IssueFeature && in.Type != domain.IssueResearch {
		return domain.Issue{}, fmt.Errorf("%w: issue type must be task, bug, feature, or research", domain.ErrInvalid)
	}
	status := in.Status
	if status == "" {
		status = domain.IssueOpen
	}
	if status != domain.IssueOpen && status != domain.IssueInProgress && status != domain.IssueCancelled {
		return domain.Issue{}, fmt.Errorf("%w: new issue status must be open, in_progress, or cancelled", domain.ErrInvalid)
	}
	id := in.ID
	if id == "" {
		id = domain.NewID()
	}
	now := nowText()
	var number int64
	err = s.run(ctx, func(p provider) error {
		// Recheck parent/owner within the write transaction to prevent racing a parent completion.
		if in.ParentIssueID != nil {
			if *in.ParentIssueID == id {
				return fmt.Errorf("%w: issue cannot be its own parent", domain.ErrInvalid)
			}
			parent, e := p.must(ctx, "parent issue", `SELECT repository_id,status FROM issues WHERE id=?`, *in.ParentIssueID)
			if e != nil {
				return e
			}
			if str(parent, "repository_id") != repo.ID {
				return fmt.Errorf("%w: parent and child issues must belong to the same repository", domain.ErrInvalid)
			}
			if str(parent, "status") != string(domain.IssueOpen) {
				return fmt.Errorf("%w: the parent Issue must be open while defining its Sub-issues", domain.ErrInvalidTransition)
			}
		}
		next, _, e := p.row(ctx, `SELECT COALESCE(MAX(number),0)+1 AS next_number FROM issues WHERE repository_id=?`, repo.ID)
		if e != nil {
			return e
		}
		number = i64(next["next_number"])
		if _, e := p.changes(ctx, `INSERT INTO issues(id,number,repository_id,title,description,status,issue_type,priority,due_date,parent_issue_id,branch_id,created_at,updated_at,closed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,NULL)`,
			id, number, repo.ID, in.Title, strings.TrimSpace(in.Description), string(status), string(in.Type), in.Priority, in.DueDate, in.ParentIssueID, in.BranchID, now, now); e != nil {
			return e
		}
		return activity(ctx, p, "issue", id, "issue.created", marshalPayload(map[string]any{"repository": repo.Name, "number": number, "title": in.Title}))
	})
	if err != nil {
		return domain.Issue{}, err
	}
	return s.GetIssue(ctx, id)
}

func (s *Store) GetIssue(ctx context.Context, ref string) (domain.Issue, error) {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "issue:"))
	if at := strings.LastIndex(ref, "#"); at > 0 {
		number, err := strconv.ParseInt(ref[at+1:], 10, 64)
		if err != nil || number < 1 {
			return domain.Issue{}, fmt.Errorf("%w: issue reference must look like Repository#12", domain.ErrInvalid)
		}
		row, err := s.p().must(ctx, fmt.Sprintf("issue %q (use Repository#number or stable ID)", ref),
			issueSelect()+` WHERE r.name=? COLLATE NOCASE AND i.number=? LIMIT 1`, ref[:at], number)
		if err != nil {
			return domain.Issue{}, err
		}
		return issueFrom(row), nil
	}
	row, err := s.p().must(ctx, fmt.Sprintf("issue %q (use Repository#number or stable ID)", ref),
		issueSelect()+` WHERE i.id=? LIMIT 1`, ref)
	if err != nil {
		return domain.Issue{}, err
	}
	return issueFrom(row), nil
}

func (s *Store) ListIssues(ctx context.Context, repoRef, status string) ([]domain.Issue, error) {
	query := issueSelect() + ` WHERE 1=1`
	args := []any{}
	if repoRef != "" {
		r, err := s.GetRepository(ctx, repoRef)
		if err != nil {
			return nil, err
		}
		query += ` AND i.repository_id=?`
		args = append(args, r.ID)
	}
	if status != "" {
		query += ` AND i.status=?`
		args = append(args, status)
	}
	query += ` ORDER BY r.name COLLATE NOCASE,i.number`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, issueFrom), nil
}

// AssignIssueBranch associates an issue with an optional branch in its repository.
// An empty branch reference clears the association.
func (s *Store) AssignIssueBranch(ctx context.Context, issueRef, branchRef string) error {
	issue, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	branchRef = strings.TrimSpace(branchRef)
	var branchID any
	branchName := ""
	if branchRef != "" {
		branch, e := s.p().must(ctx, "branch", `SELECT id,name FROM branches WHERE repository_id=? AND (id=? OR name=? COLLATE NOCASE) LIMIT 1`, issue.RepositoryID, branchRef, branchRef)
		if e != nil {
			return fmt.Errorf("branch must belong to the issue's repository: %w", e)
		}
		branchID = str(branch, "id")
		branchName = str(branch, "name")
	}
	if _, err := s.p().changes(ctx, `UPDATE issues SET branch_id=?,updated_at=? WHERE id=?`, branchID, nowText(), issue.ID); err != nil {
		return err
	}
	return s.RecordActivity(ctx, "issue", issue.ID, "issue.branch_changed", marshalPayload(map[string]string{"branch": branchName}))
}

// AssignIssueParent changes the parent/sub-issue relationship after creation. Parent links stay
// inside one repository, and the recursive check prevents cycles.
func (s *Store) AssignIssueParent(ctx context.Context, issueRef, parentRef string) error {
	issue, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	var parentID any
	parentPayload := ""
	if strings.TrimSpace(parentRef) != "" {
		parent, e := s.GetIssue(ctx, parentRef)
		if e != nil {
			return fmt.Errorf("parent issue: %w", e)
		}
		if parent.ID == issue.ID {
			return fmt.Errorf("%w: an issue cannot be its own parent", domain.ErrInvalid)
		}
		if parent.RepositoryID != issue.RepositoryID {
			return fmt.Errorf("%w: parent and sub-issue must belong to the same repository", domain.ErrInvalid)
		}
		if parent.Status != domain.IssueOpen {
			return fmt.Errorf("%w: parent issue must remain open while it has sub-issues", domain.ErrInvalidTransition)
		}
		parentID = parent.ID
		parentPayload = fmt.Sprintf("%s#%d", parent.RepositoryName, parent.Number)
	}
	return s.run(ctx, func(p provider) error {
		fresh, e := p.must(ctx, "issue", `SELECT id,repository_id FROM issues WHERE id=?`, issue.ID)
		if e != nil {
			return e
		}
		if str(fresh, "repository_id") != issue.RepositoryID {
			return fmt.Errorf("%w: issue repository changed", domain.ErrInvalidTransition)
		}
		if parentID != nil {
			parent, e := p.must(ctx, "parent issue", `SELECT id,repository_id,status FROM issues WHERE id=?`, parentID)
			if e != nil {
				return e
			}
			if str(parent, "repository_id") != issue.RepositoryID {
				return fmt.Errorf("%w: parent and sub-issue must belong to the same repository", domain.ErrInvalid)
			}
			if str(parent, "status") != string(domain.IssueOpen) {
				return fmt.Errorf("%w: parent issue must remain open while it has sub-issues", domain.ErrInvalidTransition)
			}
			cycle, _, e := p.row(ctx, `WITH RECURSIVE descendants(id) AS (SELECT id FROM issues WHERE parent_issue_id=? UNION ALL SELECT child.id FROM issues child JOIN descendants d ON child.parent_issue_id=d.id) SELECT EXISTS(SELECT 1 FROM descendants WHERE id=?) AS found`, issue.ID, parentID)
			if e != nil {
				return e
			}
			if boolRow(cycle, "found") {
				return fmt.Errorf("%w: parent relationship would create a cycle", domain.ErrInvalid)
			}
		}
		if _, e := p.changes(ctx, `UPDATE issues SET parent_issue_id=?,updated_at=? WHERE id=?`, parentID, nowText(), issue.ID); e != nil {
			return e
		}
		return activity(ctx, p, "issue", issue.ID, "issue.parent.assigned", marshalPayload(map[string]any{"issue": fmt.Sprintf("%s#%d", issue.RepositoryName, issue.Number), "parent": parentPayload}))
	})
}

func (s *Store) SetIssueStatus(ctx context.Context, issueRef string, status domain.IssueStatus) (domain.Issue, error) {
	if status == domain.IssueInProgress {
		return domain.Issue{}, fmt.Errorf("%w: use StartIssue so prerequisite checks are atomic", domain.ErrInvalidTransition)
	}
	if status == domain.IssueDone {
		return domain.Issue{}, fmt.Errorf("%w: an Issue becomes done only through an accepted Pull Request", domain.ErrInvalidTransition)
	}
	if status != domain.IssueOpen && status != domain.IssueCancelled {
		return domain.Issue{}, fmt.Errorf("%w: unknown Issue status %q", domain.ErrInvalid, status)
	}
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return domain.Issue{}, err
	}
	if i.Status == domain.IssueDone {
		return domain.Issue{}, fmt.Errorf("%w: completed issues are immutable in v1", domain.ErrInvalidTransition)
	}
	if status == domain.IssueOpen && i.Status == domain.IssueCancelled {
		return domain.Issue{}, fmt.Errorf("%w: cancelled issues cannot be reopened in v1", domain.ErrInvalidTransition)
	}
	var changed int64
	now := nowText()
	if status == domain.IssueCancelled {
		changed, err = s.p().changes(ctx, `UPDATE issues SET status='cancelled',updated_at=?,closed_at=? WHERE id=? AND status IN ('open','in_progress')`, now, now, i.ID)
	} else {
		changed, err = s.p().changes(ctx, `UPDATE issues SET status='open',updated_at=?,closed_at=NULL WHERE id=? AND status='in_progress'`, now, i.ID)
	}
	if err != nil {
		return domain.Issue{}, err
	}
	if changed == 0 {
		return domain.Issue{}, fmt.Errorf("%w: issue state changed concurrently or cannot transition from %s", domain.ErrInvalidTransition, i.Status)
	}
	_ = s.RecordActivity(ctx, "issue", i.ID, "issue.status_changed", marshalPayload(map[string]string{"from": string(i.Status), "to": string(status)}))
	return s.GetIssue(ctx, i.ID)
}

func (s *Store) StartIssue(ctx context.Context, issueRef string) (domain.Issue, error) {
	initial, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return domain.Issue{}, err
	}
	err = s.run(ctx, func(p provider) error {
		fresh, e := issueByID(ctx, p, initial.ID)
		if e != nil {
			return e
		}
		if fresh.Status == domain.IssueDone || fresh.Status == domain.IssueCancelled {
			return fmt.Errorf("%w: cannot start an issue in status %s", domain.ErrInvalidTransition, fresh.Status)
		}
		if e := requireUnblocked(ctx, p, fresh); e != nil {
			return e
		}
		if _, e := p.changes(ctx, `UPDATE issues SET status='in_progress',updated_at=? WHERE id=? AND status IN ('open','in_progress')`, nowText(), fresh.ID); e != nil {
			return e
		}
		return activity(ctx, p, "issue", fresh.ID, "issue.started", "")
	})
	if err != nil {
		return domain.Issue{}, err
	}
	return s.GetIssue(ctx, initial.ID)
}

// issueByID re-reads an Issue inside the caller's transaction.
func issueByID(ctx context.Context, p provider, id string) (domain.Issue, error) {
	row, err := p.must(ctx, "issue", issueSelect()+` WHERE i.id=?`, id)
	if err != nil {
		return domain.Issue{}, err
	}
	return issueFrom(row), nil
}

// requireUnblocked enforces the prerequisite rule that Application also checks earlier.
func requireUnblocked(ctx context.Context, p provider, i domain.Issue) error {
	blockers, err := blockersFor(ctx, p, i.ID)
	if err != nil {
		return err
	}
	if len(blockers) > 0 {
		return formatBlocked(i, blockers)
	}
	return nil
}

func (s *Store) ListBlockers(ctx context.Context, issueID string) ([]domain.Blocker, error) {
	i, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}
	return blockersFor(ctx, s.p(), i.ID)
}

// blockersFor reports both unfinished prerequisites and unfinished Sub-issues.
func blockersFor(ctx context.Context, p provider, issueID string) ([]domain.Blocker, error) {
	rows, err := p.all(ctx, `SELECT p.id AS issue_id,r.name AS repository_name,p.number,p.title,p.status,'depends_on' AS reason
 FROM issue_dependencies d JOIN issues p ON p.id=d.prerequisite_issue_id JOIN repositories r ON r.id=p.repository_id
 WHERE d.issue_id=? AND p.status<>'done'
 UNION ALL
 SELECT c.id AS issue_id,r.name AS repository_name,c.number,c.title,c.status,'sub_issue_incomplete' AS reason
 FROM issues c JOIN repositories r ON r.id=c.repository_id WHERE c.parent_issue_id=? AND c.status IN ('open','in_progress')
 ORDER BY repository_name,number`, issueID, issueID)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r Row) domain.Blocker {
		return domain.Blocker{
			IssueID:        str(r, "issue_id"),
			RepositoryName: str(r, "repository_name"),
			Number:         i64(r["number"]),
			Title:          str(r, "title"),
			Status:         domain.IssueStatus(str(r, "status")),
			Reason:         str(r, "reason"),
		}
	}), nil
}

func formatBlocked(i domain.Issue, blockers []domain.Blocker) error {
	items := make([]string, 0, len(blockers))
	for _, b := range blockers {
		items = append(items, fmt.Sprintf("%s#%d %q (%s)", b.RepositoryName, b.Number, b.Title, b.Status))
	}
	return fmt.Errorf("%w: %s#%d is blocked by: %s", domain.ErrBlocked, i.RepositoryName, i.Number, strings.Join(items, "; "))
}

func (s *Store) ListIssueDependencies(ctx context.Context, issueRef string) ([]domain.Issue, error) {
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, issueSelect()+` JOIN issue_dependencies d ON d.prerequisite_issue_id=i.id WHERE d.issue_id=? ORDER BY r.name,i.number`, i.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, issueFrom), nil
}

// AddDependency records `issue depends_on prerequisite` after proving it adds no cycle.
func (s *Store) AddDependency(ctx context.Context, issueRef, prerequisiteRef string) error {
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	p, err := s.GetIssue(ctx, prerequisiteRef)
	if err != nil {
		return err
	}
	if i.ID == p.ID {
		return fmt.Errorf("%w: an issue cannot depend on itself", domain.ErrInvalid)
	}
	return s.run(ctx, func(q provider) error {
		if err := requireOpenDependent(ctx, q, i, "dependencies must be defined while the dependent Issue is open"); err != nil {
			return err
		}
		// A is blocked by B when A -> B, so adding A -> B is cyclic when B already reaches A.
		cyclic, e := q.exists(ctx, `WITH RECURSIVE prerequisites(id) AS (
 SELECT ? UNION SELECT d.prerequisite_issue_id FROM issue_dependencies d JOIN prerequisites p ON d.issue_id=p.id
) SELECT 1 AS found FROM prerequisites WHERE id=? LIMIT 1`, p.ID, i.ID)
		if e != nil {
			return e
		}
		if cyclic {
			return fmt.Errorf("%w: adding %s#%d depends_on %s#%d would create a dependency cycle", domain.ErrInvalid, i.RepositoryName, i.Number, p.RepositoryName, p.Number)
		}
		if _, e = q.changes(ctx, `INSERT INTO issue_dependencies(issue_id,prerequisite_issue_id,created_at) VALUES(?,?,?)`, i.ID, p.ID, nowText()); e != nil {
			return e
		}
		return activity(ctx, q, "issue", i.ID, "issue.dependency_added", marshalPayload(map[string]string{
			"prerequisite_issue_id":  p.ID,
			"prerequisite_reference": reference(p.RepositoryName, "#", p.Number),
		}))
	})
}

func (s *Store) RemoveDependency(ctx context.Context, issueRef, prerequisiteRef string) error {
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	p, err := s.GetIssue(ctx, prerequisiteRef)
	if err != nil {
		return err
	}
	return s.run(ctx, func(q provider) error {
		if err := requireOpenDependent(ctx, q, i, "dependencies can only be changed while the dependent Issue is open"); err != nil {
			return err
		}
		removed, e := q.changes(ctx, `DELETE FROM issue_dependencies WHERE issue_id=? AND prerequisite_issue_id=?`, i.ID, p.ID)
		if e != nil {
			return e
		}
		if removed == 0 {
			return fmt.Errorf("%w: %s#%d does not depend on %s#%d", domain.ErrNotFound, i.RepositoryName, i.Number, p.RepositoryName, p.Number)
		}
		return activity(ctx, q, "issue", i.ID, "issue.dependency_removed", marshalPayload(map[string]string{
			"prerequisite_issue_id":  p.ID,
			"prerequisite_reference": reference(p.RepositoryName, "#", p.Number),
		}))
	})
}

func requireOpenDependent(ctx context.Context, p provider, i domain.Issue, message string) error {
	fresh, err := p.must(ctx, fmt.Sprintf("dependent Issue %s", reference(i.RepositoryName, "#", i.Number)), `SELECT status FROM issues WHERE id=?`, i.ID)
	if err != nil {
		return err
	}
	if str(fresh, "status") != string(domain.IssueOpen) {
		return fmt.Errorf("%w: %s", domain.ErrInvalidTransition, message)
	}
	return nil
}

func (s *Store) ListDependencyEdges(ctx context.Context) ([]domain.GraphEdge, error) {
	rows, err := s.p().all(ctx, `SELECT issue_id,prerequisite_issue_id FROM issue_dependencies`)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r Row) domain.GraphEdge {
		return domain.GraphEdge{Source: str(r, "issue_id"), Target: str(r, "prerequisite_issue_id"), Type: "depends_on"}
	}), nil
}

// reference renders a Repository-scoped reference such as CPU#12.
func reference(name, separator string, number int64) string {
	return name + separator + strconv.FormatInt(number, 10)
}
