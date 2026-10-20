// Package tui implements a full-screen, event-driven terminal interface.
// It is a presentation adapter: every read/write operation goes through application.Service.
package tui

import (
	"fmt"
	"idios/internal/config"
	"idios/internal/domain"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *model) handleAction(action string) tea.Cmd {
	m.logEvent("DEBUG", "action="+action)
	parts := strings.SplitN(action, ":", 2)
	if len(parts) == 1 {
		return nil
	}
	kind, value := parts[0], parts[1]
	// Do not silently hide a running batch or replace a live form because a
	// navigation control was clicked. Explicit cancel/submit/quit actions remain available.
	if m.batchRun != nil && kind != "quit" && !(kind == "batch" && value == "cancel") {
		m.status, m.statusError = "A batch is running. Cancel it safely or let it finish before navigating.", true
		return nil
	}
	if m.form != "" {
		switch kind {
		case "nav", "repo", "new", "form", "commands", "command", "job-action", "quote-action", "note-action", "remove":
			m.status, m.statusError = "Finish or cancel the current form before navigating or starting another action.", true
			return nil
		}
	}
	switch kind {
	case "upgrade":
		switch value {
		case "open":
			if m.availableUpdate == nil {
				return nil
			}
			m.page = pageUpgrade
			m.cursor, m.scroll = 0, 0
			m.upgradeError = ""
		case "later":
			m.page = pageOverview
			m.cursor, m.scroll = 0, 0
		case "start":
			if m.availableUpdate == nil || m.upgradeInstalling {
				return nil
			}
			m.page = pageUpgrade
			m.upgradeInstalling = true
			m.upgradeHandedOff = false
			m.upgradeFrame = 0
			m.upgradeStartedAt = time.Now()
			m.upgradeError = ""
			m.status, m.statusError = "Preparing IDIOS upgrade…", false
			return tea.Batch(prepareUpgradeCmd(m.ctx, *m.availableUpdate), upgradeAnimationTickCmd())
		}
		return nil
	case "nav":
		m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
		m.repoContext = ""
		m.page = page(value)
		if value == string(pageRemove) {
			m.removeKind = ""
		}
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
		m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
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
	case "batch":
		switch value {
		case "retry-failed":
			m.retryFailedBatch()
		case "cancel":
			m.requestBatchCancel()
		}
	case "commands":
		if value == "open" {
			m.openCommandPalette()
		} else if value == "close" {
			m.closeCommandPalette()
		}
	case "command":
		// Restore the prior view before running the command so forms return to the right page.
		command := value
		m.closeCommandPalette()
		return m.handleAction(command)
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
		case "edit":
			m.beginForm(formEditPR)
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
		switch value {
		case "set":
			m.beginForm(formRepoSetting)
		case "theme":
			names := themeNames(m.config)
			if len(names) > 0 {
				index := 0
				for i, name := range names {
					if name == m.config.Appearance.Theme {
						index = i
						break
					}
				}
				m.config.Appearance.Theme = names[(index+1)%len(names)]
				if err := config.SaveAppConfig(m.config); err != nil {
					m.status, m.statusError = err.Error(), true
				} else {
					m.status, m.statusError = "Theme changed to "+m.config.Appearance.Theme+".", false
					m.logEvent("INFO", m.status)
				}
			}
		case "debug":
			m.config.Debug = !m.config.Debug
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = fmt.Sprintf(copyText.text("Debug logging %s."), onOff(m.config.Debug)), false
				m.logEvent("INFO", m.status)
			}
		case "notifications":
			m.config.Notifications.Enabled = !m.config.Notifications.Enabled
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = fmt.Sprintf(copyText.text("Scheduled notifications %s."), onOff(m.config.Notifications.Enabled)), false
				m.logEvent("INFO", m.status)
			}
		case "intro":
			m.config.Startup.ShowIntro = !m.config.Startup.ShowIntro
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = fmt.Sprintf(copyText.text("Startup intro %s on next launch."), onOff(m.config.Startup.ShowIntro)), false
			}
		case "density":
			if m.config.Appearance.Density == "compact" {
				m.config.Appearance.Density = "comfortable"
			} else {
				m.config.Appearance.Density = "compact"
			}
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = "Layout density: "+m.config.Appearance.Density, false
			}
		case "jobs":
			return m.handleAction("nav:jobs")
		case "notifications-page":
			return m.handleAction("nav:settings-notifications")
		case "quote-reminders":
			r := &m.config.Notifications.QuoteReminders
			r.Enabled = !r.Enabled
			if r.Enabled {
				r.NextAt = nextQuoteReminderAt(time.Now(), *r).Format(time.RFC3339Nano)
			} else {
				r.NextAt = ""
			}
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = "Random quote reminders "+onOff(r.Enabled)+".", false
				m.logEvent("INFO", m.status)
			}
		case "quote-desktop":
			r := &m.config.Notifications.QuoteReminders
			r.DesktopEnabled = !r.DesktopEnabled
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = "Desktop quote notifications "+onOff(r.DesktopEnabled)+".", false
				m.logEvent("INFO", m.status)
			}
		case "quote-interval":
			type intervalRange struct{ min, max int }
			presets := []intervalRange{{1, 5}, {15, 60}, {30, 120}, {60, 240}, {180, 720}}
			r := &m.config.Notifications.QuoteReminders
			index := -1
			for i, preset := range presets {
				if preset.min == r.MinIntervalMinutes && preset.max == r.MaxIntervalMinutes {
					index = i
					break
				}
			}
			preset := presets[(index+1)%len(presets)]
			r.MinIntervalMinutes, r.MaxIntervalMinutes = preset.min, preset.max
			if r.Enabled {
				r.NextAt = nextQuoteReminderAt(time.Now(), *r).Format(time.RFC3339Nano)
			}
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = fmt.Sprintf("Random quote interval set to %d–%d minutes.", r.MinIntervalMinutes, r.MaxIntervalMinutes), false
				m.logEvent("INFO", m.status)
			}
		case "test-quote-reminder":
			return sendTestQuoteCmd(m.ctx, m.app, m.config.Notifications.QuoteReminders.LastQuoteID)
		case "poll-interval":
			values := []int{1, 5, 15, 60}
			index := 0
			for i, v := range values {
				if v == m.config.Notifications.PollIntervalMinutes {
					index = i
					break
				}
			}
			m.config.Notifications.PollIntervalMinutes = values[(index+1)%len(values)]
			m.nextJobsCheckAt = time.Now().Add(time.Duration(m.config.Notifications.PollIntervalMinutes) * time.Minute)
			if err := config.SaveAppConfig(m.config); err != nil {
				m.status, m.statusError = err.Error(), true
			} else {
				m.status, m.statusError = fmt.Sprintf(copyText.text("Notification check interval: every %d minute(s)."), m.config.Notifications.PollIntervalMinutes), false
			}
		case "logs":
			return m.handleAction("nav:logs")
		case "profile":
			m.beginForm(formProfile)
		case "help":
			m.page = pageHelp
		}
	case "label-action":
		if value == "new" {
			m.beginForm(formLabel)
		}
	case "milestone-action":
		if value == "new" {
			m.beginForm(formMilestone)
		}
	case "note":
		m.selectedNoteID, m.page, m.repoContext = value, pageNotes, ""
		m.cursor, m.scroll = 0, 0
	case "note-action":
		switch value {
		case "new":
			m.selectedNoteID = ""
			m.beginForm(formNote)
		case "edit":
			m.beginForm(formNote)
		case "link":
			m.beginForm(formNoteLink)
		case "delete":
			if m.selectedNoteID != "" {
				id := m.selectedNoteID
				return m.handleAction("remove:select:note|" + id)
			}
		case "graph":
			for _, n := range m.notes {
				if n.ID == m.selectedNoteID && n.LinkedEntityType != "" {
					return m.runOperation("Note graph visibility updated.", func() error { return m.app.LinkNote(m.ctx, n.ID, n.LinkedEntityType, n.LinkedEntityID, true) })
				}
			}
		}
	case "note-filter":
		m.activeNoteLabel = ""
		if value != "all" {
			m.activeNoteLabel = value
		}
		m.selectedNoteID = ""
		m.cursor, m.scroll = 0, 0
	case "quote-action":
		if value == "new" {
			m.beginForm(formQuote)
		}
	case "job":
		return m.runOperation("Scheduled job toggled.", func() error { _, err := m.app.ToggleScheduledJob(m.ctx, value); return err })
	case "job-edit":
		m.selectedJobID = value
		m.beginForm(formJob)
	case "job-toggle":
		return m.runOperation("Scheduled job state updated.", func() error { _, err := m.app.ToggleScheduledJob(m.ctx, value); return err })
	case "job-run":
		id := value
		ctx, app, notificationsEnabled := m.ctx, m.app, m.config.Notifications.Enabled
		return m.runOperation("Scheduled job ran now.", func() error { return app.RunScheduledJobNow(ctx, id, time.Now(), notificationsEnabled) })
	case "job-delete":
		id := value
		return m.handleAction("remove:select:job|" + id)
	case "job-action":
		if value == "new" {
			m.selectedJobID = ""
			m.beginForm(formJob)
		}
	case "notification-action":
		if value == "read-all" {
			return m.runOperation("Notifications marked as read.", func() error { return m.app.MarkNotificationsRead(m.ctx) })
		}
	case "pin":
		pinParts := strings.SplitN(value, ":", 2)
		if len(pinParts) == 2 {
			entityType, ref := pinParts[0], pinParts[1]
			label := ref
			color := "accent"
			if entityType == "issue" {
				color = "warning"
			}
			return func() tea.Msg {
				added, err := m.app.ToggleWorkspacePin(m.ctx, entityType, ref, label, color)
				message := "Removed from Important items."
				if added {
					message = "Added to Important items."
				}
				return operationMsg{message: message, err: err}
			}
		}
	case "remove-kind":
		m.removeKind = value
		m.page, m.cursor, m.scroll = pageRemove, 0, 0
		m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
	case "remove":
		switch {
		case value == "categories":
			m.removeKind = ""
			m.page, m.cursor, m.scroll = pageRemove, 0, 0
		case value == "cancel":
			m.deleteTarget = deleteTarget{}
			m.page, m.cursor, m.scroll = pageRemove, 0, 0
		case value == "confirm":
			if m.deleteTarget.ID == "" {
				m.status, m.statusError = "No deletion target is selected.", true
				return nil
			}
			if m.deleteTarget.TypeNameRequired {
				m.beginRepositoryDeleteConfirmation()
				return nil
			}
			return m.runDeleteTarget()
		case strings.HasPrefix(value, "select:"):
			kindAndID := strings.TrimPrefix(value, "select:")
			kind, id, ok := strings.Cut(kindAndID, "|")
			if !ok || kind == "" || id == "" {
				m.status, m.statusError = "Invalid removal target.", true
				return nil
			}
			if m.beginRemoveTarget(kind, id) {
				if m.deleteTarget.TypeNameRequired {
					// First show the impact and require a deliberate second action before typing.
					m.page = pageDeleteConfirm
				}
			}
		}
	case "graph":
		switch {
		case value == "structure":
			m.graphStructure = !m.graphStructure
			m.cursor, m.scroll = 0, 0
		case value == "depth":
			// 1 → 2 → 3 → 1: how many links away from the focused node are still drawn.
			m.graphDepth = m.graphDepth%3 + 1
			m.cursor, m.scroll = 0, 0
			if m.graphFrom != "" {
				g, err := m.app.Graph(m.ctx, m.graphFrom, m.graphDepth)
				if err != nil {
					m.status, m.statusError = err.Error(), true
				} else {
					m.graph = g
				}
			}
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
	case "pr-edit":
		m.selectedPRRef = value
		m.beginForm(formEditPR)
	case "pr-accept":
		m.selectedPRRef = value
		m.beginForm(formAcceptPR)
	case "form-action":
		switch value {
		case "submit":
			return m.submitForm()
		case "cancel":
			m.form, m.formFields, m.formError = "", nil, ""
			m.batchConfirm = false
			m.batchRetrying, m.batchCarryResults = false, nil
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
			case "note":
				return m.handleAction("note:" + strings.TrimPrefix(n.Reference, "note:"))
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
		if value == "confirm" || value == "now" {
			m.openQuitConfirmation()
		} else if value == "cancel" {
			m.cancelQuitConfirmation()
		} else if value == "exit" {
			return m.startExitAnimation()
		}
	}
	return nil
}

