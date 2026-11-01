package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/storage/sqlite"
)

func TestRepositoryHubConnectDiscoverImportAndReadRemoteBranch(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	t.Setenv("IDIOS_PROVIDER_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gitlab-test-token" || r.Header.Get("PRIVATE-TOKEN") != "gitlab-test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v4/user":
			_, _ = w.Write([]byte(`{"id":17,"username":"developer","name":"Developer"}`))
		case "GET /api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":42,"name":"idios","path":"idios","path_with_namespace":"m-mdy-m/idios","description":"IDIOS","web_url":"https://gitlab.example/m-mdy-m/idios","http_url_to_repo":"https://gitlab.example/m-mdy-m/idios.git","ssh_url_to_repo":"git@gitlab.example:m-mdy-m/idios.git","default_branch":"main","visibility":"private","open_issues_count":2,"namespace":{"full_path":"m-mdy-m"},"permissions":{"project_access":{"access_level":40}}}]`))
		case "GET /api/v4/projects/42":
			_, _ = w.Write([]byte(`{"id":42,"name":"idios","path_with_namespace":"m-mdy-m/idios","description":"IDIOS","web_url":"https://gitlab.example/m-mdy-m/idios","http_url_to_repo":"https://gitlab.example/m-mdy-m/idios.git","ssh_url_to_repo":"git@gitlab.example:m-mdy-m/idios.git","default_branch":"main","visibility":"private","open_issues_count":2,"namespace":{"full_path":"m-mdy-m"},"permissions":{"project_access":{"access_level":40}}}`))
		case "GET /api/v4/projects/42/repository/branches":
			_, _ = w.Write([]byte(`[{"name":"main","default":true,"protected":true,"commit":{"id":"commit-main"}}]`))
		case "GET /api/v4/projects/42/repository/tree":
			if r.URL.Query().Get("ref") != "main" || r.URL.Query().Get("per_page") != "100" {
				t.Errorf("unexpected GitLab tree request: %v", r.URL.Query())
			}
			switch r.URL.Query().Get("path") {
			case "":
				_, _ = w.Write([]byte(`[{"id":"tree-src","name":"src","path":"src","type":"tree"}]`))
			case "src":
				_, _ = w.Write([]byte(`[{"id":"blob-main","name":"main.go","path":"src/main.go","type":"blob","size":42}]`))
			default:
				http.NotFound(w, r)
			}
		case "POST /api/v4/projects/42/issues":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if payload["title"] != "First remote issue" || payload["confidential"] != true {
				t.Errorf("unexpected GitLab create Issue payload: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"iid":5,"id":500,"title":"First remote issue","description":"body","state":"opened","confidential":true,"web_url":"https://gitlab.example/m-mdy-m/idios/-/issues/5","created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-01T10:00:00Z"} `))
		case "PUT /api/v4/projects/42/issues/5":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if payload["title"] != "Updated from IDIOS" || payload["state_event"] != "close" ||
				payload["confidential"] != true || payload["labels"] != "bug,urgent" {
				t.Errorf("unexpected GitLab update Issue payload: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"iid":5,"id":500,"title":"Updated from IDIOS","description":"edited body","state":"closed","confidential":true,"labels":["bug","urgent"],"web_url":"https://gitlab.example/m-mdy-m/idios/-/issues/5","created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-02T10:00:00Z","author":{"username":"developer"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "repository-hub-api.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := application.New(store)
	handler := New(app, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}}).routes()

	connectBody, _ := json.Marshal(map[string]string{
		"provider": "gitlab", "host": provider.URL, "access_token": "gitlab-test-token",
	})
	connectReq := httptest.NewRequest(http.MethodPost, "/api/hub/connections", bytes.NewReader(connectBody))
	connectRec := httptest.NewRecorder()
	handler.ServeHTTP(connectRec, connectReq)
	if connectRec.Code != http.StatusCreated {
		t.Fatalf("connect status = %d, want 201: %s", connectRec.Code, connectRec.Body.String())
	}
	if strings.Contains(connectRec.Body.String(), "gitlab-test-token") || strings.Contains(connectRec.Body.String(), "credential_ciphertext") {
		t.Fatalf("connection response leaked credential material: %s", connectRec.Body.String())
	}
	var connection struct {
		ID          string `json:"id"`
		AccountName string `json:"account_name"`
		Provider    string `json:"provider"`
	}
	if err := json.Unmarshal(connectRec.Body.Bytes(), &connection); err != nil {
		t.Fatal(err)
	}
	if connection.ID == "" || connection.AccountName != "developer" || connection.Provider != "gitlab" {
		t.Fatalf("unexpected connected account: %#v", connection)
	}

	storedConnection, err := store.GetHubConnection(context.Background(), connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedConnection.CredentialCiphertext == "gitlab-test-token" || !strings.HasPrefix(storedConnection.CredentialCiphertext, "v1:") {
		t.Fatalf("provider credential was not encrypted at rest: %q", storedConnection.CredentialCiphertext)
	}

	discoverReq := httptest.NewRequest(http.MethodGet, "/api/hub/discover?connection_id="+connection.ID, nil)
	discoverRec := httptest.NewRecorder()
	handler.ServeHTTP(discoverRec, discoverReq)
	if discoverRec.Code != http.StatusOK {
		t.Fatalf("discover status = %d: %s", discoverRec.Code, discoverRec.Body.String())
	}
	var discovery struct {
		Items []struct {
			ExternalID string `json:"external_id"`
			FullPath   string `json:"full_path"`
			Permission string `json:"permission"`
		} `json:"items"`
	}
	if err := json.Unmarshal(discoverRec.Body.Bytes(), &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Items) != 1 || discovery.Items[0].ExternalID != "42" || discovery.Items[0].FullPath != "m-mdy-m/idios" || discovery.Items[0].Permission != "admin" {
		t.Fatalf("unexpected discovered repository: %#v", discovery.Items)
	}

	importBody, _ := json.Marshal(map[string]string{"connection_id": connection.ID, "external_id": "42", "full_path": "m-mdy-m/idios"})
	importReq := httptest.NewRequest(http.MethodPost, "/api/hub/repositories/import", bytes.NewReader(importBody))
	importRec := httptest.NewRecorder()
	handler.ServeHTTP(importRec, importReq)
	if importRec.Code != http.StatusCreated {
		t.Fatalf("import status = %d: %s", importRec.Code, importRec.Body.String())
	}
	var imported struct {
		ID           string   `json:"id"`
		FullPath     string   `json:"full_path"`
		Provider     string   `json:"provider"`
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(importRec.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.ID == "" || imported.FullPath != "m-mdy-m/idios" || imported.Provider != "gitlab" {
		t.Fatalf("unexpected imported repository: %#v", imported)
	}

	branchReq := httptest.NewRequest(http.MethodGet, "/api/hub/repositories/"+imported.ID+"/branches", nil)
	branchRec := httptest.NewRecorder()
	handler.ServeHTTP(branchRec, branchReq)
	if branchRec.Code != http.StatusOK {
		t.Fatalf("branches status = %d: %s", branchRec.Code, branchRec.Body.String())
	}
	var branches []struct {
		Name      string `json:"name"`
		Default   bool   `json:"default"`
		CommitSHA string `json:"commit_sha"`
	}
	if err := json.Unmarshal(branchRec.Body.Bytes(), &branches); err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 || branches[0].Name != "main" || !branches[0].Default || branches[0].CommitSHA != "commit-main" {
		t.Fatalf("unexpected remote branches: %#v", branches)
	}

	treeReq := httptest.NewRequest(http.MethodGet, "/api/hub/repositories/"+imported.ID+"/tree?ref=main&page=1", nil)
	treeRec := httptest.NewRecorder()
	handler.ServeHTTP(treeRec, treeReq)
	var rootPage struct {
		Items []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA string `json:"sha"`
		} `json:"items"`
		Page int `json:"page"`
		HasMore bool `json:"has_more"`
	}
	if treeRec.Code != http.StatusOK || json.Unmarshal(treeRec.Body.Bytes(), &rootPage) != nil ||
		len(rootPage.Items) != 1 || rootPage.Items[0].Path != "src" || rootPage.Items[0].Type != "tree" ||
		rootPage.Items[0].SHA != "tree-src" || rootPage.Page != 1 || rootPage.HasMore {
		t.Fatalf("remote root tree endpoint returned an unexpected page: status=%d body=%s decoded=%#v", treeRec.Code, treeRec.Body.String(), rootPage)
	}
	childReq := httptest.NewRequest(http.MethodGet, "/api/hub/repositories/"+imported.ID+"/tree?ref=main&path=src&page=1", nil)
	childRec := httptest.NewRecorder()
	handler.ServeHTTP(childRec, childReq)
	var childPage struct {
		Items []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Size int64 `json:"size"`
		} `json:"items"`
		Page int `json:"page"`
		HasMore bool `json:"has_more"`
	}
	if childRec.Code != http.StatusOK || json.Unmarshal(childRec.Body.Bytes(), &childPage) != nil ||
		len(childPage.Items) != 1 || childPage.Items[0].Path != "src/main.go" ||
		childPage.Items[0].Type != "blob" || childPage.Items[0].Size != 42 {
		t.Fatalf("remote child tree endpoint returned an unexpected page: status=%d body=%s decoded=%#v", childRec.Code, childRec.Body.String(), childPage)
	}

	noteBody, _ := json.Marshal(map[string]any{
		"local_alias":  "personal worktree",
		"private_note": "Deployment steps: keep local and never sync upstream.",
		"pinned":       true,
	})
	noteReq := httptest.NewRequest(http.MethodPatch, "/api/hub/repositories/"+imported.ID+"/preferences", bytes.NewReader(noteBody))
	noteRec := httptest.NewRecorder()
	handler.ServeHTTP(noteRec, noteReq)
	if noteRec.Code != http.StatusOK {
		t.Fatalf("save local Repository preferences status = %d: %s", noteRec.Code, noteRec.Body.String())
	}
	var savedPrefs struct {
		LocalAlias  string `json:"local_alias"`
		PrivateNote string `json:"private_note"`
		Pinned      bool   `json:"pinned"`
	}
	if err := json.Unmarshal(noteRec.Body.Bytes(), &savedPrefs); err != nil {
		t.Fatal(err)
	}
	if savedPrefs.LocalAlias != "personal worktree" ||
		savedPrefs.PrivateNote != "Deployment steps: keep local and never sync upstream." || !savedPrefs.Pinned {
		t.Fatalf("private local preferences were not returned: %#v", savedPrefs)
	}
	getNoteReq := httptest.NewRequest(http.MethodGet, "/api/hub/repositories/"+imported.ID+"/preferences", nil)
	getNoteRec := httptest.NewRecorder()
	handler.ServeHTTP(getNoteRec, getNoteReq)
	if getNoteRec.Code != http.StatusOK || !strings.Contains(getNoteRec.Body.String(), "never sync upstream") {
		t.Fatalf("local private note was not persisted: status=%d body=%s", getNoteRec.Code, getNoteRec.Body.String())
	}

	project, err := store.CreateProject(context.Background(), "Remote Hub project", "Groups provider-linked repositories")
	if err != nil {
		t.Fatal(err)
	}
	projectLinkBody, _ := json.Marshal(map[string]any{
		"project_id": project.ID, "relationship": "primary implementation",
	})
	projectLinkReq := httptest.NewRequest(http.MethodPost, "/api/hub/repositories/"+imported.ID+"/projects", bytes.NewReader(projectLinkBody))
	projectLinkRec := httptest.NewRecorder()
	handler.ServeHTTP(projectLinkRec, projectLinkReq)
	if projectLinkRec.Code != http.StatusCreated {
		t.Fatalf("Hub repository Project link status = %d: %s", projectLinkRec.Code, projectLinkRec.Body.String())
	}
	repoProjectsReq := httptest.NewRequest(http.MethodGet, "/api/hub/repositories/"+imported.ID+"/projects", nil)
	repoProjectsRec := httptest.NewRecorder()
	handler.ServeHTTP(repoProjectsRec, repoProjectsReq)
	var repoProjects []struct {
		ProjectID    string `json:"project_id"`
		ProjectName  string `json:"project_name"`
		RepositoryID string `json:"repository_id"`
		IsPrimary    bool   `json:"is_primary"`
		Relationship string `json:"relationship"`
	}
	if repoProjectsRec.Code != http.StatusOK || json.Unmarshal(repoProjectsRec.Body.Bytes(), &repoProjects) != nil ||
		len(repoProjects) != 1 || repoProjects[0].ProjectID != project.ID ||
		repoProjects[0].ProjectName != project.Name || repoProjects[0].RepositoryID != imported.ID ||
		!repoProjects[0].IsPrimary || repoProjects[0].Relationship != "primary implementation" {
		t.Fatalf("Hub repository Project association was not persisted: status=%d body=%s decoded=%#v", repoProjectsRec.Code, repoProjectsRec.Body.String(), repoProjects)
	}
	projectReposReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/hub-repositories", nil)
	projectReposRec := httptest.NewRecorder()
	handler.ServeHTTP(projectReposRec, projectReposReq)
	if projectReposRec.Code != http.StatusOK || !strings.Contains(projectReposRec.Body.String(), imported.ID) {
		t.Fatalf("Project did not expose linked Hub repository: status=%d body=%s", projectReposRec.Code, projectReposRec.Body.String())
	}
	projectUnlinkReq := httptest.NewRequest(http.MethodDelete, "/api/hub/repositories/"+imported.ID+"/projects/"+project.ID, nil)
	projectUnlinkRec := httptest.NewRecorder()
	handler.ServeHTTP(projectUnlinkRec, projectUnlinkReq)
	if projectUnlinkRec.Code != http.StatusOK {
		t.Fatalf("Hub Project unlink status = %d: %s", projectUnlinkRec.Code, projectUnlinkRec.Body.String())
	}
	if _, err := store.GetHubRepository(context.Background(), imported.ID); err != nil {
		t.Fatalf("unlinking Project association deleted the provider-linked repository: %v", err)
	}

	createIssueBody, _ := json.Marshal(map[string]any{
		"title": "First remote issue", "body": "body", "labels": []string{"bug"}, "private": true,
	})
	createIssueReq := httptest.NewRequest(http.MethodPost, "/api/hub/repositories/"+imported.ID+"/issues", bytes.NewReader(createIssueBody))
	createIssueRec := httptest.NewRecorder()
	handler.ServeHTTP(createIssueRec, createIssueReq)
	if createIssueRec.Code != http.StatusCreated {
		t.Fatalf("remote Issue create status = %d: %s", createIssueRec.Code, createIssueRec.Body.String())
	}
	var createdIssue struct {
		Number  int  `json:"number"`
		Private bool `json:"private"`
	}
	if err := json.Unmarshal(createIssueRec.Body.Bytes(), &createdIssue); err != nil {
		t.Fatal(err)
	}
	if createdIssue.Number != 5 || !createdIssue.Private {
		t.Fatalf("unexpected created remote Issue: %#v", createdIssue)
	}

	updateIssueBody, _ := json.Marshal(map[string]any{
		"title": "Updated from IDIOS", "body": "edited body", "state": "closed",
		"labels": []string{"bug", "urgent"}, "private": true,
	})
	updateIssueReq := httptest.NewRequest(http.MethodPatch, "/api/hub/repositories/"+imported.ID+"/issues/5", bytes.NewReader(updateIssueBody))
	updateIssueRec := httptest.NewRecorder()
	handler.ServeHTTP(updateIssueRec, updateIssueReq)
	if updateIssueRec.Code != http.StatusOK {
		t.Fatalf("remote Issue update status = %d: %s", updateIssueRec.Code, updateIssueRec.Body.String())
	}
	var updatedIssue struct {
		Title   string   `json:"title"`
		State   string   `json:"state"`
		Labels  []string `json:"labels"`
		Private bool     `json:"private"`
	}
	if err := json.Unmarshal(updateIssueRec.Body.Bytes(), &updatedIssue); err != nil {
		t.Fatal(err)
	}
	if updatedIssue.Title != "Updated from IDIOS" || updatedIssue.State != "closed" || !updatedIssue.Private ||
		len(updatedIssue.Labels) != 2 || updatedIssue.Labels[1] != "urgent" {
		t.Fatalf("unexpected updated remote Issue: %#v", updatedIssue)
	}

	// The local IDIOS repositories API remains independent of the provider-linked repository.
	localReq := httptest.NewRequest(http.MethodGet, "/api/repositories", nil)
	localRec := httptest.NewRecorder()
	handler.ServeHTTP(localRec, localReq)
	if localRec.Code != http.StatusOK {
		t.Fatalf("local repositories status = %d: %s", localRec.Code, localRec.Body.String())
	}
	var local []any
	if err := json.Unmarshal(localRec.Body.Bytes(), &local); err != nil {
		t.Fatal(err)
	}
	if len(local) != 0 {
		t.Fatalf("remote import was copied into local simulated repositories: %#v", local)
	}

	if _, err := app.GetHubRepository(context.Background(), imported.ID); err != nil {
		t.Fatalf("linked external repository cannot be reloaded: %v", err)
	}
}

