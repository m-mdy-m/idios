// Package tui implements a full-screen, event-driven terminal interface.
// It is a presentation adapter: every read/write operation goes through application.Service.
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"idios/internal/application"
	"idios/internal/domain"
)

type page string
type formKind string

const (
	pageOverview       page = "overview"
	pageRepositories   page = "repositories"
	pageProjects       page = "projects"
	pageIssues         page = "issues"
	pagePullRequests   page = "pull-requests"
	pageGraph          page = "graph"
	pageConcepts       page = "concepts"
	pageSettings       page = "settings"
	pageNew            page = "new"
	pageRepository     page = "repository-overview"
	pageRepoIssues     page = "repository-issues"
	pageRepoPRs        page = "repository-pull-requests"
	pageRepoBranches   page = "repository-branches"
	pageRepoGraph      page = "repository-graph"
	pageRepoLabels     page = "repository-labels"
	pageRepoMilestones page = "repository-milestones"
	pageRepoSettings   page = "repository-settings"
	pageRepoActivity   page = "repository-activity"
	pageIssueDetail    page = "issue-detail"
	pagePRDetail       page = "pr-detail"
	pageProjectDetail  page = "project-detail"
	pageConceptDetail  page = "concept-detail"
	pageForm           page = "form"
	pageHelp           page = "help"
)

const (
	formRepository      formKind = "repository"
	formProject         formKind = "project"
	formIssue           formKind = "issue"
	formConcept         formKind = "concept"
	formPullRequest     formKind = "pull-request"
	formAcceptPR        formKind = "accept-pr"
	formDependency      formKind = "dependency"
	formConceptLink     formKind = "concept-link"
	formProjectIssue    formKind = "project-issue"
	formRepoSetting     formKind = "repository-setting"
	formLabel           formKind = "label"
	formMilestone       formKind = "milestone"
	formAssignLabel     formKind = "assign-label"
	formAssignMilestone formKind = "assign-milestone"
)

type formField struct {
	label       string
	value       string
	placeholder string
	helper      string
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
	err             error
}

type snapshotMsg snapshot

type operationMsg struct {
	message     string
	err         error
	openPage    page
	openRepo    string
	openIssue   string
	openPR      string
	openProject string
	openConcept string
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

	selectedIssueRef string
	selectedPRRef    string
	selectedProject  string
	selectedConcept  string
	graphFrom        string
	graphDepth       int

	form       formKind
	formFields []formField
	formFocus  int
	formCaret  int
	formError  string
	formTitle  string

	status       string
	statusError  bool
	databasePath string
	plain        bool
	hits         []hitbox
}

// Options controls UI rendering only; the Engine and data rules are unaffected.
type Options struct {
	NoColor bool
}

// RunInteractive runs the full-screen TUI in the terminal alternate screen.
// It intentionally does not support scripted stdin/stdout; automated callers should use the CLI/JSON API.
func RunInteractive(ctx context.Context, app *application.Service, databasePath string, opts Options) error {
	if app == nil {
		return fmt.Errorf("TUI requires an Application service")
	}
	m := newModel(ctx, app, databasePath, opts)
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
	return &model{
		ctx: ctx, app: app,
		page:  pageOverview,
		width: 110, height: 32,
		graphDepth:   2,
		databasePath: dbPath,
		plain:        opts.NoColor,
		status:       "Everything stays local · Engine rules apply to every interface",
	}
}

func (m *model) Init() tea.Cmd { return loadSnapshotCmd(m.ctx, m.app) }

