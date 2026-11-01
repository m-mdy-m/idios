package repositoryhub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"idios/internal/domain"
)

func TestGitHubListReleasesMapsPublishedReleaseAndAssetMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			t.Errorf("missing GitHub authorization")
		}
		if r.URL.Path != "/repos/acme/app/releases" || r.URL.Query().Get("per_page") != "2" || r.URL.Query().Get("page") != "1" {
			t.Errorf("unexpected release request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": 12, "tag_name": "v2.0.0", "name": "Version 2", "target_commitish": "main",
			"body": "Release notes", "draft": false, "prerelease": true,
			"created_at": "2026-10-30T10:00:00Z", "published_at": "2026-10-30T11:00:00Z",
			"html_url": "https://github.com/acme/app/releases/tag/v2.0.0",
			"assets": []map[string]any{{
				"id": 99, "name": "idios-linux-amd64.tar.gz", "label": "Linux x86_64",
				"size": 1048576, "content_type": "application/gzip", "download_count": 8,
				"digest": "sha256:abc123", "browser_download_url": "https://github.com/acme/app/releases/download/v2.0.0/idios-linux-amd64.tar.gz",
			}},
		}})
	}))
	defer server.Close()

	provider := &Provider{kind: domain.RepositoryProviderGitHub, host: server.URL, base: server.URL, http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitHub, FullPath: "acme/app", WebURL: "https://github.com/acme/app"}
	page, err := provider.ListReleases(context.Background(), "ghp_test", repo, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Availability.Available || page.Page != 1 || len(page.Items) != 1 || page.HasMore {
		t.Fatalf("unexpected GitHub release page: %#v", page)
	}
	release := page.Items[0]
	if release.TagName != "v2.0.0" || release.Name != "Version 2" || release.State != "prerelease" ||
		release.Draft == nil || *release.Draft || release.Prerelease == nil || !*release.Prerelease ||
		release.PublishedAt == nil || len(release.Assets) != 1 {
		t.Fatalf("release metadata was not preserved: %#v", release)
	}
	asset := release.Assets[0]
	if asset.Name != "idios-linux-amd64.tar.gz" || asset.Platform != "Linux x86_64" ||
		asset.Size == nil || *asset.Size != 1048576 || asset.DownloadCount == nil || *asset.DownloadCount != 8 ||
		asset.ContentType != "application/gzip" || asset.Digest != "sha256:abc123" ||
		!strings.HasPrefix(asset.URL, "https://github.com/") {
		t.Fatalf("asset metadata was not preserved: %#v", asset)
	}
}

func TestGitLabListReleasesKeepsUnavailableAssetMetadataEmpty(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "glpat_test" {
			t.Errorf("missing GitLab token")
		}
		if r.URL.Path != "/api/v4/projects/42/releases" ||
			r.URL.Query().Get("order_by") != "released_at" || r.URL.Query().Get("sort") != "desc" {
			http.Error(w, "unexpected release request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"tag_name": "v1.0", "name": "Version 1", "description": "Notes",
			"created_at": "2026-10-01T09:00:00Z", "released_at": "2026-10-01T10:00:00Z",
			"assets": map[string]any{"links": []map[string]any{{
				"id": 5, "name": "app-windows-x64.zip", "url": "https://gitlab.example/group/app/-/releases/v1.0/downloads/app.zip",
				"link_type": "package",
			}}},
		}})
	}))
	defer server.Close()

	provider := &Provider{kind: domain.RepositoryProviderGitLab, host: server.URL, base: server.URL + "/api/v4", http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitLab, ExternalID: "42", WebURL: "https://gitlab.example/group/app"}
	page, err := provider.ListReleases(context.Background(), "glpat_test", repo, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Availability.Available || len(page.Items) != 1 || page.Items[0].State != "published" {
		t.Fatalf("unexpected GitLab releases: %#v", page)
	}
	asset := page.Items[0].Assets[0]
	if asset.Name != "app-windows-x64.zip" || asset.Platform != "Windows x86_64" ||
		asset.Size != nil || asset.DownloadCount != nil || asset.Digest != "" ||
		asset.ContentType != "" || asset.URL == "" {
		t.Fatalf("unreported GitLab asset metadata must stay absent: %#v", asset)
	}
}

