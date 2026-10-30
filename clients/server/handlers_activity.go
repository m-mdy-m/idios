package server

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) getNotifications(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	items, err := s.app.ListNotifications(requestContext(r), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) markNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := s.app.MarkNotificationsRead(requestContext(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
}

func (s *Server) getActivity(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	items, err := s.app.ListActivity(requestContext(r), r.URL.Query().Get("type"), r.URL.Query().Get("id"), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// getActivityDays serves a database-wide calendar aggregate so older dates are not lost
// simply because the visible event feed is paginated.
func (s *Server) getActivityDays(w http.ResponseWriter, r *http.Request) {
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	timezone := strings.TrimSpace(r.URL.Query().Get("timezone"))
	if timezone == "" {
		timezone = "UTC"
	}
	days, err := s.app.ListActivityDays(requestContext(r), from, to, timezone)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, days)
}

// recordActivityPresence records one marker for the browser's local date. The store deduplicates
// it across StrictMode re-renders, route changes, and browser refreshes.
func (s *Server) recordActivityPresence(w http.ResponseWriter, r *http.Request) {
	var input struct {
		LocalDate string `json:"local_date"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.app.RecordDailyPresence(requestContext(r), strings.TrimSpace(input.LocalDate)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}
