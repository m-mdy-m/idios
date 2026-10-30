package application

import (
	"context"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type ProfileService struct{ store ports.ProfileStore }

func (s *ProfileService) GetWorkspaceProfile(ctx context.Context) (domain.WorkspaceProfile, error) {
	return s.store.GetWorkspaceProfile(ctx)
}

func (s *ProfileService) UpdateWorkspaceProfile(ctx context.Context, profile domain.WorkspaceProfile) (domain.WorkspaceProfile, error) {
	if err := invalidInput(validation.Text("username", profile.Username, true, 80)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("display name", profile.FullName, true, 160)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("current focus", profile.Focus, false, 200)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("topics", profile.Topics, false, 1000)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.URL("profile website", profile.Website)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("profile bio", profile.Bio, false, 10000)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	if err := invalidInput(validation.Text("workspace README", profile.Readme, false, 50000)); err != nil {
		return domain.WorkspaceProfile{}, err
	}
	return s.store.UpdateWorkspaceProfile(ctx, profile)
}

func (s *ProfileService) ListWorkspacePins(ctx context.Context) ([]domain.WorkspacePin, error) {
	return s.store.ListWorkspacePins(ctx)
}

func (s *ProfileService) ToggleWorkspacePin(ctx context.Context, entityType, entityRef, label, color string) (bool, error) {
	if err := invalidInput(validation.Text("pin entity type", entityType, true, 32)); err != nil {
		return false, err
	}
	if err := invalidInput(validation.OneOf("pin entity type", entityType, "issue", "repository", "pull_request", "project", "concept", "note", "milestone", "label")); err != nil {
		return false, err
	}
	if err := invalidInput(validation.Text("pinned entity reference", entityRef, true, 256)); err != nil {
		return false, err
	}
	if err := invalidInput(validation.Text("pin label", label, false, 100)); err != nil {
		return false, err
	}
	if strings.TrimSpace(color) == "" {
		color = "accent"
	}
	if err := invalidInput(validation.OneOf("pin color token", color, "accent", "warning", "success", "danger", "info", "muted", "blue", "violet", "amber", "green", "red", "yellow", "cyan", "neutral")); err != nil {
		return false, err
	}
	return s.store.ToggleWorkspacePin(ctx, strings.TrimSpace(entityType), strings.TrimSpace(entityRef), strings.TrimSpace(label), strings.TrimSpace(color))
}