func loadSnapshotCmd(ctx context.Context, app *application.Service) tea.Cmd {
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
		return snapshotMsg(out)
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		m.clampViewport()
		return m, nil
	case snapshotMsg:
		s := snapshot(msg)
		if s.err != nil {
			m.status, m.statusError = s.err.Error(), true
			return m, nil
		}
		m.repositories, m.projects, m.issues = s.repositories, s.projects, s.issues
		m.pullRequests, m.concepts, m.graph = s.pullRequests, s.concepts, s.graph
		m.completions, m.activity, m.labels = s.completions, s.activity, s.labels
		m.milestones, m.settings = s.milestones, s.settings
		m.issueLabels, m.issueMilestones = s.issueLabels, s.issueMilestones
		m.rebindDetails()
		m.clampViewport()
		return m, nil
	case actionClickMsg:
		return m, m.handleAction(string(msg))
	case operationMsg:
		if msg.err != nil {
			m.status, m.statusError = msg.err.Error(), true
			if m.form != "" {
				m.formError = msg.err.Error()
				return m, nil // Keep the form and its input so the user can correct it.
			}
		} else {
			m.status, m.statusError = msg.message, false
		}
		if msg.openPage != "" {
			m.page = msg.openPage
		}
		if msg.openRepo != "" {
			m.repoContext, m.page = msg.openRepo, pageRepository
		}
		if msg.openIssue != "" {
			m.selectedIssueRef, m.page = msg.openIssue, pageIssueDetail
		}
		if msg.openPR != "" {
			m.selectedPRRef, m.page = msg.openPR, pagePRDetail
		}
		if msg.openProject != "" {
			m.selectedProject, m.page = msg.openProject, pageProjectDetail
		}
		if msg.openConcept != "" {
			m.selectedConcept, m.page = msg.openConcept, pageConceptDetail
		}
		m.form, m.formFields, m.formError = "", nil, ""
		return m, loadSnapshotCmd(m.ctx, m.app)
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			for _, h := range m.hits {
				if msg.X >= h.x && msg.X < h.x+h.w && msg.Y >= h.y && msg.Y < h.y+h.h {
					return m, m.handleAction(h.action)
				}
			}
		}
		return m, nil
	case tea.PasteMsg:
		content := strings.ReplaceAll(strings.ReplaceAll(msg.Content, "\r", " "), "\n", " ")
		if m.searchMode {
			m.search += content
			m.cursor, m.scroll = 0, 0
		} else if m.form != "" && m.formFocus < len(m.formFields) {
			field := &m.formFields[m.formFocus]
			field.value = insertAtRune(field.value, m.formCaret, content)
			m.formCaret += len([]rune(content))
		}
		return m, nil
	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelDown {
			m.scroll++
		} else if msg.Button == tea.MouseWheelUp && m.scroll > 0 {
			m.scroll--
		}
		m.clampViewport()
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if m.searchMode {
			return m, m.handleSearchKey(key)
		}
		if m.form != "" {
			return m, m.handleFormKey(key)
		}
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.page == pageIssueDetail || m.page == pagePRDetail || m.page == pageProjectDetail || m.page == pageConceptDetail {
				m.goBack()
			} else if m.page == pageNew || m.page == pageHelp {
				m.page = pageOverview
			} else if m.repoContext != "" {
				m.repoContext = ""
				m.page = pageRepositories
			}
			return m, nil
		case "up", "k":
			m.moveCursor(-1)
			return m, nil
		case "down", "j":
			m.moveCursor(1)
			return m, nil
		case "enter", "space":
			return m, m.openCursor()
		case "/":
			m.searchMode = true
			m.status = "Type to filter this view · Enter to finish · Esc to clear"
			return m, nil
		case "n":
			m.page = pageNew
			m.repoContext = ""
			m.cursor, m.scroll = 0, 0
			return m, nil
		case "?", "h":
			m.page = pageHelp
			m.cursor, m.scroll = 0, 0
			return m, nil
		case "g":
			m.repoContext = ""
			m.page = pageGraph
			m.cursor, m.scroll = 0, 0
			return m, nil
		case "r":
			return m, loadSnapshotCmd(m.ctx, m.app)
		case "tab":
			m.cycleNavigation(1)
			return m, nil
		case "shift+tab":
			m.cycleNavigation(-1)
			return m, nil
		}
		// Number keys mirror tabs for people who prefer keyboard navigation.
		if len(key) == 1 && key[0] >= '1' && key[0] <= '8' {
			m.navigateIndex(int(key[0] - '1'))
		}
	}
	return m, nil
}

func (m *model) rebindDetails() {
	if m.selectedIssueRef != "" {
		for _, issue := range m.issues {
			if issueRef(issue) == m.selectedIssueRef {
				m.selectedIssueRef = issueRef(issue)
				break
			}
		}
	}
	if m.selectedPRRef != "" {
		for _, pr := range m.pullRequests {
			if prRef(pr) == m.selectedPRRef {
				break
			}
		}
	}
}

func (m *model) handleSearchKey(key string) tea.Cmd {
	switch key {
	case "enter", "esc":
		m.searchMode = false
		if key == "esc" {
			m.search = ""
		}
		m.cursor, m.scroll = 0, 0
	case "backspace":
		m.search = trimLastRune(m.search)
		m.cursor, m.scroll = 0, 0
	case "ctrl+u":
		m.search = ""
		m.cursor, m.scroll = 0, 0
	default:
		if key == "space" {
			key = " "
		}
		if isPrintableKey(key) {
			m.search += key
			m.cursor, m.scroll = 0, 0
		}
	}
	return nil
}