func TestGitHubListTagsDistinguishesAnnotatedAndLightweightTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			t.Errorf("missing GitHub authorization")
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /graphql":
			var input struct {
				Query     string `json:"query"`
				Variables struct {
					Owner string `json:"owner"`
					Name  string `json:"name"`
					First int    `json:"first"`
					After string `json:"after"`
				} `json:"variables"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input.Variables.Owner != "acme" || input.Variables.Name != "app" ||
				input.Variables.First != 3 || input.Variables.After != "" ||
				!strings.Contains(input.Query, "TAG_COMMIT_DATE") ||
				!strings.Contains(input.Query, "direction: DESC") {
				t.Errorf("GitHub tags must be requested in latest-commit-first order: %#v", input)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"refs": map[string]any{
							"nodes": []map[string]any{
								{
									"name": "v2.0",
									"target": map[string]any{
										"__typename": "Commit", "oid": "commit-two",
										"committedDate": "2026-09-01T10:00:00Z",
									},
								},
								{
									"name": "v1.0",
									"target": map[string]any{
										"__typename": "Tag", "oid": "tag-object-one",
										"message": "Annotated version one",
										"tagger":  map[string]any{"date": "2026-08-01T10:00:00Z"},
										"target": map[string]any{
											"__typename": "Commit", "oid": "commit-one",
											"committedDate": "2026-07-31T10:00:00Z",
										},
									},
								},
							},
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
						},
					},
				},
			})
		case "GET /repos/acme/app/releases":
			if r.URL.Query().Get("per_page") != "100" || r.URL.Query().Get("page") != "1" {
				t.Errorf("unexpected GitHub release association request: %v", r.URL.Query())
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"name": "Version 1", "tag_name": "v1.0",
				"html_url": "https://github.com/acme/app/releases/tag/v1.0",
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := &Provider{kind: domain.RepositoryProviderGitHub, host: server.URL, base: server.URL, http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitHub, FullPath: "acme/app", WebURL: "https://github.com/acme/app"}
	page, err := provider.ListTags(context.Background(), "ghp_test", repo, 1, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.HasMore || page.NextCursor != "" {
		t.Fatalf("unexpected GitHub tag page: %#v", page)
	}
	lightweight, annotated := page.Items[0], page.Items[1]
	if lightweight.Kind != "lightweight" || lightweight.TargetSHA != "commit-two" ||
		lightweight.CommitCreatedAt == nil || lightweight.CreatedAt != nil {
		t.Fatalf("lightweight Git tag was misclassified: %#v", lightweight)
	}
	if annotated.Kind != "annotated" || annotated.ObjectSHA != "tag-object-one" ||
		annotated.TargetSHA != "commit-one" || annotated.Annotation != "Annotated version one" ||
		annotated.CreatedAt == nil || annotated.ReleaseName != "Version 1" || annotated.ReleaseURL == "" {
		t.Fatalf("annotated Git tag or release relation was not mapped: %#v", annotated)
	}
}

func TestGitHubCompareTagsMapsCommitIntervalAndChangedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/app/compare/v1.0...v2.0" ||
			r.URL.Query().Get("per_page") != "100" {
			t.Errorf("unexpected compare request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ahead", "ahead_by": 2, "behind_by": 0, "total_commits": 2,
			"merge_base_commit": map[string]any{"sha": "base-sha"},
			"commits": []map[string]any{{
				"sha": "commit-two", "html_url": "https://github.com/acme/app/commit/commit-two",
				"commit": map[string]any{
					"message":   "feat: new version",
					"author":    map[string]any{"name": "Alice", "date": "2026-10-01T10:00:00Z"},
					"committer": map[string]any{"name": "Bob", "date": "2026-10-01T10:30:00Z"},
				},
			}},
			"files": []map[string]any{{"filename": "main.go", "status": "modified", "additions": 3, "deletions": 1, "patch": "@@ -1 +1 @@"}},
		})
	}))
	defer server.Close()
	provider := &Provider{kind: domain.RepositoryProviderGitHub, host: server.URL, base: server.URL, http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitHub, FullPath: "acme/app"}
	comparison, err := provider.CompareTags(context.Background(), "ghp_test", repo, "v1.0", "v2.0")
	if err != nil {
		t.Fatal(err)
	}
	if comparison.State != "ahead" || !comparison.CountsAvailable || comparison.AheadBy != 2 ||
		comparison.BehindBy != 0 || comparison.MergeBaseSHA != "base-sha" ||
		len(comparison.Commits) != 1 || len(comparison.Files) != 1 || comparison.Files[0].Path != "main.go" {
		t.Fatalf("GitHub tag comparison was not mapped: %#v", comparison)
	}
}

func TestSourceForgeTagParserAndPlatformHints(t *testing.T) {
	raw := []byte("v2.0\t0123456789012345678901234567890123456789\tcommit\t\t2026-10-30T10:00:00+00:00\nv1.0\tabcdefabcdefabcdefabcdefabcdefabcdefabcd\ttag\t0011223344556677889900112233445566778899\t2026-09-30T10:00:00+00:00\n")
	records := parseSourceForgeTagRefs(raw)
	if len(records) != 2 || records[0].Name != "v2.0" || records[0].ObjectType != "commit" ||
		records[1].Name != "v1.0" || records[1].PeeledSHA != "0011223344556677889900112233445566778899" {
		t.Fatalf("SourceForge tag refs were not parsed: %#v", records)
	}
	if releasePlatformFromName("idios-linux-arm64.tar.gz") != "Linux ARM64" ||
		releasePlatformFromName("idios-windows-x64.zip") != "Windows x86_64" ||
		releasePlatformFromName("unknown-artifact.bin") != "" {
		t.Fatal("release artifact platform detection returned an unexpected result")
	}
	if parseSourceForgeTime("not-a-date") != nil {
		t.Fatal("invalid SourceForge timestamp must not be accepted")
	}
}

func TestNormalizeHubPageBoundsPagination(t *testing.T) {
	page, size, err := normalizeHubPage(0, 0)
	if err != nil || page != 1 || size != defaultHubPageSize {
		t.Fatalf("default page normalization = %d/%d, err=%v", page, size, err)
	}
	if _, _, err := normalizeHubPage(1, maxHubPageSize+1); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("oversized page size should be rejected, got %v", err)
	}
	if _, _, err := normalizeHubPage(10001, 20); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("out-of-range page should be rejected, got %v", err)
	}
	if _, _, err := normalizeHubPage(1, -1); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("negative page size should be rejected, got %v", err)
	}
	if releasePlatformFromName("product-universal-macos.zip") != "macOS Universal" {
		t.Fatal("universal macOS platform hint was not detected")
	}
}

func TestGitHubListTagsUsesProviderCursorForOlderPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /graphql":
			var input struct {
				Variables struct {
					After string `json:"after"`
					First int    `json:"first"`
				} `json:"variables"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input.Variables.After != "cursor-page-one" || input.Variables.First != 20 {
				t.Errorf("GitHub next-page cursor was not passed through: %#v", input.Variables)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"refs": map[string]any{
							"nodes": []map[string]any{{
								"name": "v0.9",
								"target": map[string]any{
									"__typename": "Commit", "oid": "commit-old",
									"committedDate": "2026-01-01T00:00:00Z",
								},
							}},
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": "cursor-page-two"},
						},
					},
				},
			})
		case "GET /repos/acme/app/releases":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	provider := &Provider{kind: domain.RepositoryProviderGitHub, host: server.URL, base: server.URL, http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitHub, FullPath: "acme/app", WebURL: "https://github.com/acme/app"}
	page, err := provider.ListTags(context.Background(), "ghp_test", repo, 2, 20, "cursor-page-one")
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 2 || page.HasMore || page.NextCursor != "" ||
		len(page.Items) != 1 || page.Items[0].Name != "v0.9" || page.Items[0].TargetSHA != "commit-old" {
		t.Fatalf("unexpected cursor page: %#v", page)
	}
}

