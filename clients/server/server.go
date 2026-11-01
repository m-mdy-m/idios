package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"idios/internal/ai"
	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/desktopnotify"
	"idios/internal/diagnostics"
	"idios/internal/domain"
	"idios/internal/reminders"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
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
	aiMu              sync.Mutex
	aiRuntime         *ai.Manager
	aiRuntimeErr      error
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
	s.closeAIRuntime()
	if errors.Is(err, http.ErrServerClosed) {
		diagnostics.Write("INFO", "local HTTP server stopped", s.debugEnabled())
		return nil
	}
	diagnostics.Write("ERROR", "local HTTP server stopped unexpectedly: "+err.Error(), s.debugEnabled())
	return err
}

func (s *Server) runScheduler(ctx context.Context) {
	// Jobs respect the configured polling interval. Quote reminders are due-time based,
	// so a small scheduler tick keeps them independent of whether the Web client is open.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	nextJobsCheck := time.Now()
	interval := time.Duration(0)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			// The Web settings screen can edit the shared config file while the service runs.
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
	// Remote Repository Hub (kept separate from local simulated repository routes).
	mux.HandleFunc("GET /api/hub/status", s.getHubStatus)
	mux.HandleFunc("GET /api/hub/oauth/start", s.startHubOAuth)
	mux.HandleFunc("GET /api/hub/oauth/callback", s.completeHubOAuth)
	mux.HandleFunc("GET /api/hub/connections", s.getHubConnections)
	mux.HandleFunc("POST /api/hub/connections", s.createHubConnection)
	mux.HandleFunc("DELETE /api/hub/connections/{ref}", s.deleteHubConnection)
	mux.HandleFunc("GET /api/hub/discover", s.discoverHubRepositories)
	mux.HandleFunc("GET /api/hub/repositories", s.getHubRepositories)
	mux.HandleFunc("POST /api/hub/repositories/import", s.importHubRepository)
	mux.HandleFunc("GET /api/hub/repositories/{ref}", s.getHubRepository)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/sync", s.syncHubRepository)
	mux.HandleFunc("DELETE /api/hub/repositories/{ref}", s.deleteHubRepository)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/preferences", s.getHubPreferences)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/projects", s.getHubProjects)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/projects", s.linkHubProject)
	mux.HandleFunc("DELETE /api/hub/repositories/{ref}/projects/{project}", s.unlinkHubProject)
	mux.HandleFunc("PUT /api/hub/repositories/{ref}/projects/{project}/primary", s.setHubProjectPrimary)
	mux.HandleFunc("GET /api/projects/{ref}/hub-repositories", s.getProjectHubRepositories)
	mux.HandleFunc("PATCH /api/hub/repositories/{ref}/preferences", s.patchHubPreferences)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/relations", s.getHubRelations)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/relations", s.createHubRelation)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/branches", s.getHubBranches)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/tree", s.getHubTree)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/file", s.getHubFile)
	mux.HandleFunc("PUT /api/hub/repositories/{ref}/file", s.putHubFile)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/commits", s.getHubCommits)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/commits/{sha}", s.getHubCommit)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/issues", s.getHubIssues)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/issues", s.createHubIssue)
	mux.HandleFunc("PATCH /api/hub/repositories/{ref}/issues/{number}", s.updateHubIssue)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/issues/{number}/close", s.closeHubIssue)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/issues/{number}/comments", s.commentHubIssue)
	mux.HandleFunc("GET /api/hub/repositories/{ref}/changes", s.getHubChanges)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/changes", s.createHubChange)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/changes/{number}/close", s.closeHubChange)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/changes/{number}/approve", s.approveHubChange)
	mux.HandleFunc("POST /api/hub/repositories/{ref}/changes/{number}/merge", s.mergeHubChange)
	mux.HandleFunc("GET /api/hub/collections", s.getHubCollections)
	mux.HandleFunc("POST /api/hub/collections", s.saveHubCollection)
	mux.HandleFunc("PUT /api/hub/collections/{ref}", s.saveHubCollection)
	mux.HandleFunc("DELETE /api/hub/collections/{ref}", s.deleteHubCollection)
	mux.HandleFunc("PUT /api/hub/collections/{collection}/repositories/{repo}", s.addHubCollectionRepository)
	mux.HandleFunc("DELETE /api/hub/collections/{collection}/repositories/{repo}", s.removeHubCollectionRepository)
	mux.HandleFunc("DELETE /api/hub/relations/{ref}", s.deleteHubRelation)
	mux.HandleFunc("GET /api/repositories", s.getRepositories)
	mux.HandleFunc("POST /api/repositories", s.createRepository)
	mux.HandleFunc("GET /api/repositories/{ref}/git/status", s.getLocalGitStatus)
	mux.HandleFunc("POST /api/repositories/{ref}/git/init", s.initializeLocalGit)
	mux.HandleFunc("POST /api/repositories/{ref}/git/clone", s.cloneLocalGit)
	mux.HandleFunc("GET /api/repositories/{ref}/git/branches", s.getLocalGitBranches)
	mux.HandleFunc("POST /api/repositories/{ref}/git/branches", s.createLocalGitBranch)
	mux.HandleFunc("POST /api/repositories/{ref}/git/checkout", s.switchLocalGitBranch)
	mux.HandleFunc("POST /api/repositories/{ref}/git/stage", s.stageLocalGitPaths)
	mux.HandleFunc("POST /api/repositories/{ref}/git/commits", s.createLocalGitCommit)
	mux.HandleFunc("GET /api/repositories/{ref}/git/log", s.getLocalGitLog)
	mux.HandleFunc("GET /api/repositories/{ref}/git/tags", s.getLocalGitTags)
	mux.HandleFunc("POST /api/repositories/{ref}/git/tags", s.createLocalGitTag)
	mux.HandleFunc("GET /api/repositories/{ref}/git/diff", s.getLocalGitDiff)
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
	// Unified AI Runtime API. Shelf's older endpoints remain compatibility adapters below.
	mux.HandleFunc("GET /api/ai/status", s.getAIRuntimeStatus)
	mux.HandleFunc("GET /api/ai/config", s.getAIRuntimeConfig)
	mux.HandleFunc("PUT /api/ai/config", s.putAIRuntimeConfig)
	mux.HandleFunc("GET /api/ai/adapters", s.listAIAdapters)
	mux.HandleFunc("GET /api/ai/providers", s.listAIProviders)
	mux.HandleFunc("POST /api/ai/providers", s.createAIProvider)
	mux.HandleFunc("PUT /api/ai/providers/{id}", s.updateAIProvider)
	mux.HandleFunc("DELETE /api/ai/providers/{id}", s.deleteAIProvider)
	mux.HandleFunc("POST /api/ai/providers/{id}/test", s.testAIProvider)
	mux.HandleFunc("GET /api/ai/models", s.listAIModels)
	mux.HandleFunc("POST /api/ai/models/refresh", s.refreshAIModels)
	mux.HandleFunc("POST /api/ai/models/test", s.testAIModel)
	mux.HandleFunc("POST /api/ai/chat", s.aiChat)
	mux.HandleFunc("POST /api/ai/chat/stream", s.aiChatStream)
	mux.HandleFunc("POST /api/ai/tasks", s.createAITask)
	mux.HandleFunc("GET /api/ai/tasks", s.listAITasks)
	mux.HandleFunc("GET /api/ai/tasks/{id}", s.getAITask)
	mux.HandleFunc("POST /api/ai/tasks/{id}/cancel", s.cancelAITask)
	mux.HandleFunc("GET /api/ai/agents", s.listAIAgents)
	mux.HandleFunc("POST /api/ai/agents", s.upsertAIAgent)
	mux.HandleFunc("PUT /api/ai/agents/{id}", s.upsertAIAgent)
	mux.HandleFunc("DELETE /api/ai/agents/{id}", s.deleteAIAgent)
	mux.HandleFunc("POST /api/ai/agents/runs", s.startAIAgentRun)
	mux.HandleFunc("GET /api/ai/agents/runs/{id}", s.getAIAgentRun)
	mux.HandleFunc("POST /api/ai/agents/runs/{id}/cancel", s.cancelAIAgentRun)
	mux.HandleFunc("GET /api/shelf", s.getShelf)
	mux.HandleFunc("GET /api/shelf/reading-sessions", s.getShelfReadingSessions)
	mux.HandleFunc("POST /api/shelf/reading-sessions", s.createShelfReadingSession)
	mux.HandleFunc("POST /api/shelf/ai/models", s.shelfAIModels)
	mux.HandleFunc("POST /api/shelf/ai/chat", s.shelfAIChat)
	mux.HandleFunc("POST /api/shelf/scan", s.scanShelf)
	mux.HandleFunc("PUT /api/shelf/settings", s.updateShelfSettings)
	mux.HandleFunc("GET /api/shelf/books", s.getShelfBooks)
	mux.HandleFunc("GET /api/shelf/books/{ref}", s.getShelfBook)
	mux.HandleFunc("GET /api/shelf/books/{ref}/readme", s.getShelfBookReadme)
	mux.HandleFunc("GET /api/shelf/books/{ref}/pdf", s.getShelfBookPDF)
	mux.HandleFunc("GET /api/shelf/books/{ref}/pdf-data", s.getShelfBookPDFData)
	mux.HandleFunc("GET /api/shelf/books/{ref}/annotations", s.getShelfAnnotations)
	mux.HandleFunc("POST /api/shelf/books/{ref}/annotations", s.createShelfAnnotation)
	mux.HandleFunc("PATCH /api/shelf/annotations/{ref}", s.updateShelfAnnotation)
	mux.HandleFunc("DELETE /api/shelf/annotations/{ref}", s.deleteShelfAnnotation)
	mux.HandleFunc("PUT /api/shelf/books/{ref}/tags", s.updateShelfBookTags)
	mux.HandleFunc("PATCH /api/shelf/books/{ref}/progress", s.updateShelfBookProgress)
	mux.HandleFunc("GET /api/shelf/books/{ref}/notes", s.getShelfBookNotes)
	mux.HandleFunc("POST /api/shelf/books/{ref}/notes", s.createShelfBookNote)
	mux.HandleFunc("GET /api/shelf/books/{ref}/asks", s.getShelfAsksForBook)
	mux.HandleFunc("POST /api/shelf/books/{ref}/asks", s.createShelfAsk)
	mux.HandleFunc("POST /api/shelf/books/{ref}/repository", s.createShelfBookRepository)
	mux.HandleFunc("PUT /api/shelf/books/{ref}/repository", s.linkShelfBookRepository)
	mux.HandleFunc("GET /api/shelf/asks", s.getShelfAsks)
	mux.HandleFunc("PATCH /api/shelf/asks/{ref}", s.answerShelfAsk)
	mux.HandleFunc("GET /api/shelf/relations", s.getShelfRelations)
	mux.HandleFunc("POST /api/shelf/relations", s.createShelfRelation)
	mux.HandleFunc("DELETE /api/shelf/relations/{ref}", s.deleteShelfRelation)
	mux.HandleFunc("GET /api/shelf/graph", s.getShelfGraph)
	mux.HandleFunc("GET /api/quotes", s.getQuotes)
	mux.HandleFunc("GET /api/quotes/next", s.getNextQuote)
	mux.HandleFunc("GET /api/jobs", s.getJobs)
	mux.HandleFunc("GET /api/profile", s.getProfile)
	mux.HandleFunc("GET /api/pins", s.getPins)
	mux.HandleFunc("GET /api/notifications", s.getNotifications)
	mux.HandleFunc("POST /api/notifications/read", s.markNotificationsRead)
	mux.HandleFunc("GET /api/activity", s.getActivity)
	mux.HandleFunc("GET /api/activity/days", s.getActivityDays)
	mux.HandleFunc("POST /api/activity/presence", s.recordActivityPresence)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("PUT /api/config", s.updateConfig)
	mux.HandleFunc("POST /api/server/shutdown", s.shutdown)
	// {$} makes the root route exact. A plain "GET /" is a subtree pattern
	// in net/http and conflicts with the catch-all SPA route below.
	mux.HandleFunc("GET /{$}", s.serveUI)
	mux.HandleFunc("GET /{path...}", s.serveUI)
	return mux
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
		// A browser is allowed to cancel an in-flight request (for example when a Reader
		// route unmounts). If the handler did not commit a response, this is not an HTTP 500
		// and must not pollute the error log with a misleading server failure.
		if r.Context().Err() != nil && recorder.status == 0 {
			return
		}
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
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
	case errors.Is(err, domain.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrUnsupported):
		status = http.StatusNotImplemented
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrBlocked), errors.Is(err, domain.ErrInvalidTransition):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrPermission):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, domain.ErrUnavailable):
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
