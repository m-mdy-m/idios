package domain

import "time"

type RepositoryProvider string

const (
	RepositoryProviderGitHub      RepositoryProvider = "github"
	RepositoryProviderGitLab      RepositoryProvider = "gitlab"
	RepositoryProviderSourceForge RepositoryProvider = "sourceforge"
)

// HubConnection is a provider account or installation. CredentialCiphertext is only used by
// the server-side store and is deliberately omitted from every API response.
type HubConnection struct {
	ID                  string            `json:"id"`
	Provider            RepositoryProvider `json:"provider"`
	Host                string            `json:"host"`
	AccountID           string            `json:"account_id"`
	AccountName         string            `json:"account_name"`
	DisplayName         string            `json:"display_name"`
	Scopes              []string          `json:"scopes"`
	InstallationID      string            `json:"installation_id,omitempty"`
	Status              string            `json:"status"`
	LastError           string            `json:"last_error,omitempty"`
	ConnectedAt         time.Time         `json:"connected_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	CredentialCiphertext string            `json:"-"`
}

// HubRepository is a deliberately explicit link between one provider's stable repository
// identity and the IDIOS workspace. It is never confused with a local simulated Repository.
type HubRepository struct {
	ID                       string             `json:"id"`
	ConnectionID             string             `json:"connection_id"`
	Provider                 RepositoryProvider `json:"provider"`
	Host                     string             `json:"host"`
	ExternalID               string             `json:"external_id"`
	Namespace                string             `json:"namespace"`
	Name                     string             `json:"name"`
	FullPath                 string             `json:"full_path"`
	Description              string             `json:"description"`
	WebURL                   string             `json:"web_url"`
	CloneURL                 string             `json:"clone_url,omitempty"`
	CloneSSHURL              string             `json:"clone_ssh_url,omitempty"`
	DefaultBranch            string             `json:"default_branch,omitempty"`
	Language                 string             `json:"language,omitempty"`
	Visibility               string             `json:"visibility,omitempty"`
	RepositoryType           string             `json:"repository_type"`
	Permission               string             `json:"permission,omitempty"`
	Capabilities             []string           `json:"capabilities"`
	OpenIssueCount           int                `json:"open_issue_count"`
	OpenChangeRequestCount   int                `json:"open_change_request_count"`
	SyncState                string             `json:"sync_state"`
	LastSyncedAt             *time.Time         `json:"last_synced_at,omitempty"`
	LastError                string             `json:"last_error,omitempty"`
	LinkedAt                 time.Time          `json:"linked_at"`
	UpdatedAt                time.Time          `json:"updated_at"`
	LocalAlias               string             `json:"local_alias,omitempty"`
	Pinned                   bool               `json:"pinned"`
	CollectionIDs            []string           `json:"collection_ids,omitempty"`
}

type HubRepositoryPreferences struct {
	RepositoryID string    `json:"repository_id"`
	LocalAlias   string    `json:"local_alias,omitempty"`
	Pinned       bool      `json:"pinned"`
	NoteMarkdown string    `json:"note_markdown,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type HubCollection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Position    int       `json:"position"`
	RepositoryCount int   `json:"repository_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type HubRepositoryRelation struct {
	ID                  string    `json:"id"`
	SourceRepositoryID  string    `json:"source_repository_id"`
	TargetRepositoryID  string    `json:"target_repository_id"`
	RelationType        string    `json:"relation_type"`
	Description         string    `json:"description,omitempty"`
	TargetName          string    `json:"target_name,omitempty"`
	TargetFullPath      string    `json:"target_full_path,omitempty"`
	TargetProvider      RepositoryProvider `json:"target_provider,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

// HubOAuthState is single-use and short-lived. StateHash is the SHA-256 digest of the random
// value sent to the provider, never the bearer state itself.
type HubOAuthState struct {
	StateHash    string
	Provider     RepositoryProvider
	Host         string
	CodeVerifier string
	RedirectURI  string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}
