package sqlite

import (
	"context"
	"fmt"
	"strings"

	"idios/internal/domain"
)

const labelSelect = `SELECT l.id,l.name,l.description,l.color,l.scope_type,l.repository_id,COALESCE(r.name,'') AS repository_name,l.created_at
 FROM labels l LEFT JOIN repositories r ON r.id=l.repository_id`

const milestoneSelect = `SELECT m.id,m.title,m.description,m.scope_type,m.scope_id,COALESCE(r.name,p.name,'') AS scope_name,m.due_date,m.created_at,m.updated_at
 FROM milestones m LEFT JOIN repositories r ON m.scope_type='repository' AND r.id=m.scope_id
 LEFT JOIN projects p ON m.scope_type='project' AND p.id=m.scope_id`

func labelFrom(r Row) domain.Label {
	return domain.Label{
		ID:             str(r, "id"),
		Name:           str(r, "name"),
		Description:    str(r, "description"),
		Color:          str(r, "color"),
		ScopeType:      str(r, "scope_type"),
		RepositoryID:   ptrString(r["repository_id"]),
		RepositoryName: str(r, "repository_name"),
		CreatedAt:      parseTime(r["created_at"]),
	}
}

func milestoneFrom(r Row) domain.Milestone {
	return domain.Milestone{
		ID:          str(r, "id"),
		Title:       str(r, "title"),
		Description: str(r, "description"),
		ScopeType:   str(r, "scope_type"),
		ScopeID:     str(r, "scope_id"),
		ScopeName:   str(r, "scope_name"),
		DueDate:     ptrString(r["due_date"]),
		CreatedAt:   parseTime(r["created_at"]),
		UpdatedAt:   parseTime(r["updated_at"]),
	}
}