func (m *model) runOperation(message string, fn func() error) tea.Cmd {
	return func() tea.Msg {
		return operationMsg{message: message, err: fn()}
	}
}

func (m *model) beginForm(kind formKind) {
	m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
	m.formReturnPage, m.formReturnRepo = m.page, m.repoContext
	m.form, m.formFocus, m.formError = kind, 0, ""
	m.batchConfirm = false
	m.formTitle = formTitle(kind)
	m.batchRetrying, m.batchCarryResults = false, nil
	m.activeIssueTemplate = m.config.IssueTemplate.Default
	m.page = pageForm
	m.formFields = m.fieldsFor(kind)
	if kind == formIssue {
		if definition, ok := m.config.IssueTemplate.Templates[m.activeIssueTemplate]; ok {
			m.formTitle = "Create Issue · " + fallback(definition.Name, m.activeIssueTemplate)
		}
	}
	if kind == formJob && m.selectedJobID != "" {
		m.formTitle = "Edit scheduled job"
	}
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
	if kind == formDeleteRepositoryConfirm {
		return []formField{{label: "Type the repository name", placeholder: m.deleteTarget.Label, helper: repositoryDeleteConfirmItem(m.deleteTarget)}}
	}
	if kind == formRepository {
		return []formField{
			{label: "Repository name", placeholder: "CPU", helper: "Must be unique. Used in references such as CPU#4. A main branch is created for you."},
			{label: "README / repository overview", value: m.config.RepositoryTemplate.Description, placeholder: "Purpose, scope, how to get started", helper: "Shown on the repository page. To change the starting text, edit REPOSITORY_TEMPLATE in config.json.", multiline: true},
		}
	}
	if kind == formIssue {
		key := strings.TrimSpace(m.config.IssueTemplate.Default)
		definition, ok := m.config.IssueTemplate.Templates[key]
		if !ok {
			key, definition = defaultIssueTemplate(m.config)
		}
		m.activeIssueTemplate = key
		return []formField{
			{label: "Issue template", value: key, placeholder: "task / bug / feature / research", helper: "Type a template name and press Enter to load it. Available: " + strings.Join(issueTemplateKeys(m.config), ", ")},
			{label: "Repository", value: issueRepoDefault, placeholder: "CPU", helper: "The repository this issue belongs to."},
			{label: "Issue title", value: definition.TitlePrefix, placeholder: "What should be true when this is done?", helper: "Be specific. The template may add a prefix such as [Bug]:."},
			{label: "Description / issue brief", value: definition.Body, placeholder: "Context and how to tell it is done", helper: "Enter adds a line. Tab moves to the next field. Ctrl+Enter creates the issue.", multiline: true},
			{label: "Priority", value: "normal", placeholder: "low / normal / high", helper: "low, normal or high."},
			{label: "Parent Issue", placeholder: "CPU#3 (optional)", helper: "Makes this a sub-issue of that issue (same repository only). For work in other repositories use a prerequisite."},
		}
	}
	if kind == formPullRequest {
		title := ""
		if selected := m.findIssue(issueRef); selected != nil {
			title = strings.TrimSpace(m.config.PullRequestTemplate.TitlePrefix + selected.Title)
		}
		return []formField{
			{label: "Related Issue", value: issueRef, placeholder: "CPU#3", helper: "The issue must be in progress and have no unfinished prerequisites."},
			{label: "Pull request title", value: title, placeholder: "What was done", helper: "Should match the issue and what you actually did."},
			{label: "Pull request description", value: m.config.PullRequestTemplate.Body, placeholder: "What changed, how you checked it, what is left", helper: "Explain what changed, how you verified it, and any remaining risk. Enter adds a line; Tab changes fields; Ctrl+Enter creates the PR.", multiline: true},
		}
	}
	if kind == formEditPR {
		title, description := "", ""
		if pr := m.findPR(prRefValue); pr != nil {
			title, description = pr.Title, pr.Description
		}
		return []formField{
			{label: "Pull request", value: prRefValue, placeholder: "CPU!2", helper: "Only open pull requests can be edited."},
			{label: "Pull request title", value: title, placeholder: "What was done", helper: "Change it if the scope of the work changed."},
			{label: "Pull request description", value: description, placeholder: "What you have done so far", helper: "Update this as you make progress. Enter adds a line; Tab changes fields; Ctrl+Enter saves.", multiline: true},
		}
	}
	fields := map[formKind][]formField{
		formProject: {
			{label: "Project name", placeholder: "OS research", helper: "A project can contain issues from several repositories."},
			{label: "Description", placeholder: "The goal of this project", helper: "Optional."},
		},
		formConcept: {
			{label: "Concept name", placeholder: "Virtual memory", helper: "A topic you can link from issues in any repository."},
			{label: "Description", placeholder: "Short definition or notes", helper: "Optional."},
		},
		formAcceptPR: {
			{label: "Pull request", value: prRefValue, placeholder: "CPU!2", helper: "Accepting marks its issue done."},
			{label: "Completion summary", placeholder: "One-line result", helper: "Optional. The pull request keeps the full description."},
		},
		formDependency: {
			{label: "Issue that is blocked", value: issueRef, placeholder: "ARLIZ#3", helper: "This issue cannot start until the prerequisite is done."},
			{label: "Prerequisite Issue", placeholder: "CPU#4", helper: "The issue that must finish first. It can be in another repository."},
		},
		formConceptLink: {
			{label: "Issue reference", value: issueRef, placeholder: "ARLIZ#3", helper: "A concept link never blocks work."},
			{label: "Concept", placeholder: "Electricity or a Concept ID", helper: "Name of an existing concept."},
		},
		formProjectIssue: {
			{label: "Project", value: m.selectedProject, placeholder: "OS research", helper: "The issue is added to this project."},
			{label: "Issue reference", placeholder: "CPU#4", helper: "Any repository works."},
		},
		formRepoSetting: {
			{label: "Setting key", placeholder: "description", helper: "Stored with this repository."},
			{label: "Value", placeholder: "New value", helper: "Any text."},
		},
		formLabel: {
			{label: "Label name", placeholder: "research", helper: "Short tag you can put on many issues."},
			{label: "Scope", value: "repository", placeholder: "global or repository", helper: "global = usable everywhere, repository = only in this repository."},
			{label: "Description", placeholder: "When to use this label", helper: "Optional."},
			{label: "Color", placeholder: "optional color token", helper: "Optional. Only changes this label."},
		},
		formMilestone: {
			{label: "Title", placeholder: "v1 release", helper: "A target for this repository."},
			{label: "Description", placeholder: "What has to be true to call it done?", helper: "Optional."},
			{label: "Due date", placeholder: "YYYY-MM-DD", helper: "Optional."},
		},
		formAssignLabel: {
			{label: "Issue reference", value: issueRef, placeholder: "CPU#4", helper: "The label is added to this issue."},
			{label: "Label name or ID", placeholder: "research", helper: "Global labels and labels of this repository."},
		},
		formAssignMilestone: {
			{label: "Issue reference", value: issueRef, placeholder: "CPU#4", helper: "The issue must be in the milestone's repository."},
			{label: "Milestone title or ID", placeholder: "v1 release", helper: "Milestone title or ID."},
		},
		formNote: func() []formField {
			if m.selectedNoteID != "" {
				for _, n := range m.notes {
					if n.ID == m.selectedNoteID {
						return []formField{
							{label: "Title", value: n.Title, placeholder: "A thought worth keeping"},
							{label: "Note", value: n.Content, placeholder: "Write the note", helper: "Enter adds a line; Ctrl+Enter saves.", multiline: true},
							{label: "Labels", value: noteLabelNames(n.Labels), placeholder: "research, idea, read-later", helper: "Comma-separated tags. Reusing a name groups notes under the same label."},
						}
					}
				}
			}
			return []formField{
				{label: "Title", placeholder: "Short title", helper: "Used when searching and in lists."},
				{label: "Note", placeholder: "Write the note", helper: "Keep raw thoughts here, then link them to work or the graph. Enter adds a line; Ctrl+Enter saves.", multiline: true},
				{label: "Labels", placeholder: "research, idea, read-later", helper: "Add several comma-separated labels; existing labels are reused automatically."},
			}
		}(),
		formNoteLink: {
			{label: "Note ID", value: m.selectedNoteID, placeholder: "Select a note first", helper: "Choose an existing note from the Notes page."},
			{label: "Entity type", value: "issue", placeholder: "issue / repository / project / concept", helper: "The type of item this note belongs to."},
			{label: "Entity reference", placeholder: "IDIOS#2 or IDIOS", helper: "Use a repository name, Issue reference, Project name or Concept name."},
			{label: "Show in graph?", value: "yes", placeholder: "yes / no", helper: "Set yes to make the note a node connected to the selected entity."},
		},
		formQuote: {
			{label: "Quote", placeholder: "The quote", helper: "One saved quote is shown at each start."},
			{label: "Author / source", placeholder: "Optional attribution", helper: "Leave blank for your own words."},
		},
		formJob: func() []formField {
			if m.selectedJobID != "" {
				for _, job := range m.jobs {
					if job.ID == m.selectedJobID {
						return []formField{
							{label: "Job name", value: job.Name, placeholder: "Weekly review"},
							{label: "Schedule type", value: job.ScheduleType, placeholder: "interval / once / daily / weekly / monthly", helper: "interval repeats; once runs a single time; calendar schedules use local time."},
							{label: "Schedule details", value: jobScheduleInput(job), placeholder: "1h | 2026-10-25 14:30 | 09:00 | Mon 09:00 | 15 09:00", helper: "interval: 1h/30m/2d; once: YYYY-MM-DD HH:MM; daily: HH:MM; weekly: Mon HH:MM; monthly: DD HH:MM."},
							{label: "Reminder kind", value: job.ActionType, placeholder: "daily-plan / weekly-review / blocked-issues / open-issues / custom-reminder"},
							{label: "Extra note", value: job.Description, placeholder: "Optional text", helper: "Shown with the reminder; mainly for custom-reminder."},
							{label: "Send notification?", value: yesNo(job.Notify), placeholder: "yes / no", helper: "The global notification switch must also be on."},
						}
					}
				}
			}
			return []formField{
				{label: "Job name", placeholder: "Weekly review", helper: "Title of the notification."},
				{label: "Schedule type", value: "interval", placeholder: "interval / once / daily / weekly / monthly", helper: "interval repeats; once runs a single time; calendar schedules use local time."},
				{label: "Schedule details", value: "1h", placeholder: "1h | 2026-10-25 14:30 | 09:00 | Mon 09:00 | 15 09:00", helper: "interval: 1h/30m/2d; once: YYYY-MM-DD HH:MM; daily: HH:MM; weekly: Mon HH:MM; monthly: DD HH:MM."},
				{label: "Reminder kind", value: "custom-reminder", placeholder: "daily-plan / weekly-review / blocked-issues / open-issues / custom-reminder", helper: "What the notification should contain."},
				{label: "Extra note", placeholder: "Optional text", helper: "Shown with the reminder; mainly for custom-reminder."},
				{label: "Send notification?", value: "yes", placeholder: "yes / no", helper: "The global notification switch must also be on."},
			}
		}(),
		formBulkIssues: {
			{label: "Repository", value: issueRepoDefault, placeholder: "IDIOS", helper: "All Issues in this batch go to this repository."},
			{label: "Issue titles", placeholder: "One Issue title per line", helper: "Paste a list; blank lines are ignored. Up to 100 Issues per batch.", multiline: true},
			{label: "Shared description", placeholder: "Optional context shared by every Issue", helper: "Use this for common instructions; add specific details later on each Issue.", multiline: true},
			{label: "Priority", value: "normal", placeholder: "low / normal / high", helper: "Applied to every Issue in the batch."},
		},
		formBulkQuotes: {
			{label: "Quotes", placeholder: "One quote per line", helper: "Each non-empty line becomes a separate quote. Up to 100 per batch.", multiline: true},
			{label: "Author / source", placeholder: "Optional shared attribution", helper: "Applied to every quote in this batch."},
		},
		formBulkJobs: {
			{label: "Job names", placeholder: "Weekly review\nRead later\nPlan tomorrow", helper: "One scheduled job per line. Up to 100 jobs; the schedule below is shared.", multiline: true},
			{label: "Schedule type", value: "interval", placeholder: "interval / once / daily / weekly / monthly", helper: "All jobs use this schedule."},
			{label: "Schedule details", value: "1h", placeholder: "1h | 2026-10-25 14:30 | 09:00 | Mon 09:00 | 15 09:00", helper: "interval: 1h/30m/2d; once: YYYY-MM-DD HH:MM; daily: HH:MM; weekly: Mon HH:MM; monthly: 15 09:00."},
			{label: "Reminder kind", value: "custom-reminder", placeholder: "daily-plan / weekly-review / blocked-issues / open-issues / custom-reminder", helper: "Applied to every job."},
			{label: "Extra note", placeholder: "Optional shared reminder note", helper: "Shown with each reminder; mainly for custom-reminder."},
			{label: "Send notification?", value: "yes", placeholder: "yes / no", helper: "The global notification switch must also be on."},
		},
		formProfile: {
			{label: "Username", value: m.profile.Username, placeholder: "workspace-owner", helper: "Shown in your Overview profile block."},
			{label: "Display name", value: m.profile.FullName, placeholder: "Workspace owner", helper: "Shown as the heading on Overview."},
			{label: "Bio", value: m.profile.Bio, placeholder: "A line about you or this workspace", helper: "One or two lines."},
			{label: "README / overview", value: m.profile.Readme, placeholder: "Plain text; shown on the Overview", helper: "Goals, current focus, links."},
		},
	}
	return append([]formField(nil), fields[kind]...)
}