func (m *model) handleFormKey(key string) tea.Cmd {
	fieldIsFocused := m.formFocus >= 0 && m.formFocus < len(m.formFields)
	setCaretToEnd := func() {
		if fieldIsFocused {
			m.formCaret = len([]rune(m.formFields[m.formFocus].value))
		} else {
			m.formCaret = 0
		}
	}
	switch key {
	case "esc":
		m.form, m.formFields, m.formError = "", nil, ""
		m.page, m.repoContext = m.formReturnPage, m.formReturnRepo
		return nil
	case "tab", "down":
		m.formFocus = min(m.formFocus+1, len(m.formFields))
		setCaretToEnd()
		return nil
	case "shift+tab", "up":
		m.formFocus = max(0, m.formFocus-1)
		setCaretToEnd()
		return nil
	case "enter":
		if m.formFocus >= len(m.formFields)-1 {
			m.formFocus = len(m.formFields)
			return m.submitForm()
		}
		m.formFocus++
		setCaretToEnd()
		return nil
	case "ctrl+enter", "ctrl+s":
		return m.submitForm()
	case "left":
		m.formCaret = max(0, m.formCaret-1)
		return nil
	case "right":
		if fieldIsFocused {
			m.formCaret = min(len([]rune(m.formFields[m.formFocus].value)), m.formCaret+1)
		}
		return nil
	case "home":
		m.formCaret = 0
		return nil
	case "end":
		setCaretToEnd()
		return nil
	case "backspace":
		if fieldIsFocused {
			field := &m.formFields[m.formFocus]
			field.value, m.formCaret = deleteRuneBefore(field.value, m.formCaret)
		}
		return nil
	case "delete":
		if fieldIsFocused {
			field := &m.formFields[m.formFocus]
			field.value = deleteRuneAt(field.value, m.formCaret)
		}
		return nil
	case "ctrl+u":
		if fieldIsFocused {
			m.formFields[m.formFocus].value = ""
			m.formCaret = 0
		}
		return nil
	default:
		if key == "space" {
			key = " "
		}
		if fieldIsFocused && isPrintableKey(key) {
			field := &m.formFields[m.formFocus]
			field.value = insertAtRune(field.value, m.formCaret, key)
			m.formCaret += len([]rune(key))
		}
		return nil
	}
}

