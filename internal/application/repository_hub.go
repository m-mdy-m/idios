package application

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/repositoryhub"
	"idios/internal/validation"
)

type providerCredentialEnvelope struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

type RepositoryHubService struct {
	store     ports.RepositoryHubStore
	key       []byte
	keyErr    error
	refreshMu sync.Mutex
}

func NewRepositoryHubService(store ports.RepositoryHubStore) *RepositoryHubService {
	key, err := providerEncryptionKey()
	return &RepositoryHubService{store: store, key: key, keyErr: err}
}

func providerEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("IDIOS_PROVIDER_ENCRYPTION_KEY"))
	if raw == "" {
		return nil, fmt.Errorf("provider credential encryption is not configured; set IDIOS_PROVIDER_ENCRYPTION_KEY to a base64-encoded 32-byte key")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(raw)
	}
	if err != nil {
		decoded, err = hex.DecodeString(raw)
	}
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("IDIOS_PROVIDER_ENCRYPTION_KEY must decode to exactly 32 bytes (base64 or hex)")
	}
	return decoded, nil
}

func (s *RepositoryHubService) CredentialStorageReady() bool {
	return s.keyErr == nil && len(s.key) == 32
}
func (s *RepositoryHubService) CredentialStorageError() string {
	if s.keyErr == nil {
		return ""
	}
	return s.keyErr.Error()
}

func credentialAAD(provider domain.RepositoryProvider, host, accountID string) []byte {
	return []byte(string(provider) + "\x00" + strings.ToLower(host) + "\x00" + accountID)
}

func sealProviderCredential(plain, key, aad []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("credential encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, plain, aad)
	payload := append(nonce, sealed...)
	return "v1:" + base64.RawStdEncoding.EncodeToString(payload), nil
}

func openProviderCredential(encoded string, key, aad []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("credential encryption key must be 32 bytes")
	}
	version, payload, ok := strings.Cut(encoded, ":")
	if !ok || version != "v1" {
		return nil, fmt.Errorf("unsupported encrypted credential format")
	}
	raw, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted credential: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return nil, fmt.Errorf("encrypted credential is truncated")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt provider credential: authentication failed")
	}
	return plain, nil
}

func normalizeHubHost(provider domain.RepositoryProvider, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	switch provider {
	case domain.RepositoryProviderGitHub:
		if raw == "" || raw == "https://github.com" || raw == "github.com" {
			return "https://github.com", nil
		}
		return "", fmt.Errorf("%w: GitHub.com is the supported GitHub App host", domain.ErrInvalid)
	case domain.RepositoryProviderSourceForge:
		if raw == "" || raw == "https://sourceforge.net" || raw == "sourceforge.net" {
			return "https://sourceforge.net", nil
		}
		return "", fmt.Errorf("%w: SourceForge Allura is hosted at sourceforge.net", domain.ErrInvalid)
	case domain.RepositoryProviderGitLab:
		if raw == "" {
			return "", fmt.Errorf("%w: GitLab host is required", domain.ErrInvalid)
		}
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil ||
			parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return "", fmt.Errorf("%w: GitLab host must be an origin URL without credentials, path, query, or fragment", domain.ErrInvalid)
		}
		if parsed.Scheme != "https" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1" {
			return "", fmt.Errorf("%w: self-managed GitLab must use HTTPS", domain.ErrInvalid)
		}
		if !repositoryhub.GitLabHostAllowed(parsed.String()) {
			return "", fmt.Errorf("%w: GitLab.com is allowed by default; self-managed origins must be listed in IDIOS_GITLAB_ALLOWED_HOSTS", domain.ErrInvalid)
		}
		return strings.TrimSuffix(parsed.String(), "/"), nil
	default:
		return "", fmt.Errorf("%w: unsupported repository provider", domain.ErrInvalid)
	}
}

func (s *RepositoryHubService) ListConnections(ctx context.Context) ([]domain.HubConnection, error) {
	return s.store.ListHubConnections(ctx)
}

