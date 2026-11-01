package repositoryhub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"idios/internal/domain"
)

func TestGitHubReadFileAndOptimisticWrite(t *testing.T) {
	var puts int
	var putBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			t.Errorf("missing provider authorization header")
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /repos/acme/app/contents/README.md":
			_ = json.NewEncoder(w).Encode(map[string]any{"path": "README.md", "sha": "blob-1", "size": 3, "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("old"))})
		case "PUT /repos/acme/app/contents/README.md":
			puts++
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"path": "README.md", "sha": "blob-2", "size": 3}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &Provider{kind: domain.RepositoryProviderGitHub, host: server.URL, base: server.URL, http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitHub, Host: server.URL, ExternalID: "1", FullPath: "acme/app", DefaultBranch: "main"}
	file, err := p.ReadFile(context.Background(), "ghp_test", repo, "README.md", "main")
	if err != nil {
		t.Fatal(err)
	}
	if file.Content != "old" || file.SHA != "blob-1" {
		t.Fatalf("unexpected file: %#v", file)
	}

	updated, err := p.WriteFile(context.Background(), "ghp_test", repo, FileWrite{
		Path: "README.md", Branch: "main", Content: "new", CommitMessage: "update readme", ExpectedSHA: "blob-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SHA != "blob-2" || puts != 1 {
		t.Fatalf("unexpected write result: %#v puts=%d", updated, puts)
	}
	if putBody["sha"] != "blob-1" || putBody["message"] != "update readme" {
		t.Fatalf("optimistic revision was not sent: %#v", putBody)
	}

	_, err = p.WriteFile(context.Background(), "ghp_test", repo, FileWrite{
		Path: "README.md", Branch: "main", Content: "stale", CommitMessage: "stale update", ExpectedSHA: "older",
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale revision must conflict, got %v", err)
	}
	if puts != 1 {
		t.Fatalf("stale write reached the provider: puts=%d", puts)
	}
}

func TestGitLabBranchesAndUpdateUsesPUT(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	var updated bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer glpat_test" && r.Header.Get("PRIVATE-TOKEN") != "glpat_test" {
			t.Errorf("missing GitLab token")
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v4/projects/42/repository/branches":
			_, _ = w.Write([]byte(`[{"name":"main","default":true,"protected":true,"commit":{"id":"commit-1"}}]`))
		case "GET /api/v4/projects/42/repository/files/README.md":
			_, _ = w.Write([]byte(`{"file_path":"README.md","blob_id":"blob-1","last_commit_id":"commit-1","size":3,"encoding":"base64","content":"b2xk"}`))
		case "PUT /api/v4/projects/42/repository/files/README.md":
			updated = true
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["last_commit_id"] != "commit-1" {
				t.Errorf("missing optimistic last_commit_id: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"file_path":"README.md","blob_id":"blob-2","last_commit_id":"commit-2","size":3}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &Provider{kind: domain.RepositoryProviderGitLab, host: server.URL, base: server.URL + "/api/v4", http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitLab, Host: server.URL, ExternalID: "42", FullPath: "group/project", DefaultBranch: "main"}
	branches, err := p.ListBranches(context.Background(), "glpat_test", repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 || branches[0].Name != "main" || !branches[0].Default {
		t.Fatalf("unexpected branches: %#v", branches)
	}
	file, err := p.WriteFile(context.Background(), "glpat_test", repo, FileWrite{
		Path: "README.md", Branch: "main", Content: "new", CommitMessage: "update readme", ExpectedSHA: "commit-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated || file.LastCommitID != "commit-2" {
		t.Fatalf("GitLab update did not use PUT or return new revision: %#v", file)
	}
}

func TestSourceForgeUsesAlluraToolDiscoveryAndDoesNotPretendToSupportMergeRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer allura_token" {
			t.Errorf("missing Allura bearer token")
		}
		switch r.URL.Path {
		case "/user":
			_, _ = w.Write([]byte(`{"id":"user-1","username":"developer","display_name":"Developer"}`))
		case "/rest/auth/tools/git":
			_, _ = w.Write([]byte(`{"tools":[{"url":"https://sourceforge.net/p/project/code/","api_url":"https://sourceforge.net/rest/p/project/code","mount_point":"code","mount_label":"Code","name":"git","project_name":"project","clone_url_https_anon":"https://sourceforge.net/git/p/project/code/"}]}`))
		case "/rest/auth/tools/tracker":
			_, _ = w.Write([]byte(`{"tools":[{"project_name":"project","mount_point":"tickets","name":"tracker"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &Provider{kind: domain.RepositoryProviderSourceForge, host: server.URL, base: server.URL + "/rest", http: server.Client()}
	account, err := p.Account(context.Background(), "allura_token")
	if err != nil {
		t.Fatal(err)
	}
	if account.Username != "developer" || account.ID != "user-1" {
		t.Fatalf("unexpected SourceForge identity: %#v", account)
	}
	items, err := p.ListRepositories(context.Background(), "allura_token", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].FullPath != "project/code" || items[0].TrackerMountPoint != "tickets" {
		t.Fatalf("unexpected Allura tool mapping: %#v", items)
	}
	if !contains(items[0].Capabilities, "issues.write") {
		t.Fatalf("Tracker capability not derived from real mounted tool: %#v", items[0].Capabilities)
	}
	if _, err := p.ListChanges(context.Background(), "allura_token", items[0], "open", 1); !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("Allura must not advertise merge requests: %v", err)
	}
}

func TestGitLabHostRejectsInsecureNonLoopbackAndPathHosts(t *testing.T) {
	if _, err := New(domain.RepositoryProviderGitLab, "http://gitlab.example.com"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected cleartext host to be rejected: %v", err)
	}
	if _, err := New(domain.RepositoryProviderGitLab, "https://gitlab.example.com/group"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected path in origin to be rejected: %v", err)
	}
	if _, err := New(domain.RepositoryProviderGitHub, "https://github.example.com"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected arbitrary GitHub host to be rejected: %v", err)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}

func TestSourceForgeCloneURLAndGitPathValidation(t *testing.T) {
	t.Setenv("IDIOS_SOURCEFORGE_GIT_TRANSPORT", "https")
	repository := RemoteRepository{Provider: domain.RepositoryProviderSourceForge, Host: "https://sourceforge.net", FullPath: "project/code"}
	for _, raw := range []string{
		"https://sourceforge.net/git/p/project/code/",
		"https://git.code.sf.net/p/project/code",
	} {
		repository.CloneURL = raw
		if _, err := sourceForgeCloneURL(repository); err != nil {
			t.Errorf("expected trusted clone URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"http://sourceforge.net/git/p/project/code/",
		"https://attacker.example/git/p/project/code/",
		"https://user:secret@sourceforge.net/git/p/project/code/",
		"https://sourceforge.net:8443/git/p/project/code/",
	} {
		repository.CloneURL = raw
		if _, err := sourceForgeCloneURL(repository); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("expected unsafe clone URL %q to be rejected, got %v", raw, err)
		}
	}
	for _, path := range []string{"README.md", "src/main.go", "nested/deep/file.txt"} {
		if !safeGitPath(path) {
			t.Errorf("safe path rejected: %q", path)
		}
	}
	for _, path := range []string{"", "../secret", ".", "/etc/passwd", "src/../../secret", "src//main.go"} {
		if safeGitPath(path) {
			t.Errorf("unsafe Git path accepted: %q", path)
		}
	}
}

func TestSourceForgeTrackerUsesAlluraTicketAndMultipartContracts(t *testing.T) {
	var gotMultipart bool
	var gotComment bool
	var createdSummary string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer allura-token" {
			t.Errorf("missing Allura API bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /rest/p/project/tickets":
			if r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("page") != "0" {
				t.Errorf("unexpected Allura page query: %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"tickets":[{"summary":"Native ticket","ticket_num":12}],"count":1}`))
		case "GET /rest/p/project/tickets/12":
			_, _ = w.Write([]byte(`{"ticket":{"summary":"Native ticket","description":"Ticket body","status":"open","reported_by":"alice","assigned_to":"bob","private":true,"discussion_disabled":false,"labels":["bug"],"custom_fields":{"_milestone":"v1","_reviewer":"carol"},"created_date":"2026-10-01T10:00:00Z","mod_date":"2026-10-02T11:00:00Z"},"discussion_thread_url":"http://` + r.Host + `/rest/p/project/tickets/_discuss/thread/thread-12/"}`))
		case "GET /rest/p/project/tickets/13":
			_, _ = w.Write([]byte(`{"ticket":{"summary":"Created from IDIOS","description":"body","status":"open","reported_by":"developer","assigned_to":"alice","private":true,"discussion_disabled":true,"labels":["bug"],"custom_fields":{"_milestone":"v2"},"created_date":"2026-10-01T10:00:00Z","mod_date":"2026-10-02T11:00:00Z"}}`))
		case "POST /rest/p/project/tickets/new":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
				t.Errorf("Allura new ticket must be multipart/form-data, got %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart ticket: %v", err)
			} else {
				gotMultipart = true
				createdSummary = r.FormValue("ticket_form.summary")
				for key, want := range map[string]string{
					"ticket_form.private":                  "true",
					"ticket_form.discussion_disabled":      "true",
					"ticket_form.status":                   "open",
					"ticket_form.assigned_to":              "alice",
					"ticket_form.custom_fields._milestone": "v2",
				} {
					if got := r.FormValue(key); got != want {
						t.Errorf("Tracker form field %s = %q, want %q", key, got, want)
					}
				}
			}
			_, _ = w.Write([]byte(`{"ticket":{"_id":"abcdef0123456789","summary":"Created from IDIOS","description":"body","status":"open","private":true},"ticket_num":13}`))
		case "POST /rest/p/project/tickets/_discuss/thread/thread-12/new":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse discussion form: %v", err)
			}
			gotComment = r.Form.Get("text") == "A real remote comment"
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := &Provider{kind: domain.RepositoryProviderSourceForge, host: server.URL, base: server.URL + "/rest", http: server.Client()}
	repo := RemoteRepository{
		Provider: domain.RepositoryProviderSourceForge, Host: server.URL, FullPath: "project/code",
		WebURL: server.URL + "/p/project/code/", TrackerMountPoint: "tickets",
	}

	issues, err := p.ListIssues(context.Background(), "allura-token", repo, "open", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Number != 12 || issues[0].Title != "Native ticket" {
		t.Fatalf("unexpected SourceForge Tracker list: %#v", issues)
	}
	item := issues[0]
	if !item.Private || item.DiscussionDisabled || item.Assignee != "bob" || item.Author != "alice" ||
		item.CustomFields["_milestone"] != "v1" || len(item.Labels) != 1 || item.Labels[0] != "bug" {
		t.Fatalf("SourceForge-native ticket fields were not preserved: %#v", item)
	}

	created, err := p.CreateIssueWithOptions(context.Background(), "allura-token", repo, "Created from IDIOS", "body", []string{"bug"}, IssueCreateOptions{
		Status: "open", Assignee: "alice", Private: true, DiscussionDisabled: true,
		CustomFields: map[string]string{"_milestone": "v2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !gotMultipart || createdSummary != "Created from IDIOS" || created.Number != 13 || !created.Private ||
		!created.DiscussionDisabled || created.Assignee != "alice" || created.CustomFields["_milestone"] != "v2" {
		t.Fatalf("Allura create-ticket response/form mismatch: multipart=%v summary=%q issue=%#v", gotMultipart, createdSummary, created)
	}

	if err := p.CommentIssue(context.Background(), "allura-token", repo, 12, "A real remote comment"); err != nil {
		t.Fatal(err)
	}
	if !gotComment {
		t.Fatal("Allura discussion post was not sent to the real Tracker discussion endpoint")
	}
}

func TestUpdateGitHubIssueUsesPatchAndReplacesLabels(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/repos/acme/app/issues/9" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			t.Errorf("missing GitHub token")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"number":9,"title":"Updated title","body":"updated body","state":"closed","html_url":"https://github.com/acme/app/issues/9","created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-02T10:00:00Z","user":{"login":"alice"},"labels":[{"name":"bug"},{"name":"priority"}]}`))
	}))
	defer server.Close()
	p := &Provider{kind: domain.RepositoryProviderGitHub, host: server.URL, base: server.URL, http: server.Client()}
	title, body, assignee := "Updated title", "updated body", "bob"
	labels := []string{"bug", "priority"}
	item, err := p.UpdateIssue(context.Background(), "ghp_test", RemoteRepository{Provider: domain.RepositoryProviderGitHub, FullPath: "acme/app"}, 9, IssueUpdateOptions{
		Title: &title, Body: &body, State: "closed", Assignee: &assignee, Labels: &labels,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Number != 9 || item.Title != title || item.State != "closed" || item.Author != "alice" || len(item.Labels) != 2 {
		t.Fatalf("unexpected updated GitHub Issue: %#v", item)
	}
	if gotPayload["state"] != "closed" || gotPayload["title"] != title {
		t.Fatalf("GitHub update body missing issue fields: %#v", gotPayload)
	}
	if _, ok := gotPayload["assignees"]; !ok {
		t.Fatalf("GitHub update did not use the documented assignees array: %#v", gotPayload)
	}
}

func TestUpdateGitLabIssueUsesPutConfidentialAndStateEvent(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v4/projects/42/issues/7" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("PRIVATE-TOKEN") != "glpat_test" {
			t.Errorf("missing GitLab token")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"iid":7,"id":700,"title":"Updated GitLab issue","description":"body","state":"closed","confidential":true,"web_url":"https://gitlab.example/group/project/-/issues/7","created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-02T10:00:00Z","author":{"username":"alice"},"labels":["bug"]}`))
	}))
	defer server.Close()
	p := &Provider{kind: domain.RepositoryProviderGitLab, host: server.URL, base: server.URL + "/api/v4", http: server.Client()}
	private := true
	title := "Updated GitLab issue"
	item, err := p.UpdateIssue(context.Background(), "glpat_test", RemoteRepository{Provider: domain.RepositoryProviderGitLab, ExternalID: "42"}, 7, IssueUpdateOptions{
		Title: &title, State: "closed", Private: &private,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Number != 7 || !item.Private || item.State != "closed" {
		t.Fatalf("unexpected updated GitLab issue: %#v", item)
	}
	if gotPayload["state_event"] != "close" || gotPayload["confidential"] != true {
		t.Fatalf("GitLab update body missing native issue attributes: %#v", gotPayload)
	}
}

func TestUpdateSourceForgeTicketUsesSaveThenVerifiesNativeFields(t *testing.T) {
	var saveSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer allura-token" {
			t.Errorf("missing Allura bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /rest/p/project/tickets/12/save":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
				t.Errorf("Allura ticket save must use multipart/form-data, got %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse ticket save: %v", err)
			} else {
				saveSeen = true
				for key, want := range map[string]string{
					"ticket_form.summary":                  "Changed",
					"ticket_form.description":              "Changed body",
					"ticket_form.status":                   "review",
					"ticket_form.assigned_to":              "alice",
					"ticket_form.labels":                   "bug,urgent",
					"ticket_form.private":                  "false",
					"ticket_form.discussion_disabled":      "true",
					"ticket_form.custom_fields._milestone": "v2",
				} {
					if got := r.FormValue(key); got != want {
						t.Errorf("Tracker save %s=%q, want %q", key, got, want)
					}
				}
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "GET /rest/p/project/tickets/12":
			_, _ = w.Write([]byte(`{"ticket":{"summary":"Changed","description":"Changed body","status":"review","reported_by":"bob","assigned_to":"alice","private":false,"discussion_disabled":true,"labels":["bug","urgent"],"custom_fields":{"_milestone":"v2"},"created_date":"2026-10-01T10:00:00Z","mod_date":"2026-10-02T10:00:00Z"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &Provider{kind: domain.RepositoryProviderSourceForge, host: server.URL, base: server.URL + "/rest", http: server.Client()}
	title, body, assignee := "Changed", "Changed body", "alice"
	labels := []string{"bug", "urgent"}
	private, discussion := false, true
	item, err := p.UpdateIssue(context.Background(), "allura-token", RemoteRepository{
		Provider: domain.RepositoryProviderSourceForge, Host: server.URL, FullPath: "project/code",
		WebURL: server.URL + "/p/project/code/", TrackerMountPoint: "tickets",
	}, 12, IssueUpdateOptions{
		Title: &title, Body: &body, State: "review", Assignee: &assignee, Labels: &labels, Private: &private,
		DiscussionDisabled: &discussion, CustomFields: map[string]string{"_milestone": "v2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !saveSeen || item.Number != 12 || item.Title != "Changed" || item.State != "review" || !item.DiscussionDisabled || item.Private ||
		item.Assignee != "alice" || item.CustomFields["_milestone"] != "v2" {
		t.Fatalf("SourceForge update was not verified from Tracker: save=%v issue=%#v", saveSeen, item)
	}
}

func TestProviderCapabilitiesRespectGitHubTriageAndGitLabReporterRoles(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"triaged","full_name":"acme/triaged","html_url":"https://github.com/acme/triaged","owner":{"login":"acme"},"permissions":{"triage":true,"pull":true}},
			{"id":2,"name":"writer","full_name":"acme/writer","html_url":"https://github.com/acme/writer","owner":{"login":"acme"},"permissions":{"push":true,"pull":true}}
		]`))
	}))
	defer githubServer.Close()
	github := &Provider{kind: domain.RepositoryProviderGitHub, host: githubServer.URL, base: githubServer.URL, http: githubServer.Client()}
	githubRepos, err := github.ListRepositories(context.Background(), "ghp_test", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(githubRepos) != 2 {
		t.Fatalf("unexpected GitHub repositories: %#v", githubRepos)
	}
	triageCaps := strings.Join(githubRepos[0].Capabilities, ",")
	if !strings.Contains(triageCaps, "issues.write") || strings.Contains(triageCaps, "code.write") || strings.Contains(triageCaps, "changes.merge") {
		t.Fatalf("GitHub Triage role must write Issues but not code or merge changes: %#v", githubRepos[0].Capabilities)
	}
	writerCaps := strings.Join(githubRepos[1].Capabilities, ",")
	if !strings.Contains(writerCaps, "issues.write") || !strings.Contains(writerCaps, "code.write") || !strings.Contains(writerCaps, "changes.merge") {
		t.Fatalf("GitHub push role missing expected write capabilities: %#v", githubRepos[1].Capabilities)
	}

	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
			{"id":20,"name":"reported","path":"reported","path_with_namespace":"group/reported","web_url":"https://gitlab.example/group/reported","namespace":{"full_path":"group"},"permissions":{"project_access":{"access_level":20}}},
			{"id":30,"name":"developed","path":"developed","path_with_namespace":"group/developed","web_url":"https://gitlab.example/group/developed","namespace":{"full_path":"group"},"permissions":{"project_access":{"access_level":30}}}
		]`))
	}))
	defer gitlabServer.Close()
	gitlab := &Provider{kind: domain.RepositoryProviderGitLab, host: gitlabServer.URL, base: gitlabServer.URL + "/api/v4", http: gitlabServer.Client()}
	gitlabRepos, err := gitlab.ListRepositories(context.Background(), "glpat_test", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(gitlabRepos) != 2 {
		t.Fatalf("unexpected GitLab repositories: %#v", gitlabRepos)
	}
	reporter := strings.Join(gitlabRepos[0].Capabilities, ",")
	if !strings.Contains(reporter, "issues.write") || strings.Contains(reporter, "code.write") || strings.Contains(reporter, "changes.merge") {
		t.Fatalf("GitLab Reporter role must write Issues but not code or merge changes: %#v", gitlabRepos[0].Capabilities)
	}
	developer := strings.Join(gitlabRepos[1].Capabilities, ",")
	if !strings.Contains(developer, "issues.write") || !strings.Contains(developer, "code.write") {
		t.Fatalf("GitLab Developer role missing expected write capabilities: %#v", gitlabRepos[1].Capabilities)
	}
}

func TestSourceForgeSSHCloneURLUsesAgentTransportAndRejectsCredentialsInURL(t *testing.T) {
	clone, err := SourceForgeSSHCloneURL("developer", "project/code")
	if err != nil {
		t.Fatal(err)
	}
	if clone != "ssh://developer@git.code.sf.net/p/project/code" {
		t.Fatalf("unexpected SourceForge SSH URL: %q", clone)
	}
	for _, tc := range []struct{ account, path string }{
		{"", "project/code"}, {"developer", "../secret"}, {"developer", "project/../../secret"},
		{"dev/evil", "project/code"}, {"developer ", "project/code"},
	} {
		if _, err := SourceForgeSSHCloneURL(tc.account, tc.path); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("unsafe SSH identity/path accepted account=%q path=%q: %v", tc.account, tc.path, err)
		}
	}
	t.Setenv("IDIOS_SOURCEFORGE_GIT_TRANSPORT", "ssh")
	repo := RemoteRepository{Provider: domain.RepositoryProviderSourceForge, Host: "https://sourceforge.net", FullPath: "project/code", CloneSSHURL: clone, SSHAccountName: "developer"}
	got, err := sourceForgeCloneURL(repo)
	if err != nil || got != clone {
		t.Fatalf("trusted SourceForge SSH URL rejected: got=%q err=%v", got, err)
	}
	for _, bad := range []string{
		"ssh://developer:password@git.code.sf.net/p/project/code",
		"ssh://attacker@git.code.sf.net/p/project/code",
		"ssh://developer@git.code.sf.net:2222/p/project/code",
		"ssh://developer@git.code.sf.net/p/project/../secret",
	} {
		repo.CloneSSHURL = bad
		if _, err := sourceForgeCloneURL(repo); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("unsafe SourceForge SSH URL accepted %q: %v", bad, err)
		}
	}
}

func TestGitLabSelfManagedHostRequiresExplicitAllowlistAndRejectsEscapedRequestOrigin(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOWED_HOSTS", "")
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "")
	if _, err := New(domain.RepositoryProviderGitLab, "https://gitlab.internal"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("unapproved self-managed host should be rejected, got %v", err)
	}

	t.Setenv("IDIOS_GITLAB_ALLOWED_HOSTS", "https://gitlab.internal")
	provider, err := New(domain.RepositoryProviderGitLab, "https://gitlab.internal")
	if err != nil {
		t.Fatalf("explicitly allowlisted GitLab host should be accepted: %v", err)
	}
	if _, _, err := provider.request(context.Background(), "", http.MethodGet,
		"https://169.254.169.254/latest/meta-data", nil, nil, ""); !errors.Is(err, domain.ErrPermission) {
		t.Fatalf("request escaping the configured GitLab origin should be rejected before network I/O, got %v", err)
	}
}

func TestProviderEndpointPreservesAbsoluteHTTPAndHTTPSURLs(t *testing.T) {
	provider := &Provider{base: "https://gitlab.example.com/api/v4"}
	for _, target := range []string{
		"http://127.0.0.1:43170/oauth/token",
		"https://gitlab.example.com/oauth/token",
	} {
		if got := provider.endpoint(target, nil); got != target {
			t.Errorf("absolute endpoint %q was rewritten as %q", target, got)
		}
	}
	if got := provider.endpoint("projects/42", nil); got != "https://gitlab.example.com/api/v4/projects/42" {
		t.Errorf("relative provider endpoint = %q", got)
	}
}