func defaultIssueTemplate(cfg config.AppConfig) (string, config.IssueTemplateDefinition) {
	if definition, ok := cfg.IssueTemplate.Templates[cfg.IssueTemplate.Default]; ok {
		return cfg.IssueTemplate.Default, definition
	}
	keys := issueTemplateKeys(cfg)
	if len(keys) > 0 {
		return keys[0], cfg.IssueTemplate.Templates[keys[0]]
	}
	return "task", config.IssueTemplateDefinition{Name: "Task", Body: "## Goal\n\nDescribe the outcome and acceptance criteria."}
}

func issueTemplateKeys(cfg config.AppConfig) []string {
	keys := make([]string, 0, len(cfg.IssueTemplate.Templates))
	for key := range cfg.IssueTemplate.Templates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *model) applyIssueTemplateSelection() bool {
	if len(m.formFields) < 4 {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(m.formFields[0].value))
	definition, ok := m.config.IssueTemplate.Templates[key]
	if !ok {
		m.formError = fmt.Sprintf("Unknown Issue template %q. Available templates: %s", key, strings.Join(issueTemplateKeys(m.config), ", "))
		return false
	}
	currentTitle := m.formFields[2].value
	oldDefinition := m.config.IssueTemplate.Templates[m.activeIssueTemplate]
	switch {
	case strings.TrimSpace(currentTitle) == "" || currentTitle == oldDefinition.TitlePrefix:
		currentTitle = definition.TitlePrefix
	case oldDefinition.TitlePrefix != "" && strings.HasPrefix(currentTitle, oldDefinition.TitlePrefix):
		currentTitle = definition.TitlePrefix + strings.TrimPrefix(currentTitle, oldDefinition.TitlePrefix)
	case definition.TitlePrefix != "" && !strings.HasPrefix(currentTitle, definition.TitlePrefix):
		currentTitle = definition.TitlePrefix + currentTitle
	}
	m.formFields[0].value = key
	m.formFields[2].value = currentTitle
	m.formFields[3].value = definition.Body
	m.activeIssueTemplate = key
	m.formTitle = "Create Issue · " + fallback(definition.Name, key)
	m.formError = ""
	return true
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
	if kind == formDeleteRepositoryConfirm {
		if m.deleteTarget.ID == "" || m.deleteTarget.Kind != "repository" {
			m.formError = "There is no repository selected for deletion."
			return nil
		}
		if get(0) != m.deleteTarget.Label {
			m.formError = "The name must match exactly. Nothing has been deleted."
			return nil
		}
		m.formError = ""
		return m.runDeleteTarget()
	}
	if isBulkForm(kind) {
		if !m.batchConfirm {
			count, err := validateBulkForm(kind, fields, m.repositories)
			if err != nil {
				m.formError = err.Error()
				return nil
			}
			m.batchConfirm = true
			m.formError = ""
			m.status = fmt.Sprintf("Preview ready: %d item(s). Press Ctrl+Enter again to create the batch.", count)
			m.statusError = false
			return nil
		}
		if !m.batchRetrying {
			m.batchCarryResults = nil
		}
		m.lastBatchForm = kind
		m.lastBatchFields = append([]formField(nil), m.formFields...)
		m.batchConfirm = false
		m.batchTitle = map[formKind]string{formBulkIssues: "Issue batch", formBulkQuotes: "Quote batch", formBulkJobs: "Scheduled job batch"}[kind]
		inputIndex := batchInputIndex(kind)
		items := batchLines(get(inputIndex))
		seen := map[string]bool{}
		if kind == formBulkIssues {
			seen = map[string]bool{}
		}
		m.batchRun = &batchRunState{kind: kind, items: items, fields: append([]string(nil), fields...), seen: seen}
		m.batchResults = nil
		m.form, m.formFields, m.formError = "", nil, ""
		m.page = pageBatchProgress
		m.cursor, m.scroll = 0, 0
		m.status, m.statusError = fmt.Sprintf("Starting batch: %d item(s)…", len(items)), false
		return m.nextBatchItemCmd()
	}
	app, ctx, repoContext, selectedProject, selectedPR, selectedNoteID, selectedJobID := m.app, m.ctx, m.repoContext, m.selectedProject, m.selectedPRRef, m.selectedNoteID, m.selectedJobID
	appConfig := m.config
	activeIssueTemplate := m.activeIssueTemplate
	selectedJobEnabled := true
	var selectedJobLastRunAt *time.Time
	if selectedJobID != "" {
		for _, current := range m.jobs {
			if current.ID == selectedJobID {
				selectedJobEnabled = current.Enabled || current.CompletedAt != nil
				selectedJobLastRunAt = current.LastRunAt
				break
			}
		}
	}
	return func() tea.Msg {
		result := operationMsg{}
		switch kind {
		case formRepository:
			if get(0) == "" {
				result.err = fmt.Errorf("repository name is required")
				break
			}
			description := strings.ReplaceAll(get(1), "{{repository}}", get(0))
			_, err := app.CreateRepository(ctx, get(0), description)
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
			templateKey := strings.TrimSpace(get(0))
			definition, exists := appConfig.IssueTemplate.Templates[templateKey]
			title := strings.TrimSpace(get(2))
			if !exists {
				result.err = fmt.Errorf("unknown Issue template %q; choose one of: %s", templateKey, strings.Join(issueTemplateKeys(appConfig), ", "))
				break
			}
			if templateKey != activeIssueTemplate {
				result.err = fmt.Errorf("press Enter on the Issue template field to apply %q before creating the Issue", templateKey)
				break
			}
			if definition.TitlePrefix != "" && title != definition.TitlePrefix && !strings.HasPrefix(title, definition.TitlePrefix) {
				title = definition.TitlePrefix + title
			}
			if get(1) == "" || title == "" || title == definition.TitlePrefix {
				result.err = fmt.Errorf("repository and a specific Issue title are required; keep the title prefix and add a short description after it")
				break
			}
			i, err := app.CreateIssue(ctx, get(1), title, get(3), get(5), "", get(4))
			result.err = err
			if err == nil {
				result.message, result.openIssue = fmt.Sprintf(copyText.text("Created %s#%d using the %s template."), i.RepositoryName, i.Number, definition.Name), fmt.Sprintf("%s#%d", i.RepositoryName, i.Number)
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
			if get(0) == "" || strings.TrimSpace(get(2)) == "" {
				result.err = fmt.Errorf("Issue reference and a completed Pull Request description are required")
				break
			}
			title := strings.TrimSpace(get(1))
			prefix := appConfig.PullRequestTemplate.TitlePrefix
			if prefix != "" && title != "" && !strings.HasPrefix(title, prefix) {
				title = prefix + title
			}
			p, err := app.CreatePullRequest(ctx, get(0), title, get(2))
			result.err = err
			if err == nil {
				result.message, result.openPR = fmt.Sprintf(copyText.text("Created %s!%d."), p.RepositoryName, p.Number), fmt.Sprintf("%s!%d", p.RepositoryName, p.Number)
			}
		case formEditPR:
			ref := get(0)
			if ref == "" {
				ref = selectedPR
			}
			if ref == "" {
				result.err = fmt.Errorf("pull request reference is required")
				break
			}
			p, err := app.UpdatePullRequest(ctx, ref, get(1), get(2))
			result.err = err
			if err == nil {
				result.message, result.openPR = "Pull request updated.", fmt.Sprintf("%s!%d", p.RepositoryName, p.Number)
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
				result.message = fmt.Sprintf(copyText.text("Accepted %s!%d · completion %s/%s/%d · Issue marked done."), p.RepositoryName, p.Number, c.RepositoryName, c.BranchName, c.Sequence)
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
		case formNote:
			var n domain.Note
			var err error
			if get(0) == "" {
				result.err = fmt.Errorf("note title is required")
				break
			}
			labels := splitNoteLabels(get(2))
			if selectedNoteID != "" {
				n, err = app.UpdateNoteWithLabels(ctx, selectedNoteID, get(0), get(1), labels)
			} else {
				n, err = app.CreateNoteWithLabels(ctx, get(0), get(1), labels)
			}
			result.err = err
			if err == nil {
				result.message, result.openPage, result.openNote = "Note saved.", pageNotes, n.ID
			}
		case formNoteLink:
			if get(0) == "" || get(1) == "" || get(2) == "" {
				result.err = fmt.Errorf("note ID, entity type and entity reference are required")
				break
			}
			show := strings.EqualFold(get(3), "yes") || strings.EqualFold(get(3), "true") || get(3) == "1"
			result.err = app.LinkNote(ctx, get(0), strings.ToLower(get(1)), get(2), show)
			if result.err == nil {
				result.message, result.openPage = "Note linked to "+get(1)+".", pageNotes
			}
		case formQuote:
			if get(0) == "" {
				result.err = fmt.Errorf("quote text is required")
				break
			}
			_, err := app.CreateQuote(ctx, get(0), get(1))
			result.err = err
			if err == nil {
				result.message, result.openPage = "Quote saved.", pageQuotes
			}
		case formJob:
			if get(0) == "" {
				result.err = fmt.Errorf("job name is required")
				break
			}
			job, err := parseScheduledJobFields(get(1), get(2))
			if err != nil {
				result.err = err
				break
			}
			job.ID = selectedJobID
			job.Name = get(0)
			job.ActionType = strings.ToLower(get(3))
			job.Description = get(4)
			job.Enabled = selectedJobEnabled
			job.LastRunAt = selectedJobLastRunAt
			job.Notify = !strings.EqualFold(get(5), "no") && !strings.EqualFold(get(5), "false") && get(5) != "0"
			allowed := map[string]bool{"daily-plan": true, "weekly-review": true, "blocked-issues": true, "open-issues": true, "custom-reminder": true}
			if !allowed[job.ActionType] {
				result.err = fmt.Errorf("unsupported reminder kind %q", job.ActionType)
				break
			}
			var saved domain.ScheduledJob
			if selectedJobID != "" {
				saved, err = app.UpdateScheduledJob(ctx, job)
			} else {
				saved, err = app.CreateScheduledJob(ctx, job)
			}
			result.err = err
			if err == nil {
				result.message, result.openPage, result.openJob = "Scheduled job saved.", pageJobs, saved.ID
				if selectedJobID == "" {
					result.message = "Scheduled job created."
				}
			}
		case formProfile:
			p, err := app.UpdateWorkspaceProfile(ctx, domain.WorkspaceProfile{Username: get(0), FullName: get(1), Bio: get(2), Readme: get(3)})
			result.err = err
			if err == nil {
				result.message, result.openPage = "Workspace profile saved.", pageOverview
				_ = p
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
