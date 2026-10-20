package tui

import (
	"fmt"
	"strings"
)

type commandEntry struct {
	title       string
	description string
	action      string
}

func (m *model) commandEntries() []commandEntry {
	return []commandEntry{
		{"Open Overview", "Workspace summary and recent activity", "nav:overview"},
		{"Browse repositories", "Search and manage repositories", "nav:repositories"},
		{"Open projects", "Group work across repositories", "nav:projects"},
		{"Open issues", "Filter and manage issues", "nav:issues"},
		{"Open pull requests", "Review local pull-request records", "nav:pull-requests"},
		{"Open knowledge graph", "Explore typed relationships", "nav:graph"},
		{"Open concepts", "Manage reusable knowledge", "nav:concepts"},
		{"Open notes", "Capture and link notes", "nav:notes"},
		{"Open quotes", "Manage saved quotes", "nav:quotes"},
		{"Open scheduled jobs", "Manage reminders and schedules", "nav:jobs"},
		{"Remove items", "Safely remove repositories, Issues, PRs, concepts, notes, quotes, jobs, labels and milestones", "nav:remove-center"},
		{"Open notifications", "Review reminder notifications", "nav:notifications"},
		{"Open logs", "Inspect recent UI events", "nav:logs"},
		{"Open settings", "Theme, density, reminders and random quote notifications", "nav:settings"},
		{"Toggle random quote reminders", "Enable or disable randomized quote notifications", "settings-action:quote-reminders"},
		{"Change quote reminder interval", "Cycle between minute and hour ranges", "settings-action:quote-interval"},
		{"Send test quote notification", "Check whether the desktop notification integration works", "settings-action:test-quote-reminder"},
		{"Create repository", "Create a single repository", "form:repository"},
		{"Create Issue", "Create one issue with its full template", "form:issue"},
		{"Bulk create Issues", "Paste one title per line", "form:bulk-issues"},
		{"Create quote", "Save one quote with attribution", "quote-action:new"},
		{"Bulk save quotes", "Paste one quote per line", "form:bulk-quotes"},
		{"Create scheduled job", "Configure one reminder", "job-action:new"},
		{"Bulk create scheduled jobs", "Paste job names with a shared schedule", "form:bulk-jobs"},
		{"Create project", "Group related work", "form:project"},
		{"Create concept", "Add a reusable knowledge concept", "form:concept"},
		{"Exit IDIOS", "Close the workspace gracefully", "quit:confirm"},
	}
}

func (m *model) filteredCommandEntries() []commandEntry {
	query := strings.TrimSpace(m.commandQuery)
	out := make([]commandEntry, 0)
	for _, entry := range m.commandEntries() {
		if query == "" || containsFold(entry.title+" "+entry.description, query) {
			out = append(out, entry)
		}
	}
	return out
}

func (m *model) openCommandPalette() {
	if m.form != "" {
		m.status, m.statusError = "Finish or cancel the current form before opening the command palette.", true
		return
	}
	if m.page == pageQuit || m.page == pageBatchProgress {
		m.status, m.statusError = "Close this confirmation or finish the running batch before opening commands.", true
		return
	}
	m.commandPreviousPage, m.commandPreviousRepo = m.page, m.repoContext
	m.commandQuery = ""
	m.page = pageCommands
	m.repoContext = ""
	m.cursor, m.scroll = 0, 0
	m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
}

func (m *model) closeCommandPalette() {
	m.page, m.repoContext = m.commandPreviousPage, m.commandPreviousRepo
	if m.page == "" || m.page == pageCommands {
		m.page = pageOverview
	}
	m.cursor, m.scroll = 0, 0
}

func (m *model) renderCommandPalette(c *canvas, p palette) {
	c.title("Command palette")
	c.muted("Run an action without navigating through menus. Start typing to filter commands.")
	query := m.commandQuery
	if query == "" {
		query = "Type to search commands…"
	}
	c.panel("SEARCH", query)
	entries := m.filteredCommandEntries()
	c.muted(stringsForCommandCount(len(entries)))
	if len(entries) == 0 {
		c.panel("NO MATCHES", "Try a shorter search, or Ctrl+U to clear it.")
		return
	}
	for i, entry := range entries {
		m.selectLines = append(m.selectLines, len(c.lines))
		c.button(entry.title, "command:"+entry.action, i == m.cursor)
		c.space(1)
		c.muted(entry.description)
		c.blank()
	}
	c.buttonRow(buttonSpec{"Close", "commands:close", false})
}

func stringsForCommandCount(count int) string {
	if count == 1 {
		return "1 command"
	}
	return fmt.Sprintf("%d commands", count)
}