func (s *RepositoryHubService) SaveConnection(ctx context.Context, provider domain.RepositoryProvider, host, accountID, accountName, displayName, installationID, credential string, scopes []string) (domain.HubConnection, error) {
	if s.keyErr != nil {
		return domain.HubConnection{}, fmt.Errorf("%w: %v", domain.ErrUnavailable, s.keyErr)
	}
	accountID, accountName = strings.TrimSpace(accountID), strings.TrimSpace(accountName)
	if accountID == "" || accountName == "" {
		return domain.HubConnection{}, fmt.Errorf("%w: provider account ID and account name are required", domain.ErrInvalid)
	}
	if strings.TrimSpace(credential) == "" {
		return domain.HubConnection{}, fmt.Errorf("%w: provider credential is required", domain.ErrInvalid)
	}
	host, err := normalizeHubHost(provider, host)
	if err != nil {
		return domain.HubConnection{}, err
	}
	ciphertext, err := sealProviderCredential([]byte(credential), s.key, credentialAAD(provider, host, accountID))
	if err != nil {
		return domain.HubConnection{}, fmt.Errorf("encrypt provider credential: %w", err)
	}
	return s.store.SaveHubConnection(ctx, domain.HubConnection{
		Provider: provider, Host: host, AccountID: accountID, AccountName: accountName,
		DisplayName: strings.TrimSpace(displayName), InstallationID: strings.TrimSpace(installationID),
		Scopes: normalizeHubScopes(scopes), Status: "connected", CredentialCiphertext: ciphertext,
	})
}

func normalizeHubScopes(scopes []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(scopes))
	for _, item := range scopes {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func (s *RepositoryHubService) ProviderCredential(ctx context.Context, connectionID string) ([]byte, domain.HubConnection, error) {
	if s.keyErr != nil {
		return nil, domain.HubConnection{}, fmt.Errorf("%w: %v", domain.ErrUnavailable, s.keyErr)
	}
	item, err := s.store.GetHubConnection(ctx, connectionID)
	if err != nil {
		return nil, domain.HubConnection{}, err
	}
	plain, err := openProviderCredential(item.CredentialCiphertext, s.key, credentialAAD(item.Provider, item.Host, item.AccountID))
	if err != nil {
		return nil, domain.HubConnection{}, fmt.Errorf("%w: provider credential cannot be decrypted; reconnect the provider", domain.ErrUnavailable)
	}
	var envelope providerCredentialEnvelope
	if json.Unmarshal(plain, &envelope) != nil || envelope.AccessToken == "" {
		// Personal access tokens remain encrypted at rest but are not automatically refreshable.
		return plain, item, nil
	}
	zeroBytes(plain)
	if envelope.ExpiresAt.IsZero() || time.Until(envelope.ExpiresAt) > 60*time.Second {
		return []byte(envelope.AccessToken), item, nil
	}

	// OAuth providers can rotate refresh tokens. Serialize the refresh operation and re-read
	// the stored envelope after taking the lock so concurrent requests do not reuse a stale token.
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	item, err = s.store.GetHubConnection(ctx, connectionID)
	if err != nil {
		return nil, domain.HubConnection{}, err
	}
	plain, err = openProviderCredential(item.CredentialCiphertext, s.key, credentialAAD(item.Provider, item.Host, item.AccountID))
	if err != nil {
		return nil, item, fmt.Errorf("%w: provider credential cannot be decrypted; reconnect the provider", domain.ErrUnavailable)
	}
	if json.Unmarshal(plain, &envelope) != nil || envelope.AccessToken == "" {
		return plain, item, nil
	}
	zeroBytes(plain)
	if envelope.ExpiresAt.IsZero() || time.Until(envelope.ExpiresAt) > 60*time.Second {
		return []byte(envelope.AccessToken), item, nil
	}
	if envelope.RefreshToken == "" {
		return nil, item, fmt.Errorf("%w: provider OAuth access token expired; reconnect the account", domain.ErrUnavailable)
	}
	clientID, clientSecret := providerOAuthCredentials(item.Provider, item.Host)
	if clientID == "" {
		return nil, item, fmt.Errorf("%w: provider OAuth client is not configured for token refresh", domain.ErrUnavailable)
	}
	client, err := repositoryhub.New(item.Provider, item.Host)
	if err != nil {
		return nil, item, err
	}
	refreshed, err := client.RefreshAuthorizationToken(ctx, envelope.RefreshToken, clientID, clientSecret)
	if err != nil {
		return nil, item, err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = envelope.RefreshToken
	}
	expiresAt := time.Time{}
	if refreshed.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(refreshed.ExpiresIn) * time.Second)
	}
	rotated := providerCredentialEnvelope{
		AccessToken: refreshed.AccessToken, RefreshToken: refreshed.RefreshToken,
		TokenType: refreshed.TokenType, ExpiresAt: expiresAt,
	}
	serialized, err := json.Marshal(rotated)
	if err != nil {
		return nil, item, err
	}
	ciphertext, err := sealProviderCredential(serialized, s.key, credentialAAD(item.Provider, item.Host, item.AccountID))
	zeroBytes(serialized)
	if err != nil {
		return nil, item, err
	}
	item.CredentialCiphertext = ciphertext
	item.Status = "connected"
	item.LastError = ""
	saved, err := s.store.SaveHubConnection(ctx, item)
	if err != nil {
		return nil, item, fmt.Errorf("%w: refreshed provider token could not be persisted", domain.ErrUnavailable)
	}
	return []byte(rotated.AccessToken), saved, nil
}

