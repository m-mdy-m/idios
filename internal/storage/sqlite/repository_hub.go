package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"idios/internal/domain"
)

func hubConnectionFrom(row Row) domain.HubConnection {
	var scopes []string
	_ = json.Unmarshal([]byte(str(row, "scopes_json")), &scopes)
	if scopes == nil {
		scopes = []string{}
	}
	return domain.HubConnection{
		ID: str(row, "id"), Provider: domain.RepositoryProvider(str(row, "provider")),
		Host: str(row, "host"), AccountID: str(row, "account_id"),
		AccountName: str(row, "account_name"), DisplayName: str(row, "display_name"),
		Scopes: scopes, InstallationID: str(row, "installation_id"),
		Status: str(row, "status"), LastError: str(row, "last_error"),
		CredentialCiphertext: str(row, "credential_ciphertext"),
		ConnectedAt: parseTime(row["connected_at"]), UpdatedAt: parseTime(row["updated_at"]),
	}
}

func (s *Store) ListHubConnections(ctx context.Context) ([]domain.HubConnection, error) {
	rows, err := s.p().all(ctx, `SELECT id,provider,host,account_id,account_name,display_name,scopes_json,
		credential_ciphertext,installation_id,status,last_error,connected_at,updated_at
		FROM hub_connections ORDER BY provider,host,account_name`)
	if err != nil { return nil, err }
	return collect(rows, hubConnectionFrom), nil
}

func (s *Store) GetHubConnection(ctx context.Context, ref string) (domain.HubConnection, error) {
	row, err := s.p().must(ctx, "provider connection "+ref, `SELECT id,provider,host,account_id,account_name,display_name,scopes_json,
		credential_ciphertext,installation_id,status,last_error,connected_at,updated_at
		FROM hub_connections WHERE id=?`, strings.TrimSpace(ref))
	if err != nil { return domain.HubConnection{}, err }
	return hubConnectionFrom(row), nil
}

func (s *Store) SaveHubConnection(ctx context.Context, item domain.HubConnection) (domain.HubConnection, error) {
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" { item.ID = domain.NewID() }
	item.Host = strings.TrimSpace(item.Host)
	item.AccountID = strings.TrimSpace(item.AccountID)
	item.AccountName = strings.TrimSpace(item.AccountName)
	if item.Provider == "" || item.Host == "" || item.AccountID == "" || item.AccountName == "" || item.CredentialCiphertext == "" {
		return domain.HubConnection{}, fmt.Errorf("%w: provider, host, account identity, and encrypted credentials are required", domain.ErrInvalid)
	}
	if item.Status == "" { item.Status = "connected" }
	if item.ConnectedAt.IsZero() { item.ConnectedAt = domain.Now() }
	item.UpdatedAt = domain.Now()
	scopes, _ := json.Marshal(item.Scopes)
	_, err := s.p().changes(ctx, `INSERT INTO hub_connections(id,provider,host,account_id,account_name,display_name,scopes_json,
		credential_ciphertext,installation_id,status,last_error,connected_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(provider,host,account_id) DO UPDATE SET account_name=excluded.account_name,
		display_name=excluded.display_name,scopes_json=excluded.scopes_json,
		credential_ciphertext=excluded.credential_ciphertext,installation_id=excluded.installation_id,
		status='connected',last_error='',updated_at=excluded.updated_at`,
		item.ID, item.Provider, item.Host, item.AccountID, item.AccountName, item.DisplayName, string(scopes),
		item.CredentialCiphertext, item.InstallationID, item.Status, item.LastError, timeText(item.ConnectedAt), timeText(item.UpdatedAt))
	if err != nil { return domain.HubConnection{}, err }
	row, err := s.p().must(ctx, "provider connection", `SELECT id,provider,host,account_id,account_name,display_name,scopes_json,
		credential_ciphertext,installation_id,status,last_error,connected_at,updated_at
		FROM hub_connections WHERE provider=? AND host=? AND account_id=?`, item.Provider, item.Host, item.AccountID)
	if err != nil { return domain.HubConnection{}, err }
	return hubConnectionFrom(row), nil
}

func (s *Store) DeleteHubConnection(ctx context.Context, ref string) error {
	changed, err := s.p().changes(ctx, `DELETE FROM hub_connections WHERE id=?`, strings.TrimSpace(ref))
	if err != nil { return err }
	if changed == 0 { return fmt.Errorf("%w: provider connection %q", domain.ErrNotFound, ref) }
	return nil
}

