package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type PlanningMetadataService struct{ store ports.PlanningMetadataStore }

func (s *PlanningMetadataService) CreateLabel(ctx context.Context, name, scope, repo, description, color string) (domain.Label, error) {
	if err := invalidInput(validation.Text("label name", name, true, 100)); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.Text("label scope", scope, true, 32)); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.OneOf("label scope", scope, "global", "repository")); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.Text("repository reference", repo, false, 256)); err != nil {
		return domain.Label{}, err
	}
	if err := invalidInput(validation.Text("label description", description, false, 3000)); err != nil {
		return domain.Label{}, err
	}
	if strings.TrimSpace(color) == "" {
		color = "#7c9cff"
	}
	if err := invalidInput(validation.HexColor("label color", color)); err != nil {
		return domain.Label{}, err
	}
	if strings.TrimSpace(scope) == "repository" && strings.TrimSpace(repo) == "" {
		return domain.Label{}, invalidInput(fmt.Errorf("repository reference is required for repository-scoped labels"))
	}
	if strings.TrimSpace(scope) == "global" && strings.TrimSpace(repo) != "" {
		return domain.Label{}, invalidInput(fmt.Errorf("repository reference must be empty for global labels"))
	}
	return s.store.CreateLabel(ctx, strings.TrimSpace(name), strings.TrimSpace(scope), strings.TrimSpace(repo), strings.TrimSpace(description), strings.TrimSpace(color))
}

func (s *PlanningMetadataService) DeleteLabel(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("label reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteLabel(ctx, strings.TrimSpace(ref))
}

func (s *PlanningMetadataService) ListLabels(ctx context.Context, scope string) ([]domain.Label, error) {
	return s.store.ListLabels(ctx, scope)
}

func (s *PlanningMetadataService) ListIssueLabels(ctx context.Context) (map[string][]domain.Label, error) {
	return s.store.ListIssueLabels(ctx)
}

func (s *PlanningMetadataService) AssignLabel(ctx context.Context, issue, label string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issue}, {"label reference", label}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.AssignLabel(ctx, strings.TrimSpace(issue), strings.TrimSpace(label))
}

func (s *PlanningMetadataService) CreateMilestone(ctx context.Context, scopeType, scopeRef, title, description, due string) (domain.Milestone, error) {
	if err := invalidInput(validation.Text("milestone scope", scopeType, true, 32)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.OneOf("milestone scope", scopeType, "repository", "project")); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.Text("milestone owner", scopeRef, true, 256)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.Text("milestone title", title, true, 200)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.Text("milestone description", description, false, 10000)); err != nil {
		return domain.Milestone{}, err
	}
	if err := invalidInput(validation.ISODate("due date", due, false)); err != nil {
		return domain.Milestone{}, err
	}
	var ptr *string
	if strings.TrimSpace(due) != "" {
		cleanDue := strings.TrimSpace(due)
		ptr = &cleanDue
	}
	return s.store.CreateMilestone(ctx, strings.TrimSpace(scopeType), strings.TrimSpace(scopeRef), strings.TrimSpace(title), strings.TrimSpace(description), ptr)
}

func (s *PlanningMetadataService) DeleteMilestone(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("milestone reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteMilestone(ctx, strings.TrimSpace(ref))
}

func (s *PlanningMetadataService) ListMilestones(ctx context.Context, scopeType, scopeRef string) ([]domain.Milestone, error) {
	return s.store.ListMilestones(ctx, scopeType, scopeRef)
}

func (s *PlanningMetadataService) ListIssueMilestones(ctx context.Context) (map[string][]domain.Milestone, error) {
	return s.store.ListIssueMilestones(ctx)
}

func (s *PlanningMetadataService) AssignMilestone(ctx context.Context, issue, milestone string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issue}, {"milestone reference", milestone}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.AssignMilestone(ctx, strings.TrimSpace(issue), strings.TrimSpace(milestone))
}