func (s *RepositoryHubService) DeleteConnection(ctx context.Context, id string) error {
	return s.store.DeleteHubConnection(ctx, strings.TrimSpace(id))
}

func (s *RepositoryHubService) CreateOAuthState(ctx context.Context, provider domain.RepositoryProvider, host, redirectURI, verifier string) (string, error) {
	host, err := normalizeHubHost(provider, host)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(redirectURI) == "" {
		return "", fmt.Errorf("%w: OAuth redirect URI is required", domain.ErrInvalid)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(state))
	now := domain.Now()
	err = s.store.SaveHubOAuthState(ctx, domain.HubOAuthState{
		StateHash: hex.EncodeToString(hash[:]), Provider: provider, Host: host,
		CodeVerifier: verifier, RedirectURI: redirectURI, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		return "", err
	}
	return state, nil
}

type OAuthClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
}

func gitLabOAuthClientMap() map[string]OAuthClientConfig {
	raw := strings.TrimSpace(os.Getenv("IDIOS_GITLAB_OAUTH_CLIENTS"))
	if raw == "" {
		return nil
	}
	var values map[string]OAuthClientConfig
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	normalized := make(map[string]OAuthClientConfig, len(values))
	for host, value := range values {
		key, err := normalizeHubHost(domain.RepositoryProviderGitLab, host)
		if err == nil && strings.TrimSpace(value.ClientID) != "" {
			normalized[key] = OAuthClientConfig{ClientID: strings.TrimSpace(value.ClientID), ClientSecret: strings.TrimSpace(value.ClientSecret)}
		}
	}
	return normalized
}

func OAuthConfigured(provider domain.RepositoryProvider) bool {
	switch provider {
	case domain.RepositoryProviderGitHub:
		return strings.TrimSpace(os.Getenv("IDIOS_GITHUB_OAUTH_CLIENT_ID")) != "" &&
			strings.TrimSpace(os.Getenv("IDIOS_GITHUB_OAUTH_CLIENT_SECRET")) != ""
	case domain.RepositoryProviderGitLab:
		return strings.TrimSpace(os.Getenv("IDIOS_GITLAB_OAUTH_CLIENT_ID")) != "" || len(gitLabOAuthClientMap()) > 0
	default:
		return false
	}
}

func providerOAuthCredentials(provider domain.RepositoryProvider, host string) (string, string) {
	switch provider {
	case domain.RepositoryProviderGitHub:
		return strings.TrimSpace(os.Getenv("IDIOS_GITHUB_OAUTH_CLIENT_ID")), strings.TrimSpace(os.Getenv("IDIOS_GITHUB_OAUTH_CLIENT_SECRET"))
	case domain.RepositoryProviderGitLab:
		if normalizedHost, err := normalizeHubHost(provider, host); err == nil {
			if config, ok := gitLabOAuthClientMap()[normalizedHost]; ok {
				return config.ClientID, config.ClientSecret
			}
		}
		return strings.TrimSpace(os.Getenv("IDIOS_GITLAB_OAUTH_CLIENT_ID")), strings.TrimSpace(os.Getenv("IDIOS_GITLAB_OAUTH_CLIENT_SECRET"))
	default:
		return "", ""
	}
}

