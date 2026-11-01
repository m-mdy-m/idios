package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"idios/internal/application"
	"idios/internal/domain"
	"idios/internal/repositoryhub"
)

func (s *Server) getHubStatus(w http.ResponseWriter, r *http.Request) {
	connections, err := s.app.ListConnections(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	ready := s.app.CredentialStorageReady()
	writeJSON(w, http.StatusOK, map[string]any{
		"credential_storage_ready": ready,
		"credential_storage_error": s.app.CredentialStorageError(),
		"providers": []map[string]any{
			{"id": "github", "label": "GitHub", "oauth_available": application.OAuthConfigured(domain.RepositoryProviderGitHub), "capabilities": []string{"code.read", "branches.read", "commits.read", "issues.read", "issues.write", "changes.read", "changes.write", "changes.merge"}},
			{"id": "gitlab", "label": "GitLab", "oauth_available": application.OAuthConfigured(domain.RepositoryProviderGitLab), "capabilities": []string{"code.read", "branches.read", "commits.read", "issues.read", "issues.write", "changes.read", "changes.write", "changes.merge"}},
			{"id": "sourceforge", "label": "SourceForge", "capabilities": []string{"code.clone", "issues.read", "issues.write"}},
		},
		"connection_count": len(connections),
	})
}

func (s *Server) getHubConnections(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListConnections(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createHubConnection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider    string `json:"provider"`
		Host        string `json:"host"`
		AccessToken string `json:"access_token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.AccessToken) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "access_token is required"})
		return
	}
	item, err := s.app.ConnectProvider(requestContext(r), domain.RepositoryProvider(strings.TrimSpace(input.Provider)), input.Host, input.AccessToken)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) deleteHubConnection(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteConnection(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) discoverHubRepositories(w http.ResponseWriter, r *http.Request) {
	connectionID := strings.TrimSpace(r.URL.Query().Get("connection_id"))
	if connectionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "connection_id is required"})
		return
	}
	page := queryInt(r, "page", 1, 10000)
	items, err := s.app.DiscoverProviderRepositories(requestContext(r), connectionID, r.URL.Query().Get("q"), page)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": page})
}

func (s *Server) importHubRepository(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ConnectionID string `json:"connection_id"`
		ExternalID   string `json:"external_id"`
		FullPath     string `json:"full_path"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.ConnectionID) == "" || strings.TrimSpace(input.ExternalID) == "" || strings.TrimSpace(input.FullPath) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "connection_id, external_id and full_path are required"})
		return
	}
	item, err := s.app.ImportSelectedProviderRepository(requestContext(r), input.ConnectionID, input.ExternalID, input.FullPath)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getHubRepositories(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListHubRepositories(requestContext(r), r.URL.Query().Get("provider"), r.URL.Query().Get("q"), r.URL.Query().Get("collection_id"), r.URL.Query().Get("pinned") == "true")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getHubRepository(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetHubRepository(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteHubRepository(w http.ResponseWriter, r *http.Request) {
	if err := s.app.UnlinkRepository(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"unlinked": true})
}

func (s *Server) getHubPreferences(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetPreferences(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) patchHubPreferences(w http.ResponseWriter, r *http.Request) {
	var input struct {
		LocalAlias  string `json:"local_alias"`
		PrivateNote string `json:"private_note"`
		Pinned      bool   `json:"pinned"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.SavePreferences(requestContext(r), domain.HubRepositoryPreferences{
		RepositoryID: r.PathValue("ref"), LocalAlias: input.LocalAlias,
		PrivateNote: input.PrivateNote, Pinned: input.Pinned,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getHubProjects(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListHubProjects(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) linkHubProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID    string `json:"project_id"`
		Relationship string `json:"relationship"`
		Primary      bool   `json:"primary"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LinkHubRepositoryToProject(requestContext(r), input.ProjectID, r.PathValue("ref"), input.Relationship, input.Primary); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"linked": true})
}

func (s *Server) unlinkHubProject(w http.ResponseWriter, r *http.Request) {
	if err := s.app.UnlinkHubRepositoryFromProject(requestContext(r), r.PathValue("project"), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"unlinked": true})
}

func (s *Server) setHubProjectPrimary(w http.ResponseWriter, r *http.Request) {
	if err := s.app.SetPrimaryHubProjectRepository(requestContext(r), r.PathValue("project"), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"primary": true})
}

