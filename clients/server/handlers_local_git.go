package server

import (
	"net/http"
	"strings"
)

func (s *Server) getLocalGitStatus(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.LocalGitService.Status(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) initializeLocalGit(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Branch string `json:"branch"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LocalGitService.Initialize(requestContext(r), r.PathValue("ref"), strings.TrimSpace(input.Branch)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"initialized": true})
}

func (s *Server) cloneLocalGit(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RemoteURL string `json:"remote_url"`
		Branch    string `json:"branch"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LocalGitService.Clone(requestContext(r), r.PathValue("ref"), input.RemoteURL, strings.TrimSpace(input.Branch)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"cloned": true})
}

func (s *Server) getLocalGitBranches(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.LocalGitService.Branches(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createLocalGitBranch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
		Base string `json:"base"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LocalGitService.CreateLocalBranch(requestContext(r), r.PathValue("ref"), input.Name, input.Base); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"created": true})
}

func (s *Server) switchLocalGitBranch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LocalGitService.SwitchBranch(requestContext(r), r.PathValue("ref"), input.Name); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"switched": true})
}

func (s *Server) stageLocalGitPaths(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Paths []string `json:"paths"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.LocalGitService.Stage(requestContext(r), r.PathValue("ref"), input.Paths); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"staged": true})
}

func (s *Server) createLocalGitCommit(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Message     string `json:"message"`
		AuthorName  string `json:"author_name"`
		AuthorEmail string `json:"author_email"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.LocalGitService.Commit(requestContext(r), r.PathValue("ref"), input.Message, input.AuthorName, input.AuthorEmail)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getLocalGitLog(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.LocalGitService.Log(requestContext(r), r.PathValue("ref"), queryInt(r, "limit", 20, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getLocalGitDiff(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.LocalGitService.Diff(requestContext(r), r.PathValue("ref"), r.URL.Query().Get("staged") == "true")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": item})
}