func (s *RepositoryHubService) OAuthAuthorizationURL(ctx context.Context, provider domain.RepositoryProvider, host, redirectURI string) (string, error) {
	clientID, _ := providerOAuthCredentials(provider, host)
	if clientID == "" {
		return "", fmt.Errorf("%w: OAuth client is not configured for %s", domain.ErrUnavailable, provider)
	}
	normalizedHost, err := normalizeHubHost(provider, host)
	if err != nil {
		return "", err
	}
	redirect, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || redirect.Host == "" || redirect.User != nil || redirect.RawQuery != "" || redirect.Fragment != "" ||
		redirect.Path != "/api/hub/oauth/callback" || (redirect.Scheme != "https" && redirect.Scheme != "http") {
		return "", fmt.Errorf("%w: OAuth callback must be the IDIOS local callback URL", domain.ErrInvalid)
	}
	if redirect.Scheme == "http" && !isOAuthLoopbackHost(redirect.Hostname()) {
		return "", fmt.Errorf("%w: OAuth callback must use HTTPS outside loopback", domain.ErrInvalid)
	}
	verifier := ""
	challenge := ""
	if provider == domain.RepositoryProviderGitLab {
		random := make([]byte, 32)
		if _, err := rand.Read(random); err != nil {
			return "", err
		}
		verifier = base64.RawURLEncoding.EncodeToString(random)
		digest := sha256.Sum256([]byte(verifier))
		challenge = base64.RawURLEncoding.EncodeToString(digest[:])
	}
	state, err := s.CreateOAuthState(ctx, provider, normalizedHost, redirectURI, verifier)
	if err != nil {
		return "", err
	}
	var authorize string
	switch provider {
	case domain.RepositoryProviderGitHub:
		authorize = "https://github.com/login/oauth/authorize"
	case domain.RepositoryProviderGitLab:
		authorize = normalizedHost + "/oauth/authorize"
	default:
		return "", fmt.Errorf("%w: SourceForge Allura uses a manually supplied Bearer token; it is not part of this OAuth 2.0 callback", domain.ErrUnsupported)
	}
	target, _ := url.Parse(authorize)
	query := target.Query()
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("state", state)
	if provider == domain.RepositoryProviderGitHub {
		query.Set("scope", "repo read:user offline_access")
	} else {
		query.Set("scope", "api")
		query.Set("code_challenge", challenge)
		query.Set("code_challenge_method", "S256")
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func isOAuthLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *RepositoryHubService) CompleteProviderOAuth(ctx context.Context, rawState, code string) (domain.HubConnection, error) {
	state, err := s.ConsumeOAuthState(ctx, rawState)
	if err != nil {
		return domain.HubConnection{}, err
	}
	clientID, clientSecret := providerOAuthCredentials(state.Provider, state.Host)
	if clientID == "" {
		return domain.HubConnection{}, fmt.Errorf("%w: OAuth client is no longer configured", domain.ErrUnavailable)
	}
	client, err := repositoryhub.New(state.Provider, state.Host)
	if err != nil {
		return domain.HubConnection{}, err
	}
	tokens, err := client.ExchangeAuthorizationCode(ctx, code, state.RedirectURI, clientID, clientSecret, state.CodeVerifier)
	if err != nil {
		return domain.HubConnection{}, err
	}
	account, err := client.Account(ctx, tokens.AccessToken)
	if err != nil {
		return domain.HubConnection{}, err
	}
	scopes := strings.Fields(strings.ReplaceAll(tokens.Scope, ",", " "))
	expiresAt := time.Time{}
	if tokens.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	}
	envelope := providerCredentialEnvelope{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, TokenType: tokens.TokenType, ExpiresAt: expiresAt}
	serialized, err := json.Marshal(envelope)
	if err != nil {
		return domain.HubConnection{}, err
	}
	defer zeroBytes(serialized)
	return s.SaveConnection(ctx, state.Provider, state.Host, account.ID, account.Username, account.DisplayName, "", string(serialized), scopes)
}

func (s *RepositoryHubService) ConsumeOAuthState(ctx context.Context, rawState string) (domain.HubOAuthState, error) {
	rawState = strings.TrimSpace(rawState)
	if rawState == "" {
		return domain.HubOAuthState{}, fmt.Errorf("%w: OAuth state is required", domain.ErrInvalid)
	}
	hash := sha256.Sum256([]byte(rawState))
	item, err := s.store.ConsumeHubOAuthState(ctx, hex.EncodeToString(hash[:]))
	if err != nil {
		return domain.HubOAuthState{}, err
	}
	if time.Now().After(item.ExpiresAt) {
		return domain.HubOAuthState{}, fmt.Errorf("%w: OAuth state expired", domain.ErrInvalid)
	}
	return item, nil
}

