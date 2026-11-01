package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"idios/internal/domain"
)

func TestRepositoryHubLocalDataIsSeparateAndCascadesOnUnlink(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	connection, err := store.SaveHubConnection(ctx, domain.HubConnection{
		Provider: domain.RepositoryProviderGitLab, Host: "https://gitlab.example.test",
		AccountID: "user-17", AccountName: "dev", CredentialCiphertext: "v1:encrypted",
	})
	if err != nil {
		t.Fatal(err)
	}

	repo, err := store.UpsertHubRepository(ctx, domain.HubRepository{
		ConnectionID: connection.ID, Provider: connection.Provider, Host: connection.Host,
		ExternalID: "group/project:42", Namespace: "group", Name: "project", FullPath: "group/project",
		WebURL: "https://gitlab.example.test/group/project", RepositoryType: "git",
		Capabilities: []string{"code.read", "issues.read"}, SyncState: "ok",
	})
	if err != nil {
		t.Fatal(err)
	}

	preferences, err := store.SaveHubRepositoryPreferences(ctx, domain.HubRepositoryPreferences{
		RepositoryID: repo.ID, LocalAlias: "work copy",
		PrivateNote: "Keep this repository's deployment checklist in IDIOS only.",
		Pinned:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !preferences.Pinned || preferences.LocalAlias != "work copy" ||
		preferences.PrivateNote != "Keep this repository's deployment checklist in IDIOS only." {
		t.Fatalf("unexpected preferences: %#v", preferences)
	}

	first, err := store.SaveHubCollection(ctx, domain.HubCollection{Name: "Backend"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.SaveHubCollection(ctx, domain.HubCollection{Name: "Infrastructure"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddHubRepositoryToCollection(ctx, first.ID, repo.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.AddHubRepositoryToCollection(ctx, second.ID, repo.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.AddHubRepositoryToCollection(ctx, second.ID, repo.ID); err != nil {
		t.Fatal(err)
	}

	items, err := store.ListHubRepositories(ctx, "", "group/pro", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(items[0].CollectionIDs) != 2 || !items[0].Pinned {
		t.Fatalf("repository search/pin/collections mismatch: %#v", items)
	}

	relation, err := store.UpsertHubRepository(ctx, domain.HubRepository{
		ConnectionID: connection.ID, Provider: connection.Provider, Host: connection.Host,
		ExternalID: "group/docs:43", Namespace: "group", Name: "docs", FullPath: "group/docs",
		WebURL: "https://gitlab.example.test/group/docs", RepositoryType: "git",
	})
	if err != nil {
		t.Fatal(err)
	}
	createdRelation, err := store.CreateHubRepositoryRelation(ctx, domain.HubRepositoryRelation{
		SourceRepositoryID: repo.ID, TargetRepositoryID: relation.ID, RelationType: "documentation_for",
	})
	if err != nil {
		t.Fatal(err)
	}
	relations, err := store.ListHubRepositoryRelations(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].ID != createdRelation.ID || relations[0].TargetFullPath != "group/docs" {
		t.Fatalf("unexpected repository relations: %#v", relations)
	}

	// Unlinking removes only the selected Hub association and its local metadata. It does not
	// revoke the provider account connection or change a provider-side object.
	if err := store.UnlinkHubRepository(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetHubRepository(ctx, repo.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected linked repository to be removed, got %v", err)
	}
	if _, err := store.GetHubRepositoryPreferences(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	if items, err := store.ListHubRepositories(ctx, "", "", "", false); err != nil || len(items) != 1 {
		t.Fatalf("expected only the second linked repository, items=%#v err=%v", items, err)
	}
	if _, err := store.GetHubConnection(ctx, connection.ID); err != nil {
		t.Fatalf("unlink revoked the connection: %v", err)
	}
}

func TestRepositoryHubOAuthStateIsSingleUseAndExpires(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	item := domain.HubOAuthState{
		StateHash: "state-hash", Provider: domain.RepositoryProviderGitLab, Host: "https://gitlab.com",
		CodeVerifier: "verifier", RedirectURI: "http://127.0.0.1/callback",
		CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	if err := store.SaveHubOAuthState(ctx, item); err != nil {
		t.Fatal(err)
	}
	consumed, err := store.ConsumeHubOAuthState(ctx, item.StateHash)
	if err != nil {
		t.Fatal(err)
	}
	if consumed.CodeVerifier != item.CodeVerifier || consumed.Provider != item.Provider {
		t.Fatalf("unexpected consumed state: %#v", consumed)
	}
	if _, err := store.ConsumeHubOAuthState(ctx, item.StateHash); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("OAuth state should be single-use, got %v", err)
	}
	if err := store.SaveHubOAuthState(ctx, domain.HubOAuthState{
		StateHash: "expired", Provider: domain.RepositoryProviderGitLab, Host: item.Host,
		RedirectURI: item.RedirectURI, CreatedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeHubOAuthState(ctx, "expired"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired OAuth state must not be consumable, got %v", err)
	}
}

func TestHubRepositoryProjectLinksAreLocalAndMaintainOnePrimary(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	connection, err := store.SaveHubConnection(ctx, domain.HubConnection{
		Provider: domain.RepositoryProviderGitLab, Host: "https://gitlab.example.test",
		AccountID: "developer-42", AccountName: "developer", CredentialCiphertext: "v1:encrypted",
	})
	if err != nil {
		t.Fatal(err)
	}
	makeRepository := func(name, externalID string) domain.HubRepository {
		t.Helper()
		item, err := store.UpsertHubRepository(ctx, domain.HubRepository{
			ConnectionID: connection.ID, Provider: connection.Provider, Host: connection.Host,
			ExternalID: externalID, Namespace: "group", Name: name, FullPath: "group/" + name,
			WebURL: "https://gitlab.example.test/group/" + name, RepositoryType: "git",
			Capabilities: []string{"code.read"}, SyncState: "ok",
		})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	first := makeRepository("api", "remote-101")
	second := makeRepository("worker", "remote-102")
	project, err := store.CreateProject(ctx, "Platform", "Local IDIOS project")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.LinkHubRepositoryToProject(ctx, project.ID, first.ID, "primary implementation", false); err != nil {
		t.Fatal(err)
	}
	if err := store.LinkHubRepositoryToProject(ctx, project.ID, second.ID, "background worker", false); err != nil {
		t.Fatal(err)
	}
	projectLinks, err := store.ListHubProjectRepositories(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectLinks) != 2 || projectLinks[0].RepositoryID != first.ID || !projectLinks[0].IsPrimary ||
		projectLinks[1].RepositoryID != second.ID || projectLinks[1].IsPrimary {
		t.Fatalf("first association must be primary and the second secondary: %#v", projectLinks)
	}
	firstLinks, err := store.ListHubProjects(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstLinks) != 1 || firstLinks[0].ProjectID != project.ID || firstLinks[0].ProjectName != "Platform" ||
		firstLinks[0].FullPath != "group/api" || firstLinks[0].Provider != domain.RepositoryProviderGitLab {
		t.Fatalf("unexpected Hub repository Project details: %#v", firstLinks)
	}

	if err := store.SetPrimaryHubProjectRepository(ctx, project.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	projectLinks, err = store.ListHubProjectRepositories(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectLinks) != 2 || projectLinks[0].RepositoryID != second.ID || !projectLinks[0].IsPrimary ||
		projectLinks[1].RepositoryID != first.ID || projectLinks[1].IsPrimary {
		t.Fatalf("primary selection did not move to the second Hub repository: %#v", projectLinks)
	}

	if err := store.UnlinkHubRepositoryFromProject(ctx, project.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	projectLinks, err = store.ListHubProjectRepositories(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectLinks) != 1 || projectLinks[0].RepositoryID != first.ID || !projectLinks[0].IsPrimary {
		t.Fatalf("unlinking the primary did not promote the remaining Hub repository: %#v", projectLinks)
	}
	if err := store.UnlinkHubRepositoryFromProject(ctx, project.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	projectLinks, err = store.ListHubProjectRepositories(ctx, project.ID)
	if err != nil || len(projectLinks) != 0 {
		t.Fatalf("Project association remained after unlink: %#v err=%v", projectLinks, err)
	}
	if _, err := store.GetHubRepository(ctx, first.ID); err != nil {
		t.Fatalf("unlinking the Project association deleted the remote Hub repository: %v", err)
	}
	if _, err := store.GetHubRepository(ctx, second.ID); err != nil {
		t.Fatalf("unlinking the Project association deleted the second Hub repository: %v", err)
	}
	if _, err := store.GetHubConnection(ctx, connection.ID); err != nil {
		t.Fatalf("Project association changed the provider connection: %v", err)
	}
	if err := store.SetPrimaryHubProjectRepository(ctx, project.ID, first.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("making an unlinked Hub repository primary should fail, got %v", err)
	}
}