func (s *Store) SaveHubOAuthState(ctx context.Context, item domain.HubOAuthState) error {
	if item.StateHash == "" || item.Provider == "" || item.RedirectURI == "" || item.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: OAuth state, provider, redirect URI, and expiry are required", domain.ErrInvalid)
	}
	return s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `DELETE FROM hub_oauth_states WHERE expires_at<=?`, nowText()); err != nil { return err }
		_, err := p.changes(ctx, `INSERT INTO hub_oauth_states(state_hash,provider,host,code_verifier,redirect_uri,created_at,expires_at)
			VALUES(?,?,?,?,?,?,?)`, item.StateHash, item.Provider, item.Host, item.CodeVerifier, item.RedirectURI, timeText(item.CreatedAt), timeText(item.ExpiresAt))
		return err
	})
}

func (s *Store) ConsumeHubOAuthState(ctx context.Context, stateHash string) (domain.HubOAuthState, error) {
	var item domain.HubOAuthState
	err := s.run(ctx, func(p provider) error {
		row, err := p.must(ctx, "OAuth state", `SELECT state_hash,provider,host,code_verifier,redirect_uri,created_at,expires_at
			FROM hub_oauth_states WHERE state_hash=? AND expires_at>?`, strings.TrimSpace(stateHash), nowText())
		if err != nil { return err }
		item = domain.HubOAuthState{
			StateHash: str(row, "state_hash"), Provider: domain.RepositoryProvider(str(row, "provider")),
			Host: str(row, "host"), CodeVerifier: str(row, "code_verifier"), RedirectURI: str(row, "redirect_uri"),
			CreatedAt: parseTime(row["created_at"]), ExpiresAt: parseTime(row["expires_at"]),
		}
		_, err = p.changes(ctx, `DELETE FROM hub_oauth_states WHERE state_hash=?`, stateHash)
		return err
	})
	return item, err
}

func hubRepositoryFrom(row Row) domain.HubRepository {
	var caps []string
	var collectionIDs []string
	_ = json.Unmarshal([]byte(str(row, "capabilities_json")), &caps)
	for _, raw := range strings.Split(str(row, "collection_ids"), ",") {
		if raw = strings.TrimSpace(raw); raw != "" { collectionIDs = append(collectionIDs, raw) }
	}
	if caps == nil { caps = []string{} }
	return domain.HubRepository{
		ID: str(row, "id"), ConnectionID: str(row, "connection_id"),
		Provider: domain.RepositoryProvider(str(row, "provider")), Host: str(row, "host"),
		ExternalID: str(row, "external_id"), Namespace: str(row, "namespace"), Name: str(row, "name"),
		FullPath: str(row, "full_path"), Description: str(row, "description"), WebURL: str(row, "web_url"),
		CloneURL: str(row, "clone_url"), CloneSSHURL: str(row, "clone_ssh_url"),
		DefaultBranch: str(row, "default_branch"), Language: str(row, "language"),
		Visibility: str(row, "visibility"), RepositoryType: str(row, "repository_type"),
		Permission: str(row, "permission"), Capabilities: caps,
		OpenIssueCount: int(i64(row["open_issue_count"])), OpenChangeRequestCount: int(i64(row["open_change_request_count"])),
		SyncState: str(row, "sync_state"), LastSyncedAt: ptrTime(row["last_synced_at"]),
		LastError: str(row, "last_error"), LinkedAt: parseTime(row["linked_at"]), UpdatedAt: parseTime(row["updated_at"]),
		LocalAlias: str(row, "local_alias"), Pinned: boolRow(row, "pinned"), CollectionIDs: collectionIDs,
	}
}

const hubRepositorySelect = `SELECT r.id,r.connection_id,r.provider,r.host,r.external_id,r.namespace,r.name,r.full_path,
	r.description,r.web_url,r.clone_url,r.clone_ssh_url,r.default_branch,r.language,r.visibility,r.repository_type,
	r.permission,r.capabilities_json,r.open_issue_count,r.open_change_request_count,r.sync_state,r.last_synced_at,
	r.last_error,r.linked_at,r.updated_at,COALESCE(p.local_alias,'') AS local_alias,COALESCE(p.pinned,0) AS pinned,
	COALESCE((SELECT group_concat(cr.collection_id, ',') FROM hub_collection_repositories cr WHERE cr.repository_id=r.id),'') AS collection_ids
	FROM hub_repositories r LEFT JOIN hub_repository_preferences p ON p.repository_id=r.id`