func (s *RepositoryHubService) ListHubRepositories(ctx context.Context, provider, search, collectionID string, pinnedOnly bool) ([]domain.HubRepository, error) {
	return s.store.ListHubRepositories(ctx, strings.TrimSpace(provider), strings.TrimSpace(search), strings.TrimSpace(collectionID), pinnedOnly)
}

func (s *RepositoryHubService) GetHubRepository(ctx context.Context, id string) (domain.HubRepository, error) {
	return s.store.GetHubRepository(ctx, id)
}

// ImportRepository is intentionally explicit: only a user-selected repository is persisted.
func (s *RepositoryHubService) ImportRepository(ctx context.Context, item domain.HubRepository) (domain.HubRepository, error) {
	connection, err := s.store.GetHubConnection(ctx, item.ConnectionID)
	if err != nil {
		return domain.HubRepository{}, err
	}
	if item.Provider != connection.Provider || item.Host != connection.Host {
		return domain.HubRepository{}, fmt.Errorf("%w: repository provider/host does not match its connection", domain.ErrInvalid)
	}
	// The external ID is provider-scoped and immutable; owner/name changes cannot create duplicates.
	return s.store.UpsertHubRepository(ctx, item)
}

func (s *RepositoryHubService) SyncHubRepository(ctx context.Context, id string) (domain.HubRepository, error) {
	current, client, token, err := s.remoteContext(ctx, id)
	if err != nil {
		return domain.HubRepository{}, err
	}
	defer zeroBytes(token)
	remote, err := client.GetRepository(ctx, string(token), current.ExternalID, current.FullPath)
	if err == nil && remote.Provider == domain.RepositoryProviderSourceForge && remote.CloneSSHURL == "" {
		remote.CloneSSHURL = current.CloneSSHURL
	}
	if err != nil {
		saved, loadErr := s.store.GetHubRepository(ctx, id)
		if loadErr == nil {
			saved.SyncState = "error"
			saved.LastError = err.Error()
			saved.UpdatedAt = domain.Now()
			_, _ = s.store.UpsertHubRepository(ctx, saved)
		}
		return domain.HubRepository{}, err
	}
	item, err := s.store.GetHubRepository(ctx, id)
	if err != nil {
		return domain.HubRepository{}, err
	}
	now := domain.Now()
	item.Namespace, item.Name, item.FullPath = remote.Namespace, remote.Name, remote.FullPath
	item.Description, item.WebURL, item.CloneURL, item.CloneSSHURL = remote.Description, remote.WebURL, remote.CloneURL, remote.CloneSSHURL
	item.DefaultBranch, item.Language, item.Visibility = remote.DefaultBranch, remote.Language, remote.Visibility
	item.RepositoryType, item.Permission, item.Capabilities = remote.RepositoryType, remote.Permission, remote.Capabilities
	item.TrackerMountPoint, item.DiscussionMountPoint = remote.TrackerMountPoint, remote.DiscussionMountPoint
	item.OpenIssueCount, item.OpenChangeRequestCount = remote.OpenIssueCount, remote.OpenChangeCount
	item.SyncState, item.LastError, item.LastSyncedAt = "ok", "", &now
	return s.store.UpsertHubRepository(ctx, item)
}

func (s *RepositoryHubService) UnlinkRepository(ctx context.Context, id string) error {
	return s.store.UnlinkHubRepository(ctx, id)
}

func (s *RepositoryHubService) SavePreferences(ctx context.Context, item domain.HubRepositoryPreferences) (domain.HubRepositoryPreferences, error) {
	if len(item.LocalAlias) > 160 {
		return domain.HubRepositoryPreferences{}, fmt.Errorf("%w: repository alias is too long", domain.ErrInvalid)
	}
	if len(item.PrivateNote) > 20000 {
		return domain.HubRepositoryPreferences{}, fmt.Errorf("%w: private Repository note exceeds 20000 characters", domain.ErrInvalid)
	}
	return s.store.SaveHubRepositoryPreferences(ctx, item)
}

func (s *RepositoryHubService) GetPreferences(ctx context.Context, repositoryID string) (domain.HubRepositoryPreferences, error) {
	return s.store.GetHubRepositoryPreferences(ctx, repositoryID)
}

func (s *RepositoryHubService) ListCollections(ctx context.Context) ([]domain.HubCollection, error) {
	return s.store.ListHubCollections(ctx)
}

