// Package server exposes the shared IDIOS application through a loopback-only HTTP API.
package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/desktopnotify"
	"idios/internal/diagnostics"
	"idios/internal/domain"
	"idios/internal/features"
	"idios/internal/protocol"
	"idios/internal/reminders"
	"idios/internal/updater"
)

// Options configures the local HTTP adapter. The application service and its store are owned by
// the caller, so the server never closes or replaces the Engine.
type Options struct {
	Config     config.AppConfig
	ConfigPath string
	DBPath     string
	Version    string
	Address    string
	OnStart    func(string)
}

// Server serves API requests, static UI assets, and scheduled-work processing over one listener.
type Server struct {
	app               *application.Service
	opts              Options
	startedAt         time.Time
	listenAddr        string
	mu                sync.RWMutex
	cfg               config.AppConfig
	http              *http.Server
	stopOnce          sync.Once
	updateState       string
	updateError       string
	updateVersion     string
	requestCount      uint64
	requestErrorCount uint64
	totalRequestNanos uint64
	lastRequestAt     time.Time
}

// New creates an HTTP adapter over the existing Application service.
func New(app *application.Service, opts Options) *Server {
	return &Server{app: app, opts: opts, cfg: opts.Config}
}

// Address returns the configured loopback address.
func (s *Server) Address() (string, error) {
	s.mu.RLock()
	if s.listenAddr != "" {
		addr := s.listenAddr
		s.mu.RUnlock()
		return addr, nil
	}
	cfg := s.cfg
	s.mu.RUnlock()
	if s.opts.Address != "" {
		return validateLoopbackAddress(s.opts.Address)
	}
	host := strings.TrimSpace(cfg.Server.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Server.Port
	if port < 1 || port > 65535 {
		port = 43170
	}
	return validateLoopbackAddress(net.JoinHostPort(host, strconv.Itoa(port)))
}

func validateLoopbackAddress(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid server address %q: expected host:port", addr)
	}
	if portNum, err := strconv.Atoi(port); err != nil || portNum < 1 || portNum > 65535 {
		return "", fmt.Errorf("invalid server port %q", port)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("IDIOS server must bind to loopback only; got host %q", host)
	}
	return net.JoinHostPort(host, port), nil
}

// Run starts the local server and exits after ctx is cancelled or the HTTP server is stopped.
func (s *Server) Run(ctx context.Context) error {
	if s.app == nil {
		return errors.New("IDIOS server requires an initialized Application service")
	}
	addr, err := s.Address()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	s.mu.Lock()
	s.listenAddr = addr
	s.mu.Unlock()
	if err := writeActiveState(addr); err != nil {
		_ = listener.Close()
		return fmt.Errorf("record active server state: %w", err)
	}
	defer removeActiveState(addr)
	s.startedAt = time.Now()
	mux := s.routes()
	s.http = &http.Server{
		Addr:              addr,
		Handler:           s.requestLog(s.securityHeaders(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if s.opts.OnStart != nil {
		s.opts.OnStart("http://" + addr)
	}
	diagnostics.Write("INFO", "local HTTP server started at http://"+addr, s.debugEnabled())
	if err := s.ensureQuoteReminderSchedule(time.Now()); err != nil {
		// The HTTP API remains available if the optional reminder-state write fails;
		// the scheduler will retry on its first tick and all interactive controls still work.
		_ = err
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go s.runScheduler(runCtx)
	serverExited := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.http.Shutdown(shutdownCtx)
		case <-serverExited:
		}
	}()
	err = s.http.Serve(listener)
	close(serverExited)
	cancelRun()
	if errors.Is(err, http.ErrServerClosed) {
		diagnostics.Write("INFO", "local HTTP server stopped", s.debugEnabled())
		return nil
	}
	diagnostics.Write("ERROR", "local HTTP server stopped unexpectedly: "+err.Error(), s.debugEnabled())
	return err
}

func (s *Server) runScheduler(ctx context.Context) {
	// Jobs respect the configured polling interval. Quote reminders are due-time based,
	// so a small scheduler tick keeps them independent of whether Web or TUI is open.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	nextJobsCheck := time.Now()
	interval := time.Duration(0)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			// The TUI can edit the shared config file directly while the service runs.
			// Reload it on the server tick so both clients see one effective config.
			s.refreshConfig()
			s.mu.RLock()
			cfg := s.cfg
			s.mu.RUnlock()
			currentInterval := jobPollingInterval(cfg)
			if currentInterval != interval {
				interval = currentInterval
				nextJobsCheck = now
			}
			if !now.Before(nextJobsCheck) {
				_, _ = s.app.ProcessDueJobs(ctx, now, cfg.Notifications.Enabled)
				nextJobsCheck = now.Add(interval)
			}
			if cfg.Notifications.Enabled && cfg.Notifications.PlanReminders.Enabled {
				if period := duePlanReminderPeriod(now, cfg.Notifications.PlanReminders); period != "" {
					date := now.Format("2006-01-02")
					item, err := s.app.CreatePlanReminder(ctx, date, period, now)
					if err == nil && item != nil && cfg.Notifications.PlanReminders.DesktopEnabled {
						// On Windows, clicking the native reminder opens the selected plan date. The
						// Plans page opens the plan detail (or a chooser when the date has several plans).
						actionURL := fmt.Sprintf("http://127.0.0.1:%d/plans?date=%s", cfg.Server.Port, date)
						_ = desktopnotify.SendWithAction(ctx, "IDIOS · "+item.Title, item.Body, actionURL)
					}
				}
			}
			if reminders.Due(cfg.Notifications.QuoteReminders, now) {
				s.processQuoteReminder(ctx, now)
			}
		}
	}
}

