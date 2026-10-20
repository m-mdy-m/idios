package tui

import (
	"fmt"
	"strings"
	"time"

	"idios/internal/domain"

	"charm.land/lipgloss/v2"
)

func (m *model) renderIntro(c *canvas, p palette) {
	c.blank()
	// The intro wordmark is purpose-built ASCII rather than a plain text heading.
	// Use the heavy version when there is room and a compact mark in narrow terminals.
	logo := idiosASCIILogo(c.width >= 74)
	for _, line := range logo {
		c.lineText(p.brand().Render(centerASCII(line, c.width)))
	}
	logoWidth := len(logo[0])
	ruleWidth := min(c.width-4, logoWidth+8)
	rule := centerASCII(strings.Repeat("-", max(1, ruleWidth)), c.width)
	c.lineText(p.muted().Render(rule))
	c.lineText(p.title().Render(centerASCII("Issues and pull requests for your own work", c.width)))
	c.lineText(p.muted().Render(rule))
	c.blank()
	c.title(copyText.IntroTitle)
	c.muted(copyText.IntroSubtitle)
	if m.demoMode {
		c.blank()
		c.panel(copyText.DemoWorkspaceTitle, copyText.DemoWorkspaceNotice)
	}
	c.blank()

	// The checklist follows the real state of the database, so it doubles as a progress
	// indicator for a new user and disappears into a short reminder for an experienced one.
	counts := m.issueCounts()
	step := func(done bool, text string) string {
		if done {
			return "✓  " + text
		}
		return "○  " + text
	}
	c.panel("HOW IT WORKS",
		step(len(m.issues) > 0, "ISSUE: write the task. A repository groups related issues, for example one per subject or project."),
		step(counts[domain.IssueInProgress]+counts[domain.IssueDone] > 0, "START it when you begin working on it."),
		step(m.openPRCount()+counts[domain.IssueDone] > 0, "PULL REQUEST: write what you have done so far. Edit it as the work progresses."),
		step(counts[domain.IssueDone] > 0, "ACCEPT the pull request when the work is finished. The issue becomes done."),
		"Also: sub-issues split a task, prerequisites make an issue wait for another, projects group issues across repositories, concepts and notes keep what you learn.")
	c.muted(fmt.Sprintf(copyText.text("%d repositories · %d issues · %d concepts · %d notes"), len(m.repositories), len(m.issues), len(m.concepts), len(m.notes)))
	c.blank()
	if m.quote.ID != "" {
		c.panel("QUOTE", "“"+m.quote.Text+"”", fallback(m.quote.Author, "Saved quote"))
		c.blank()
	}
	c.buttonRow(
		buttonSpec{copyText.IntroContinue, "nav:overview", true},
		buttonSpec{copyText.IntroHelp, "nav:help", false},
		buttonSpec{copyText.IntroSettings, "nav:settings", false},
	)
	c.blank()
	c.panel("KEYS", "Ctrl+N new item   /  filter list   ?  help   g  graph   q  quit", "Tab / Shift+Tab switch sections   F6 / Ctrl+Tab focus buttons   Enter open   Esc back")
}

