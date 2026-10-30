package application

import (
	"context"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type RepositoryService struct {
	store ports.RepositoryStore
}

func (s *RepositoryService) CreateRepository(ctx context.Context, name, description string) (domain.Repository, error) {
	if err := invalidInput(validation.Text("repository name", name, true, 100)); err != nil {
		return domain.Repository{}, err
	}
	if err := invalidInput(validation.Text("repository description", description, false, 5000)); err != nil {
		return domain.Repository{}, err
	}
	return s.store.CreateRepository(ctx, strings.TrimSpace(name), strings.TrimSpace(description))
}

func (s *RepositoryService) CreateRepositoryWithDefaultBranch(ctx context.Context, name, description, defaultBranch string) (domain.Repository, error) {
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

func (s *RepositoryService) UpdateRepositoryDescription(ctx context.Context, repository, description string) (domain.Repository, error) {
	if err := invalidInput(validation.Text("repository reference", repository, true, 256)); err != nil {
		return domain.Repository{}, err
	}
	if err := invalidInput(validation.Text("repository description", description, false, 5000)); err != nil {
		return domain.Repository{}, err
	}
	return s.store.UpdateRepositoryDescription(ctx, strings.TrimSpace(repository), strings.TrimSpace(description))
}

func (s *RepositoryService) DeleteRepository(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("repository reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteRepository(ctx, strings.TrimSpace(ref))
}

func (s *RepositoryService) GetRepository(ctx context.Context, ref string) (domain.Repository, error) {
	return s.store.GetRepository(ctx, ref)
}

func (s *RepositoryService) ListRepositories(ctx context.Context) ([]domain.Repository, error) {
	return s.store.ListRepositories(ctx)
}

func (s *RepositoryService) ListBranches(ctx context.Context, repo string) ([]domain.Branch, error) {
	return s.store.ListBranches(ctx, repo)
}

func (s *RepositoryService) CreateBranch(ctx context.Context, repo, name, base string) (domain.Branch, error) {
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

func (s *RepositoryService) AssignIssueBranch(ctx context.Context, issue, branch string) error {
	if err := invalidInput(validation.Text("issue reference", issue, true, 256)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("branch reference", branch, false, 256)); err != nil {
		return err
	}
	return s.store.AssignIssueBranch(ctx, strings.TrimSpace(issue), strings.TrimSpace(branch))
}

func (s *RepositoryService) AssignIssueParent(ctx context.Context, issue, parent string) error {
	if err := invalidInput(validation.Text("issue reference", issue, true, 256)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("parent issue reference", parent, false, 256)); err != nil {
		return err
	}
	return s.store.AssignIssueParent(ctx, strings.TrimSpace(issue), strings.TrimSpace(parent))
}

func (s *RepositoryService) SetRepositorySetting(ctx context.Context, repo, key, value string) error {
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

func (s *RepositoryService) GetRepositorySettings(ctx context.Context, repo string) ([]domain.RepositorySetting, error) {
	return s.store.GetRepositorySettings(ctx, repo)
}

func (s *RepositoryService) ListRepositoryFiles(ctx context.Context, repo string) ([]domain.RepositoryFile, error) {
	return s.store.ListRepositoryFiles(ctx, repo)
}

func (s *RepositoryService) SaveRepositoryFile(ctx context.Context, repo, path, content string) (domain.RepositoryFile, error) {
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

func (s *RepositoryService) DeleteRepositoryFile(ctx context.Context, repo, path string) error {
	return s.store.DeleteRepositoryFile(ctx, repo, path)
}

func (s *RepositoryService) ListRepositoryTags(ctx context.Context, repo string) ([]domain.RepositoryTag, error) {
	return s.store.ListRepositoryTags(ctx, repo)
}

func (s *RepositoryService) CreateRepositoryTag(ctx context.Context, repo, name, message string) (domain.RepositoryTag, error) {
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

func (s *RepositoryService) GetRepositoryTag(ctx context.Context, repo, tag string) (domain.RepositoryTag, error) {
	return s.store.GetRepositoryTag(ctx, repo, tag)
}

func (s *RepositoryService) GetRepositoryTagFiles(ctx context.Context, tagID string) ([]domain.RepositoryFile, error) {
	return s.store.GetRepositoryTagFiles(ctx, tagID)
}

func (s *RepositoryService) CompareRepositoryTags(ctx context.Context, repo, tag string) ([]domain.TagFileChange, error) {
	return s.store.CompareRepositoryTags(ctx, repo, tag)
}

func (s *RepositoryService) ListTagHistory(ctx context.Context, tagID string) ([]domain.CompletionRecord, error) {
	return s.store.ListTagHistory(ctx, tagID)
}
