package sqlite

import (
	"context"
	"fmt"
	"strings"

	"idios/internal/domain"
)

const repositoryColumns = `id,name,description,default_branch_id,created_at,updated_at`

func repositoryFrom(r Row) domain.Repository {
	return domain.Repository{
		ID:              str(r, "id"),
		Name:            str(r, "name"),
		Description:     str(r, "description"),
		DefaultBranchID: str(r, "default_branch_id"),
		CreatedAt:       parseTime(r["created_at"]),
		UpdatedAt:       parseTime(r["updated_at"]),
	}
}

// CreateRepository creates a repository and its single default `main` branch in one
// transaction; v1 never creates another branch.
func (s *Store) CreateRepository(ctx context.Context, name, description string) (domain.Repository, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Repository{}, fmt.Errorf("%w: repository name is required", domain.ErrInvalid)
	}
	now := nowText()
	repositoryID, branchID := domain.NewID(), domain.NewID()
	err := s.run(ctx, func(p provider) error {
		if _, e := p.changes(ctx, `INSERT INTO repositories(id,name,description,default_branch_id,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
			repositoryID, name, strings.TrimSpace(description), branchID, now, now); e != nil {
			return e
		}
		if _, e := p.changes(ctx, `INSERT INTO branches(id,repository_id,name,is_default,created_at,updated_at) VALUES(?,?, 'main',1,?,?)`,
			branchID, repositoryID, now, now); e != nil {
			return e
		}
		return activity(ctx, p, "repository", repositoryID, "repository.created", marshalPayload(map[string]string{"name": name}))
	})
	if err != nil {
		return domain.Repository{}, err
	}
	return s.GetRepository(ctx, repositoryID)
}

func (s *Store) GetRepository(ctx context.Context, ref string) (domain.Repository, error) {
	row, err := s.p().must(ctx, fmt.Sprintf("repository %q", ref),
		`SELECT `+repositoryColumns+` FROM repositories WHERE id=? OR name=? COLLATE NOCASE LIMIT 1`, ref, ref)
	if err != nil {
		return domain.Repository{}, err
	}
	return repositoryFrom(row), nil
}

func (s *Store) ListRepositories(ctx context.Context) ([]domain.Repository, error) {
	rows, err := s.p().all(ctx, `SELECT `+repositoryColumns+` FROM repositories ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	return collect(rows, repositoryFrom), nil
}

func (s *Store) SetRepositorySetting(ctx context.Context, repoRef, key, value string) error {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: setting key is required", domain.ErrInvalid)
	}
	_, err = s.p().changes(ctx, `INSERT INTO repository_settings(repository_id,key,value,updated_at) VALUES(?,?,?,?)
 ON CONFLICT(repository_id,key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, repo.ID, key, value, nowText())
	return err
}

func (s *Store) GetRepositorySettings(ctx context.Context, repoRef string) ([]domain.RepositorySetting, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, `SELECT key,value FROM repository_settings WHERE repository_id=? ORDER BY key`, repo.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r Row) domain.RepositorySetting {
		return domain.RepositorySetting{Key: str(r, "key"), Value: str(r, "value")}
	}), nil
}

// Branches ------------------------------------------------------------------

func (s *Store) ListBranches(ctx context.Context, repoRef string) ([]domain.Branch, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, `SELECT id,repository_id,name,is_default,base_branch_id,forked_from_completion_id,created_at,updated_at FROM branches WHERE repository_id=? ORDER BY is_default DESC,name`, repo.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, branchFrom), nil
}

func branchFrom(r Row) domain.Branch {
	return domain.Branch{
		ID:                     str(r, "id"),
		RepositoryID:           str(r, "repository_id"),
		Name:                   str(r, "name"),
		IsDefault:              boolRow(r, "is_default"),
		BaseBranchID:           ptrString(r["base_branch_id"]),
		ForkedFromCompletionID: ptrString(r["forked_from_completion_id"]),
		CreatedAt:              parseTime(r["created_at"]),
		UpdatedAt:              parseTime(r["updated_at"]),
	}
}

// Projects ------------------------------------------------------------------

const projectColumns = `id,name,description,created_at,updated_at`

func projectFrom(r Row) domain.Project {
	return domain.Project{
		ID:          str(r, "id"),
		Name:        str(r, "name"),
		Description: str(r, "description"),
		CreatedAt:   parseTime(r["created_at"]),
		UpdatedAt:   parseTime(r["updated_at"]),
	}
}

func (s *Store) CreateProject(ctx context.Context, name, description string) (domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Project{}, fmt.Errorf("%w: project name is required", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	if _, err := s.p().changes(ctx, `INSERT INTO projects(id,name,description,created_at,updated_at) VALUES(?,?,?,?,?)`,
		id, name, strings.TrimSpace(description), now, now); err != nil {
		return domain.Project{}, err
	}
	_ = s.RecordActivity(ctx, "project", id, "project.created", marshalPayload(map[string]string{"name": name}))
	return s.GetProject(ctx, id)
}

func (s *Store) GetProject(ctx context.Context, ref string) (domain.Project, error) {
	row, err := s.p().must(ctx, fmt.Sprintf("project %q", ref),
		`SELECT `+projectColumns+` FROM projects WHERE id=? OR name=? COLLATE NOCASE LIMIT 1`, ref, ref)
	if err != nil {
		return domain.Project{}, err
	}
	return projectFrom(row), nil
}

func (s *Store) ListProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.p().all(ctx, `SELECT `+projectColumns+` FROM projects ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	return collect(rows, projectFrom), nil
}

// AddIssueToProject links an Issue to a Project; ownership stays with its Repository.
func (s *Store) AddIssueToProject(ctx context.Context, projectRef, issueRef string) error {
	p, err := s.GetProject(ctx, projectRef)
	if err != nil {
		return err
	}
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	if _, err := s.p().changes(ctx, `INSERT INTO project_issues(project_id,issue_id,created_at) VALUES(?,?,?)`, p.ID, i.ID, nowText()); err != nil {
		return err
	}
	_ = s.RecordActivity(ctx, "issue", i.ID, "issue.added_to_project", marshalPayload(map[string]string{"project_id": p.ID}))
	return nil
}

func (s *Store) ListProjectIssues(ctx context.Context, projectRef string) ([]domain.Issue, error) {
	p, err := s.GetProject(ctx, projectRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, `SELECT i.id,i.number,i.repository_id,r.name AS repository_name,i.title,i.description,i.status,i.priority,i.due_date,i.parent_issue_id,i.created_at,i.updated_at,i.closed_at
 FROM project_issues pi JOIN issues i ON i.id=pi.issue_id JOIN repositories r ON r.id=i.repository_id
 WHERE pi.project_id=? ORDER BY r.name,i.number`, p.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, issueFrom), nil
}