func (m *model) handleAction(action string) tea.Cmd {
	parts := strings.SplitN(action, ":", 2)
	if len(parts) == 1 {
		return nil
	}
	kind, value := parts[0], parts[1]
	switch kind {
	case "nav":
		m.repoContext = ""
		m.page = page(value)
		if value == string(pageGraph) {
			m.graphFrom = ""
			g, err := m.app.Graph(m.ctx, "", 0)
			if err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.graph = g
			}
		}
		m.cursor, m.scroll = 0, 0
		m.search = ""
	case "repo":
		m.repoContext = value
		m.page = pageRepository
		m.cursor, m.scroll = 0, 0
		m.settings, _ = m.app.GetRepositorySettings(m.ctx, value)
	case "repo-tab":
		m.page = page(value)
		if value == string(pageRepoGraph) && m.repoContext != "" {
			m.graphFrom = "repo:" + m.repoContext
			g, err := m.app.Graph(m.ctx, m.graphFrom, m.graphDepth)
			if err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.graph = g
			}
		}
		m.cursor, m.scroll = 0, 0
		if value == string(pageRepoSettings) && m.repoContext != "" {
			m.settings, _ = m.app.GetRepositorySettings(m.ctx, m.repoContext)
		}
		if value == string(pageRepoLabels) && m.repoContext != "" {
			m.labels, _ = m.app.ListLabels(m.ctx, m.repoContext)
		}
		if value == string(pageRepoMilestones) && m.repoContext != "" {
			m.milestones, _ = m.app.ListMilestones(m.ctx, "repository", m.repoContext)
		}
		if value == string(pageRepoActivity) && m.repoContext != "" {
			if repo, err := m.app.GetRepository(m.ctx, m.repoContext); err == nil {
				m.activity, _ = m.app.ListActivity(m.ctx, "repository", repo.ID, 40)
			}
			m.completions, _ = m.app.ListCompletions(m.ctx, m.repoContext)
		}
	case "new":
		m.page = pageNew
		m.repoContext = ""
	case "form":
		m.beginForm(formKind(value))
	case "form-issue":
		m.repoContext = value
		m.beginForm(formIssue)
	case "issue":
		m.previousPage, m.previousRepo = m.page, m.repoContext
		m.selectedIssueRef = value
		m.page, m.repoContext = pageIssueDetail, ""
		m.cursor, m.scroll = 0, 0
	case "issue-labels":
		m.repoContext, m.page = value, pageRepoLabels
		m.labels, _ = m.app.ListLabels(m.ctx, value)
		m.cursor, m.scroll = 0, 0
	case "issue-milestones":
		m.repoContext, m.page = value, pageRepoMilestones
		m.milestones, _ = m.app.ListMilestones(m.ctx, "repository", value)
		m.cursor, m.scroll = 0, 0
	case "pr":
		m.previousPage, m.previousRepo = m.page, m.repoContext
		m.selectedPRRef = value
		m.page, m.repoContext = pagePRDetail, ""
		m.cursor, m.scroll = 0, 0
	case "project":
		m.previousPage, m.previousRepo = m.page, m.repoContext
		m.selectedProject = value
		m.page, m.repoContext = pageProjectDetail, ""
		m.cursor, m.scroll = 0, 0
	case "concept":
		m.previousPage, m.previousRepo = m.page, m.repoContext
		m.selectedConcept = value
		m.page, m.repoContext = pageConceptDetail, ""
		m.cursor, m.scroll = 0, 0
	case "issue-action":
		switch value {
		case "start":
			return m.runOperation("Issue started.", func() error { _, err := m.app.StartIssue(m.ctx, m.selectedIssueRef); return err })
		case "cancel":
			return m.runOperation("Issue cancelled.", func() error { _, err := m.app.CancelIssue(m.ctx, m.selectedIssueRef); return err })
		case "pr":
			m.beginForm(formPullRequest)
		case "dependency":
			m.beginForm(formDependency)
		case "concept":
			m.beginForm(formConceptLink)
		case "label":
			m.beginForm(formAssignLabel)
		case "milestone":
			m.beginForm(formAssignMilestone)
		case "graph":
			m.graphFrom = "issue:" + m.selectedIssueRef
			m.page, m.repoContext = pageGraph, ""
			g, err := m.app.Graph(m.ctx, m.graphFrom, m.graphDepth)
			if err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.graph = g
			}
		}
	case "pr-action":
		switch value {
		case "accept":
			m.beginForm(formAcceptPR)
		case "close":
			return m.runOperation("Pull request closed.", func() error { _, err := m.app.ClosePullRequest(m.ctx, m.selectedPRRef); return err })
		case "issue":
			for _, pr := range m.pullRequests {
				if prRef(pr) == m.selectedPRRef {
					return m.handleAction("issue:" + fmt.Sprintf("%s#%d", pr.RepositoryName, pr.IssueNumber))
				}
			}
		}
	case "project-action":
		if value == "add-issue" {
			m.beginForm(formProjectIssue)
		}
	case "project-add-issue":
		m.selectedProject = value
		m.beginForm(formProjectIssue)
	case "settings-action":
		if value == "set" {
			m.beginForm(formRepoSetting)
		}
	case "label-action":
		if value == "new" {
			m.beginForm(formLabel)
		}
	case "milestone-action":
		if value == "new" {
			m.beginForm(formMilestone)
		}
	case "graph":
		switch {
		case value == "all":
			m.graphFrom = ""
			m.page = pageGraph
			m.repoContext = ""
			g, err := m.app.Graph(m.ctx, "", 0)
			if err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.graph = g
			}
		case strings.HasPrefix(value, "focus-concept:"):
			m.graphFrom = "concept:" + strings.TrimPrefix(value, "focus-concept:")
			m.page, m.repoContext = pageGraph, ""
			g, err := m.app.Graph(m.ctx, m.graphFrom, m.graphDepth)
			if err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.graph = g
			}
		case strings.HasPrefix(value, "focus-issue:"):
			m.graphFrom = "issue:" + strings.TrimPrefix(value, "focus-issue:")
			m.page, m.repoContext = pageGraph, ""
			g, err := m.app.Graph(m.ctx, m.graphFrom, m.graphDepth)
			if err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.graph = g
			}
		}
	case "pr-accept":
		m.selectedPRRef = value
		m.beginForm(formAcceptPR)
	case "form-action":
		switch value {
		case "submit":
			return m.submitForm()
		case "cancel":
			m.form, m.formFields, m.formError = "", nil, ""
			m.page = m.formReturnPage
			m.repoContext = m.formReturnRepo
		}
	case "form-field":
		var index int
		if _, err := fmt.Sscanf(value, "%d", &index); err == nil && index >= 0 && index < len(m.formFields) {
			m.formFocus = index
			m.formCaret = len([]rune(m.formFields[index].value))
		}
	case "graph-node":
		if n := m.findGraphNode(value); n != nil {
			switch n.Type {
			case "issue":
				return m.handleAction("issue:" + strings.TrimPrefix(n.Reference, "issue:"))
			case "repo", "repository":
				return m.handleAction("repo:" + strings.TrimPrefix(n.Reference, "repo:"))
			case "concept":
				return m.handleAction("concept:" + strings.TrimPrefix(n.Reference, "concept:"))
			case "project":
				return m.handleAction("project:" + strings.TrimPrefix(n.Reference, "project:"))
			case "pull_request":
				return m.handleAction("pr:" + strings.TrimPrefix(n.Reference, "pr:"))
			case "branch":
				ref := strings.TrimPrefix(n.Reference, "branch:")
				if repo, _, ok := strings.Cut(ref, "/"); ok {
					return m.handleAction("repo:" + repo)
				}
			}
		}
	case "dependency-remove":
		refs := strings.SplitN(value, "|", 2)
		if len(refs) == 2 {
			issueRefValue, prerequisiteRef := refs[0], refs[1]
			return m.runOperation("Blocking prerequisite removed.", func() error { return m.app.RemoveDependency(m.ctx, issueRefValue, prerequisiteRef) })
		}
	case "back":
		m.goBack()
	case "search":
		m.searchMode = true
	case "help":
		m.page = pageHelp
	case "refresh":
		return loadSnapshotCmd(m.ctx, m.app)
	case "quit":
		return tea.Quit
	}
	return nil
}

