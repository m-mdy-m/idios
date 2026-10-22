// Package tui implements a full-screen, event-driven terminal interface.
// It is a presentation adapter: every read/write operation goes through application.Service.
package tui

import (
	"context"
	"fmt"
	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/domain"
	"idios/internal/updater"
	"time"

	tea "charm.land/bubbletea/v2"
)

type page string

type formKind string

const (
	pageIntro                 page = "intro"
	pageOverview              page = "overview"
	pageRepositories          page = "repositories"
	pageProjects              page = "projects"
	pageIssues                page = "issues"
	pagePullRequests          page = "pull-requests"
	pageGraph                 page = "graph"
	pageConcepts              page = "concepts"
	pageNotes                 page = "notes"
	pageQuotes                page = "quotes"
	pageJobs                  page = "jobs"
	pageNotifications         page = "notifications"
	pageSettingsNotifications page = "settings-notifications"
	pageLogs                  page = "logs"
	pageSettings              page = "settings"
	pageNew                   page = "new"
	pageRepository            page = "repository-overview"
	pageRepoIssues            page = "repository-issues"
	pageRepoPRs               page = "repository-pull-requests"
	pageRepoBranches          page = "repository-branches"
	pageRepoGraph             page = "repository-graph"
	pageRepoLabels            page = "repository-labels"
	pageRepoMilestones        page = "repository-milestones"
	pageRepoSettings          page = "repository-settings"
	pageRepoActivity          page = "repository-activity"
	pageIssueDetail           page = "issue-detail"
	pagePRDetail              page = "pr-detail"
	pageProjectDetail         page = "project-detail"
	pageConceptDetail         page = "concept-detail"
	pageForm                  page = "form"
	pageHelp                  page = "help"
	pageUpgrade               page = "upgrade"
	pageQuit                  page = "quit-confirmation"
	pageBatchResults          page = "batch-results"
	pageBatchProgress         page = "batch-progress"
	pageCommands              page = "command-palette"
	pageRemove                page = "remove-center"
	pageDeleteConfirm         page = "delete-confirmation"
)

const (
	formRepository              formKind = "repository"
	formProject                 formKind = "project"
	formIssue                   formKind = "issue"
	formConcept                 formKind = "concept"
	formPullRequest             formKind = "pull-request"
	formAcceptPR                formKind = "accept-pr"
	formEditPR                  formKind = "edit-pr"
	formDependency              formKind = "dependency"
	formConceptLink             formKind = "concept-link"
	formProjectIssue            formKind = "project-issue"
	formRepoSetting             formKind = "repository-setting"
	formLabel                   formKind = "label"
	formMilestone               formKind = "milestone"
	formAssignLabel             formKind = "assign-label"
	formAssignMilestone         formKind = "assign-milestone"
	formNote                    formKind = "note"
	formNoteLink                formKind = "note-link"
	formQuote                   formKind = "quote"
	formJob                     formKind = "job"
	formProfile                 formKind = "profile"
	formBulkIssues              formKind = "bulk-issues"
	formBulkQuotes              formKind = "bulk-quotes"
	formBulkJobs                formKind = "bulk-jobs"
	formDeleteRepositoryConfirm formKind = "delete-repository-confirm"
	formProtocolCommand         formKind = "protocol-command"
)

type formField struct {
	label       string
	value       string
	placeholder string
	helper      string
	multiline   bool
}

type hitbox struct {
	x, y, w, h int
	action     string
}

type snapshot struct {
	repositories    []domain.Repository
	projects        []domain.Project
	issues          []domain.Issue
	pullRequests    []domain.PullRequest
	concepts        []domain.Concept
	graph           domain.Graph
	completions     []domain.CompletionRecord
	activity        []domain.ActivityEvent
	labels          []domain.Label
	milestones      []domain.Milestone
	settings        []domain.RepositorySetting
	issueLabels     map[string][]domain.Label
	issueMilestones map[string][]domain.Milestone
	notes           []domain.Note
	noteLabels      []domain.NoteLabel
	quotes          []domain.Quote
	quote           domain.Quote
	jobs            []domain.ScheduledJob
	notifications   []domain.Notification
	profile         domain.WorkspaceProfile
	pins            []domain.WorkspacePin
	err             error
}

type snapshotMsg snapshot

