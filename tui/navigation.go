// Package tui implements a full-screen, event-driven terminal interface.
// It is a presentation adapter: every read/write operation goes through application.Service.
package tui

import (
	tea "charm.land/bubbletea/v2"
	"idios/internal/domain"
	"strings"
)

func (m *model) goBack() {
	m.page = m.previousPage
	m.repoContext = m.previousRepo
	if m.page == "" {
		m.page = pageOverview
	}
	m.cursor, m.scroll = 0, 0
}

func (m *model) cycleNavigation(direction int) {
	nav := []page{pageOverview, pageRepositories, pageProjects, pageIssues, pagePullRequests, pageGraph, pageConcepts, pageNotes, pageQuotes, pageJobs, pageNotifications, pageLogs, pageSettings}
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

// cycleButtonFocus exposes every currently visible button and input to keyboard-only users.
// It uses the same action IDs as mouse clicks, so keyboard and pointer behavior cannot drift.
func (m *model) cycleButtonFocus(direction int) {
	if len(m.hits) == 0 {
		m.buttonFocus = true
		m.focusAction = ""
		return
	}
	if !m.buttonFocus || m.focusIndex < 0 || m.focusIndex >= len(m.hits) {
		if direction < 0 {
			m.focusIndex = len(m.hits) - 1
		} else {
			m.focusIndex = 0
		}
	} else {
		m.focusIndex = (m.focusIndex + direction + len(m.hits)) % len(m.hits)
	}
	m.buttonFocus = true
	m.focusAction = m.hits[m.focusIndex].action
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
	case pageNotes:
		for _, n := range m.notes {
			items = append(items, cursorItem{n.Title, "note:" + n.ID})
		}
	case pageJobs:
		for _, job := range m.jobs {
			items = append(items, cursorItem{job.Name, "job:" + job.ID})
		}
	case pageNotifications:
		items = append(items, cursorItem{"Mark all notifications read", "notification-action:read-all"})
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

func (m *model) findGraphNode(id string) *domain.GraphNode {
	for i := range m.graph.Nodes {
		if m.graph.Nodes[i].ID == id {
			return &m.graph.Nodes[i]
		}
	}
	return nil
}

// compile-time assertions keep the adapter aligned with Bubble Tea's v2 model contract.

func (m *model) shortcutAction(key string) string {
	// Explicit order avoids map iteration randomness if a user accidentally configures two
	// actions with the same key. First match wins and the Settings page documents defaults.
	for _, action := range []string{"quit", "new", "search", "help", "overview", "repositories", "projects", "issues", "pull_requests", "graph", "concepts", "settings", "notes", "quotes", "jobs", "notifications", "logs", "profile", "edit_profile", "theme", "density", "debug", "toggle_notifications", "intro", "poll_interval", "new_note", "new_quote", "new_job", "mark_notifications_read", "refresh", "seed"} {
		if shortcut := strings.TrimSpace(m.config.Keyboard[action]); shortcut != "" && shortcut == key {
			return action
		}
	}
	return ""
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func formTitle(kind formKind) string {
	titles := map[formKind]string{
		formRepository: "Create repository", formProject: "Create project", formIssue: "Create issue",
		formConcept: "Create concept", formPullRequest: "Create pull request", formAcceptPR: "Accept pull request",
		formDependency: "Add blocking prerequisite", formConceptLink: "Link issue to concept", formProjectIssue: "Add issue to project",
		formRepoSetting: "Edit repository setting", formLabel: "Create label", formMilestone: "Create milestone",
		formAssignLabel: "Assign label", formAssignMilestone: "Assign milestone", formNote: "Write a note",
		formNoteLink: "Link note to an item", formQuote: "Save a quote", formJob: "Schedule a reminder", formProfile: "Edit workspace profile",
	}
	if title := titles[kind]; title != "" {
		return title
	}
	return "IDIOS form"
}
