package ports

import (
	"context"
	"idios/internal/domain"
)

// RepositoryHubStore persists IDIOS-local provider links and organization metadata. It never
// performs network calls and never writes to a provider.
type RepositoryHubStore interface {
	ListHubConnections(context.Context) ([]domain.HubConnection, error)
	GetHubConnection(context.Context, string) (domain.HubConnection, error)
	SaveHubConnection(context.Context, domain.HubConnection) (domain.HubConnection, error)
	DeleteHubConnection(context.Context, string) error
	SaveHubOAuthState(context.Context, domain.HubOAuthState) error
	ConsumeHubOAuthState(context.Context, string) (domain.HubOAuthState, error)

	ListHubRepositories(context.Context, string, string, string, bool) ([]domain.HubRepository, error)
	GetHubRepository(context.Context, string) (domain.HubRepository, error)
	UpsertHubRepository(context.Context, domain.HubRepository) (domain.HubRepository, error)
	UnlinkHubRepository(context.Context, string) error
	SaveHubRepositoryPreferences(context.Context, domain.HubRepositoryPreferences) (domain.HubRepositoryPreferences, error)
	GetHubRepositoryPreferences(context.Context, string) (domain.HubRepositoryPreferences, error)

	ListHubCollections(context.Context) ([]domain.HubCollection, error)
	SaveHubCollection(context.Context, domain.HubCollection) (domain.HubCollection, error)
	DeleteHubCollection(context.Context, string) error
	AddHubRepositoryToCollection(context.Context, string, string) error
	RemoveHubRepositoryFromCollection(context.Context, string, string) error

	ListHubRepositoryRelations(context.Context, string) ([]domain.HubRepositoryRelation, error)
	CreateHubRepositoryRelation(context.Context, domain.HubRepositoryRelation) (domain.HubRepositoryRelation, error)
	DeleteHubRepositoryRelation(context.Context, string) error
}