func (s *Store) GetHubRepository(ctx context.Context, ref string) (domain.HubRepository, error) {
	row, err := s.p().must(ctx, "Hub repository "+ref, hubRepositorySelect+` WHERE r.id=?`, strings.TrimSpace(ref))
	if err != nil { return domain.HubRepository{}, err }
	return hubRepositoryFrom(row), nil
}

func (s *Store) ListHubRepositories(ctx context.Context, providerName, search, collectionID string, pinnedOnly bool) ([]domain.HubRepository, error) {
	query := hubRepositorySelect + ` WHERE 1=1`
	args := []any{}
	if providerName = strings.TrimSpace(providerName); providerName != "" {
		query += ` AND r.provider=?`; args = append(args, providerName)
	}
	if search = strings.TrimSpace(search); search != "" {
		like := "%" + search + "%"
		query += ` AND (r.namespace LIKE ? OR r.name LIKE ? OR r.full_path LIKE ? OR r.description LIKE ? OR r.host LIKE ? OR r.language LIKE ?)`
		args = append(args, like, like, like, like, like, like)
	}
	if collectionID = strings.TrimSpace(collectionID); collectionID != "" {
		query += ` AND EXISTS(SELECT 1 FROM hub_collection_repositories cr WHERE cr.repository_id=r.id AND cr.collection_id=?)`
		args = append(args, collectionID)
	}
	if pinnedOnly { query += ` AND COALESCE(p.pinned,0)=1` }
	query += ` ORDER BY COALESCE(p.pinned,0) DESC,r.namespace COLLATE NOCASE,r.name COLLATE NOCASE`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil { return nil, err }
	return collect(rows, hubRepositoryFrom), nil
}

func (s *Store) UpsertHubRepository(ctx context.Context, item domain.HubRepository) (domain.HubRepository, error) {
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" { item.ID = domain.NewID() }
	if item.ConnectionID == "" || item.Provider == "" || item.Host == "" || item.ExternalID == "" || item.Name == "" || item.FullPath == "" || item.WebURL == "" {
		return domain.HubRepository{}, fmt.Errorf("%w: provider connection, stable ID, name, full path, and web URL are required", domain.ErrInvalid)
	}
	if item.RepositoryType == "" { item.RepositoryType = "git" }
	if item.SyncState == "" { item.SyncState = "never" }
	if item.LinkedAt.IsZero() { item.LinkedAt = domain.Now() }
	item.UpdatedAt = domain.Now()
	caps, _ := json.Marshal(item.Capabilities)
	_, err := s.p().changes(ctx, `INSERT INTO hub_repositories(id,connection_id,provider,host,external_id,namespace,name,full_path,
		description,web_url,clone_url,clone_ssh_url,default_branch,language,visibility,repository_type,permission,capabilities_json,
		open_issue_count,open_change_request_count,sync_state,last_synced_at,last_error,linked_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(provider,host,external_id) DO UPDATE SET connection_id=excluded.connection_id,
		namespace=excluded.namespace,name=excluded.name,full_path=excluded.full_path,description=excluded.description,
		web_url=excluded.web_url,clone_url=excluded.clone_url,clone_ssh_url=excluded.clone_ssh_url,
		default_branch=excluded.default_branch,language=excluded.language,visibility=excluded.visibility,
		repository_type=excluded.repository_type,permission=excluded.permission,capabilities_json=excluded.capabilities_json,
		open_issue_count=excluded.open_issue_count,open_change_request_count=excluded.open_change_request_count,
		sync_state=excluded.sync_state,last_synced_at=excluded.last_synced_at,last_error=excluded.last_error,updated_at=excluded.updated_at`,
		item.ID,item.ConnectionID,item.Provider,item.Host,item.ExternalID,item.Namespace,item.Name,item.FullPath,
		item.Description,item.WebURL,item.CloneURL,item.CloneSSHURL,item.DefaultBranch,item.Language,item.Visibility,item.RepositoryType,
		item.Permission,string(caps),item.OpenIssueCount,item.OpenChangeRequestCount,item.SyncState,
		nullableTime(item.LastSyncedAt),item.LastError,timeText(item.LinkedAt),timeText(item.UpdatedAt))
	if err != nil { return domain.HubRepository{}, err }
	row, err := s.p().must(ctx, "linked Hub repository", hubRepositorySelect+` WHERE r.provider=? AND r.host=? AND r.external_id=?`, item.Provider,item.Host,item.ExternalID)
	if err != nil { return domain.HubRepository{}, err }
	return hubRepositoryFrom(row), nil
}

func nullableTime(t *time.Time) any {
	if t == nil || t.IsZero() { return nil }
	return timeText(*t)
}