func (s *Server) refreshConfig() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if latest, err := config.LoadAppConfig(); err == nil {
		s.cfg = latest
	}
}

func (s *Server) ensureQuoteReminderSchedule(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	if !reminders.EnsureSchedule(&next, now) {
		return nil
	}
	if err := config.SaveAppConfig(next); err != nil {
		return fmt.Errorf("save initial quote-reminder schedule: %w", err)
	}
	s.cfg = next
	return nil
}

func (s *Server) processQuoteReminder(ctx context.Context, now time.Time) {
	// Reserve the next time before doing storage work. If the process is interrupted after
	// this point, it won't emit the same reminder repeatedly on every scheduler tick.
	s.mu.Lock()
	current := s.cfg
	r := current.Notifications.QuoteReminders
	if !reminders.Due(r, now) {
		s.mu.Unlock()
		return
	}
	reservedAt := reminders.NextAt(now, r).Format(time.RFC3339Nano)
	current.Notifications.QuoteReminders.NextAt = reservedAt
	if err := config.SaveAppConfig(current); err != nil {
		s.mu.Unlock()
		return
	}
	s.cfg = current
	s.mu.Unlock()

	items, err := s.app.ListQuotes(ctx)
	if err != nil || len(items) == 0 {
		return
	}
	item := reminders.Choose(items, r.LastQuoteID)
	body := reminders.FormatText(item)
	if _, err := s.app.CreateNotification(ctx, "Quote of the moment", body, "quote"); err != nil {
		return
	}
	if r.DesktopEnabled {
		_ = desktopnotify.Send(ctx, "IDIOS · Quote of the moment", body)
	}

	// Store the last quote without overwriting unrelated config changes made through Web.
	s.mu.Lock()
	updated := s.cfg
	if updated.Notifications.QuoteReminders.Enabled && updated.Notifications.QuoteReminders.NextAt == reservedAt {
		updated.Notifications.QuoteReminders.LastQuoteID = item.ID
		if config.SaveAppConfig(updated) == nil {
			s.cfg = updated
		}
	}
	s.mu.Unlock()
}

func duePlanReminderPeriod(now time.Time, settings config.PlanReminderConfig) string {
	// Select only the most recent configured check-in. Starting the service late in the day
	// must not create a burst of all three old reminders at once.
	candidates := []struct{ period, clock string }{{"morning", settings.MorningTime}, {"midday", settings.MiddayTime}, {"evening", settings.EveningTime}}
	selected := ""
	var latest time.Time
	for _, candidate := range candidates {
		parsed, err := time.Parse("15:04", candidate.clock)
		if err != nil {
			continue
		}
		due := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
		if !now.Before(due) && (selected == "" || due.After(latest)) {
			selected, latest = candidate.period, due
		}
	}
	return selected
}