func (s *RepositoryHubService) SaveCollection(ctx context.Context, item domain.HubCollection) (domain.HubCollection, error) {
	if len(item.Description) > 2000 {
		return domain.HubCollection{}, fmt.Errorf("%w: collection description is too long", domain.ErrInvalid)
	}
	return s.store.SaveHubCollection(ctx, item)
}

func (s *RepositoryHubService) DeleteCollection(ctx context.Context, id string) error {
	return s.store.DeleteHubCollection(ctx, id)
}

func (s *RepositoryHubService) AddToCollection(ctx context.Context, collectionID, repositoryID string) error {
	return s.store.AddHubRepositoryToCollection(ctx, collectionID, repositoryID)
}

func (s *RepositoryHubService) RemoveFromCollection(ctx context.Context, collectionID, repositoryID string) error {
	return s.store.RemoveHubRepositoryFromCollection(ctx, collectionID, repositoryID)
}

func (s *RepositoryHubService) ListRelations(ctx context.Context, repositoryID string) ([]domain.HubRepositoryRelation, error) {
	return s.store.ListHubRepositoryRelations(ctx, repositoryID)
}

func (s *RepositoryHubService) CreateHubRepositoryRelation(ctx context.Context, item domain.HubRepositoryRelation) (domain.HubRepositoryRelation, error) {
	return s.store.CreateHubRepositoryRelation(ctx, item)
}

func (s *RepositoryHubService) DeleteRelation(ctx context.Context, id string) error {
	return s.store.DeleteHubRepositoryRelation(ctx, id)
}

// ConnectProvider verifies a supplied access token against the selected provider before saving
// it with authenticated encryption. Tokens are never returned to the Web client.
func (s *RepositoryHubService) ConnectProvider(ctx context.Context, provider domain.RepositoryProvider, host, token string) (domain.HubConnection, error) {
	client, err := repositoryhub.New(provider, host)
	if err != nil {
		return domain.HubConnection{}, err
	}
	account, err := client.Account(ctx, token)
	if err != nil {
		return domain.HubConnection{}, err
	}
	return s.SaveConnection(ctx, provider, client.Host(), account.ID, account.Username, account.DisplayName, "", token, nil)
}

