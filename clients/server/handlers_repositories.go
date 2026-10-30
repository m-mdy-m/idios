package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func (s *Server) getRepositories(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListRepositories(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createRepository(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateRepositoryWithDefaultBranch(requestContext(r), input.Name, input.Description, input.DefaultBranch)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getRepositoryFiles(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListRepositoryFiles(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) putRepositoryFile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.SaveRepositoryFile(requestContext(r), r.PathValue("ref"), input.Path, input.Content)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteRepositoryFile(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteRepositoryFile(requestContext(r), r.PathValue("ref"), r.URL.Query().Get("path")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) getRepositoryTags(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListRepositoryTags(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createRepositoryTag(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateRepositoryTag(requestContext(r), r.PathValue("ref"), input.Name, input.Message)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getRepositoryTag(w http.ResponseWriter, r *http.Request) {
	tag, err := s.app.GetRepositoryTag(requestContext(r), r.PathValue("ref"), r.PathValue("tag"))
	if err != nil {
		writeError(w, err)
		return
	}
	files, err := s.app.GetRepositoryTagFiles(requestContext(r), tag.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag, "files": files})
}

func (s *Server) compareRepositoryTag(w http.ResponseWriter, r *http.Request) {
	tag, err := s.app.GetRepositoryTag(requestContext(r), r.PathValue("ref"), r.PathValue("tag"))
	if err != nil {
		writeError(w, err)
		return
	}
	changes, err := s.app.CompareRepositoryTags(requestContext(r), r.PathValue("ref"), r.PathValue("tag"))
	if err != nil {
		writeError(w, err)
		return
	}
	history, err := s.app.ListTagHistory(requestContext(r), tag.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag, "previous_tag": tag.PreviousTagName, "changes": changes, "history": history})
}

func (s *Server) downloadRepositoryTag(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)
	repoRef, tagRef := r.PathValue("ref"), r.PathValue("tag")
	tag, err := s.app.GetRepositoryTag(ctx, repoRef, tagRef)
	if err != nil {
		writeError(w, err)
		return
	}
	files, err := s.app.GetRepositoryTagFiles(ctx, tag.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	history, err := s.app.ListTagHistory(ctx, tag.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	var body bytes.Buffer
	zw := zip.NewWriter(&body)
	add := func(name string, data []byte) error {
		clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
		if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
			return fmt.Errorf("unsafe archive path")
		}
		w, e := zw.Create(clean)
		if e != nil {
			return e
		}
		_, e = w.Write(data)
		return e
	}
	for _, file := range files {
		if err := add(file.Path, []byte(file.Content)); err != nil {
			_ = zw.Close()
			writeError(w, err)
			return
		}
	}
	manifest := map[string]any{"repository": tag.RepositoryName, "tag": tag.Name, "message": tag.Message, "created_at": tag.CreatedAt, "completion_sequence": tag.CompletionSequence, "completion_id": tag.CompletionID, "previous_tag": tag.PreviousTagName, "previous_tag_download": func() string {
		if tag.PreviousTagName == "" {
			return ""
		}
		return "/api/repositories/" + url.PathEscape(repoRef) + "/tags/" + url.PathEscape(tag.PreviousTagName) + "/archive"
	}(), "note": "This is an IDIOS simulated repository snapshot, not a Git bundle."}
	metadata, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = zw.Close()
		writeError(w, err)
		return
	}
	if err = add(".idios/tag.json", metadata); err != nil {
		_ = zw.Close()
		writeError(w, err)
		return
	}
	historyBytes, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		_ = zw.Close()
		writeError(w, err)
		return
	}
	if err = add(".idios/history.json", historyBytes); err != nil {
		_ = zw.Close()
		writeError(w, err)
		return
	}
	if err = zw.Close(); err != nil {
		writeError(w, err)
		return
	}
	filename := strings.NewReplacer(" ", "-", "\r", "", "\n", "", "\"", "").Replace(tag.RepositoryName) + "-" + tag.Name + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filename, "\"", "")+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body.Bytes())
}

func (s *Server) getRepository(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetRepository(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getRepositoryBranches(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListBranches(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getRepositorySettings(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.GetRepositorySettings(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getBranches(w http.ResponseWriter, r *http.Request) {
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repo query parameter is required"})
		return
	}
	items, err := s.app.ListBranches(requestContext(r), repo)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getCompletions(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListCompletions(requestContext(r), r.URL.Query().Get("repo"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