func (m *model) runOperation(message string, fn func() error) tea.Cmd {
	return func() tea.Msg {
		return operationMsg{message: message, err: fn()}
	}
}

func (m *model) beginForm(kind formKind) {
	m.formReturnPage, m.formReturnRepo = m.page, m.repoContext
	m.form, m.formFocus, m.formError = kind, 0, ""
	m.formTitle = formTitle(kind)
	m.page = pageForm
	m.formFields = m.fieldsFor(kind)
	m.formCaret = 0
	if len(m.formFields) > 0 {
		m.formCaret = len([]rune(m.formFields[0].value))
	}
}

func (m *model) fieldsFor(kind formKind) []formField {
	issueRef := m.selectedIssueRef
	prRefValue := m.selectedPRRef
	issueRepoDefault := m.repoContext
	if issueRepoDefault == "" && len(m.repositories) > 0 {
		issueRepoDefault = m.repositories[0].Name
	}
	fields := map[formKind][]formField{
		formRepository: {
			{label: "Repository name", placeholder: "CPU", helper: "A unique workspace repository; main is created automatically."},
			{label: "Description", placeholder: "What belongs in this repository?", helper: "Optional. Keep it short and specific."},
		},
		formProject: {
			{label: "Project name", placeholder: "OS research", helper: "Projects can group Issues from multiple repositories."},
			{label: "Description", placeholder: "Outcome or goal", helper: "Optional."},
		},
		formIssue: {
			{label: "Repository", value: issueRepoDefault, placeholder: "CPU", helper: "The repository that owns this Issue."},
			{label: "Title", placeholder: "Learn electricity fundamentals", helper: "Describe one clear unit of work."},
			{label: "Description", placeholder: "Expected result and context", helper: "Optional, one line. Detailed result belongs in the PR."},
			{label: "Priority", value: "normal", placeholder: "low / normal / high", helper: "Use low, normal, or high."},
			{label: "Parent Issue", placeholder: "CPU#3 (optional)", helper: "Creates a Sub-issue. Cross-repository prerequisites use depends_on instead."},
		},
		formConcept: {
			{label: "Concept name", placeholder: "Virtual memory", helper: "Concepts can connect knowledge across repositories."},
			{label: "Description", placeholder: "Definition or notes", helper: "Optional."},
		},
		formPullRequest: {
			{label: "Issue reference", value: issueRef, placeholder: "CPU#3", helper: "The Issue must be in progress and unblocked."},
			{label: "Pull request title", placeholder: "Implement paging overview", helper: "Optional; defaults to the Issue title."},
			{label: "Work explanation", placeholder: "What was done and what is the result?", helper: "Required. This is the main record of the work."},
		},
		formAcceptPR: {
			{label: "Pull request", value: prRefValue, placeholder: "CPU!2", helper: "Accepting records completion on main and marks its Issue done."},
			{label: "Completion summary", placeholder: "Outcome and useful notes", helper: "Optional; the PR remains the full explanation."},
		},
		formDependency: {
			{label: "Issue that is blocked", value: issueRef, placeholder: "ARLIZ#3", helper: "This Issue cannot start before the prerequisite is complete."},
			{label: "Prerequisite Issue", placeholder: "CPU#4", helper: "May belong to another repository."},
		},
		formConceptLink: {
			{label: "Issue reference", value: issueRef, placeholder: "ARLIZ#3", helper: "Semantic links do not block work."},
			{label: "Concept", placeholder: "Electricity or a Concept ID", helper: "Connect the Issue to existing knowledge."},
		},
		formProjectIssue: {
			{label: "Project", value: m.selectedProject, placeholder: "OS research", helper: "The selected Project receives the Issue."},
			{label: "Issue reference", placeholder: "CPU#4", helper: "Issues from other repositories are supported."},
		},
		formRepoSetting: {
			{label: "Setting key", placeholder: "description", helper: "Saved in this repository's local settings."},
			{label: "Value", placeholder: "New value", helper: "Use repository Settings for custom local metadata."},
		},
		formLabel: {
			{label: "Label name", placeholder: "research", helper: "A reusable filter label."},
			{label: "Scope", value: "repository", placeholder: "global or repository", helper: "Use global for workspace-wide labels, repository for this repository."},
			{label: "Description", placeholder: "When to use this label", helper: "Optional."},
			{label: "Color", placeholder: "#79c0ff", helper: "Optional display color."},
		},
		formMilestone: {
			{label: "Title", placeholder: "Chapter 3 draft", helper: "A named checkpoint for this repository."},
			{label: "Description", placeholder: "What does completion mean?", helper: "Optional."},
			{label: "Due date", placeholder: "YYYY-MM-DD", helper: "Optional."},
		},
		formAssignLabel: {
			{label: "Issue reference", value: issueRef, placeholder: "CPU#4", helper: "The label is attached to this Issue."},
			{label: "Label name or ID", placeholder: "research", helper: "Global labels and this Repository's labels are accepted."},
		},
		formAssignMilestone: {
			{label: "Issue reference", value: issueRef, placeholder: "CPU#4", helper: "The Issue must belong to a compatible scope."},
			{label: "Milestone title or ID", placeholder: "Chapter 3 draft", helper: "Choose a Repository milestone or its stable ID."},
		},
	}
	return append([]formField(nil), fields[kind]...)
}