func TestGitLabListTagsUsesAnnotationMessageAndKeepsObjectSHAUnknown(t *testing.T) {
	t.Setenv("IDIOS_GITLAB_ALLOW_LOOPBACK", "true")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/42/repository/tags" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name": "v2.0", "message": "", "target": "commit-two",
				"created_at": "2026-10-02T10:00:00Z",
				"commit":     map[string]any{"id": "commit-two", "created_at": "2026-10-02T09:00:00Z"},
			},
			{
				"name": "v1.0", "message": "annotated version one", "target": "commit-one",
				"created_at": "2026-10-01T10:00:00Z",
				"commit":     map[string]any{"id": "commit-one", "created_at": "2026-10-01T09:00:00Z"},
			},
		})
	}))
	defer server.Close()

	provider := &Provider{kind: domain.RepositoryProviderGitLab, host: server.URL, base: server.URL + "/api/v4", http: server.Client()}
	repo := RemoteRepository{Provider: domain.RepositoryProviderGitLab, ExternalID: "42", WebURL: "https://gitlab.example/group/app"}
	page, err := provider.ListTags(context.Background(), "glpat_test", repo, 1, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.HasMore {
		t.Fatalf("unexpected GitLab tag page: %#v", page)
	}
	if page.Items[0].Kind != "lightweight" || page.Items[0].TargetSHA != "commit-two" ||
		page.Items[0].ObjectSHA != "" || page.Items[0].ObjectType != "commit" {
		t.Fatalf("GitLab lightweight tag metadata was misclassified: %#v", page.Items[0])
	}
	if page.Items[1].Kind != "annotated" || page.Items[1].TargetSHA != "commit-one" ||
		page.Items[1].ObjectSHA != "" || page.Items[1].ObjectType != "commit" ||
		page.Items[1].Annotation != "annotated version one" {
		t.Fatalf("GitLab annotated tag metadata was misclassified: %#v", page.Items[1])
	}
}

func TestSourceForgeReleasesReportProviderLimitation(t *testing.T) {
	provider := &Provider{kind: domain.RepositoryProviderSourceForge}
	repo := RemoteRepository{Provider: domain.RepositoryProviderSourceForge, FullPath: "project/code"}
	page, err := provider.ListReleases(context.Background(), "token", repo, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Availability.Available || page.Availability.State != "unsupported" || len(page.Items) != 0 {
		t.Fatalf("SourceForge release limitation was not reported: %#v", page)
	}
	if !strings.Contains(page.Availability.Reason, "Git tags remain available separately") {
		t.Fatalf("SourceForge limitation should direct users to Tags: %#v", page.Availability)
	}
}