func (s *RepositoryHubService) DiscoverProviderRepositories(ctx context.Context, connectionID, search string, page int) ([]repositoryhub.RemoteRepository, error) {
	token, connection, err := s.ProviderCredential(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(token)
	client, err := repositoryhub.New(connection.Provider, connection.Host)
	if err != nil {
		return nil, err
	}
	return client.ListRepositories(ctx, string(token), search, page)
}

func (s *RepositoryHubService) ImportSelectedProviderRepository(ctx context.Context, connectionID, externalID, fullPath string) (domain.HubRepository, error) {
	token, connection, err := s.ProviderCredential(ctx, connectionID)
	if err != nil {
		return domain.HubRepository{}, err
	}
	defer zeroBytes(token)
	client, err := repositoryhub.New(connection.Provider, connection.Host)
	if err != nil {
		return domain.HubRepository{}, err
	}
	remote, err := client.GetRepository(ctx, string(token), externalID, fullPath)
	if err != nil {
		return domain.HubRepository{}, err
	}
	if remote.Provider == domain.RepositoryProviderSourceForge {
		if remote.CloneSSHURL == "" {
			remote.CloneSSHURL, err = repositoryhub.SourceForgeSSHCloneURL(connection.AccountName, remote.FullPath)
			if err != nil {
				return domain.HubRepository{}, err
			}
		}
	}
	return s.ImportRepository(ctx, domain.HubRepository{
		ConnectionID: connection.ID, Provider: remote.Provider, Host: remote.Host,
		ExternalID: remote.ExternalID, Namespace: remote.Namespace, Name: remote.Name,
		FullPath: remote.FullPath, Description: remote.Description, WebURL: remote.WebURL,
		CloneURL: remote.CloneURL, CloneSSHURL: remote.CloneSSHURL, DefaultBranch: remote.DefaultBranch,
		Language: remote.Language, Visibility: remote.Visibility, RepositoryType: remote.RepositoryType,
		TrackerMountPoint: remote.TrackerMountPoint, DiscussionMountPoint: remote.DiscussionMountPoint,
		Permission: remote.Permission, Capabilities: remote.Capabilities,
		OpenIssueCount: remote.OpenIssueCount, OpenChangeRequestCount: remote.OpenChangeCount,
		SyncState: "never",
	})
}

func zeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func (s *RepositoryHubService) remoteContext(ctx context.Context, repositoryID string) (repositoryhub.RemoteRepository, *repositoryhub.Provider, []byte, error) {
	repo, err := s.store.GetHubRepository(ctx, repositoryID)
	if err != nil {
		return repositoryhub.RemoteRepository{}, nil, nil, err
	}
	credential, connection, err := s.ProviderCredential(ctx, repo.ConnectionID)
	if err != nil {
		return repositoryhub.RemoteRepository{}, nil, nil, err
	}
	client, err := repositoryhub.New(repo.Provider, repo.Host)
	if err != nil {
		zeroBytes(credential)
		return repositoryhub.RemoteRepository{}, nil, nil, err
	}
	if connection.ID != repo.ConnectionID {
		zeroBytes(credential)
		return repositoryhub.RemoteRepository{}, nil, nil, fmt.Errorf("%w: repository connection changed", domain.ErrConflict)
	}
	remote := repositoryhub.RemoteRepository{
		Provider:             repo.Provider,
		Host:                 repo.Host,
		ExternalID:           repo.ExternalID,
		Namespace:            repo.Namespace,
		SSHAccountName:       connection.AccountName,
		Name:                 repo.Name,
		FullPath:             repo.FullPath,
		Description:          repo.Description,
		WebURL:               repo.WebURL,
		CloneURL:             repo.CloneURL,
		CloneSSHURL:          repo.CloneSSHURL,
		DefaultBranch:        repo.DefaultBranch,
		Language:             repo.Language,
		Visibility:           repo.Visibility,
		RepositoryType:       repo.RepositoryType,
		Permission:           repo.Permission,
		Capabilities:         repo.Capabilities,
		OpenIssueCount:       repo.OpenIssueCount,
		OpenChangeCount:      repo.OpenChangeRequestCount,
		TrackerMountPoint:    repo.TrackerMountPoint,
		DiscussionMountPoint: repo.DiscussionMountPoint,
	}
	return remote, client, credential, nil
}

func (s *RepositoryHubService) RemoteBranches(ctx context.Context, repositoryID string) ([]repositoryhub.Branch, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(token)
	return client.ListBranches(ctx, string(token), repo)
}
func (s *RepositoryHubService) RemoteTree(ctx context.Context, repositoryID, ref string) ([]repositoryhub.TreeEntry, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(token)
	return client.ListTree(ctx, string(token), repo, ref)
}
func (s *RepositoryHubService) RemoteFile(ctx context.Context, repositoryID, path, ref string) (repositoryhub.File, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return repositoryhub.File{}, err
	}
	defer zeroBytes(token)
	return client.ReadFile(ctx, string(token), repo, path, ref)
}
func (s *RepositoryHubService) RemoteCommits(ctx context.Context, repositoryID, ref string, page int) ([]repositoryhub.Commit, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(token)
	return client.ListCommits(ctx, string(token), repo, ref, page)
}
func (s *RepositoryHubService) RemoteIssues(ctx context.Context, repositoryID, state string, page int) ([]repositoryhub.Issue, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(token)
	return client.ListIssues(ctx, string(token), repo, state, page)
}
func (s *RepositoryHubService) CreateRemoteIssue(ctx context.Context, repositoryID, title, body string, labels []string) (repositoryhub.Issue, error) {
	return s.CreateRemoteIssueWithOptions(ctx, repositoryID, title, body, labels, repositoryhub.IssueCreateOptions{})
}

func (s *RepositoryHubService) CreateRemoteIssueWithOptions(ctx context.Context, repositoryID, title, body string, labels []string, options repositoryhub.IssueCreateOptions) (repositoryhub.Issue, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return repositoryhub.Issue{}, err
	}
	defer zeroBytes(token)
	return client.CreateIssueWithOptions(ctx, string(token), repo, title, body, labels, options)
}
func (s *RepositoryHubService) UpdateRemoteIssue(ctx context.Context, repositoryID string, number int, options repositoryhub.IssueUpdateOptions) (repositoryhub.Issue, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return repositoryhub.Issue{}, err
	}
	defer zeroBytes(token)
	return client.UpdateIssue(ctx, string(token), repo, number, options)
}