func (s *Server) getProjectHubRepositories(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListHubProjectRepositories(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getHubCollections(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListCollections(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) saveHubCollection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Position    int    `json:"position"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.SaveCollection(requestContext(r), domain.HubCollection{ID: r.PathValue("ref"), Name: input.Name, Description: input.Description, Position: input.Position})
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if r.PathValue("ref") == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, item)
}

func (s *Server) deleteHubCollection(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteCollection(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) addHubCollectionRepository(w http.ResponseWriter, r *http.Request) {
	if err := s.app.AddToCollection(requestContext(r), r.PathValue("collection"), r.PathValue("repo")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"added": true})
}
func (s *Server) removeHubCollectionRepository(w http.ResponseWriter, r *http.Request) {
	if err := s.app.RemoveFromCollection(requestContext(r), r.PathValue("collection"), r.PathValue("repo")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
}

func (s *Server) getHubRelations(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListRelations(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) createHubRelation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TargetRepositoryID string `json:"target_repository_id"`
		RelationType       string `json:"relation_type"`
		Description        string `json:"description"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateHubRepositoryRelation(requestContext(r), domain.HubRepositoryRelation{SourceRepositoryID: r.PathValue("ref"), TargetRepositoryID: input.TargetRepositoryID, RelationType: input.RelationType, Description: input.Description})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) deleteHubRelation(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteRelation(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) getHubBranches(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.RemoteBranches(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getHubReleases(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.RemoteReleases(
		requestContext(r), r.PathValue("ref"),
		queryInt(r, "page", 1, 10000), queryInt(r, "page_size", 20, 50),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getHubTags(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.RemoteTags(
		requestContext(r), r.PathValue("ref"),
		queryInt(r, "page", 1, 10000), queryInt(r, "page_size", 20, 50),
		r.URL.Query().Get("cursor"),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) compareHubTags(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.CompareRemoteTags(
		requestContext(r), r.PathValue("ref"),
		r.URL.Query().Get("from"), r.URL.Query().Get("to"),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) getHubTree(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.RemoteTree(
		requestContext(r), r.PathValue("ref"), r.URL.Query().Get("ref"),
		r.URL.Query().Get("path"), r.URL.Query().Get("tree_sha"),
		queryInt(r, "page", 1, 10000),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) getHubFile(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.RemoteFile(requestContext(r), r.PathValue("ref"), r.URL.Query().Get("path"), r.URL.Query().Get("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) putHubFile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path          string `json:"path"`
		Branch        string `json:"branch"`
		Content       string `json:"content"`
		CommitMessage string `json:"commit_message"`
		ExpectedSHA   string `json:"expected_sha"`
		CreateOnly    bool   `json:"create_only"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.WriteRemoteFile(requestContext(r), r.PathValue("ref"), repositoryhub.FileWrite{Path: input.Path, Branch: input.Branch, Content: input.Content, CommitMessage: input.CommitMessage, ExpectedSHA: input.ExpectedSHA, CreateOnly: input.CreateOnly})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) getHubCommits(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.RemoteCommits(requestContext(r), r.PathValue("ref"), r.URL.Query().Get("ref"), queryInt(r, "page", 1, 10000))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) getHubCommit(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.RemoteCommit(requestContext(r), r.PathValue("ref"), r.URL.Query().Get("ref"), r.PathValue("sha"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) getHubIssues(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.RemoteIssues(requestContext(r), r.PathValue("ref"), r.URL.Query().Get("state"), queryInt(r, "page", 1, 10000))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) createHubIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title              string            `json:"title"`
		Body               string            `json:"body"`
		Labels             []string          `json:"labels"`
		Status             string            `json:"status"`
		Assignee           string            `json:"assignee"`
		Private            bool              `json:"private"`
		DiscussionDisabled bool              `json:"discussion_disabled"`
		CustomFields       map[string]string `json:"custom_fields"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateRemoteIssueWithOptions(requestContext(r), r.PathValue("ref"), input.Title, input.Body, input.Labels, repositoryhub.IssueCreateOptions{
		Status: input.Status, Assignee: input.Assignee, Private: input.Private,
		DiscussionDisabled: input.DiscussionDisabled, CustomFields: input.CustomFields,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateHubIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title              *string           `json:"title"`
		Body               *string           `json:"body"`
		State              string            `json:"state"`
		Assignee           *string           `json:"assignee"`
		Labels             *[]string         `json:"labels"`
		Private            *bool             `json:"private"`
		DiscussionDisabled *bool             `json:"discussion_disabled"`
		CustomFields       map[string]string `json:"custom_fields"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid issue number"})
		return
	}
	item, err := s.app.UpdateRemoteIssue(requestContext(r), r.PathValue("ref"), number, repositoryhub.IssueUpdateOptions{
		Title: input.Title, Body: input.Body, State: input.State, Assignee: input.Assignee,
		Labels: input.Labels, Private: input.Private, DiscussionDisabled: input.DiscussionDisabled,
		CustomFields: input.CustomFields,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) closeHubIssue(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid issue number"})
		return
	}
	if err := s.app.CloseRemoteIssue(requestContext(r), r.PathValue("ref"), number); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"closed": true})
}

func (s *Server) commentHubIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Body string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid issue number"})
		return
	}
	if err := s.app.CommentRemoteIssue(requestContext(r), r.PathValue("ref"), number, input.Body); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"commented": true})
}
func (s *Server) getHubChanges(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.RemoteChanges(requestContext(r), r.PathValue("ref"), r.URL.Query().Get("state"), queryInt(r, "page", 1, 10000))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) createHubChange(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateRemoteChange(requestContext(r), r.PathValue("ref"), input.Title, input.Body, input.SourceBranch, input.TargetBranch)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) closeHubChange(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid change request number"})
		return
	}
	if err := s.app.CloseRemoteChange(requestContext(r), r.PathValue("ref"), number); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"closed": true})
}

func (s *Server) approveHubChange(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid change request number"})
		return
	}
	if err := s.app.ApproveRemoteChange(requestContext(r), r.PathValue("ref"), number); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"approved": true})
}
func (s *Server) mergeHubChange(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid change request number"})
		return
	}
	if err := s.app.MergeRemoteChange(requestContext(r), r.PathValue("ref"), number); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"merged": true})
}

func queryInt(r *http.Request, key string, fallback, max int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 1 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

// Keep the provider package imported here so request DTOs remain local to the HTTP boundary.
var _ repositoryhub.FileWrite

func (s *Server) syncHubRepository(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.SyncHubRepository(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) startHubOAuth(w http.ResponseWriter, r *http.Request) {
	if !s.validHost(r.Host) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "OAuth may only start from this local IDIOS origin"})
		return
	}
	provider := domain.RepositoryProvider(strings.TrimSpace(r.URL.Query().Get("provider")))
	host := strings.TrimSpace(r.URL.Query().Get("host"))
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	redirectURI := scheme + "://" + r.Host + "/api/hub/oauth/callback"
	authorizationURL, err := s.app.OAuthAuthorizationURL(requestContext(r), provider, host, redirectURI)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authorization_url": authorizationURL})
}

func (s *Server) completeHubOAuth(w http.ResponseWriter, r *http.Request) {
	if !s.validHost(r.Host) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "OAuth callback did not arrive on the local IDIOS origin"})
		return
	}
	if providerError := strings.TrimSpace(r.URL.Query().Get("error")); providerError != "" {
		description := strings.TrimSpace(r.URL.Query().Get("error_description"))
		http.Redirect(w, r, "/repositories?hub_error="+url.QueryEscape(providerError+": "+description), http.StatusSeeOther)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	connection, err := s.app.CompleteProviderOAuth(requestContext(r), state, code)
	if err != nil {
		http.Redirect(w, r, "/repositories?hub_error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/repositories?hub_connected="+url.QueryEscape(string(connection.Provider)), http.StatusSeeOther)
}
