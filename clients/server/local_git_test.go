package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/localgit"
	"idios/internal/storage/sqlite"
)

func TestLocalGitHTTPWorkflowUsesRealOfflineWorkingCopy(t *testing.T) {
	if !localgit.NewCLIEngine().Availability().Available {
		t.Skip("Git CLI is not installed in this test environment")
	}
	workspaceRoot := filepath.Join(t.TempDir(), "managed-working-copies")
	t.Setenv("IDIOS_GIT_WORKSPACES_DIR", workspaceRoot)

	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "local-git-api.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	repository, err := store.CreateRepository(context.Background(), "Local Git API", "Offline Local Git regression test")
	if err != nil {
		t.Fatal(err)
	}
	handler := New(application.New(store), Options{
		Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}},
	}).routes()

	request := func(method, path string, payload any) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		if payload != nil {
			if err := json.NewEncoder(&body).Encode(payload); err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, &body)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	base := "/api/repositories/" + repository.ID + "/git"
	statusResponse := request(http.MethodGet, base+"/status", nil)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("initial Local Git status = %d: %s", statusResponse.Code, statusResponse.Body.String())
	}
	var status struct {
		Available   bool   `json:"available"`
		Initialized bool   `json:"initialized"`
		Path        string `json:"path"`
		Branch      string `json:"branch"`
		Clean       bool   `json:"clean"`
		Staged      int    `json:"staged"`
	}
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.Initialized || status.Path == "" || filepath.Dir(status.Path) != workspaceRoot {
		t.Fatalf("unexpected initial Local Git status: %#v", status)
	}

	initResponse := request(http.MethodPost, base+"/init", map[string]string{"branch": "main"})
	if initResponse.Code != http.StatusCreated {
		t.Fatalf("initialize Local Git = %d: %s", initResponse.Code, initResponse.Body.String())
	}
	if err := os.WriteFile(filepath.Join(status.Path, "README.md"), []byte("# Local first\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stageResponse := request(http.MethodPost, base+"/stage", map[string][]string{"paths": {"README.md"}})
	if stageResponse.Code != http.StatusOK {
		t.Fatalf("stage Local Git path = %d: %s", stageResponse.Code, stageResponse.Body.String())
	}
	stagedResponse := request(http.MethodGet, base+"/status", nil)
	if stagedResponse.Code != http.StatusOK || json.Unmarshal(stagedResponse.Body.Bytes(), &status) != nil || status.Staged != 1 {
		t.Fatalf("staged file was not reflected in Local Git status: status=%d body=%s decoded=%#v", stagedResponse.Code, stagedResponse.Body.String(), status)
	}

	commitResponse := request(http.MethodPost, base+"/commits", map[string]string{
		"message": "Add local README", "author_name": "IDIOS Test", "author_email": "idios-api@example.invalid",
	})
	if commitResponse.Code != http.StatusCreated {
		t.Fatalf("create Local Git commit = %d: %s", commitResponse.Code, commitResponse.Body.String())
	}
	var commit struct {
		SHA         string `json:"sha"`
		AuthorEmail string `json:"author_email"`
	}
	if err := json.Unmarshal(commitResponse.Body.Bytes(), &commit); err != nil {
		t.Fatal(err)
	}
	if commit.SHA == "" || commit.AuthorEmail != "idios-api@example.invalid" {
		t.Fatalf("Local Git API did not return the actual commit: %#v", commit)
	}

	branchResponse := request(http.MethodPost, base+"/branches", map[string]string{"name": "feature/offline", "base": "main"})
	if branchResponse.Code != http.StatusCreated {
		t.Fatalf("create Local Git branch = %d: %s", branchResponse.Code, branchResponse.Body.String())
	}
	branchesResponse := request(http.MethodGet, base+"/branches", nil)
	if branchesResponse.Code != http.StatusOK {
		t.Fatalf("list Local Git branches = %d: %s", branchesResponse.Code, branchesResponse.Body.String())
	}
	var branches []struct {
		Name    string `json:"name"`
		Current bool   `json:"current"`
	}
	if err := json.Unmarshal(branchesResponse.Body.Bytes(), &branches); err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected two real Git branches, got %#v", branches)
	}
	switchResponse := request(http.MethodPost, base+"/checkout", map[string]string{"name": "main"})
	if switchResponse.Code != http.StatusOK {
		t.Fatalf("switch Local Git branch = %d: %s", switchResponse.Code, switchResponse.Body.String())
	}

	tagResponse := request(http.MethodPost, base+"/tags", map[string]string{"name": "v0.1.0", "message": "First local milestone"})
	if tagResponse.Code != http.StatusCreated {
		t.Fatalf("create Local Git tag = %d: %s", tagResponse.Code, tagResponse.Body.String())
	}
	tagsResponse := request(http.MethodGet, base+"/tags", nil)
	if tagsResponse.Code != http.StatusOK || !bytes.Contains(tagsResponse.Body.Bytes(), []byte(commit.SHA)) {
		t.Fatalf("Local Git tag did not point to the real commit: status=%d body=%s", tagsResponse.Code, tagsResponse.Body.String())
	}

	logResponse := request(http.MethodGet, base+"/log?limit=10", nil)
	if logResponse.Code != http.StatusOK || !bytes.Contains(logResponse.Body.Bytes(), []byte(commit.SHA)) {
		t.Fatalf("Local Git history did not expose the real commit: status=%d body=%s", logResponse.Code, logResponse.Body.String())
	}
	diffResponse := request(http.MethodGet, base+"/diff", nil)
	if diffResponse.Code != http.StatusOK {
		t.Fatalf("read clean Local Git diff = %d: %s", diffResponse.Code, diffResponse.Body.String())
	}
	finalStatus := request(http.MethodGet, base+"/status", nil)
	if finalStatus.Code != http.StatusOK || json.Unmarshal(finalStatus.Body.Bytes(), &status) != nil || !status.Clean || status.Branch != "main" {
		t.Fatalf("expected a clean working copy on main after the API workflow: status=%d body=%s decoded=%#v", finalStatus.Code, finalStatus.Body.String(), status)
	}
}
