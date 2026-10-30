package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type NotesService struct{ store ports.NotesStore }

func (s *NotesService) ListNotes(ctx context.Context) ([]domain.Note, error) {
	return s.store.ListNotes(ctx)
}

func (s *NotesService) ListNoteLabels(ctx context.Context) ([]domain.NoteLabel, error) {
	return s.store.ListNoteLabels(ctx)
}

func (s *NotesService) SetNoteLabelColor(ctx context.Context, name, color string) error {
	if err := invalidInput(validation.Text("note label name", name, true, 100)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("note label color", color, true, 16)); err != nil {
		return err
	}
	if err := invalidInput(validation.HexColor("note label color", color)); err != nil {
		return err
	}
	return s.store.SetNoteLabelColor(ctx, strings.TrimSpace(name), strings.TrimSpace(color))
}

func (s *NotesService) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	if err := invalidInput(validation.Text("note title", title, true, 200)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note content", content, false, 100000)); err != nil {
		return domain.Note{}, err
	}
	return s.store.CreateNote(ctx, strings.TrimSpace(title), content)
}

func (s *NotesService) CreateNoteWithLabels(ctx context.Context, title, content string, labels []string) (domain.Note, error) {
	if err := invalidInput(validation.Text("note title", title, true, 200)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note content", content, false, 100000)); err != nil {
		return domain.Note{}, err
	}
	if len(labels) > 50 {
		return domain.Note{}, fmt.Errorf("%w: a note can have at most 50 labels", domain.ErrInvalid)
	}
	for _, label := range labels {
		if err := invalidInput(validation.Text("note label", label, false, 80)); err != nil {
			return domain.Note{}, err
		}
	}
	return s.store.CreateNoteWithLabels(ctx, strings.TrimSpace(title), content, labels)
}

func (s *NotesService) UpdateNote(ctx context.Context, id, title, content string) (domain.Note, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(title) == "" {
		return domain.Note{}, fmt.Errorf("%w: note ID and title are required", domain.ErrInvalid)
	}
	return s.store.UpdateNote(ctx, id, title, content)
}

func (s *NotesService) UpdateNoteWithLabels(ctx context.Context, id, title, content string, labels []string) (domain.Note, error) {
	if err := invalidInput(validation.Text("note reference", id, true, 256)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note title", title, true, 200)); err != nil {
		return domain.Note{}, err
	}
	if err := invalidInput(validation.Text("note content", content, false, 100000)); err != nil {
		return domain.Note{}, err
	}
	if len(labels) > 50 {
		return domain.Note{}, fmt.Errorf("%w: a note can have at most 50 labels", domain.ErrInvalid)
	}
	for _, label := range labels {
		if err := invalidInput(validation.Text("note label", label, false, 80)); err != nil {
			return domain.Note{}, err
		}
	}
	return s.store.UpdateNoteWithLabels(ctx, strings.TrimSpace(id), strings.TrimSpace(title), content, labels)
}

func (s *NotesService) DeleteNote(ctx context.Context, id string) error {
	if err := invalidInput(validation.Text("note reference", id, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteNote(ctx, strings.TrimSpace(id))
}

func (s *NotesService) LinkNote(ctx context.Context, id, entityType, entityRef string, graphVisible bool) error {
	for _, item := range []struct{ name, value string }{{"note reference", id}, {"target entity type", entityType}, {"target entity reference", entityRef}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	if err := invalidInput(validation.OneOf("target entity type", entityType, "issue", "repository", "pull_request", "concept", "project", "label", "milestone", "branch", "completion", "shelf_book")); err != nil {
		return err
	}
	return s.store.LinkNote(ctx, strings.TrimSpace(id), strings.TrimSpace(entityType), strings.TrimSpace(entityRef), graphVisible)
}
