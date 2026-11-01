package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"idios/internal/domain"
	"idios/internal/localgit"
	"idios/internal/ports"
)

// LocalGitService owns real working copies separately from IDIOS's simulated
// repository snapshots. These methods never call a remote provider.
type LocalGitService struct {
	store  ports.RepositoryStore
	engine localgit.Engine
	root   string
}

func DefaultLocalGitWorkspaceRoot() string {
	if configured := strings.TrimSpace(os.Getenv("IDIOS_GIT_WORKSPACES_DIR")); configured != "" {
		return configured
	}
	base, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(base) == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "idios", "git-workspaces")
}

func NewLocalGitService(store ports.RepositoryStore, engine localgit.Engine, root string) *LocalGitService {
	if engine == nil {
		engine = localgit.NewCLIEngine()
	}
	if strings.TrimSpace(root) == "" {
		root = DefaultLocalGitWorkspaceRoot()
	}
	return &LocalGitService{store: store, engine: engine, root: root}
}

func (s *LocalGitService) workspacePath(ctx context.Context, repositoryRef string) (string, error) {
	if s.store == nil || s.engine == nil {
		return "", fmt.Errorf("%w: Local Git service is not configured", domain.ErrUnavailable)
	}
	repository, err := s.store.GetRepository(ctx, strings.TrimSpace(repositoryRef))
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve Local Git workspace root: %w", err)
	}
	root = filepath.Clean(root)
	path := filepath.Join(root, repository.ID)
	if filepath.Dir(path) != root {
		return "", fmt.Errorf("%w: invalid local repository identity", domain.ErrInvalid)
	}
	if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: managed Git working copy must not be a symbolic link", domain.ErrInvalid)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect managed Git working copy: %w", statErr)
	}
	return path, nil
}

func (s *LocalGitService) Status(ctx context.Context, repositoryRef string) (localgit.Status, error) {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return localgit.Status{}, err
	}
	return s.engine.Status(ctx, path)
}

func (s *LocalGitService) Initialize(ctx context.Context, repositoryRef, branch string) error {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return err
	}
	availability := s.engine.Availability()
	if !availability.Available {
		return fmt.Errorf("%w: %s", domain.ErrUnavailable, availability.Reason)
	}
	return s.engine.Init(ctx, path, branch)
}

func (s *LocalGitService) Clone(ctx context.Context, repositoryRef, remoteURL, branch string) error {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return err
	}
	availability := s.engine.Availability()
	if !availability.Available {
		return fmt.Errorf("%w: %s", domain.ErrUnavailable, availability.Reason)
	}
	return s.engine.Clone(ctx, remoteURL, path, branch)
}

func (s *LocalGitService) Branches(ctx context.Context, repositoryRef string) ([]localgit.Branch, error) {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return nil, err
	}
	if !s.engine.Availability().Available {
		return []localgit.Branch{}, nil
	}
	return s.engine.Branches(ctx, path)
}

func (s *LocalGitService) CreateBranch(ctx context.Context, repositoryRef, name, base string) error {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return err
	}
	if !s.engine.Availability().Available {
		return fmt.Errorf("%w: Git executable is not available", domain.ErrUnavailable)
	}
	return s.engine.CreateBranch(ctx, path, name, base)
}

func (s *LocalGitService) SwitchBranch(ctx context.Context, repositoryRef, name string) error {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return err
	}
	if !s.engine.Availability().Available {
		return fmt.Errorf("%w: Git executable is not available", domain.ErrUnavailable)
	}
	return s.engine.SwitchBranch(ctx, path, name)
}

func (s *LocalGitService) Stage(ctx context.Context, repositoryRef string, paths []string) error {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return err
	}
	if !s.engine.Availability().Available {
		return fmt.Errorf("%w: Git executable is not available", domain.ErrUnavailable)
	}
	return s.engine.Stage(ctx, path, paths)
}

func (s *LocalGitService) Commit(ctx context.Context, repositoryRef, message, authorName, authorEmail string) (localgit.Commit, error) {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return localgit.Commit{}, err
	}
	if !s.engine.Availability().Available {
		return localgit.Commit{}, fmt.Errorf("%w: Git executable is not available", domain.ErrUnavailable)
	}
	return s.engine.Commit(ctx, path, message, authorName, authorEmail)
}

func (s *LocalGitService) Log(ctx context.Context, repositoryRef string, limit int) ([]localgit.Commit, error) {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return nil, err
	}
	if !s.engine.Availability().Available {
		return []localgit.Commit{}, nil
	}
	return s.engine.Log(ctx, path, limit)
}

func (s *LocalGitService) Diff(ctx context.Context, repositoryRef string, staged bool) (string, error) {
	path, err := s.workspacePath(ctx, repositoryRef)
	if err != nil {
		return "", err
	}
	if !s.engine.Availability().Available {
		return "", fmt.Errorf("%w: Git executable is not available", domain.ErrUnavailable)
	}
	return s.engine.Diff(ctx, path, staged)
}
