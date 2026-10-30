package server

import (
	"idios/internal/domain"
	"net/http"
	"strconv"
)

func (s *Server) getGraph(w http.ResponseWriter, r *http.Request) {
	depth := 1
	if raw := r.URL.Query().Get("depth"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			depth = parsed
		}
	}
	if depth < 1 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}
	graph, err := s.app.Graph(requestContext(r), r.URL.Query().Get("from"), depth)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (s *Server) getGraphLayout(w http.ResponseWriter, r *http.Request) {
	layout, err := s.app.GraphLayout(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, layout)
}

func (s *Server) putGraphLayout(w http.ResponseWriter, r *http.Request) {
	var layout domain.GraphLayout
	if !decodeJSON(w, r, &layout) {
		return
	}
	saved, err := s.app.SaveGraphLayout(requestContext(r), layout)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) resetGraphLayout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Mode string `json:"mode"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	layout, err := s.app.ResetGraphLayout(requestContext(r), input.Mode)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, layout)
}

func (s *Server) getNotes(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListNotes(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Labels  []string `json:"labels,omitempty"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreateNoteWithLabels(requestContext(r), input.Title, input.Content, input.Labels)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteNote(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getConcepts(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListConcepts(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getConcept(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetConcept(requestContext(r), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getQuotes(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListQuotes(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getNextQuote(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.NextQuote(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	item, err := s.app.GetWorkspaceProfile(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getPins(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListWorkspacePins(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