func (m *model) renderNotes(c *canvas, p palette) {
	c.title("Notes")
	c.muted("Capture ideas like a lightweight Notion: edit notes, reuse multiple labels, search, filter and link to work.")
	c.buttonRow(buttonSpec{"+ New note", "note-action:new", true}, buttonSpec{"Quotes", "nav:quotes", false}, buttonSpec{"Explore graph", "nav:graph", false})
	filters := []buttonSpec{{"All notes", "note-filter:all", m.activeNoteLabel == ""}}
	for _, label := range m.noteLabels {
		filters = append(filters, buttonSpec{"#" + label.Name, "note-filter:" + label.Name, strings.EqualFold(m.activeNoteLabel, label.Name)})
	}
	if len(filters) > 0 {
		c.buttonRow(filters...)
	}
	items := m.filteredNotes()
	c.muted(fmt.Sprintf("%d note(s) · %d reusable label(s) · use / to search title, content and labels", len(items), len(m.noteLabels)))
	if len(items) == 0 {
		if len(m.notes) == 0 {
			c.panel("NO NOTES YET", copyText.NotesEmpty, "Create a note and add labels such as #idea, #research or #read-later.")
		} else {
			c.panel("NO MATCHING NOTES", "Try another label or clear the search with / then Ctrl+U.")
		}
		return
	}
	for idx, n := range items {
		marker := "  "
		if n.ID == m.selectedNoteID || idx == m.cursor {
			marker = "▸ "
		}
		label := marker + n.Title
		m.selectLines = append(m.selectLines, len(c.lines))
		c.button(label, "note:"+n.ID, idx == m.cursor || n.ID == m.selectedNoteID)
		c.space(1)
		if n.LinkedEntityLabel != "" {
			c.text("↳ " + truncateVisible(n.LinkedEntityLabel, max(8, c.width/2)))
		} else {
			c.text("not linked")
		}
		c.end()
		if len(n.Labels) > 0 {
			c.space(2)
			for _, tag := range n.Labels {
				c.raw(p.style(true, noteLabelColorToken(tag.Color), "default").Render("#" + tag.Name))
				c.space(2)
			}
			c.end()
		}
		if n.GraphVisible {
			c.lineText("  ◉ visible in graph")
		}
		c.muted(strings.Repeat("─", max(1, c.width-2)))
	}
	if n := m.selectedNote(); n != nil {
		visible := false
		for _, item := range items {
			if item.ID == n.ID {
				visible = true
				break
			}
		}
		if !visible {
			return
		}
		c.blank()
		c.title("Selected note")
		c.panel(n.Title, strings.Split(n.Content, "\n")...)
		c.muted("Created " + n.CreatedAt.Format("2006-01-02 15:04") + " · Updated " + n.UpdatedAt.Format("2006-01-02 15:04"))
		if n.LinkedEntityLabel != "" {
			c.muted("Linked to: " + n.LinkedEntityLabel)
		}
		if len(n.Labels) > 0 {
			c.text("Labels: ")
			for _, tag := range n.Labels {
				c.raw(p.style(true, noteLabelColorToken(tag.Color), "default").Render("#" + tag.Name))
				c.space(2)
			}
			c.end()
		}
		c.buttonRow(buttonSpec{"Edit", "note-action:edit", true}, buttonSpec{"Link / graph settings", "note-action:link", false}, buttonSpec{"Delete", "note-action:delete", false})
	}
}

func noteLabelColorToken(color string) string {
	switch color {
	case "green":
		return "success"
	case "orange", "yellow":
		return "warning"
	case "pink":
		return "danger"
	default:
		return "accent"
	}
}

func stringsToTagNames(labels []domain.NoteLabel) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		out = append(out, "#"+label.Name)
	}
	return out
}

func (m *model) selectedNote() *domain.Note {
	for i := range m.notes {
		if m.notes[i].ID == m.selectedNoteID {
			return &m.notes[i]
		}
	}
	return nil
}

func (m *model) renderQuotes(c *canvas, p palette) {
	c.title("Quotes")
	c.muted("Curated quotations and your own saved lines. Each sourced quote includes its original work and reference link.")
	c.buttonRow(buttonSpec{"+ Save a quote", "quote-action:new", true}, buttonSpec{"+ Save many", "form:" + string(formBulkQuotes), false}, buttonSpec{"Reminder settings", "settings-action:notifications-page", false}, buttonSpec{"Back to Overview", "nav:overview", false})
	if len(m.quotes) == 0 {
		c.panel("NO SAVED QUOTES", "Choose “+ Save a quote” to add one.")
		return
	}
	for i, q := range m.quotes {
		if i >= 80 {
			c.muted(fmt.Sprintf(copyText.text("And %d more…"), len(m.quotes)-i))
			break
		}
		c.panel("“"+q.Text+"”", fallback(q.Author, "No attribution"), fallback(q.Category, "Personal quote"), "Saved "+q.CreatedAt.Format("2006-01-02"))
		if q.Source != "" {
			c.muted("Source: " + q.Source)
		}
		if q.SourceURL != "" {
			c.muted("Reference: " + q.SourceURL)
		}
		c.buttonRow(buttonSpec{"Remove quote…", "remove:select:quote|" + q.ID, false})
		c.blank()
	}
}

