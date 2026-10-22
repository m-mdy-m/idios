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

// CreateRepository creates a repository with a default `main` workflow branch.
func (s *Store) CreateRepository(ctx context.Context, name, description string) (domain.Repository, error) {
	return s.CreateRepositoryWithDefaultBranch(ctx, name, description, "main")
}

// CreateRepositoryWithDefaultBranch allows a workspace to choose its initial workflow branch.
// It does not initialize or alter a Git working tree.
func (s *Store) CreateRepositoryWithDefaultBranch(ctx context.Context, name, description, defaultBranch string) (domain.Repository, error) {
	name = strings.TrimSpace(name)
	defaultBranch = strings.TrimSpace(defaultBranch)
	if name == "" {
		return domain.Repository{}, fmt.Errorf("%w: repository name is required", domain.ErrInvalid)
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if err := validateBranchName(defaultBranch); err != nil {
		return domain.Repository{}, err
	}
	now := nowText()
	repositoryID, branchID := domain.NewID(), domain.NewID()
	err := s.run(ctx, func(p provider) error {
		if _, e := p.changes(ctx, `INSERT INTO repositories(id,name,description,default_branch_id,created_at,updated_at) VALUES(?,?,?,?,?,?)`, repositoryID, name, strings.TrimSpace(description), branchID, now, now); e != nil {
			return e
		}
		if _, e := p.changes(ctx, `INSERT INTO branches(id,repository_id,name,is_default,created_at,updated_at) VALUES(?,?,?,?,?,?)`, branchID, repositoryID, defaultBranch, 1, now, now); e != nil {
			return e
		}
		return activity(ctx, p, "repository", repositoryID, "repository.created", marshalPayload(map[string]string{"name": name, "default_branch": defaultBranch}))
	})
	if err != nil {
		return domain.Repository{}, err
	}
	return s.GetRepository(ctx, repositoryID)
}

func (s *Store) UpdateRepositoryDescription(ctx context.Context, ref, description string) (domain.Repository, error) {
	repo, err := s.GetRepository(ctx, ref)
	if err != nil {
		return domain.Repository{}, err
	}
	_, err = s.p().changes(ctx, `UPDATE repositories SET description=?,updated_at=? WHERE id=?`, strings.TrimSpace(description), nowText(), repo.ID)
	if err != nil {
		return domain.Repository{}, err
	}
	return s.GetRepository(ctx, repo.ID)
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

// CreateBranch adds a named workflow branch based on another branch in the same repository.
// These are IDIOS workflow branches; they do not create or modify filesystem Git refs.
func (s *Store) CreateBranch(ctx context.Context, repoRef, name, baseRef string) (domain.Branch, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return domain.Branch{}, err
	}
	name = strings.TrimSpace(name)
	if err := validateBranchName(name); err != nil {
		return domain.Branch{}, err
	}
	if baseRef == "" {
		baseRef = repo.DefaultBranchID
	}
	base, err := s.p().must(ctx, "base branch", `SELECT id FROM branches WHERE repository_id=? AND (id=? OR name=? COLLATE NOCASE) LIMIT 1`, repo.ID, baseRef, baseRef)
	if err != nil {
		return domain.Branch{}, fmt.Errorf("base branch must belong to repository %q: %w", repo.Name, err)
	}
	id, now := domain.NewID(), nowText()
	err = s.run(ctx, func(p provider) error {
		if _, e := p.changes(ctx, `INSERT INTO branches(id,repository_id,name,is_default,base_branch_id,created_at,updated_at) VALUES(?,?,?,0,?,?,?)`, id, repo.ID, name, str(base, "id"), now, now); e != nil {
			return e
		}
		return activity(ctx, p, "branch", id, "branch.created", marshalPayload(map[string]any{"repository": repo.Name, "name": name, "base": baseRef}))
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.Branch{}, fmt.Errorf("%w: branch %q already exists in %s", domain.ErrInvalid, name, repo.Name)
		}
		return domain.Branch{}, err
	}
	rows, err := s.ListBranches(ctx, repo.ID)
	if err != nil {
		return domain.Branch{}, err
	}
	for _, item := range rows {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.Branch{}, fmt.Errorf("created branch could not be reloaded")
}

func validateBranchName(name string) error {
	if name == "" || name == "@" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") || strings.Contains(name, "//") || strings.Contains(name, "@{") || strings.ContainsAny(name, " ~^:?*[\\") {
		return fmt.Errorf("%w: invalid branch name %q", domain.ErrInvalid, name)
	}
	for _, ch := range name {
		if ch < 32 || ch == 127 {
			return fmt.Errorf("%w: branch names cannot contain control characters", domain.ErrInvalid)
		}
	}
	return nil
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

const projectRepositoryColumns = `pr.project_id,pr.repository_id,r.name AS repository_name,r.description AS repository_description,
 pr.relationship,pr.is_primary,pr.created_at,pr.updated_at`

func projectRepositoryFrom(row Row) domain.ProjectRepository {
	return domain.ProjectRepository{
		ProjectID: str(row, "project_id"), RepositoryID: str(row, "repository_id"),
		RepositoryName: str(row, "repository_name"), RepositoryDescription: str(row, "repository_description"),
		Relationship: str(row, "relationship"), IsPrimary: boolRow(row, "is_primary"),
		CreatedAt: parseTime(row["created_at"]), UpdatedAt: parseTime(row["updated_at"]),
	}
}

func (s *Store) ListProjectRepositories(ctx context.Context, projectRef string) ([]domain.ProjectRepository, error) {
	project, err := s.GetProject(ctx, projectRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, `SELECT `+projectRepositoryColumns+`
 FROM project_repositories pr JOIN repositories r ON r.id=pr.repository_id
 WHERE pr.project_id=? ORDER BY pr.is_primary DESC,r.name COLLATE NOCASE`, project.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, projectRepositoryFrom), nil
}

// LinkRepositoryToProject records why a repository belongs to a project. The first linked
// repository is automatically primary unless another primary has already been selected.
func (s *Store) LinkRepositoryToProject(ctx context.Context, projectRef, repositoryRef, relationship string, primary bool) error {
	project, err := s.GetProject(ctx, projectRef)
	if err != nil {
		return err
	}
	repo, err := s.GetRepository(ctx, repositoryRef)
	if err != nil {
		return err
	}
	relationship = strings.TrimSpace(relationship)
	now := nowText()
	return s.run(ctx, func(p provider) error {
		primaryRow, hasPrimary, err := p.row(ctx, `SELECT repository_id FROM project_repositories WHERE project_id=? AND is_primary=1 LIMIT 1`, project.ID)
		if err != nil {
			return err
		}
		makePrimary := primary || !hasPrimary || (hasPrimary && str(primaryRow, "repository_id") == repo.ID)
		if makePrimary {
			if _, err := p.changes(ctx, `UPDATE project_repositories SET is_primary=0,updated_at=? WHERE project_id=?`, now, project.ID); err != nil {
				return err
			}
		}
		if _, err := p.changes(ctx, `INSERT INTO project_repositories(project_id,repository_id,relationship,is_primary,created_at,updated_at)
 VALUES(?,?,?,?,?,?) ON CONFLICT(project_id,repository_id) DO UPDATE SET relationship=excluded.relationship,is_primary=excluded.is_primary,updated_at=excluded.updated_at`,
			project.ID, repo.ID, relationship, makePrimary, now, now); err != nil {
			return err
		}
		return activity(ctx, p, "project", project.ID, "project.repository.linked", marshalPayload(map[string]any{"project": project.Name, "repository": repo.Name, "primary": makePrimary, "relationship": relationship}))
	})
}

func (s *Store) UnlinkRepositoryFromProject(ctx context.Context, projectRef, repositoryRef string) error {
	project, err := s.GetProject(ctx, projectRef)
	if err != nil {
		return err
	}
	repo, err := s.GetRepository(ctx, repositoryRef)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `DELETE FROM project_repositories WHERE project_id=? AND repository_id=?`, project.ID, repo.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: repository %q is not linked to project %q", domain.ErrNotFound, repo.Name, project.Name)
		}
		return activity(ctx, p, "project", project.ID, "project.repository.unlinked", marshalPayload(map[string]string{"project": project.Name, "repository": repo.Name}))
	})
}

func (s *Store) SetPrimaryProjectRepository(ctx context.Context, projectRef, repositoryRef string) error {
	project, err := s.GetProject(ctx, projectRef)
	if err != nil {
		return err
	}
	repo, err := s.GetRepository(ctx, repositoryRef)
	if err != nil {
		return err
	}
	now := nowText()
	return s.run(ctx, func(p provider) error {
		if _, err := p.must(ctx, "repository link", `SELECT repository_id FROM project_repositories WHERE project_id=? AND repository_id=?`, project.ID, repo.ID); err != nil {
			return fmt.Errorf("%w: link repository %q to project %q before making it primary", domain.ErrInvalid, repo.Name, project.Name)
		}
		if _, err := p.changes(ctx, `UPDATE project_repositories SET is_primary=0,updated_at=? WHERE project_id=?`, now, project.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE project_repositories SET is_primary=1,updated_at=? WHERE project_id=? AND repository_id=?`, now, project.ID, repo.ID); err != nil {
			return err
		}
		return activity(ctx, p, "project", project.ID, "project.repository.primary_changed", marshalPayload(map[string]string{"project": project.Name, "repository": repo.Name}))
	})
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
