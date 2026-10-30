package application

import (
	"context"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type ConceptService struct{ store ports.ConceptStore }

func (s *ConceptService) CreateConcept(ctx context.Context, name, description string) (domain.Concept, error) {
	if err := invalidInput(validation.Text("concept name", name, true, 160)); err != nil {
		return domain.Concept{}, err
	}
	if err := invalidInput(validation.Text("concept description", description, false, 20000)); err != nil {
		return domain.Concept{}, err
	}
	return s.store.CreateConcept(ctx, strings.TrimSpace(name), strings.TrimSpace(description))
}

func (s *ConceptService) DeleteConcept(ctx context.Context, ref string) error {
	if err := invalidInput(validation.Text("concept reference", ref, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteConcept(ctx, strings.TrimSpace(ref))
}

func (s *ConceptService) GetConcept(ctx context.Context, ref string) (domain.Concept, error) {
	return s.store.GetConcept(ctx, ref)
}

func (s *ConceptService) ListConcepts(ctx context.Context) ([]domain.Concept, error) {
	return s.store.ListConcepts(ctx)
}

func (s *ConceptService) CreateRelation(ctx context.Context, sourceType, sourceID, targetType, targetID, kind string) (domain.Relation, error) {
	for _, item := range []struct{ label, value string }{{"source type", sourceType}, {"source reference", sourceID}, {"target type", targetType}, {"target reference", targetID}, {"relation type", kind}} {
		if err := invalidInput(validation.Text(item.label, item.value, true, 256)); err != nil {
			return domain.Relation{}, err
		}
	}
	entityTypes := []string{"repository", "branch", "project", "issue", "pull_request", "concept", "note", "label", "milestone", "completion"}
	if err := invalidInput(validation.OneOf("source type", sourceType, entityTypes...)); err != nil {
		return domain.Relation{}, err
	}
	if err := invalidInput(validation.OneOf("target type", targetType, entityTypes...)); err != nil {
		return domain.Relation{}, err
	}
	relationKinds := []string{"related_to", "uses", "contains", "part_of", "explains", "contrasts_with", "uses_concept", "references", "supports", "contradicts", "implements", "implemented_by"}
	if err := invalidInput(validation.OneOf("relation type", kind, relationKinds...)); err != nil {
		return domain.Relation{}, err
	}
	return s.store.CreateRelation(ctx, strings.TrimSpace(sourceType), strings.TrimSpace(sourceID), strings.TrimSpace(targetType), strings.TrimSpace(targetID), strings.TrimSpace(kind))
}

func (s *ConceptService) LinkIssueConcept(ctx context.Context, issue, concept string) error {
	for _, item := range []struct{ name, value string }{{"issue reference", issue}, {"concept reference", concept}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.LinkIssueConcept(ctx, strings.TrimSpace(issue), strings.TrimSpace(concept))
}
