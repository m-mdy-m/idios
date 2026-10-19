// Package tui implements a full-screen, event-driven terminal interface.
// It is a presentation adapter: every read/write operation goes through application.Service.
package tui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"idios/internal/config"
	"idios/internal/domain"
	"strings"
)

func (m *model) handleAction(action string) tea.Cmd {
	m.logEvent("DEBUG", "action="+action)
	parts := strings.SplitN(action, ":", 2)
	if len(parts) == 1 {
		return nil
	}
	kind, value := parts[0], parts[1]
	switch kind {
	case "nav":
		m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
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
				return m.runOperation("Note deleted.", func() error { return m.app.DeleteNote(m.ctx, id) })
			}
		case "graph":
			for _, n := range m.notes {
				if n.ID == m.selectedNoteID && n.LinkedEntityType != "" {
					return m.runOperation("Note graph visibility updated.", func() error { return m.app.LinkNote(m.ctx, n.ID, n.LinkedEntityType, n.LinkedEntityID, true) })
				}
			}
		}
	case "quote-action":
		if value == "new" {
			m.beginForm(formQuote)
		}
	case "job":
		return m.runOperation("Scheduled job toggled.", func() error { _, err := m.app.ToggleScheduledJob(m.ctx, value); return err })
	case "job-action":
		if value == "new" {
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
	case "demo":
		if value == "seed" {
			return m.runOperation("Demo workspace loaded.", func() error { _, err := m.app.SeedDemo(m.ctx); return err })
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
	m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
	m.formReturnPage, m.formReturnRepo = m.page, m.repoContext
	m.form, m.formFocus, m.formError = kind, 0, ""
	m.formTitle = formTitle(kind)
	m.activeIssueTemplate = m.config.IssueTemplate.Default
	m.page = pageForm
	m.formFields = m.fieldsFor(kind)
	if kind == formIssue {
		if definition, ok := m.config.IssueTemplate.Templates[m.activeIssueTemplate]; ok {
			m.formTitle = "Create Issue · " + fallback(definition.Name, m.activeIssueTemplate)
		}
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
	if kind == formRepository {
		return []formField{
			{label: "Repository name", placeholder: "CPU", helper: "A unique workspace repository; the main branch is created automatically."},
			{label: "README / repository overview", value: m.config.RepositoryTemplate.Description, placeholder: "Purpose, scope, getting started and architecture", helper: "A structured overview shown on the Repository page. Edit REPOSITORY_TEMPLATE in config.json to change the starter content.", multiline: true},
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
			{label: "Issue template", value: key, placeholder: "task / bug / feature / research", helper: "Type a configured template key, then press Enter to apply it. Available: " + strings.Join(issueTemplateKeys(m.config), ", ")},
			{label: "Repository", value: issueRepoDefault, placeholder: "CPU", helper: "The Repository that owns this Issue."},
			{label: "Issue title", value: definition.TitlePrefix, placeholder: "One clear, actionable outcome", helper: "Keep the title specific. A template prefix is added automatically when configured."},
			{label: "Description / issue brief", value: definition.Body, placeholder: "Describe the context and acceptance criteria", helper: "This is a multiline template. Enter adds a line; Tab changes fields; Ctrl+Enter creates the Issue.", multiline: true},
			{label: "Priority", value: "normal", placeholder: "low / normal / high", helper: "Use low, normal, or high."},
			{label: "Parent Issue", placeholder: "CPU#3 (optional)", helper: "Creates a Sub-issue. Cross-repository prerequisites use depends_on instead."},
		}
	}
	if kind == formPullRequest {
		title := ""
		if selected := m.findIssue(issueRef); selected != nil {
			title = strings.TrimSpace(m.config.PullRequestTemplate.TitlePrefix + selected.Title)
		}
		return []formField{
			{label: "Related Issue", value: issueRef, placeholder: "CPU#3", helper: "The Issue must be in progress and have no unfinished blockers."},
			{label: "Pull request title", value: title, placeholder: "Describe the reviewed change", helper: "Keep this aligned with the related Issue and the actual outcome."},
			{label: "Pull request description", value: m.config.PullRequestTemplate.Body, placeholder: "Summary, verification, risks and reviewer notes", helper: "Explain what changed, how it was verified, and any remaining risk. Enter adds a line; Tab changes fields; Ctrl+Enter creates the PR.", multiline: true},
		}
	}
	fields := map[formKind][]formField{
		formProject: {
			{label: "Project name", placeholder: "OS research", helper: "Projects can group Issues from multiple repositories."},
			{label: "Description", placeholder: "Outcome or goal", helper: "Optional."},
		},
		formConcept: {
			{label: "Concept name", placeholder: "Virtual memory", helper: "Concepts can connect knowledge across repositories."},
			{label: "Description", placeholder: "Definition or notes", helper: "Optional."},
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
			{label: "Color", placeholder: "optional color token", helper: "Optional label color; it does not change the application theme."},
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
		formNote: func() []formField {
			if m.selectedNoteID != "" {
				for _, n := range m.notes {
					if n.ID == m.selectedNoteID {
						return []formField{{label: "Title", value: n.Title, placeholder: "A thought worth keeping"}, {label: "Note", value: n.Content, placeholder: "Write freely; connect it later.", helper: "Enter adds a line; Ctrl+Enter saves.", multiline: true}}
					}
				}
			}
			return []formField{{label: "Title", placeholder: "A thought worth keeping", helper: "A short name makes notes searchable."}, {label: "Note", placeholder: "Write freely; connect it later.", helper: "Keep raw thoughts here, then link them to work or the graph. Enter adds a line; Ctrl+Enter saves.", multiline: true}}
		}(),
		formNoteLink: {
			{label: "Note ID", value: m.selectedNoteID, placeholder: "Select a note first", helper: "Choose an existing note from the Notes page."},
			{label: "Entity type", value: "issue", placeholder: "issue / repository / project / concept", helper: "The type of item this note belongs to."},
			{label: "Entity reference", placeholder: "IDIOS#2 or IDIOS", helper: "Use a repository name, Issue reference, Project name or Concept name."},
			{label: "Show in graph?", value: "yes", placeholder: "yes / no", helper: "Set yes to make the note a node connected to the selected entity."},
		},
		formQuote: {
			{label: "Quote", placeholder: "A line worth seeing again", helper: "Quotes rotate on each application launch."},
			{label: "Author / source", placeholder: "Optional attribution", helper: "Leave blank for your own words."},
		},
		formJob: {
			{label: "Reminder name", placeholder: "Weekly workspace review", helper: "A clear title for the notification."},
			{label: "Schedule", value: "daily", placeholder: "hourly / daily / weekly / monthly", helper: "Supported intervals: hourly, daily, weekly or monthly."},
			{label: "Reminder kind", value: "weekly-review", placeholder: "daily-plan / weekly-review / blocked-issues / open-issues / custom-reminder", helper: "Select the checklist IDIOS should remind you about."},
			{label: "Extra note", placeholder: "Optional text shown with the reminder", helper: "Useful for custom-reminder."},
		},
		formProfile: {
			{label: "Username", value: m.profile.Username, placeholder: "workspace-owner", helper: "Shown in your Overview profile block."},
			{label: "Display name", value: m.profile.FullName, placeholder: "Workspace owner", helper: "Shown as the heading on Overview."},
			{label: "Bio", value: m.profile.Bio, placeholder: "What is this workspace for?", helper: "One or two lines are ideal."},
			{label: "README / overview", value: m.profile.Readme, placeholder: "Markdown-like plain text for your workspace.", helper: "A short overview shown on your profile page."},
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
	app, ctx, repoContext, selectedProject, selectedPR, selectedNoteID := m.app, m.ctx, m.repoContext, m.selectedProject, m.selectedPRRef, m.selectedNoteID
	appConfig := m.config
	activeIssueTemplate := m.activeIssueTemplate
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
			if selectedNoteID != "" {
				n, err = app.UpdateNote(ctx, selectedNoteID, get(0), get(1))
			} else {
				n, err = app.CreateNote(ctx, get(0), get(1))
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
				result.err = fmt.Errorf("reminder name is required")
				break
			}
			intervals := map[string]int{"hourly": 60, "daily": 1440, "weekly": 10080, "monthly": 43200}
			schedule := strings.ToLower(get(1))
			interval, ok := intervals[schedule]
			if !ok {
				result.err = fmt.Errorf("schedule must be hourly, daily, weekly or monthly")
				break
			}
			action := strings.ToLower(get(2))
			allowed := map[string]bool{"daily-plan": true, "weekly-review": true, "blocked-issues": true, "open-issues": true, "custom-reminder": true}
			if !allowed[action] {
				result.err = fmt.Errorf("unsupported reminder kind %q", action)
				break
			}
			_, err := app.CreateScheduledJob(ctx, domain.ScheduledJob{Name: get(0), Description: get(3), ActionType: action, IntervalMinutes: interval, Enabled: true, Notify: true})
			result.err = err
			if err == nil {
				result.message, result.openPage = "Scheduled reminder created.", pageJobs
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