func jobPollingInterval(cfg config.AppConfig) time.Duration {
	minutes := cfg.Notifications.PollIntervalMinutes
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 1440 {
		minutes = 1440
	}
	return time.Duration(minutes) * time.Minute
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.getHealth)
	mux.HandleFunc("GET /api/update/check", s.checkUpdate)
	mux.HandleFunc("GET /api/update/status", s.getUpdateStatus)
	mux.HandleFunc("POST /api/update/install", s.installUpdate)
	mux.HandleFunc("GET /api/logs", s.getLogs)
	mux.HandleFunc("DELETE /api/logs", s.clearLogs)
	mux.HandleFunc("GET /api/monitoring", s.getMonitoring)
	mux.HandleFunc("GET /api/protocol", s.getProtocolCatalog)
	mux.HandleFunc("GET /api/features", s.getFeatureCatalog)
	mux.HandleFunc("GET /api/protocol/options", s.getProtocolOptions)
	mux.HandleFunc("POST /api/protocol/execute", s.executeProtocolCommand)
	mux.HandleFunc("GET /api/dashboard", s.getDashboard)
	mux.HandleFunc("GET /api/repositories", s.getRepositories)
	mux.HandleFunc("POST /api/repositories", s.createRepository)
	mux.HandleFunc("GET /api/repositories/{ref}", s.getRepository)
	mux.HandleFunc("GET /api/repositories/{ref}/branches", s.getRepositoryBranches)
	mux.HandleFunc("GET /api/repositories/{ref}/settings", s.getRepositorySettings)
	mux.HandleFunc("GET /api/repositories/{ref}/files", s.getRepositoryFiles)
	mux.HandleFunc("PUT /api/repositories/{ref}/files", s.putRepositoryFile)
	mux.HandleFunc("DELETE /api/repositories/{ref}/files", s.deleteRepositoryFile)
	mux.HandleFunc("GET /api/repositories/{ref}/tags", s.getRepositoryTags)
	mux.HandleFunc("POST /api/repositories/{ref}/tags", s.createRepositoryTag)
	mux.HandleFunc("GET /api/repositories/{ref}/tags/{tag}", s.getRepositoryTag)
	mux.HandleFunc("GET /api/repositories/{ref}/tags/{tag}/compare", s.compareRepositoryTag)
	mux.HandleFunc("GET /api/repositories/{ref}/tags/{tag}/archive", s.downloadRepositoryTag)
	mux.HandleFunc("GET /api/issues", s.getIssues)
	mux.HandleFunc("POST /api/issues", s.createIssue)
	mux.HandleFunc("GET /api/issues/{ref}", s.getIssue)
	mux.HandleFunc("GET /api/issues/{ref}/dependencies", s.getIssueDependencies)
	mux.HandleFunc("POST /api/issues/{ref}/start", s.startIssue)
	mux.HandleFunc("POST /api/issues/{ref}/cancel", s.cancelIssue)
	mux.HandleFunc("GET /api/projects", s.getProjects)
	mux.HandleFunc("POST /api/projects", s.createProject)
	mux.HandleFunc("GET /api/projects/{ref}", s.getProject)
	mux.HandleFunc("GET /api/projects/{ref}/repositories", s.getProjectRepositories)
	mux.HandleFunc("POST /api/projects/{ref}/repositories", s.linkProjectRepository)
	mux.HandleFunc("PUT /api/projects/{ref}/repositories/{repoRef}/primary", s.setPrimaryProjectRepository)
	mux.HandleFunc("DELETE /api/projects/{ref}/repositories/{repoRef}", s.unlinkProjectRepository)
	mux.HandleFunc("GET /api/projects/{ref}/issues", s.getProjectIssues)
	mux.HandleFunc("POST /api/projects/{ref}/issues", s.addProjectIssue)
	mux.HandleFunc("GET /api/pull-requests", s.getPullRequests)
	mux.HandleFunc("GET /api/pull-requests/{ref}", s.getPullRequest)
	mux.HandleFunc("GET /api/pull-requests/{ref}/issues", s.getPullRequestIssues)
	mux.HandleFunc("GET /api/concepts", s.getConcepts)
	mux.HandleFunc("GET /api/concepts/{ref}", s.getConcept)
	mux.HandleFunc("GET /api/branches", s.getBranches)
	mux.HandleFunc("GET /api/completions", s.getCompletions)
	mux.HandleFunc("GET /api/labels", s.getLabels)
	mux.HandleFunc("GET /api/issue-labels", s.getIssueLabels)
	mux.HandleFunc("GET /api/milestones", s.getMilestones)
	mux.HandleFunc("GET /api/issue-milestones", s.getIssueMilestones)
	mux.HandleFunc("GET /api/graph", s.getGraph)
	mux.HandleFunc("GET /api/graph/layout", s.getGraphLayout)
	mux.HandleFunc("PUT /api/graph/layout", s.putGraphLayout)
	mux.HandleFunc("POST /api/graph/layout/reset", s.resetGraphLayout)
	mux.HandleFunc("GET /api/plans", s.getPlans)
	mux.HandleFunc("POST /api/plans", s.createPlan)
	mux.HandleFunc("PUT /api/plans/{ref}", s.updatePlan)
	mux.HandleFunc("DELETE /api/plans/{ref}", s.deletePlan)
	mux.HandleFunc("POST /api/plans/{ref}/tasks", s.addPlanTask)
	mux.HandleFunc("PATCH /api/plans/{ref}/tasks/{taskRef}", s.setPlanTaskDone)
	mux.HandleFunc("GET /api/notes", s.getNotes)
	mux.HandleFunc("POST /api/notes", s.createNote)
	mux.HandleFunc("DELETE /api/notes/{ref}", s.deleteNote)
	mux.HandleFunc("GET /api/quotes", s.getQuotes)
	mux.HandleFunc("GET /api/quotes/next", s.getNextQuote)
	mux.HandleFunc("GET /api/jobs", s.getJobs)
	mux.HandleFunc("GET /api/profile", s.getProfile)
	mux.HandleFunc("GET /api/pins", s.getPins)
	mux.HandleFunc("GET /api/notifications", s.getNotifications)
	mux.HandleFunc("POST /api/notifications/read", s.markNotificationsRead)
	mux.HandleFunc("GET /api/activity", s.getActivity)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("PUT /api/config", s.updateConfig)
	mux.HandleFunc("POST /api/server/shutdown", s.shutdown)
	// {$} makes the root route exact. A plain "GET /" is a subtree pattern
	// in net/http and conflicts with the catch-all SPA route below.
	mux.HandleFunc("GET /{$}", s.serveUI)
	mux.HandleFunc("GET /{path...}", s.serveUI)
	return mux
}

