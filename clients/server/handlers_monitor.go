package server

import (
	"errors"
	"fmt"
	"idios/internal/diagnostics"
	"idios/internal/domain"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// getLogs returns the shared local diagnostics stream used by the Web client and service.
func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000 {
			writeError(w, fmt.Errorf("limit must be between 1 and 1000"))
			return
		}
		limit = parsed
	}
	lines := diagnostics.ReadRecent(limit)
	if len(lines) == 1 && (lines[0] == "The log file is empty." || strings.HasPrefix(lines[0], "No log file yet.")) {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":  diagnostics.Path(),
		"count": len(lines),
		"lines": lines,
	})
}

func (s *Server) clearLogs(w http.ResponseWriter, _ *http.Request) {
	if err := diagnostics.Clear(); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getMonitoring is a local operational snapshot, not a remote telemetry endpoint. It exposes
// process and request counters only to the loopback-bound API and performs a fresh database ping.
func (s *Server) getMonitoring(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	var databaseError error
	if s.app == nil {
		databaseError = errors.New("application is not initialized")
	} else {
		databaseError = s.app.Ping(requestContext(r))
	}
	databaseLatency := float64(time.Since(started)) / float64(time.Millisecond)
	databaseStatus := "ok"
	status := "ok"
	if databaseError != nil {
		databaseStatus, status = "error", "degraded"
	}

	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	s.mu.RLock()
	startedAt := s.startedAt
	requestCount := s.requestCount
	requestErrors := s.requestErrorCount
	totalRequestNanos := s.totalRequestNanos
	lastRequestAt := s.lastRequestAt
	s.mu.RUnlock()
	uptime := 0.0
	if !startedAt.IsZero() {
		uptime = time.Since(startedAt).Seconds()
	}
	averageRequestMS := 0.0
	if requestCount > 0 {
		averageRequestMS = float64(totalRequestNanos) / float64(requestCount) / float64(time.Millisecond)
	}
	logStats := diagnostics.Stats()
	databaseDetail := ""
	if databaseError != nil {
		databaseDetail = databaseError.Error()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      status,
		"observed_at": time.Now().UTC(),
		"database":    map[string]any{"status": databaseStatus, "latency_ms": databaseLatency, "error": databaseDetail},
		"process":     map[string]any{"uptime_seconds": uptime, "goroutines": runtime.NumGoroutine(), "memory_alloc_bytes": memory.Alloc, "heap_in_use_bytes": memory.HeapInuse},
		"http":        map[string]any{"requests_total": requestCount, "requests_failed": requestErrors, "average_request_ms": averageRequestMS, "last_request_at": lastRequestAt},
		"logs":        map[string]any{"path": logStats.Path, "size_bytes": logStats.SizeBytes, "modified_at": logStats.ModifiedAt},
	})
}

func (s *Server) getHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Ping(requestContext(r)); err != nil {
		writeError(w, err)
		return
	}
	addr, _ := s.Address()
	s.mu.RLock()
	refresh := s.cfg.Server.RefreshIntervalSeconds
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "version": s.opts.Version, "address": "http://" + addr,
		"database": "ok", "started_at": s.startedAt, "refresh_interval_seconds": refresh,
	})
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)
	repositories, err := s.app.ListRepositories(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	issues, err := s.app.ListIssues(ctx, "", "")
	if err != nil {
		writeError(w, err)
		return
	}
	projects, err := s.app.ListProjects(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	notes, err := s.app.ListNotes(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	notifications, err := s.app.ListNotifications(ctx, 100)
	if err != nil {
		writeError(w, err)
		return
	}
	activity, err := s.app.ListActivity(ctx, "", "", 12)
	if err != nil {
		writeError(w, err)
		return
	}
	profile, err := s.app.GetWorkspaceProfile(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	metrics := map[string]int{
		"repositories":         len(repositories),
		"issues":               len(issues),
		"projects":             len(projects),
		"notes":                len(notes),
		"open_issues":          0,
		"in_progress_issues":   0,
		"done_issues":          0,
		"cancelled_issues":     0,
		"blocked_issues":       0,
		"unread_notifications": 0,
	}
	for _, issue := range issues {
		switch issue.Status {
		case domain.IssueOpen:
			metrics["open_issues"]++
		case domain.IssueInProgress:
			metrics["in_progress_issues"]++
		case domain.IssueDone:
			metrics["done_issues"]++
		case domain.IssueCancelled:
			metrics["cancelled_issues"]++
		}
		if issue.Blocked {
			metrics["blocked_issues"]++
		}
	}
	unread := 0
	for _, notification := range notifications {
		if notification.ReadAt == nil {
			unread++
		}
	}
	metrics["unread_notifications"] = unread
	writeJSON(w, http.StatusOK, map[string]any{
		"profile": profile, "repositories": repositories, "issues": issues, "projects": projects,
		"notes": notes, "notifications": notifications, "activity": activity, "metrics": metrics,
	})
}
