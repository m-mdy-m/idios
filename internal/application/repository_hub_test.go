package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"idios/internal/domain"
	"idios/internal/storage/sqlite"
)

func TestProviderCredentialRefreshRotatesAndPersistsGitLabOAuthTokens(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	t.Setenv("IDIOS_PROVIDER_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	t.Setenv("IDIOS_GITLAB_OAUTH_CLIENT_ID", "idios-client")
	t.Setenv("IDIOS_GITLAB_OAUTH_CLIENT_SECRET", "idios-secret")
	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "old-refresh" ||
			r.Form.Get("client_id") != "idios-client" || r.Form.Get("client_secret") != "idios-secret" {
			t.Errorf("unexpected refresh request form: %#v", r.Form)
		}
		refreshCalls.Add(1)
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","scope":"api","expires_in":7200}`))
	}))
	defer server.Close()

	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "hub-refresh.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewRepositoryHubService(store)
	envelope := providerCredentialEnvelope{
		AccessToken: "expired-access", RefreshToken: "old-refresh", TokenType: "Bearer", ExpiresAt: time.Now().Add(-time.Minute),
	}
	serialized, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := sealProviderCredential(serialized, service.key, credentialAAD(domain.RepositoryProviderGitLab, server.URL, "user-7"))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.SaveHubConnection(context.Background(), domain.HubConnection{
		Provider: domain.RepositoryProviderGitLab, Host: server.URL, AccountID: "user-7", AccountName: "developer",
		CredentialCiphertext: encrypted, Scopes: []string{"api"},
	})
	if err != nil {
		t.Fatal(err)
	}

	token, updated, err := service.ProviderCredential(context.Background(), connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(token) != "new-access" {
		t.Fatalf("refreshed access token = %q", token)
	}
	for i := range token {
		token[i] = 0
	}
	if updated.CredentialCiphertext == encrypted || strings.Contains(updated.CredentialCiphertext, "new-access") || strings.Contains(updated.CredentialCiphertext, "new-refresh") {
		t.Fatal("rotated OAuth tokens were not encrypted before persistence")
	}

	token2, again, err := service.ProviderCredential(context.Background(), connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(token2) != "new-access" {
		t.Fatalf("second access token = %q", token2)
	}
	for i := range token2 {
		token2[i] = 0
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("refresh call count = %d, want 1", refreshCalls.Load())
	}
	stored, err := store.GetHubConnection(context.Background(), connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CredentialCiphertext != again.CredentialCiphertext || stored.Status != "connected" {
		t.Fatalf("rotated credentials were not persisted to the connection: %#v", stored)
	}

}

func TestGitLabOAuthClientCredentialsAreScopedToInstanceHost(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOWED_HOSTS", "https://gitlab.example.com,https://code.example.net")
	t.Setenv("IDIOS_GITLAB_OAUTH_CLIENT_ID", "")
	t.Setenv("IDIOS_GITLAB_OAUTH_CLIENT_SECRET", "")
	t.Setenv("IDIOS_GITLAB_OAUTH_CLIENTS", `{"https://gitlab.example.com":{"client_id":"one","client_secret":"secret-one"},"https://code.example.net":{"client_id":"two","client_secret":"secret-two"}}`)
	firstID, firstSecret := providerOAuthCredentials(domain.RepositoryProviderGitLab, "https://gitlab.example.com/")
	secondID, secondSecret := providerOAuthCredentials(domain.RepositoryProviderGitLab, "code.example.net")
	if firstID != "one" || firstSecret != "secret-one" {
		t.Fatalf("first GitLab OAuth client = %q/%q", firstID, firstSecret)
	}
	if secondID != "two" || secondSecret != "secret-two" {
		t.Fatalf("second GitLab OAuth client = %q/%q", secondID, secondSecret)
	}
	if OAuthConfigured(domain.RepositoryProviderGitLab) != true {
		t.Fatal("GitLab OAuth should be advertised when at least one instance client is configured")
	}
}
