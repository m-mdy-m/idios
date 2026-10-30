package server

import (
	"net/http"
)

func (s *Server) getIssues(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListIssues(requestContext(r), r.URL.Query().Get("repo"), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Repository  string `json:"repository"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Parent      string `json:"parent,omitempty"`
		DueDate     string `json:"due_date,omitempty"`
		Priority    string `json:"priority,omitempty"`
		Type        string `json:"type,omitempty"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateIssueWithTypeAndBranch(requestContext(r), input.Repository, input.Title, input.Description, input.Parent, input.DueDate, input.Priority, "", input.Type)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) startIssue(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.StartIssue(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) cancelIssue(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.CancelIssue(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getProjects(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListProjects(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateProject(requestContext(r), input.Name, input.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getProjectRepositories(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListProjectRepositories(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) linkProjectRepository(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Repository   string `json:"repository"`
		Relationship string `json:"relationship"`
		Primary      bool   `json:"primary"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LinkRepositoryToProject(requestContext(r), r.PathValue("ref"), input.Repository, input.Relationship, input.Primary); err != nil {
		writeError(w, err)
		return
	}
	items, err := s.app.ListProjectRepositories(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, items)
}

func (s *Server) unlinkProjectRepository(w http.ResponseWriter, r *http.Request) {
	if err := s.app.UnlinkRepositoryFromProject(requestContext(r), r.PathValue("ref"), r.PathValue("repoRef")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"unlinked": true})
}

func (s *Server) setPrimaryProjectRepository(w http.ResponseWriter, r *http.Request) {
	if err := s.app.SetPrimaryProjectRepository(requestContext(r), r.PathValue("ref"), r.PathValue("repoRef")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"primary": true})
}

func (s *Server) getProjectIssues(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListProjectIssues(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) addProjectIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Issue string `json:"issue"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.AddIssueToProject(requestContext(r), r.PathValue("ref"), input.Issue); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetProject(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getIssue(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetIssue(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getIssueDependencies(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListDependencies(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getPullRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListPullRequests(requestContext(r), r.URL.Query().Get("repo"), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getPullRequestIssues(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListPullRequestIssues(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getPullRequest(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetPullRequest(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getLabels(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListLabels(requestContext(r), r.URL.Query().Get("scope"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getIssueLabels(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListIssueLabels(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getMilestones(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListMilestones(requestContext(r), r.URL.Query().Get("scope"), r.URL.Query().Get("owner"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getIssueMilestones(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListIssueMilestones(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
