package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"idios/internal/domain"
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
	c.lineText(p.title().Render(centerASCII("PERSONAL WORK GRAPH / LOCAL-FIRST", c.width)))
	c.lineText(p.muted().Render(rule))
	c.blank()
	c.title(copyText.IntroTitle)
	c.muted(copyText.IntroSubtitle)
	c.blank()
	c.panel("A LOCAL-FIRST WORKSPACE", copyText.IntroPrivacy,
		fmt.Sprintf(copyText.text("%d repositories · %d open issues · %d concepts · %d notes"), len(m.repositories), len(m.issues), len(m.concepts), len(m.notes)))
	c.blank()
	if m.quote.ID != "" {
		c.panel("QUOTE FOR THIS SESSION", "“"+m.quote.Text+"”", fallback(m.quote.Author, "Saved quote"))
	} else {
		c.panel("YOUR QUOTE SPACE", "Save a quote you like; IDIOS will rotate saved quotes each time it starts.")
	}
	c.blank()
	c.buttonRow(
		buttonSpec{copyText.IntroContinue, "nav:overview", true},
		buttonSpec{copyText.IntroSeed, "demo:seed", false},
		buttonSpec{copyText.IntroHelp, "nav:help", false},
		buttonSpec{copyText.IntroSettings, "nav:settings", false},
	)
	c.blank()
	c.panel("KEYBOARD FIRST", "Ctrl+N  New item     /  Search     ?  Help     g  Graph", "Tab / Shift+Tab  Sections     F6 / Ctrl+Tab  Focus buttons     Enter  Activate     Esc  Back")
}

func (m *model) renderNotes(c *canvas, p palette) {
	c.title("Notes")
	c.muted("Catch a thought now. Link it to an Issue or Repository when it becomes actionable.")
	c.buttonRow(buttonSpec{"+ New note", "note-action:new", true}, buttonSpec{"Quotes", "nav:quotes", false}, buttonSpec{"Explore graph", "nav:graph", false})
	if len(m.notes) == 0 {
		c.panel("NO NOTES YET", copyText.NotesEmpty)
		return
	}
	for idx, n := range m.notes {
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
			c.text("Unlinked")
		}
		c.space(1)
		if n.GraphVisible {
			c.raw(p.badge("success", "graph"))
		} else {
			c.raw(p.badge("info", "note"))
		}
		c.end()
		c.muted(strings.Repeat("─", max(1, c.width-2)))
	}
	if n := m.selectedNote(); n != nil {
		c.blank()
		c.title("Selected note")
		c.panel(n.Title, strings.Split(n.Content, "\n")...)
		c.muted("Created " + n.CreatedAt.Format("2006-01-02 15:04"))
		if n.LinkedEntityLabel != "" {
			c.muted("Linked to: " + n.LinkedEntityLabel)
		}
		c.buttonRow(buttonSpec{"Edit", "note-action:edit", true}, buttonSpec{"Link / graph settings", "note-action:link", false}, buttonSpec{"Delete", "note-action:delete", false})
	}
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
	c.muted("A small shelf of lines worth remembering. The Overview quote advances on each launch.")
	c.buttonRow(buttonSpec{"+ Save a quote", "quote-action:new", true}, buttonSpec{"Back to Overview", "nav:overview", false})
	if len(m.quotes) == 0 {
		c.panel("NO SAVED QUOTES", "Save your first quote to start the rotation.")
		return
	}
	for i, q := range m.quotes {
		if i >= 80 {
			c.muted(fmt.Sprintf(copyText.text("And %d more…"), len(m.quotes)-i))
			break
		}
		c.panel("“"+q.Text+"”", fallback(q.Author, "Personal quote"), "Saved "+q.CreatedAt.Format("2006-01-02"))
		c.blank()
	}
}

func (m *model) renderJobs(c *canvas, p palette) {
	c.title("Scheduled jobs")
	c.muted("Reminders are stored locally. They are processed while IDIOS is running; notifications can be disabled in Settings.")
	c.buttonRow(buttonSpec{"+ Add reminder", "job-action:new", true}, buttonSpec{"Notification settings", "settings-action:notifications-page", false}, buttonSpec{"View notifications", "nav:notifications", false})
	if len(m.jobs) == 0 {
		c.panel("NO SCHEDULED JOBS", copyText.JobsEmpty)
		return
	}
	for idx, job := range m.jobs {
		m.selectLines = append(m.selectLines, len(c.lines))
		state := "paused"
		if job.Enabled {
			state = "enabled"
		}
		next := "not scheduled"
		if job.NextRunAt != nil {
			next = job.NextRunAt.Local().Format("2006-01-02 15:04")
		}
		last := "never"
		if job.LastRunAt != nil {
			last = job.LastRunAt.Local().Format("2006-01-02 15:04")
		}
		c.title(job.Name + "  " + p.badge(func() string {
			if job.Enabled {
				return "success"
			}
			return "warning"
		}(), state))
		c.muted(scheduleLabel(job.IntervalMinutes) + " · " + strings.ReplaceAll(job.ActionType, "-", " "))
		if job.Description != "" {
			c.lineText(job.Description)
		}
		c.muted("Next run: " + next + " · Last run: " + last)
		actionLabel := "Pause"
		if !job.Enabled {
			actionLabel = "Enable"
		}
		c.buttonRow(buttonSpec{actionLabel, "job:" + job.ID, idx == m.cursor})
		c.muted(strings.Repeat("─", max(1, c.width-2)))
	}
}

func (m *model) renderNotifications(c *canvas, p palette) {
	c.title("Notifications")
	c.muted("A local inbox for scheduled reminders. No external push service is required.")
	c.buttonRow(buttonSpec{"Mark all read", "notification-action:read-all", false}, buttonSpec{"Manage jobs", "nav:jobs", false})
	unread := 0
	for _, n := range m.notifications {
		if n.ReadAt == nil {
			unread++
		}
	}
	c.muted(fmt.Sprintf(copyText.text("%d unread · %d recent notifications"), unread, len(m.notifications)))
	if len(m.notifications) == 0 {
		c.panel("ALL QUIET", "No notifications yet. Add a scheduled job to receive reminders.")
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
		c.blank()
	}
}

func (m *model) renderLogs(c *canvas, p palette) {
	c.title("Application logs")
	c.muted("Recent UI events and errors. Debug lines are collected only when Debug is enabled.")
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
		return "Every day"
	case 10080:
		return "Every week"
	case 43200:
		return "Every month (30 days)"
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