func TestRepositoryHubGitLabOAuthCallbackUsesPKCEAndStoresEncryptedToken(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	t.Setenv("IDIOS_PROVIDER_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	t.Setenv("IDIOS_GITLAB_OAUTH_CLIENT_ID", "idios-oauth-client")
	t.Setenv("IDIOS_GITLAB_OAUTH_CLIENT_SECRET", "idios-oauth-secret")

	var verifierSeen bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /oauth/token":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			verifierSeen = r.Form.Get("code_verifier") != "" && r.Form.Get("client_id") == "idios-oauth-client" &&
				r.Form.Get("client_secret") == "idios-oauth-secret" && r.Form.Get("grant_type") == "authorization_code"
			_, _ = w.Write([]byte(`{"access_token":"oauth-issued-token","token_type":"Bearer","scope":"api","expires_in":7200}`))
		case "GET /api/v4/user":
			if r.Header.Get("Authorization") != "Bearer oauth-issued-token" {
				http.Error(w, "bad token", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"id":91,"username":"oauth-user","name":"OAuth User"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "repository-hub-oauth.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(application.New(store), Options{
		Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}},
	}).routes()

	startReq := httptest.NewRequest(http.MethodGet, "/api/hub/oauth/start?provider=gitlab&host="+provider.URL, nil)
	startReq.Host = "127.0.0.1:43170"
	startRec := httptest.NewRecorder()
	handler.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("OAuth start status=%d: %s", startRec.Code, startRec.Body.String())
	}
	var start struct {
		AuthorizationURL string `json:"authorization_url"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}
	authorization, err := url.Parse(start.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	query := authorization.Query()
	if authorization.Path != "/oauth/authorize" || query.Get("client_id") != "idios-oauth-client" ||
		query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || query.Get("state") == "" {
		t.Fatalf("GitLab OAuth did not include state and PKCE: %s", start.AuthorizationURL)
	}
	callbackURL := "/api/hub/oauth/callback?code=temporary-code&state=" + url.QueryEscape(query.Get("state"))
	callbackReq := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	callbackReq.Host = "127.0.0.1:43170"
	callbackRec := httptest.NewRecorder()
	handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusSeeOther {
		t.Fatalf("OAuth callback status=%d: %s", callbackRec.Code, callbackRec.Body.String())
	}
	if got := callbackRec.Header().Get("Location"); !strings.Contains(got, "hub_connected=gitlab") {
		t.Fatalf("unexpected OAuth callback redirect: %q", got)
	}
	if !verifierSeen {
		t.Fatal("GitLab token exchange omitted the PKCE verifier or registered client credentials")
	}

	connections, err := store.ListHubConnections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].AccountName != "oauth-user" {
		t.Fatalf("OAuth did not persist verified provider identity: %#v", connections)
	}
	if connections[0].CredentialCiphertext == "oauth-issued-token" || !strings.HasPrefix(connections[0].CredentialCiphertext, "v1:") {
		t.Fatal("OAuth access token was not encrypted before persistence")
	}
}
