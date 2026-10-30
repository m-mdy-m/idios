package server

import (
	"idios/internal/features"
	"idios/internal/protocol"
	"net/http"
)

func (s *Server) getProtocolCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"protocol_version": protocol.Version, "commands": protocol.Catalog()})
}

// getFeatureCatalog exposes the shared user-facing feature inventory to the Web client.
// Routes and navigation labels are metadata only; command behavior stays in Application/Protocol.
func (s *Server) getFeatureCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"catalog_version": 1, "features": features.Catalog()})
}

func (s *Server) getProtocolOptions(w http.ResponseWriter, r *http.Request) {
	items, err := protocol.Options(requestContext(r), s.app, r.URL.Query().Get("source"), r.URL.Query().Get("entity_type"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) executeProtocolCommand(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Command string         `json:"command"`
		Input   map[string]any `json:"input"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Input == nil {
		input.Input = map[string]any{}
	}
	result, err := protocol.Execute(requestContext(r), s.app, input.Command, input.Input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"protocol_version": protocol.Version, "command": input.Command, "result": result})
}