// getLogs returns the shared local diagnostics stream used by the TUI and Web clients.
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

type statusRecorder struct {
	http.ResponseWriter
	status int
	body   []byte // bounded copy of error responses only, for actionable local diagnostics
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.status >= http.StatusBadRequest && len(w.body) < 2048 {
		remaining := 2048 - len(w.body)
		captured := body
		if len(captured) > remaining {
			captured = captured[:remaining]
		}
		w.body = append(w.body, captured...)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) debugEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Debug
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		elapsed := time.Since(started)
		level := "DEBUG"
		if status >= http.StatusInternalServerError {
			level = "ERROR"
		} else if status >= http.StatusBadRequest {
			level = "WARN"
		}
		s.mu.Lock()
		s.requestCount++
		if status >= http.StatusBadRequest {
			s.requestErrorCount++
		}
		s.totalRequestNanos += uint64(max(0, elapsed.Nanoseconds()))
		s.lastRequestAt = time.Now().UTC()
		s.mu.Unlock()
		// Diagnostic reads and successful clear operations must not create a feedback loop in
		// the log stream. Failed requests to these routes are still recorded below.
		successfulDiagnosticRequest := status < http.StatusBadRequest && r.Method == http.MethodGet && (r.URL.Path == "/api/logs" || r.URL.Path == "/api/monitoring")
		successfulClearRequest := status < http.StatusBadRequest && r.Method == http.MethodDelete && r.URL.Path == "/api/logs"
		if !successfulDiagnosticRequest && !successfulClearRequest {
			message := fmt.Sprintf("%s %s -> %d (%s)", r.Method, r.URL.Path, status, elapsed.Round(time.Millisecond))
			if status >= http.StatusBadRequest {
				var responseError struct {
					Error string `json:"error"`
				}
				if json.Unmarshal(recorder.body, &responseError) == nil && strings.TrimSpace(responseError.Error) != "" {
					// Errors are bounded and line-sanitized. Request bodies and successful responses
					// are never captured or logged.
					detail := strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(responseError.Error))
					if len(detail) > 240 {
						detail = detail[:240] + "…"
					}
					message += ": " + detail
				}
			}
			diagnostics.Write(level, message, s.debugEnabled())
		}
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			if !s.validHost(r.Host) {
				http.Error(w, "invalid local Host header", http.StatusForbidden)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && !s.validOrigin(origin, r.Host) {
				http.Error(w, "cross-origin requests are not allowed", http.StatusForbidden)
				return
			}
			if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
				http.Error(w, "cross-site requests are not allowed", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validHost(requestHost string) bool {
	addr, err := s.Address()
	if err != nil {
		return false
	}
	_, wantPort, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	host, port, err := net.SplitHostPort(requestHost)
	if err != nil || port != wantPort {
		return false
	}
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (s *Server) validOrigin(raw, requestHost string) bool {
	origin, err := url.Parse(raw)
	if err != nil || origin.Scheme != "http" || origin.User != nil || origin.Path != "" && origin.Path != "/" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	host := strings.ToLower(origin.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return false
	}
	_, requestPort, err := net.SplitHostPort(requestHost)
	if err != nil {
		return false
	}
	originPort := origin.Port()
	if originPort == "" {
		originPort = "80"
	}
	if originPort == requestPort {
		return true
	}
	// The only cross-port exception is the Vite development server running on loopback.
	return originPort == "5173"
}

func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "shutting_down"})
	s.stopOnce.Do(func() {
		go func() {
			time.Sleep(80 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.http.Shutdown(ctx)
		}()
	})
}

func requestContext(r *http.Request) context.Context { return r.Context() }

type updateReleaseResponse struct {
	Version string `json:"version"`
	Tag     string `json:"tag"`
	URL     string `json:"url"`
	Notes   string `json:"notes"`
}

func (s *Server) currentVersion() string {
	if version := strings.TrimSpace(s.opts.Version); version != "" {
		return version
	}
	return updater.Version
}

// checkUpdate uses the same release and semantic-version logic as the TUI updater.
func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	release, err := updater.Check(requestContext(r), s.currentVersion())
	if err != nil {
		writeError(w, fmt.Errorf("check for IDIOS updates: %w", err))
		return
	}
	response := map[string]any{"current_version": s.currentVersion(), "update_available": release != nil}
	if release != nil {
		response["release"] = updateReleaseResponse{Version: release.Version, Tag: release.Tag, URL: release.URL, Notes: release.Notes}
	}
	writeJSON(w, http.StatusOK, response)
}