func (m *model) submitForm() tea.Cmd {
	kind := m.form
	fields := make([]string, len(m.formFields))
	for i, f := range m.formFields {
		fields[i] = strings.TrimSpace(f.value)
	}
	get := func(i int) string {
		if i < 0 || i >= len(fields) {
			return ""
		}
		return fields[i]
	}
	app, ctx, repoContext, selectedProject, selectedPR := m.app, m.ctx, m.repoContext, m.selectedProject, m.selectedPRRef
	return func() tea.Msg {
		result := operationMsg{}
		switch kind {
		case formRepository:
			if get(0) == "" {
				result.err = fmt.Errorf("repository name is required")
				break
			}
			_, err := app.CreateRepository(ctx, get(0), get(1))
			result.err = err
			if err == nil {
				result.message, result.openPage = "Repository created with default branch main.", pageRepositories
			}
		case formProject:
			if get(0) == "" {
				result.err = fmt.Errorf("project name is required")
				break
			}
			_, err := app.CreateProject(ctx, get(0), get(1))
			result.err = err
			if err == nil {
				result.message, result.openPage = "Project created.", pageProjects
			}
		case formIssue:
			if get(0) == "" || get(1) == "" {
				result.err = fmt.Errorf("repository and issue title are required")
				break
			}
			i, err := app.CreateIssue(ctx, get(0), get(1), get(2), get(4), "", get(3))
			result.err = err
			if err == nil {
				result.message, result.openIssue = fmt.Sprintf("Created %s#%d.", i.RepositoryName, i.Number), fmt.Sprintf("%s#%d", i.RepositoryName, i.Number)
			}
		case formConcept:
			if get(0) == "" {
				result.err = fmt.Errorf("concept name is required")
				break
			}
			c, err := app.CreateConcept(ctx, get(0), get(1))
			result.err = err
			if err == nil {
				result.message, result.openConcept = "Concept created.", c.Name
			}
		case formPullRequest:
			if get(0) == "" || get(2) == "" {
				result.err = fmt.Errorf("issue reference and work explanation are required")
				break
			}
			p, err := app.CreatePullRequest(ctx, get(0), get(1), get(2))
			result.err = err
			if err == nil {
				result.message, result.openPR = fmt.Sprintf("Created %s!%d.", p.RepositoryName, p.Number), fmt.Sprintf("%s!%d", p.RepositoryName, p.Number)
			}
		case formAcceptPR:
			ref := get(0)
			if ref == "" {
				ref = selectedPR
			}
			if ref == "" {
				result.err = fmt.Errorf("pull request reference is required")
				break
			}
			p, c, i, err := app.AcceptPullRequest(ctx, ref, get(1))
			result.err = err
			if err == nil {
				result.message = fmt.Sprintf("Accepted %s!%d · completion %s/%s/%d · Issue marked done.", p.RepositoryName, p.Number, c.RepositoryName, c.BranchName, c.Sequence)
				result.openIssue = fmt.Sprintf("%s#%d", i.RepositoryName, i.Number)
			}
		case formDependency:
			if get(0) == "" || get(1) == "" {
				result.err = fmt.Errorf("both Issue references are required")
				break
			}
			result.err = app.AddDependency(ctx, get(0), get(1))
			if result.err == nil {
				result.message = "Blocking prerequisite added."
				result.openIssue = get(0)
			}
		case formConceptLink:
			if get(0) == "" || get(1) == "" {
				result.err = fmt.Errorf("Issue and Concept are required")
				break
			}
			result.err = app.LinkIssueConcept(ctx, get(0), get(1))
			if result.err == nil {
				result.message = "Concept linked. This is a semantic relation, not a blocker."
				result.openIssue = get(0)
			}
		case formProjectIssue:
			project := get(0)
			if project == "" {
				project = selectedProject
			}
			if project == "" || get(1) == "" {
				result.err = fmt.Errorf("project and Issue reference are required")
				break
			}
			result.err = app.AddIssueToProject(ctx, project, get(1))
			if result.err == nil {
				result.message, result.openProject = "Issue added to Project.", project
			}
		case formRepoSetting:
			if repoContext == "" || get(0) == "" {
				result.err = fmt.Errorf("repository and setting key are required")
				break
			}
			result.err = app.SetRepositorySetting(ctx, repoContext, get(0), get(1))
			if result.err == nil {
				result.message, result.openPage = "Repository setting saved.", pageRepoSettings
			}
		case formLabel:
			if get(0) == "" {
				result.err = fmt.Errorf("label name is required")
				break
			}
			scope, owner := get(1), repoContext
			if scope != "global" {
				scope = "repository"
			}
			if scope == "global" {
				owner = ""
			}
			_, err := app.CreateLabel(ctx, get(0), scope, owner, get(2), get(3))
			result.err = err
			if err == nil {
				result.message, result.openPage = "Label created.", pageRepoLabels
			}
		case formMilestone:
			if get(0) == "" || repoContext == "" {
				result.err = fmt.Errorf("milestone title and repository are required")
				break
			}
			_, err := app.CreateMilestone(ctx, "repository", repoContext, get(0), get(1), get(2))
			result.err = err
			if err == nil {
				result.message, result.openPage = "Milestone created.", pageRepoMilestones
			}
		case formAssignLabel:
			if get(0) == "" || get(1) == "" {
				result.err = fmt.Errorf("Issue reference and label are required")
				break
			}
			result.err = app.AssignLabel(ctx, get(0), get(1))
			if result.err == nil {
				result.message, result.openIssue = "Label assigned.", get(0)
			}
		case formAssignMilestone:
			if get(0) == "" || get(1) == "" {
				result.err = fmt.Errorf("Issue reference and milestone are required")
				break
			}
			result.err = app.AssignMilestone(ctx, get(0), get(1))
			if result.err == nil {
				result.message, result.openIssue = "Milestone assigned.", get(0)
			}
		default:
			result.err = fmt.Errorf("unsupported form")
		}
		if result.err == nil && result.message == "" {
			result.message = "Changes saved."
		}
		return result
	}
}

