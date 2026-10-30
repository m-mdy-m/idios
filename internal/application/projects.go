package application

import (
	"context"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type ProjectService struct {
	store  ports.ProjectStore
	issues *IssueService
}

func (s *ProjectService) CreateProject(ctx context.Context, name, description string) (domain.Project, error) {
	if err := invalidInput(validation.Text("project name", name, true, 160)); err != nil {
		return domain.Project{}, err
	}
	if err := invalidInput(validation.Text("project description", description, false, 10000)); err != nil {
		return domain.Project{}, err
	}
	return s.store.CreateProject(ctx, strings.TrimSpace(name), strings.TrimSpace(description))
}

func (s *ProjectService) DeleteProject(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("project reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteProject(ctx, strings.TrimSpace(ref))
}

func (s *ProjectService) GetProject(ctx context.Context, ref string) (domain.Project, error) {
	return s.store.GetProject(ctx, ref)
}

func (s *ProjectService) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return s.store.ListProjects(ctx)
}

func (s *ProjectService) ListProjectRepositories(ctx context.Context, project string) ([]domain.ProjectRepository, error) {
	return s.store.ListProjectRepositories(ctx, project)
}

func (s *ProjectService) LinkRepositoryToProject(ctx context.Context, project, repository, relationship string, primary bool) error {
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

func (s *ProjectService) UnlinkRepositoryFromProject(ctx context.Context, project, repository string) error {
	for _, item := range []struct{ name, value string }{{"project reference", project}, {"repository reference", repository}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.UnlinkRepositoryFromProject(ctx, strings.TrimSpace(project), strings.TrimSpace(repository))
}

func (s *ProjectService) SetPrimaryProjectRepository(ctx context.Context, project, repository string) error {
	for _, item := range []struct{ name, value string }{{"project reference", project}, {"repository reference", repository}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.SetPrimaryProjectRepository(ctx, strings.TrimSpace(project), strings.TrimSpace(repository))
}

func (s *ProjectService) AddIssueToProject(ctx context.Context, project, issue string) error {
	for _, item := range []struct{ name, value string }{{"project reference", project}, {"issue reference", issue}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.AddIssueToProject(ctx, strings.TrimSpace(project), strings.TrimSpace(issue))
}

func (s *ProjectService) ListProjectIssues(ctx context.Context, project string) ([]domain.Issue, error) {
	items, err := s.store.ListProjectIssues(ctx, project)
	if err != nil {
		return nil, err
	}
	return s.issues.withReadiness(ctx, items)
}