func (s *Store) CreateLabel(ctx context.Context, name, scope, repoRef, description, color string) (domain.Label, error) {
	name = strings.TrimSpace(name)
	scope = strings.ToLower(strings.TrimSpace(scope))
	if name == "" {
		return domain.Label{}, fmt.Errorf("%w: label name is required", domain.ErrInvalid)
	}
	var repositoryID any
	switch scope {
	case "global":
		repositoryID = nil
	case "repository", "repo":
		r, err := s.GetRepository(ctx, repoRef)
		if err != nil {
			return domain.Label{}, err
		}
		scope, repositoryID = "repository", r.ID
	default:
		return domain.Label{}, fmt.Errorf("%w: label scope must be global or repository", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	if _, err := s.p().changes(ctx, `INSERT INTO labels(id,name,description,color,scope_type,repository_id,created_at) VALUES(?,?,?,?,?,?,?)`,
		id, name, strings.TrimSpace(description), strings.TrimSpace(color), scope, repositoryID, now); err != nil {
		return domain.Label{}, err
	}
	return s.getLabel(ctx, id)
}

func (s *Store) getLabel(ctx context.Context, ref string) (domain.Label, error) {
	row, err := s.p().must(ctx, fmt.Sprintf("label %q", ref), labelSelect+` WHERE l.id=? LIMIT 1`, ref)
	if err != nil {
		return domain.Label{}, err
	}
	return labelFrom(row), nil
}

// ListLabels shows global labels plus, when a repository is named, its local labels.
func (s *Store) ListLabels(ctx context.Context, scope string) ([]domain.Label, error) {
	query := labelSelect + ` WHERE 1=1`
	args := []any{}
	switch {
	case scope == "":
	case strings.EqualFold(scope, "global"):
		query += ` AND l.scope_type='global'`
	default:
		repo, err := s.GetRepository(ctx, scope)
		if err != nil {
			return nil, err
		}
		query += ` AND (l.scope_type='global' OR l.repository_id=?)`
		args = append(args, repo.ID)
	}
	query += ` ORDER BY l.scope_type,l.name COLLATE NOCASE`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, labelFrom), nil
}

// ListIssueLabels returns Issue-to-label assignments keyed by the stable Issue ID.
// It is a bulk read intended for overview/list views, avoiding one query per Issue.
func (s *Store) ListIssueLabels(ctx context.Context) (map[string][]domain.Label, error) {
	rows, err := s.p().all(ctx, `SELECT il.issue_id AS issue_id,l.id,l.name,l.description,l.color,l.scope_type,l.repository_id,COALESCE(r.name,'') AS repository_name,l.created_at
 FROM issue_labels il JOIN labels l ON l.id=il.label_id LEFT JOIN repositories r ON r.id=l.repository_id
 ORDER BY il.issue_id,l.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]domain.Label)
	for _, row := range rows {
		issueID := str(row, "issue_id")
		out[issueID] = append(out[issueID], labelFrom(row))
	}
	return out, nil
}

// labelForIssue prefers a Repository-local label over a same-name global one.
func (s *Store) labelForIssue(ctx context.Context, issue domain.Issue, ref string) (domain.Label, error) {
	row, err := s.p().must(ctx, fmt.Sprintf("compatible label %q", ref),
		labelSelect+` WHERE l.id=? OR (l.name=? COLLATE NOCASE AND (l.scope_type='global' OR l.repository_id=?))
 ORDER BY CASE WHEN l.repository_id=? THEN 0 ELSE 1 END LIMIT 1`, ref, ref, issue.RepositoryID, issue.RepositoryID)
	if err != nil {
		return domain.Label{}, err
	}
	return labelFrom(row), nil
}

func (s *Store) AssignLabel(ctx context.Context, issueRef, labelRef string) error {
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	l, err := s.labelForIssue(ctx, i, labelRef)
	if err != nil {
		return err
	}
	if _, err := s.p().changes(ctx, `INSERT INTO issue_labels(issue_id,label_id,created_at) VALUES(?,?,?)`, i.ID, l.ID, nowText()); err != nil {
		return err
	}
	return s.RecordActivity(ctx, "issue", i.ID, "issue.label_assigned", marshalPayload(map[string]string{"label_id": l.ID, "label": l.Name}))
}

func (s *Store) CreateMilestone(ctx context.Context, scopeType, scopeRef, title, description string, dueDate *string) (domain.Milestone, error) {
	scopeType = strings.ToLower(strings.TrimSpace(scopeType))
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Milestone{}, fmt.Errorf("%w: milestone title is required", domain.ErrInvalid)
	}
	var scopeID string
	switch scopeType {
	case "repository", "repo":
		r, err := s.GetRepository(ctx, scopeRef)
		if err != nil {
			return domain.Milestone{}, err
		}
		scopeType, scopeID = "repository", r.ID
	case "project":
		p, err := s.GetProject(ctx, scopeRef)
		if err != nil {
			return domain.Milestone{}, err
		}
		scopeID = p.ID
	default:
		return domain.Milestone{}, fmt.Errorf("%w: milestone scope must be repository or project", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	if _, err := s.p().changes(ctx, `INSERT INTO milestones(id,title,description,scope_type,scope_id,due_date,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		id, title, strings.TrimSpace(description), scopeType, scopeID, dueDate, now, now); err != nil {
		return domain.Milestone{}, err
	}
	return s.getMilestone(ctx, id)
}

// getMilestone resolves a stable ID, a scoped reference, or a bare title when unique.
func (s *Store) getMilestone(ctx context.Context, ref string) (domain.Milestone, error) {
	if raw := strings.TrimPrefix(ref, "milestone:"); raw != ref {
		parts := strings.SplitN(raw, "/", 3)
		if len(parts) != 3 {
			return domain.Milestone{}, fmt.Errorf("%w: scoped milestone reference must look like milestone:repository/CPU/Release or milestone:project/Project/Release", domain.ErrInvalid)
		}
		scopeType, ownerID, err := s.milestoneScope(ctx, parts[0], parts[1])
		if err != nil {
			return domain.Milestone{}, err
		}
		row, err := s.p().must(ctx, fmt.Sprintf("milestone %q", ref),
			milestoneSelect+` WHERE m.scope_type=? AND m.scope_id=? AND m.title=? COLLATE NOCASE LIMIT 1`, scopeType, ownerID, parts[2])
		if err != nil {
			return domain.Milestone{}, err
		}
		return milestoneFrom(row), nil
	}
	row, ok, err := s.p().row(ctx, milestoneSelect+` WHERE m.id=? LIMIT 1`, ref)
	if err != nil {
		return domain.Milestone{}, err
	}
	if !ok {
		// A bare title is accepted only when it resolves uniquely.
		rows, err := s.p().all(ctx, milestoneSelect+` WHERE m.title=? COLLATE NOCASE ORDER BY m.scope_type,m.scope_id LIMIT 2`, ref)
		if err != nil {
			return domain.Milestone{}, err
		}
		if len(rows) > 1 {
			return domain.Milestone{}, fmt.Errorf("%w: milestone title %q is ambiguous; use milestone:repository/CPU/title or milestone:project/Project/title", domain.ErrInvalid, ref)
		}
		if len(rows) == 0 {
			return domain.Milestone{}, fmt.Errorf("%w: milestone %q", domain.ErrNotFound, ref)
		}
		row = rows[0]
	}
	return milestoneFrom(row), nil
}

func (s *Store) milestoneScope(ctx context.Context, scopeType, ownerRef string) (string, string, error) {
	switch strings.ToLower(scopeType) {
	case "repository", "repo":
		repo, err := s.GetRepository(ctx, ownerRef)
		if err != nil {
			return "", "", err
		}
		return "repository", repo.ID, nil
	case "project":
		project, err := s.GetProject(ctx, ownerRef)
		if err != nil {
			return "", "", err
		}
		return "project", project.ID, nil
	}
	return "", "", fmt.Errorf("%w: milestone scope must be repository or project", domain.ErrInvalid)
}

func (s *Store) ListMilestones(ctx context.Context, scopeType, scopeRef string) ([]domain.Milestone, error) {
	query := milestoneSelect + ` WHERE 1=1`
	args := []any{}
	if scopeType != "" {
		normalized := strings.ToLower(scopeType)
		if normalized == "repo" {
			normalized = "repository"
		}
		query += ` AND m.scope_type=?`
		args = append(args, normalized)
		scopeType = normalized
	}
	if scopeRef != "" {
		if scopeType == "" {
			return nil, fmt.Errorf("%w: scope type required with scope reference", domain.ErrInvalid)
		}
		_, scopeID, err := s.milestoneScope(ctx, scopeType, scopeRef)
		if err != nil {
			return nil, err
		}
		query += ` AND m.scope_id=?`
		args = append(args, scopeID)
	}
	query += ` ORDER BY m.due_date IS NULL,m.due_date,m.title COLLATE NOCASE`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows, milestoneFrom), nil
}

// ListIssueMilestones returns Issue-to-milestone assignments keyed by stable Issue ID.
// Results are bulk-loaded so Web views do not issue one query per row.
func (s *Store) ListIssueMilestones(ctx context.Context) (map[string][]domain.Milestone, error) {
	rows, err := s.p().all(ctx, `SELECT im.issue_id AS issue_id,m.id,m.title,m.description,m.scope_type,m.scope_id,COALESCE(r.name,p.name,'') AS scope_name,m.due_date,m.created_at,m.updated_at
 FROM issue_milestones im JOIN milestones m ON m.id=im.milestone_id
 LEFT JOIN repositories r ON m.scope_type='repository' AND r.id=m.scope_id
 LEFT JOIN projects p ON m.scope_type='project' AND p.id=m.scope_id
 ORDER BY im.issue_id,m.title COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]domain.Milestone)
	for _, row := range rows {
		issueID := str(row, "issue_id")
		out[issueID] = append(out[issueID], milestoneFrom(row))
	}
	return out, nil
}

func (s *Store) AssignMilestone(ctx context.Context, issueRef, milestoneRef string) error {
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	m, err := s.getMilestone(ctx, milestoneRef)
	if err != nil {
		return err
	}
	if m.ScopeType == "repository" && m.ScopeID != i.RepositoryID {
		return fmt.Errorf("%w: repository milestone can only be assigned to issues in its repository", domain.ErrInvalid)
	}
	if m.ScopeType == "project" {
		member, err := s.p().exists(ctx, `SELECT 1 AS member FROM project_issues WHERE project_id=? AND issue_id=?`, m.ScopeID, i.ID)
		if err != nil {
			return err
		}
		if !member {
			return fmt.Errorf("%w: add the issue to the milestone's project first", domain.ErrInvalid)
		}
	}
	if _, err := s.p().changes(ctx, `INSERT INTO issue_milestones(issue_id,milestone_id,created_at) VALUES(?,?,?)`, i.ID, m.ID, nowText()); err != nil {
		return err
	}
	return s.RecordActivity(ctx, "issue", i.ID, "issue.milestone_assigned", marshalPayload(map[string]string{"milestone_id": m.ID, "milestone": m.Title}))
}