type updateCheckMsg struct {
	release *updater.Release
	err     error
}

type updatePreparedMsg struct {
	prepared updater.Prepared
	err      error
}

type updateAnimationTickMsg struct{}
type exitAnimationTickMsg struct{}

type batchItemResult struct {
	Item      string
	Success   bool
	Retryable bool
	Skipped   bool
	NotRun    bool
	Detail    string
}

type batchItemDoneMsg struct {
	result batchItemResult
}

type batchRunState struct {
	kind            formKind
	items           []string
	fields          []string
	index           int
	results         []batchItemResult
	seen            map[string]bool
	cancelRequested bool
}

// actionClickMsg routes mouse events through the same Bubble Tea update loop as key actions.
type actionClickMsg string

type operationMsg struct {
	message           string
	err               error
	batchResults      []batchItemResult
	openPage          page
	openRepo          string
	openIssue         string
	openPR            string
	openProject       string
	openConcept       string
	openNote          string
	openJob           string
	clearDeleteTarget bool
}

type model struct {
	ctx context.Context
	app *application.Service

	page           page
	repoContext    string
	previousPage   page
	previousRepo   string
	formReturnPage page
	formReturnRepo string

	width, height       int
	bodyTop, bodyHeight int
	contentLines        int
	selectLines         []int
	cursor              int
	scroll              int
	search              string
	searchMode          bool
	showHelp            bool
	newMenu             bool

	repositories     []domain.Repository
	projects         []domain.Project
	issues           []domain.Issue
	pullRequests     []domain.PullRequest
	concepts         []domain.Concept
	graph            domain.Graph
	completions      []domain.CompletionRecord
	activity         []domain.ActivityEvent
	labels           []domain.Label
	milestones       []domain.Milestone
	settings         []domain.RepositorySetting
	issueLabels      map[string][]domain.Label
	issueMilestones  map[string][]domain.Milestone
	notes            []domain.Note
	noteLabels       []domain.NoteLabel
	quotes           []domain.Quote
	quote            domain.Quote
	jobs             []domain.ScheduledJob
	notifications    []domain.Notification
	profile          domain.WorkspaceProfile
	pins             []domain.WorkspacePin
	uiLogs           []string
	selectedNoteID   string
	selectedQuoteID  string
	removeKind       string
	deleteTarget     deleteTarget
	selectedJobID    string
	activeNoteLabel  string
	schedulerChecked bool
	nextJobsCheckAt  time.Time

	availableUpdate   *updater.Release
	upgradeInstalling bool
	upgradeHandedOff  bool
	upgradeFrame      int
	upgradeStartedAt  time.Time
	upgradeHandoffAt  time.Time
	upgradeError      string

	selectedIssueRef string
	selectedPRRef    string
	selectedProject  string
	selectedConcept  string
	graphFrom        string
	graphDepth       int
	graphStructure   bool // include ownership links (repository owns issue, branch ...) on the Graph page

	form                formKind
	formFields          []formField
	formFocus           int
	formCaret           int
	formError           string
	formTitle           string
	activeIssueTemplate string
	protocolCommandID   string

	status       string
	statusError  bool
	databasePath string
	plain        bool
	demoMode     bool
	config       config.AppConfig
	hits         []hitbox
	buttonFocus  bool
	focusAction  string
	focusIndex   int

	batchConfirm      bool
	batchResults      []batchItemResult
	batchRun          *batchRunState
	batchCarryResults []batchItemResult
	batchRetrying     bool
	batchTitle        string
	lastBatchForm     formKind
	lastBatchFields   []formField

	commandPreviousPage page
	commandPreviousRepo string
	commandQuery        string
	quitPreviousPage    page
	quitPreviousRepo    string
	exitAnimating       bool
	exitAfterBatch      bool
	exitFrame           int
}

// Options controls TUI presentation and identifies the isolated demo presentation mode.

type Options struct {
	NoColor  bool
	DemoMode bool
	Config   config.AppConfig
}

// RunInteractive runs the full-screen TUI in the terminal alternate screen.
// It intentionally does not support scripted stdin/stdout; automated callers should use the CLI/JSON API.