func (s *RepositoryHubService) CloseRemoteIssue(ctx context.Context, repositoryID string, number int) error {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return err
	}
	defer zeroBytes(token)
	return client.CloseIssue(ctx, string(token), repo, number)
}

func (s *RepositoryHubService) CommentRemoteIssue(ctx context.Context, repositoryID string, number int, body string) error {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return err
	}
	defer zeroBytes(token)
	return client.CommentIssue(ctx, string(token), repo, number, body)
}
func (s *RepositoryHubService) RemoteChanges(ctx context.Context, repositoryID, state string, page int) ([]repositoryhub.ChangeRequest, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(token)
	return client.ListChanges(ctx, string(token), repo, state, page)
}
func (s *RepositoryHubService) CreateRemoteChange(ctx context.Context, repositoryID, title, body, source, target string) (repositoryhub.ChangeRequest, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return repositoryhub.ChangeRequest{}, err
	}
	defer zeroBytes(token)
	return client.CreateChange(ctx, string(token), repo, title, body, source, target)
}
func (s *RepositoryHubService) CloseRemoteChange(ctx context.Context, repositoryID string, number int) error {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return err
	}
	defer zeroBytes(token)
	return client.CloseChange(ctx, string(token), repo, number)
}

func (s *RepositoryHubService) ApproveRemoteChange(ctx context.Context, repositoryID string, number int) error {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return err
	}
	defer zeroBytes(token)
	return client.ApproveChange(ctx, string(token), repo, number)
}
func (s *RepositoryHubService) MergeRemoteChange(ctx context.Context, repositoryID string, number int) error {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return err
	}
	defer zeroBytes(token)
	return client.MergeChange(ctx, string(token), repo, number)
}
func (s *RepositoryHubService) WriteRemoteFile(ctx context.Context, repositoryID string, input repositoryhub.FileWrite) (repositoryhub.File, error) {
	repo, client, token, err := s.remoteContext(ctx, repositoryID)
	if err != nil {
		return repositoryhub.File{}, err
	}
	defer zeroBytes(token)
	return client.WriteFile(ctx, string(token), repo, input)
}

func (s *RepositoryHubService) ListHubProjects(ctx context.Context, repositoryID string) ([]domain.HubProjectRepository, error) {
	if strings.TrimSpace(repositoryID) == "" {
		return nil, fmt.Errorf("%w: Hub repository ID is required", domain.ErrInvalid)
	}
	return s.store.ListHubProjects(ctx, strings.TrimSpace(repositoryID))
}

func (s *RepositoryHubService) ListHubProjectRepositories(ctx context.Context, projectID string) ([]domain.HubProjectRepository, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("%w: Project ID is required", domain.ErrInvalid)
	}
	return s.store.ListHubProjectRepositories(ctx, strings.TrimSpace(projectID))
}

func (s *RepositoryHubService) LinkHubRepositoryToProject(ctx context.Context, projectID, repositoryID, relationship string, primary bool) error {
	for _, item := range []struct {
		name, value string
		required    bool
		max         int
	}{
		{"Project ID", projectID, true, 256},
		{"Hub repository ID", repositoryID, true, 256},
		{"Project relationship", relationship, false, 1000},
	} {
		if err := invalidInput(validation.Text(item.name, item.value, item.required, item.max)); err != nil {
			return err
		}
	}
	return s.store.LinkHubRepositoryToProject(ctx, strings.TrimSpace(projectID), strings.TrimSpace(repositoryID), strings.TrimSpace(relationship), primary)
}

func (s *RepositoryHubService) UnlinkHubRepositoryFromProject(ctx context.Context, projectID, repositoryID string) error {
	for _, item := range []struct{ name, value string }{
		{"Project ID", projectID}, {"Hub repository ID", repositoryID},
	} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.UnlinkHubRepositoryFromProject(ctx, strings.TrimSpace(projectID), strings.TrimSpace(repositoryID))
}

func (s *RepositoryHubService) SetPrimaryHubProjectRepository(ctx context.Context, projectID, repositoryID string) error {
	for _, item := range []struct{ name, value string }{
		{"Project ID", projectID}, {"Hub repository ID", repositoryID},
	} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return err
		}
	}
	return s.store.SetPrimaryHubProjectRepository(ctx, strings.TrimSpace(projectID), strings.TrimSpace(repositoryID))
}