func (s *Store) UnlinkHubRepository(ctx context.Context, ref string) error {
	changed, err := s.p().changes(ctx, `DELETE FROM hub_repositories WHERE id=?`, strings.TrimSpace(ref))
	if err != nil { return err }
	if changed == 0 { return fmt.Errorf("%w: Hub repository %q", domain.ErrNotFound, ref) }
	return nil
}

func hubPreferencesFrom(row Row) domain.HubRepositoryPreferences {
	return domain.HubRepositoryPreferences{RepositoryID:str(row,"repository_id"),LocalAlias:str(row,"local_alias"),Pinned:boolRow(row,"pinned"),NoteMarkdown:str(row,"note_markdown"),UpdatedAt:parseTime(row["updated_at"])}
}

func (s *Store) SaveHubRepositoryPreferences(ctx context.Context, item domain.HubRepositoryPreferences) (domain.HubRepositoryPreferences, error) {
	if strings.TrimSpace(item.RepositoryID) == "" { return domain.HubRepositoryPreferences{}, fmt.Errorf("%w: Hub repository ID is required", domain.ErrInvalid) }
	item.LocalAlias = strings.TrimSpace(item.LocalAlias)
	item.UpdatedAt = domain.Now()
	_, err := s.p().changes(ctx, `INSERT INTO hub_repository_preferences(repository_id,local_alias,pinned,note_markdown,updated_at)
		VALUES(?,?,?,?,?) ON CONFLICT(repository_id) DO UPDATE SET local_alias=excluded.local_alias,pinned=excluded.pinned,
		note_markdown=excluded.note_markdown,updated_at=excluded.updated_at`,
		item.RepositoryID,item.LocalAlias,boolInt(item.Pinned),item.NoteMarkdown,timeText(item.UpdatedAt))
	if err != nil { return domain.HubRepositoryPreferences{}, err }
	row, err := s.p().must(ctx, "Hub repository preferences", `SELECT repository_id,local_alias,pinned,note_markdown,updated_at FROM hub_repository_preferences WHERE repository_id=?`, item.RepositoryID)
	if err != nil { return domain.HubRepositoryPreferences{}, err }
	return hubPreferencesFrom(row), nil
}

func (s *Store) GetHubRepositoryPreferences(ctx context.Context, repositoryID string) (domain.HubRepositoryPreferences, error) {
	row, ok, err := s.p().row(ctx, `SELECT repository_id,local_alias,pinned,note_markdown,updated_at FROM hub_repository_preferences WHERE repository_id=?`, repositoryID)
	if err != nil { return domain.HubRepositoryPreferences{}, err }
	if !ok { return domain.HubRepositoryPreferences{RepositoryID:repositoryID}, nil }
	return hubPreferencesFrom(row), nil
}

func hubCollectionFrom(row Row) domain.HubCollection {
	return domain.HubCollection{ID:str(row,"id"),Name:str(row,"name"),Description:str(row,"description"),Position:int(i64(row["position"])),RepositoryCount:int(i64(row["repository_count"])),CreatedAt:parseTime(row["created_at"]),UpdatedAt:parseTime(row["updated_at"])}
}

func (s *Store) ListHubCollections(ctx context.Context) ([]domain.HubCollection, error) {
	rows, err := s.p().all(ctx, `SELECT c.id,c.name,c.description,c.position,c.created_at,c.updated_at,
		(SELECT COUNT(*) FROM hub_collection_repositories cr WHERE cr.collection_id=c.id) AS repository_count
		FROM hub_collections c ORDER BY c.position,c.name COLLATE NOCASE`)
	if err != nil { return nil, err }
	return collect(rows, hubCollectionFrom), nil
}