func (m *model) renderJobs(c *canvas, p palette) {
	c.title("Scheduled jobs")
	c.muted("Create, reschedule, pause, run or delete reminders. Jobs are processed while IDIOS is open; all schedule dates/times use your local timezone.")
	c.buttonRow(buttonSpec{"+ Add job", "job-action:new", true}, buttonSpec{"+ Add many", "form:" + string(formBulkJobs), false}, buttonSpec{"Notification settings", "settings-action:notifications-page", false}, buttonSpec{"View notifications", "nav:notifications", false})
	if len(m.jobs) == 0 {
		c.panel("NO SCHEDULED JOBS", copyText.JobsEmpty, "Examples: every 1h, once at a date/time, daily at 09:00 or every Monday at 10:30.")
		return
	}
	for idx, job := range m.jobs {
		m.selectLines = append(m.selectLines, len(c.lines))
		state := "paused"
		stateColor := "warning"
		if job.CompletedAt != nil {
			state, stateColor = "completed", "info"
		} else if job.Enabled {
			state, stateColor = "enabled", "success"
		}
		c.title(job.Name + "  " + p.badge(stateColor, state))
		c.muted(scheduleSummary(job) + " · " + strings.ReplaceAll(job.ActionType, "-", " ") + " · " + notificationPreference(job.Notify))
		if job.Description != "" {
			c.lineText(job.Description)
		}
		if job.CompletedAt != nil {
			c.muted("Completed: " + job.CompletedAt.Local().Format("2006-01-02 15:04"))
		} else {
			next := "not scheduled"
			if job.NextRunAt != nil {
				next = job.NextRunAt.Local().Format("2006-01-02 15:04 MST")
			}
			c.muted("Next run: " + next)
		}
		last := "never"
		if job.LastRunAt != nil {
			last = job.LastRunAt.Local().Format("2006-01-02 15:04 MST")
		}
		c.muted("Last run: " + last)
		if job.CompletedAt == nil {
			actionLabel := "Pause"
			if !job.Enabled {
				actionLabel = "Enable"
			}
			c.buttonRow(
				buttonSpec{"Edit schedule", "job-edit:" + job.ID, idx == m.cursor},
				buttonSpec{actionLabel, "job-toggle:" + job.ID, false},
				buttonSpec{"Run now", "job-run:" + job.ID, false},
				buttonSpec{"Delete", "job-delete:" + job.ID, false},
			)
		} else {
			c.buttonRow(buttonSpec{"Edit / reschedule", "job-edit:" + job.ID, idx == m.cursor}, buttonSpec{"Delete", "job-delete:" + job.ID, false})
		}
		c.muted(strings.Repeat("─", max(1, c.width-2)))
	}
}

func notificationPreference(notify bool) string {
	if notify {
		return "notification on"
	}
	return "notification off"
}

func scheduleSummary(job domain.ScheduledJob) string {
	switch job.ScheduleType {
	case domain.ScheduleOnce:
		if job.NextRunAt != nil && job.CompletedAt == nil {
			return "Once · " + job.NextRunAt.Local().Format("2006-01-02 15:04 MST")
		}
		return "One-time job"
	case domain.ScheduleDaily:
		return "Daily at " + job.ScheduleTime
	case domain.ScheduleWeekly:
		return "Every " + time.Weekday(job.ScheduleWeekday).String() + " at " + job.ScheduleTime
	case domain.ScheduleMonthly:
		return fmt.Sprintf("Monthly on day %d at %s", job.ScheduleDay, job.ScheduleTime)
	default:
		return scheduleLabel(job.IntervalMinutes)
	}
}

func (m *model) renderNotifications(c *canvas, p palette) {
	c.title("Notifications")
	c.muted("Messages created by your scheduled jobs.")
	c.buttonRow(buttonSpec{"Mark all read", "notification-action:read-all", false}, buttonSpec{"Manage jobs", "nav:jobs", false})
	unread := 0
	for _, n := range m.notifications {
		if n.ReadAt == nil {
			unread++
		}
	}
	c.muted(fmt.Sprintf(copyText.text("%d unread · %d recent notifications"), unread, len(m.notifications)))
	if len(m.notifications) == 0 {
		c.panel("INBOX EMPTY", "No notifications yet. They appear here when a scheduled job runs.")
		return
	}
	for _, n := range m.notifications {
		marker := "○"
		if n.ReadAt != nil {
			marker = "✓"
		}
		c.title(marker + " " + n.Title)
		c.muted(n.CreatedAt.Local().Format("2006-01-02 15:04") + " · " + n.Level)
		c.lineText(n.Body)
		c.buttonRow(buttonSpec{"Remove notification…", "remove:select:notification|" + n.ID, false})
		c.blank()
	}
}