func (m *model) goBack() {
	m.page = m.previousPage
	m.repoContext = m.previousRepo
	if m.page == "" {
		m.page = pageOverview
	}
	m.cursor, m.scroll = 0, 0
}

func (m *model) cycleNavigation(direction int) {
	nav := []page{pageOverview, pageRepositories, pageProjects, pageIssues, pagePullRequests, pageGraph, pageConcepts, pageSettings}
	index := 0
	for i, p := range nav {
		if p == m.page {
			index = i
			break
		}
	}
	index = (index + direction + len(nav)) % len(nav)
	m.repoContext, m.page, m.cursor, m.scroll = "", nav[index], 0, 0
}
func (m *model) navigateIndex(index int) {
	nav := []page{pageOverview, pageRepositories, pageProjects, pageIssues, pagePullRequests, pageGraph, pageConcepts, pageSettings}
	if index < len(nav) {
		m.repoContext, m.page, m.cursor, m.scroll = "", nav[index], 0, 0
	}
}
func (m *model) clampViewport() {
	items := m.cursorItems()
	if m.cursor < 0 {
		m.cursor = 0
	}
	if len(items) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(items) {
		m.cursor = len(items) - 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	maxScroll := max(0, m.contentLines-max(1, m.bodyHeight))
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
}

func (m *model) moveCursor(delta int) {
	items := m.cursorItems()
	if len(items) == 0 {
		return
	}
	m.cursor = max(0, min(len(items)-1, m.cursor+delta))
	if m.cursor < len(m.selectLines) {
		line := m.selectLines[m.cursor]
		if line < m.scroll {
			m.scroll = line
		}
		if m.bodyHeight > 0 && line >= m.scroll+m.bodyHeight {
			m.scroll = line - m.bodyHeight + 1
		}
	}
	m.clampViewport()
}

func (m *model) openCursor() tea.Cmd {
	items := m.cursorItems()
	if len(items) == 0 {
		return nil
	}
	idx := m.cursor % len(items)
	return m.handleAction(items[idx].action)
}

type cursorItem struct{ label, action string }

func (m *model) cursorItems() []cursorItem {
	items := []cursorItem{}
	switch m.page {
	case pageRepositories:
		for _, r := range m.filteredRepositories() {
			items = append(items, cursorItem{r.Name, "repo:" + r.Name})
		}
	case pageProjects:
		for _, p := range m.filteredProjects() {
			items = append(items, cursorItem{p.Name, "project:" + p.Name})
		}
	case pageIssues, pageRepoIssues:
		for _, i := range m.filteredIssues() {
			items = append(items, cursorItem{issueRef(i), "issue:" + issueRef(i)})
		}
	case pagePullRequests, pageRepoPRs:
		for _, p := range m.filteredPRs() {
			items = append(items, cursorItem{prRef(p), "pr:" + prRef(p)})
		}
	case pageConcepts:
		for _, c := range m.filteredConcepts() {
			items = append(items, cursorItem{c.Name, "concept:" + c.Name})
		}
	case pageGraph, pageRepoGraph:
		for _, n := range sortedGraphNodes(m.graph.Nodes) {
			items = append(items, cursorItem{graphNodeRef(n), "graph-node:" + n.ID})
		}
	case pageNew:
		for _, f := range []formKind{formRepository, formProject, formIssue, formConcept} {
			items = append(items, cursorItem{string(f), "form:" + string(f)})
		}
	}
	return items
}

func (m *model) filteredRepositories() []domain.Repository {
	out := []domain.Repository{}
	for _, v := range m.repositories {
		if containsFold(v.Name+" "+v.Description, m.search) {
			out = append(out, v)
		}
	}
	return out
}
func (m *model) filteredProjects() []domain.Project {
	out := []domain.Project{}
	for _, v := range m.projects {
		if containsFold(v.Name+" "+v.Description, m.search) {
			out = append(out, v)
		}
	}
	return out
}
func (m *model) filteredIssues() []domain.Issue {
	out := []domain.Issue{}
	for _, v := range m.issues {
		if m.repoContext != "" && !strings.EqualFold(v.RepositoryName, m.repoContext) {
			continue
		}
		if containsFold(v.RepositoryName+" "+v.Title+" "+v.Description+" "+string(v.Status), m.search) {
			out = append(out, v)
		}
	}
	return out
}
func (m *model) filteredPRs() []domain.PullRequest {
	out := []domain.PullRequest{}
	for _, v := range m.pullRequests {
		if m.repoContext != "" && !strings.EqualFold(v.RepositoryName, m.repoContext) {
			continue
		}
		if containsFold(v.RepositoryName+" "+v.Title+" "+v.Description+" "+string(v.Status), m.search) {
			out = append(out, v)
		}
	}
	return out
}
func (m *model) filteredConcepts() []domain.Concept {
	out := []domain.Concept{}
	for _, v := range m.concepts {
		if containsFold(v.Name+" "+v.Description, m.search) {
			out = append(out, v)
		}
	}
	return out
}
func containsFold(haystack, needle string) bool {
	return needle == "" || strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
func issueRef(i domain.Issue) string    { return fmt.Sprintf("%s#%d", i.RepositoryName, i.Number) }
func prRef(p domain.PullRequest) string { return fmt.Sprintf("%s!%d", p.RepositoryName, p.Number) }
func trimLastRune(value string) string {
	r := []rune(value)
	if len(r) == 0 {
		return value
	}
	return string(r[:len(r)-1])
}
func isPrintableKey(value string) bool { return len([]rune(value)) == 1 && value != "\x00" }

func insertAtRune(value string, position int, insertion string) string {
	runes := []rune(value)
	position = max(0, min(position, len(runes)))
	return string(runes[:position]) + insertion + string(runes[position:])
}

func deleteRuneBefore(value string, position int) (string, int) {
	runes := []rune(value)
	position = max(0, min(position, len(runes)))
	if position == 0 {
		return value, 0
	}
	return string(runes[:position-1]) + string(runes[position:]), position - 1
}

func deleteRuneAt(value string, position int) string {
	runes := []rune(value)
	position = max(0, min(position, len(runes)))
	if position >= len(runes) {
		return value
	}
	return string(runes[:position]) + string(runes[position+1:])
}

func caretValue(value string, position int, active bool) string {
	if !active {
		return value
	}
	runes := []rune(value)
	position = max(0, min(position, len(runes)))
	return string(runes[:position]) + "▏" + string(runes[position:])
}
func (m *model) findGraphNode(id string) *domain.GraphNode {
	for i := range m.graph.Nodes {
		if m.graph.Nodes[i].ID == id {
			return &m.graph.Nodes[i]
		}
	}
	return nil
}

// compile-time assertions keep the adapter aligned with Bubble Tea's v2 model contract.
var _ tea.Model = (*model)(nil)

func formTitle(kind formKind) string {
	titles := map[formKind]string{
		formRepository:      "Create repository",
		formProject:         "Create project",
		formIssue:           "Create issue",
		formConcept:         "Create concept",
		formPullRequest:     "Open pull request",
		formAcceptPR:        "Review and accept pull request",
		formDependency:      "Add blocking prerequisite",
		formConceptLink:     "Connect issue to concept",
		formProjectIssue:    "Add issue to project",
		formRepoSetting:     "Edit repository setting",
		formLabel:           "Create label",
		formMilestone:       "Create milestone",
		formAssignLabel:     "Assign label to issue",
		formAssignMilestone: "Assign milestone to issue",
	}
	if title := titles[kind]; title != "" {
		return title
	}
	return "IDIOS form"
}
