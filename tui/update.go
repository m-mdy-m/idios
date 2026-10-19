// Package tui implements a full-screen, event-driven terminal interface.
// It is a presentation adapter: every read/write operation goes through application.Service.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

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
		m.notes, m.quotes, m.quote = s.notes, s.quotes, s.quote
		m.jobs, m.notifications, m.profile, m.pins = s.jobs, s.notifications, s.profile, s.pins
		m.rebindDetails()
		m.clampViewport()
		return m, nil
	case schedulerTickMsg:
		interval := time.Duration(m.config.Notifications.PollIntervalMinutes) * time.Minute
		if m.app == nil {
			return m, schedulerTickCmd(interval)
		}
		return m, tea.Batch(processDueJobsCmd(m.ctx, m.app, msg.at, m.config.Notifications.Enabled), schedulerTickCmd(interval))
	case schedulerProcessedMsg:
		if msg.err != nil {
			m.logEvent("ERROR", "scheduled job processor: "+msg.err.Error())
		} else if msg.count > 0 {
			m.logEvent("INFO", fmt.Sprintf("processed %d scheduled job(s)", msg.count))
			m.notifications, _ = m.app.ListNotifications(m.ctx, 50)
		}
		return m, nil
	case actionClickMsg:
		return m, m.handleAction(string(msg))
	case operationMsg:
		if msg.err != nil {
			m.logEvent("ERROR", msg.err.Error())
			m.status, m.statusError = msg.err.Error(), true
			if m.form != "" {
				m.formError = msg.err.Error()
				return m, nil // Keep the form and its input so the user can correct it.
			}
		} else {
			m.status, m.statusError = msg.message, false
			if msg.message != "" {
				m.logEvent("INFO", msg.message)
			}
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
		if msg.openNote != "" {
			m.selectedNoteID, m.page = msg.openNote, pageNotes
		}
		if msg.err == nil && msg.message == "Note deleted." {
			m.selectedNoteID = ""
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
		content := strings.ReplaceAll(strings.ReplaceAll(msg.Content, "\r\n", "\n"), "\r", "\n")
		if m.searchMode {
			m.search += strings.ReplaceAll(content, "\n", " ")
			m.cursor, m.scroll = 0, 0
		} else if m.form != "" && m.formFocus < len(m.formFields) {
			field := &m.formFields[m.formFocus]
			if !field.multiline {
				content = strings.ReplaceAll(content, "\n", " ")
			}
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
		if action := m.shortcutAction(key); action != "" {
			switch action {
			case "overview":
				return m, m.handleAction("nav:overview")
			case "repositories":
				return m, m.handleAction("nav:repositories")
			case "projects":
				return m, m.handleAction("nav:projects")
			case "issues":
				return m, m.handleAction("nav:issues")
			case "pull_requests":
				return m, m.handleAction("nav:pull-requests")
			case "graph":
				return m, m.handleAction("nav:graph")
			case "concepts":
				return m, m.handleAction("nav:concepts")
			case "settings":
				return m, m.handleAction("nav:settings")
			case "notes":
				return m, m.handleAction("nav:notes")
			case "quotes":
				return m, m.handleAction("nav:quotes")
			case "jobs":
				return m, m.handleAction("nav:jobs")
			case "notifications":
				return m, m.handleAction("nav:notifications")
			case "logs":
				return m, m.handleAction("nav:logs")
			case "new":
				m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
				m.page, m.repoContext = pageNew, ""
				m.cursor, m.scroll = 0, 0
				return m, nil
			case "search":
				m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
				m.searchMode = true
				m.status = "Type to filter this view · Enter to finish · Esc to clear"
				return m, nil
			case "help":
				m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
				m.page = pageHelp
				m.cursor, m.scroll = 0, 0
				return m, nil
			case "refresh":
				return m, loadSnapshotCmd(m.ctx, m.app)
			case "profile", "edit_profile":
				m.beginForm(formProfile)
				return m, nil
			case "theme":
				return m, m.handleAction("settings-action:theme")
			case "density":
				return m, m.handleAction("settings-action:density")
			case "debug":
				return m, m.handleAction("settings-action:debug")
			case "toggle_notifications":
				return m, m.handleAction("settings-action:notifications")
			case "intro":
				return m, m.handleAction("settings-action:intro")
			case "poll_interval":
				return m, m.handleAction("settings-action:poll-interval")
			case "new_note":
				return m, m.handleAction("note-action:new")
			case "new_quote":
				return m, m.handleAction("quote-action:new")
			case "new_job":
				return m, m.handleAction("job-action:new")
			case "mark_notifications_read":
				return m, m.handleAction("notification-action:read-all")
			case "quit":
				return m, tea.Quit
			}
		}
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.buttonFocus {
				m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
				return m, nil
			}
			if m.page == pageIssueDetail || m.page == pagePRDetail || m.page == pageProjectDetail || m.page == pageConceptDetail {
				m.goBack()
			} else if m.repoContext != "" {
				m.repoContext = ""
				m.page = pageRepositories
			} else if m.page != pageOverview {
				m.page = pageOverview
				m.cursor, m.scroll = 0, 0
			}
			return m, nil
		case "up", "k":
			if m.buttonFocus {
				m.cycleButtonFocus(-1)
			} else {
				m.moveCursor(-1)
			}
			return m, nil
		case "down", "j":
			if m.buttonFocus {
				m.cycleButtonFocus(1)
			} else {
				m.moveCursor(1)
			}
			return m, nil
		case "enter", "space":
			if m.buttonFocus && m.focusAction != "" {
				action := m.focusAction
				return m, m.handleAction(action)
			}
			return m, m.openCursor()
		case "/":
			m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
			m.searchMode = true
			m.status = "Type to filter this view · Enter to finish · Esc to clear"
			return m, nil
		case "n":
			m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
			m.page = pageNew
			m.repoContext = ""
			m.cursor, m.scroll = 0, 0
			return m, nil
		case "?", "h":
			m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
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
		case "ctrl+tab", "ctrl+right", "f6":
			m.cycleButtonFocus(1)
			return m, nil
		case "ctrl+shift+tab", "ctrl+left", "shift+f6":
			m.cycleButtonFocus(-1)
			return m, nil
		case "tab":
			m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
			m.cycleNavigation(1)
			return m, nil
		case "shift+tab":
			m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
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
	focusedField := func() *formField {
		if m.formFocus < 0 || m.formFocus >= len(m.formFields) {
			return nil
		}
		return &m.formFields[m.formFocus]
	}
	setCaretToEnd := func() {
		if m.formFocus >= 0 && m.formFocus < len(m.formFields) {
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
	case "tab":
		m.formFocus = min(m.formFocus+1, len(m.formFields))
		setCaretToEnd()
		return nil
	case "shift+tab":
		m.formFocus = max(0, m.formFocus-1)
		setCaretToEnd()
		return nil
	case "down":
		if field := focusedField(); field != nil && field.multiline {
			if next, ok := moveMultilineCaret(field.value, m.formCaret, 1); ok {
				m.formCaret = next
				return nil
			}
		}
		m.formFocus = min(m.formFocus+1, len(m.formFields))
		setCaretToEnd()
		return nil
	case "up":
		if field := focusedField(); field != nil && field.multiline {
			if next, ok := moveMultilineCaret(field.value, m.formCaret, -1); ok {
				m.formCaret = next
				return nil
			}
		}
		m.formFocus = max(0, m.formFocus-1)
		setCaretToEnd()
		return nil
	case "enter":
		if field := focusedField(); field != nil && field.multiline {
			field.value = insertAtRune(field.value, m.formCaret, "\n")
			m.formCaret++
			return nil
		}
		if m.form == formIssue && m.formFocus == 0 {
			if !m.applyIssueTemplateSelection() {
				return nil
			}
			m.formFocus = 1
			setCaretToEnd()
			return nil
		}
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
		if field := focusedField(); field != nil {
			m.formCaret = min(len([]rune(field.value)), m.formCaret+1)
		}
		return nil
	case "home":
		if field := focusedField(); field != nil && field.multiline {
			m.formCaret = multilineLineBoundary(field.value, m.formCaret, false)
		} else {
			m.formCaret = 0
		}
		return nil
	case "end":
		if field := focusedField(); field != nil && field.multiline {
			m.formCaret = multilineLineBoundary(field.value, m.formCaret, true)
		} else {
			setCaretToEnd()
		}
		return nil
	case "backspace":
		if field := focusedField(); field != nil {
			field.value, m.formCaret = deleteRuneBefore(field.value, m.formCaret)
		}
		return nil
	case "delete":
		if field := focusedField(); field != nil {
			field.value = deleteRuneAt(field.value, m.formCaret)
		}
		return nil
	case "ctrl+u":
		if field := focusedField(); field != nil {
			field.value = ""
			m.formCaret = 0
		}
		return nil
	default:
		if key == "space" {
			key = " "
		}
		if field := focusedField(); field != nil && isPrintableKey(key) {
			field.value = insertAtRune(field.value, m.formCaret, key)
			m.formCaret += len([]rune(key))
		}
		return nil
	}
}