func (m *model) renderLogs(c *canvas, p palette) {
	c.title("Application logs")
	c.muted("Recent events and errors. DEBUG lines are recorded only while Debug is on.")
	c.buttonRow(buttonSpec{"Debug: " + onOff(m.config.Debug), "settings-action:debug", true}, buttonSpec{"Settings", "nav:settings", false}, buttonSpec{"Refresh", "refresh:now", false})
	c.muted("Log file: " + logFilePath())
	c.blank()
	lines := readRecentLogs(70)
	for _, line := range lines {
		if p.plain {
			c.lineText(line)
			continue
		}
		style := p.muted()
		if strings.Contains(line, " ERROR ") {
			style = p.danger()
		} else if strings.Contains(line, " DEBUG ") {
			style = p.accent()
		} else if strings.Contains(line, " WARN ") {
			style = p.warning()
		}
		c.lineText(style.Render(line))
	}
}

func (m *model) pinLabel(pin domain.WorkspacePin) string {
	switch pin.EntityType {
	case "repository":
		for _, r := range m.repositories {
			if r.ID == pin.EntityID {
				return r.Name
			}
		}
	case "issue":
		for _, i := range m.issues {
			if i.ID == pin.EntityID {
				return issueRef(i) + " · " + i.Title
			}
		}
	case "project":
		for _, p := range m.projects {
			if p.ID == pin.EntityID {
				return p.Name
			}
		}
	case "concept":
		for _, c := range m.concepts {
			if c.ID == pin.EntityID {
				return c.Name
			}
		}
	case "note":
		for _, n := range m.notes {
			if n.ID == pin.EntityID {
				return n.Title
			}
		}
	}
	return fallback(pin.Label, pin.EntityType)
}

func (m *model) pinAction(pin domain.WorkspacePin) string {
	switch pin.EntityType {
	case "repository":
		for _, r := range m.repositories {
			if r.ID == pin.EntityID {
				return "repo:" + r.Name
			}
		}
	case "issue":
		for _, i := range m.issues {
			if i.ID == pin.EntityID {
				return "issue:" + issueRef(i)
			}
		}
	case "project":
		for _, p := range m.projects {
			if p.ID == pin.EntityID {
				return "project:" + p.Name
			}
		}
	case "concept":
		for _, c := range m.concepts {
			if c.ID == pin.EntityID {
				return "concept:" + c.Name
			}
		}
	case "note":
		return "note:" + pin.EntityID
	}
	return "nav:overview"
}

func scheduleLabel(minutes int) string {
	switch minutes {
	case 60:
		return "Every hour"
	case 1440:
		return "Every 24 hours"
	case 10080:
		return "Every 7 days"
	case 43200:
		return "Every 30 days"
	default:
		return fmt.Sprintf(copyText.text("Every %d minutes"), minutes)
	}
}

func activityBucket(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "Earlier"
	}
	today := now.Format("2006-01-02")
	if t.Local().Format("2006-01-02") == today {
		return "Today"
	}
	if t.Local().Format("2006-01-02") == now.AddDate(0, 0, -1).Local().Format("2006-01-02") {
		return "Yesterday"
	}
	if t.Local().Year() == now.Local().Year() && t.Local().Month() == now.Local().Month() {
		return "This month"
	}
	if t.Local().Year() == now.Local().Year() {
		return "Earlier this year"
	}
	return "Previous years"
}

func activityDescription(e domain.ActivityEvent) string {
	if strings.TrimSpace(e.Payload) != "" {
		return strings.TrimSpace(e.Payload)
	}
	return strings.ReplaceAll(strings.TrimPrefix(e.Action, "issue."), "_", " ")
}

func fitCardText(value string, width int) string {
	return truncateVisible(value, max(1, width-lipgloss.Width(value)+lipgloss.Width(value)))
}