// getUpdateStatus exposes the local update workflow state so a Web client can observe
// verification and restart without holding an HTTP request open during downloads.
func (s *Server) getUpdateStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	state, detail, version := s.updateState, s.updateError, s.updateVersion
	s.mu.RUnlock()
	if state == "" {
		state = "idle"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": state, "error": detail, "version": version})
}

// installUpdate starts the same verified release updater as TUI. Work continues in a Go
// goroutine so the browser gets an immediate acknowledgement instead of timing out while a
// large release archive is downloaded. The process restarts in web mode after staging.
func (s *Server) installUpdate(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	if s.updateState == "checking" || s.updateState == "downloading" || s.updateState == "scheduled" {
		state, version := s.updateState, s.updateVersion
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "an IDIOS update is already " + state, "status": state, "version": version})
		return
	}
	s.updateState, s.updateError, s.updateVersion = "checking", "", ""
	s.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "checking"})
	go s.prepareAndScheduleUpdate()
}

func (s *Server) setUpdateStatus(state, version, detail string) {
	s.mu.Lock()
	s.updateState, s.updateVersion, s.updateError = state, version, detail
	s.mu.Unlock()
}

func (s *Server) prepareAndScheduleUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	fail := func(err error) {
		detail := "prepare IDIOS update: " + err.Error()
		s.setUpdateStatus("failed", "", detail)
		diagnostics.Write("ERROR", detail, s.debugEnabled())
	}
	release, err := updater.Check(ctx, s.currentVersion())
	if err != nil {
		fail(err)
		return
	}
	if release == nil {
		fail(errors.New("no newer stable release is available"))
		return
	}
	s.setUpdateStatus("downloading", release.Version, "")
	executable, err := os.Executable()
	if err != nil {
		fail(fmt.Errorf("locate installed IDIOS executable: %w", err))
		return
	}
	prepared, err := updater.Prepare(ctx, *release, executable)
	if err != nil {
		fail(err)
		return
	}
	if err := updater.ScheduleWithArgs(prepared, os.Getpid(), []string{"web", "--no-open"}); err != nil {
		prepared.Cleanup()
		fail(fmt.Errorf("schedule replacement binaries: %w", err))
		return
	}
	s.setUpdateStatus("scheduled", release.Version, "")
	diagnostics.Write("INFO", "verified IDIOS update scheduled for version "+release.Version, s.debugEnabled())
	s.stopOnce.Do(func() {
		go func() {
			time.Sleep(120 * time.Millisecond)
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelShutdown()
			_ = s.http.Shutdown(shutdownCtx)
		}()
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

func (s *Server) getPlans(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListPlans(requestContext(r), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title            string   `json:"title"`
		Description      string   `json:"description"`
		PlanDate         string   `json:"plan_date"`
		LinkedEntityType string   `json:"linked_entity_type"`
		LinkedEntityID   string   `json:"linked_entity_id"`
		Tasks            []string `json:"tasks"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreatePlan(requestContext(r), domain.Plan{Title: input.Title, Description: input.Description, PlanDate: input.PlanDate, LinkedEntityType: input.LinkedEntityType, LinkedEntityID: input.LinkedEntityID}, input.Tasks)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title            string `json:"title"`
		Description      string `json:"description"`
		PlanDate         string `json:"plan_date"`
		LinkedEntityType string `json:"linked_entity_type"`
		LinkedEntityID   string `json:"linked_entity_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.UpdatePlan(requestContext(r), domain.Plan{ID: r.PathValue("ref"), Title: input.Title, Description: input.Description, PlanDate: input.PlanDate, LinkedEntityType: input.LinkedEntityType, LinkedEntityID: input.LinkedEntityID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deletePlan(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeletePlan(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addPlanTask(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.AddPlanTask(requestContext(r), r.PathValue("ref"), input.Title)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) setPlanTaskDone(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Done bool `json:"done"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.SetPlanTaskDone(requestContext(r), r.PathValue("ref"), r.PathValue("taskRef"), input.Done)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
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
func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg, "config_path": s.opts.ConfigPath, "config_dir": config.ConfigDir(), "db_path": s.opts.DBPath})
}
func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Config config.AppConfig `json:"config"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Config.Version == 0 {
		input.Config.Version = 1
	}
	if input.Config.Server.Port < 1 || input.Config.Server.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server.port must be between 1 and 65535"})
		return
	}
	if input.Config.Server.Host != "127.0.0.1" && input.Config.Server.Host != "localhost" && input.Config.Server.Host != "::1" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server.host must be a loopback address"})
		return
	}
	if input.Config.Server.RefreshIntervalSeconds < 10 || input.Config.Server.RefreshIntervalSeconds > 3600 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server.refresh_interval_seconds must be between 10 and 3600"})
		return
	}
	if input.Config.Notifications.PollIntervalMinutes < 1 || input.Config.Notifications.PollIntervalMinutes > 1440 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.poll_interval_minutes must be between 1 and 1440"})
		return
	}
	for name, value := range map[string]string{"morning_time": input.Config.Notifications.PlanReminders.MorningTime, "midday_time": input.Config.Notifications.PlanReminders.MiddayTime, "evening_time": input.Config.Notifications.PlanReminders.EveningTime} {
		parsed, err := time.Parse("15:04", value)
		if err != nil || parsed.Format("15:04") != value {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.plan_reminders." + name + " must use 24-hour HH:MM"})
			return
		}
	}
	planTimes := []string{input.Config.Notifications.PlanReminders.MorningTime, input.Config.Notifications.PlanReminders.MiddayTime, input.Config.Notifications.PlanReminders.EveningTime}
	parsedPlanTimes := make([]time.Time, 0, len(planTimes))
	for _, value := range planTimes {
		parsed, _ := time.Parse("15:04", value) // Format validation above has already succeeded.
		parsedPlanTimes = append(parsedPlanTimes, parsed)
	}
	if !parsedPlanTimes[0].Before(parsedPlanTimes[1]) || !parsedPlanTimes[1].Before(parsedPlanTimes[2]) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.plan_reminders times must be ordered morning < midday < evening"})
		return
	}
	minReminder, maxReminder := reminders.NormalizeRange(input.Config.Notifications.QuoteReminders)
	if input.Config.Notifications.QuoteReminders.Enabled && (input.Config.Notifications.QuoteReminders.MinIntervalMinutes < 1 || input.Config.Notifications.QuoteReminders.MaxIntervalMinutes < input.Config.Notifications.QuoteReminders.MinIntervalMinutes) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications.quote_reminders interval range must be positive and max must be greater than or equal to min"})
		return
	}
	input.Config.Notifications.QuoteReminders.MinIntervalMinutes = minReminder
	input.Config.Notifications.QuoteReminders.MaxIntervalMinutes = maxReminder
	if input.Config.Notifications.QuoteReminders.Enabled && strings.TrimSpace(input.Config.Notifications.QuoteReminders.NextAt) == "" {
		reminders.EnsureSchedule(&input.Config, time.Now())
	}
	if message := validateWebThemeConfig(input.Config.Web); message != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": message})
		return
	}
	s.mu.Lock()
	previous := s.cfg
	previousReminder := previous.Notifications.QuoteReminders
	nextReminder := &input.Config.Notifications.QuoteReminders
	if nextReminder.Enabled && previousReminder.Enabled && nextReminder.MinIntervalMinutes == previousReminder.MinIntervalMinutes && nextReminder.MaxIntervalMinutes == previousReminder.MaxIntervalMinutes {
		nextReminder.NextAt = previousReminder.NextAt
		nextReminder.LastQuoteID = previousReminder.LastQuoteID
	}
	if err := config.SaveAppConfig(input.Config); err != nil {
		s.mu.Unlock()
		writeError(w, err)
		return
	}
	resolved, err := config.LoadAppConfig()
	if err != nil {
		s.mu.Unlock()
		writeError(w, err)
		return
	}
	s.cfg = resolved
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "saved",
		"config_path":      s.opts.ConfigPath,
		"restart_required": resolved.Server.Port != previous.Server.Port || resolved.Server.Host != previous.Server.Host,
	})
}

// validateWebThemeConfig keeps the base theme and saved custom palettes aligned.
// Built-in palettes are CSS presets; custom palettes are fully validated before config is persisted.
func validateWebThemeConfig(web config.WebConfig) string {
	builtins := map[string]struct{}{
		"system": {}, "dark": {}, "light": {}, "midnight": {}, "forest": {}, "violet": {}, "rose": {}, "oled": {},
	}
	for key, theme := range web.CustomThemes {
		if !validCustomThemeKey(key) {
			return fmt.Sprintf("web.custom_themes key %q must use lowercase letters, numbers, and hyphens (max 40 characters)", key)
		}
		if _, reserved := builtins[key]; reserved {
			return fmt.Sprintf("web.custom_themes key %q is reserved for a built-in theme", key)
		}
		if strings.TrimSpace(theme.Name) == "" || len([]rune(theme.Name)) > 60 {
			return fmt.Sprintf("web.custom_themes.%s.name must contain 1–60 characters", key)
		}
		if theme.Base != "dark" && theme.Base != "light" {
			return fmt.Sprintf("web.custom_themes.%s.base must be dark or light", key)
		}
		colors := map[string]string{
			"background": theme.Background, "surface": theme.Surface, "text": theme.Text,
			"muted": theme.Muted, "accent": theme.Accent, "border": theme.Border,
			"success": theme.Success, "warning": theme.Warning, "danger": theme.Danger,
		}
		for field, color := range colors {
			if !validHexColor(color) {
				return fmt.Sprintf("web.custom_themes.%s.%s must be a six-digit hex color such as #58a6ff", key, field)
			}
		}
	}
	if _, ok := builtins[web.Theme]; ok {
		return ""
	}
	if _, ok := web.CustomThemes[web.Theme]; ok {
		return ""
	}
	return "web.theme must be system, dark, light, a built-in palette, or the key of a saved custom theme"
}

func validCustomThemeKey(value string) bool {
	if len(value) == 0 || len(value) > 40 {
		return false
	}
	first := value[0]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, char := range value[1:] {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func (s *Server) getProtocolCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"protocol_version": protocol.Version, "commands": protocol.Catalog()})
}

// getFeatureCatalog exposes the shared user-facing feature inventory to the Web and TUI clients.
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
func (s *Server) getJobs(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListScheduledJobs(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
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

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must contain exactly one JSON value"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalid), errors.Is(err, domain.ErrUnsupported):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrBlocked), errors.Is(err, domain.ErrInvalidTransition):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
