package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/ports"
)

type Service struct {
	store ports.CoreStore
	*RepositoryService
	*ProjectService
	*IssueService
	*PullRequestService
	*ConceptService
	*PlanningMetadataService
	*GraphService
	*ActivityService
	*ImportService
	*DemoService
	*NotesService
	*QuotesService
	*ProfileService
	*JobsService
	*NotificationsService
	*SettingsService
	*PlanService
	*ShelfService
}

// New composes small domain services behind one stable application facade.
// The default deployment stays a single local process; each service has its own port contract
// so it can be tested in isolation or hosted behind a separate adapter when process isolation is warranted.
func New(store ports.Store) *Service {
	repositories := &RepositoryService{store: store}
	issues := &IssueService{store: store}
	notes := &NotesService{store: store}
	shelf := &ShelfService{store: store, repositories: repositories, notes: notes}
	return &Service{
		store:                   store,
		RepositoryService:       repositories,
		ProjectService:          &ProjectService{store: store, issues: issues},
		IssueService:            issues,
		PullRequestService:      &PullRequestService{store: store},
		ConceptService:          &ConceptService{store: store},
		PlanningMetadataService: &PlanningMetadataService{store: store},
		GraphService:            &GraphService{store: store},
		ActivityService:         &ActivityService{store: store},
		ImportService:           &ImportService{store: store},
		DemoService:             &DemoService{store: store},
		NotesService:            notes,
		QuotesService:           &QuotesService{store: store},
		ProfileService:          &ProfileService{store: store},
		JobsService:             &JobsService{store: store},
		NotificationsService:    &NotificationsService{store: store},
		SettingsService:         &SettingsService{store: store},
		PlanService:             &PlanService{store: store},
		ShelfService:            shelf,
	}
}

func (s *Service) Close() error                   { return s.store.Close() }
func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func invalidInput(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
}