func (s *Store) SaveHubCollection(ctx context.Context, item domain.HubCollection) (domain.HubCollection, error) {
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" || len(item.Name) > 80 { return domain.HubCollection{}, fmt.Errorf("%w: collection name is required and must be at most 80 characters", domain.ErrInvalid) }
	now := domain.Now()
	if item.ID == "" {
		item.ID = domain.NewID()
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	_, err := s.p().changes(ctx, `INSERT INTO hub_collections(id,name,description,position,created_at,updated_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description,position=excluded.position,updated_at=excluded.updated_at`,
		item.ID,item.Name,strings.TrimSpace(item.Description),item.Position,timeText(item.CreatedAt),timeText(item.UpdatedAt))
	if err != nil { return domain.HubCollection{}, err }
	rows, err := s.ListHubCollections(ctx)
	if err != nil { return domain.HubCollection{}, err }
	for _, collection := range rows { if collection.ID == item.ID { return collection, nil } }
	return domain.HubCollection{}, fmt.Errorf("saved collection could not be reloaded")
}

func (s *Store) DeleteHubCollection(ctx context.Context, ref string) error {
	changed, err := s.p().changes(ctx, `DELETE FROM hub_collections WHERE id=?`, ref)
	if err != nil { return err }
	if changed == 0 { return fmt.Errorf("%w: Hub collection %q", domain.ErrNotFound, ref) }
	return nil
}

func (s *Store) AddHubRepositoryToCollection(ctx context.Context, collectionID, repositoryID string) error {
	_, err := s.p().changes(ctx, `INSERT INTO hub_collection_repositories(collection_id,repository_id,created_at) VALUES(?,?,?)
		ON CONFLICT(collection_id,repository_id) DO NOTHING`,collectionID,repositoryID,nowText())
	return err
}

func (s *Store) RemoveHubRepositoryFromCollection(ctx context.Context, collectionID, repositoryID string) error {
	_, err := s.p().changes(ctx, `DELETE FROM hub_collection_repositories WHERE collection_id=? AND repository_id=?`,collectionID,repositoryID)
	return err
}

func hubRelationFrom(row Row) domain.HubRepositoryRelation {
	return domain.HubRepositoryRelation{ID:str(row,"id"),SourceRepositoryID:str(row,"source_repository_id"),TargetRepositoryID:str(row,"target_repository_id"),
		RelationType:str(row,"relation_type"),Description:str(row,"description"),TargetName:str(row,"target_name"),
		TargetFullPath:str(row,"target_full_path"),TargetProvider:domain.RepositoryProvider(str(row,"target_provider")),CreatedAt:parseTime(row["created_at"])}
}

func (s *Store) ListHubRepositoryRelations(ctx context.Context, repositoryID string) ([]domain.HubRepositoryRelation, error) {
	rows, err := s.p().all(ctx, `SELECT rel.id,rel.source_repository_id,rel.target_repository_id,rel.relation_type,rel.description,rel.created_at,
		target.name AS target_name,target.full_path AS target_full_path,target.provider AS target_provider
		FROM hub_repository_relations rel JOIN hub_repositories target ON target.id=rel.target_repository_id
		WHERE rel.source_repository_id=? ORDER BY rel.relation_type,target.full_path COLLATE NOCASE`,repositoryID)
	if err != nil { return nil, err }
	return collect(rows, hubRelationFrom), nil
}

func (s *Store) CreateHubRepositoryRelation(ctx context.Context, item domain.HubRepositoryRelation) (domain.HubRepositoryRelation, error) {
	if item.ID == "" { item.ID = domain.NewID() }
	if item.SourceRepositoryID == "" || item.TargetRepositoryID == "" || item.SourceRepositoryID == item.TargetRepositoryID {
		return domain.HubRepositoryRelation{}, fmt.Errorf("%w: two different linked repositories are required", domain.ErrInvalid)
	}
	if item.RelationType != "depends_on" && item.RelationType != "related_to" && item.RelationType != "implements" && item.RelationType != "documentation_for" {
		return domain.HubRepositoryRelation{}, fmt.Errorf("%w: unsupported repository relation type", domain.ErrInvalid)
	}
	item.CreatedAt = domain.Now()
	_, err := s.p().changes(ctx, `INSERT INTO hub_repository_relations(id,source_repository_id,target_repository_id,relation_type,description,created_at) VALUES(?,?,?,?,?,?)`,
		item.ID,item.SourceRepositoryID,item.TargetRepositoryID,item.RelationType,strings.TrimSpace(item.Description),timeText(item.CreatedAt))
	if err != nil { return domain.HubRepositoryRelation{}, err }
	rows, err := s.ListHubRepositoryRelations(ctx,item.SourceRepositoryID)
	if err != nil { return domain.HubRepositoryRelation{}, err }
	for _, relation := range rows { if relation.ID == item.ID { return relation,nil } }
	return domain.HubRepositoryRelation{}, fmt.Errorf("created repository relation could not be reloaded")
}

func (s *Store) DeleteHubRepositoryRelation(ctx context.Context, ref string) error {
	changed, err := s.p().changes(ctx, `DELETE FROM hub_repository_relations WHERE id=?`,ref)
	if err != nil { return err }
	if changed == 0 { return fmt.Errorf("%w: repository relation %q",domain.ErrNotFound,ref) }
	return nil
}