func RunInteractive(ctx context.Context, app *application.Service, databasePath string, opts Options) error {
	if app == nil {
		return fmt.Errorf("TUI requires an Application service")
	}
	if opts.Config.Version == 0 || opts.Config.Themes == nil {
		opts.Config = config.DefaultAppConfig()
	}
	m := newModel(ctx, app, databasePath, opts)
	if err := ensureBuiltinQuotes(m.ctx, app); err != nil {
		// Quote provenance is an enhancement, not a reason to prevent the workspace opening.
		m.logEvent("WARN", "could not seed source-verified quote collection: "+err.Error())
	}
	if scheduleQuoteReminderIfNeeded(&m.config, time.Now()) {
		if err := config.SaveAppConfig(m.config); err != nil {
			m.logEvent("WARN", "could not persist next quote reminder: "+err.Error())
		}
	}
	program := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := program.Run()
	if err != nil {
		return fmt.Errorf("run IDIOS TUI: %w", err)
	}
	return nil
}

func newModel(ctx context.Context, app *application.Service, dbPath string, opts Options) *model {
	if ctx == nil {
		ctx = context.Background()
	}
	appConfig := opts.Config
	if appConfig.Version == 0 || appConfig.Themes == nil {
		appConfig = config.DefaultAppConfig()
	}
	startPage := pageOverview
	if appConfig.Startup.ShowIntro {
		startPage = pageIntro
	}
	return &model{
		ctx: ctx, app: app, config: appConfig,
		page:  startPage,
		width: 110, height: 32,
		graphDepth:      2,
		databasePath:    dbPath,
		plain:           opts.NoColor,
		demoMode:        opts.DemoMode,
		status:          "",
		focusIndex:      -1,
		nextJobsCheckAt: time.Now().Add(time.Duration(max(1, appConfig.Notifications.PollIntervalMinutes)) * time.Minute),
	}
}

func (m *model) Init() tea.Cmd {
	commands := []tea.Cmd{loadSnapshotCmd(m.ctx, m.app, true), schedulerTickCmd(time.Minute)}
	if !m.demoMode {
		commands = append(commands, checkUpdateCmd(m.ctx))
	}
	return tea.Batch(commands...)
}

func loadSnapshotCmd(ctx context.Context, app *application.Service, rotateQuote ...bool) tea.Cmd {
	return func() tea.Msg {
		out := snapshot{}
		var err error
		if out.repositories, err = app.ListRepositories(ctx); err != nil {
			out.err = err
			return snapshotMsg(out)
		}
		if out.projects, err = app.ListProjects(ctx); err != nil {
			out.err = err
			return snapshotMsg(out)
		}
		if out.issues, err = app.ListIssues(ctx, "", ""); err != nil {
			out.err = err
			return snapshotMsg(out)
		}
		if out.pullRequests, err = app.ListPullRequests(ctx, "", ""); err != nil {
			out.err = err
			return snapshotMsg(out)
		}
		if out.concepts, err = app.ListConcepts(ctx); err != nil {
			out.err = err
			return snapshotMsg(out)
		}
		if out.graph, err = app.Graph(ctx, "", 0); err != nil {
			out.err = err
			return snapshotMsg(out)
		}
		// These panels are useful but not required to make the primary workspace navigable.
		out.completions, _ = app.ListCompletions(ctx, "")
		out.activity, _ = app.ListActivity(ctx, "", "", 20)
		out.labels, _ = app.ListLabels(ctx, "")
		out.milestones, _ = app.ListMilestones(ctx, "", "")
		out.issueLabels, _ = app.ListIssueLabels(ctx)
		out.issueMilestones, _ = app.ListIssueMilestones(ctx)
		out.notes, _ = app.ListNotes(ctx)
		out.noteLabels, _ = app.ListNoteLabels(ctx)
		out.quotes, _ = app.ListQuotes(ctx)
		out.jobs, _ = app.ListScheduledJobs(ctx)
		out.notifications, _ = app.ListNotifications(ctx, 50)
		out.profile, _ = app.GetWorkspaceProfile(ctx)
		out.pins, _ = app.ListWorkspacePins(ctx)
		if len(rotateQuote) > 0 && rotateQuote[0] {
			out.quote, _ = app.NextQuote(ctx)
		} else if len(out.quotes) > 0 {
			out.quote = out.quotes[0]
		}
		return snapshotMsg(out)
	}
}

var _ tea.Model = (*model)(nil)
