package tui

import (
	"fmt"
	"idios/internal/domain"
	"strings"
)

func (m *model) renderIssueDetail(c *canvas, p palette) {
	issue := m.findIssue(m.selectedIssueRef)
	if issue == nil {
		c.panel("ISSUE NOT FOUND", m.selectedIssueRef, "It may have been deleted. Go back and refresh the list.")
		c.buttonRow(buttonSpec{"← Back", "back:now", false})
		return
	}
	c.title(issue.Title)
	c.muted(fmt.Sprintf(copyText.text("%s  ·  Issue #%d  ·  created %s"), issue.RepositoryName, issue.Number, issue.CreatedAt.Format("2006-01-02")))
	c.buttonRow(buttonSpec{"← Back", "back:now", false}, buttonSpec{"Open repository", "repo:" + issue.RepositoryName, false}, buttonSpec{"View graph", "issue-action:graph", false}, buttonSpec{"Mark important", "pin:issue:" + issueRef(*issue), false})
	statusToneValue := statusTone(string(issue.Status), issue.Blocked)
	c.panel("STATUS", p.badge(statusToneValue, string(issue.Status)), fmt.Sprintf(copyText.text("Priority: %s"), fallback(issue.Priority, "normal")), "Blocked: "+yesNo(issue.Blocked))
	c.panel("DESCRIPTION", fallback(issue.Description, "No description."))
	if labels := m.issueLabels[issue.ID]; len(labels) > 0 {
		c.title("Labels")
		items := make([]buttonSpec, 0, len(labels))
		for _, label := range labels {
			items = append(items, buttonSpec{"◆ " + label.Name, "issue-labels:" + issue.RepositoryName, false})
		}
		c.buttonRow(items...)
	} else {
		c.muted("No labels")
	}
	if milestones := m.issueMilestones[issue.ID]; len(milestones) > 0 {
		c.title("Milestones")
		for _, milestone := range milestones {
			detail := "▣ " + milestone.Title
			if milestone.DueDate != nil && *milestone.DueDate != "" {
				detail += " · due " + *milestone.DueDate
			}
			c.muted(detail)
		}
	} else {
		c.muted("No milestone")
	}
	if issue.Blocked {
		c.title("Prerequisites blocking this Issue")
		for _, b := range issue.Blockers {
			prerequisite := fmt.Sprintf("%s#%d", b.RepositoryName, b.Number)
			c.button(fmt.Sprintf(copyText.text("⛔ %s"), prerequisite), "issue:"+prerequisite, false)
			c.text("  " + b.Title + " · " + string(b.Status))
			c.space(1)
			c.button("Remove prerequisite", "dependency-remove:"+issueRef(*issue)+"|"+prerequisite, false)
			c.end()
		}
	}
	c.title("Actions")
	if issue.Status == domain.IssueOpen && !issue.Blocked {
		c.buttonRow(buttonSpec{"▶ Start issue", "issue-action:start", true})
	}
	if issue.Status == domain.IssueOpen && issue.Blocked {
		c.muted("This issue cannot start until the prerequisites above are done.")
	}
	if issue.Status == domain.IssueInProgress {
		c.buttonRow(buttonSpec{"↗ Create pull request", "issue-action:pr", true})
	}
	if issue.Status != domain.IssueDone && issue.Status != domain.IssueCancelled {
		c.buttonRow(buttonSpec{"Cancel issue", "issue-action:cancel", false})
	}
	c.buttonRow(
		buttonSpec{"+ Add blocking dependency", "issue-action:dependency", false},
		buttonSpec{"◇ Link concept", "issue-action:concept", false},
		buttonSpec{"◆ Assign label", "issue-action:label", false},
		buttonSpec{"▣ Assign milestone", "issue-action:milestone", false},
	)
	if issue.ParentReference != "" {
		c.muted("Parent Issue: " + issue.ParentReference)
	}
}

func (m *model) renderPRDetail(c *canvas, p palette) {
	pr := m.findPR(m.selectedPRRef)
	if pr == nil {
		c.panel("PULL REQUEST NOT FOUND", m.selectedPRRef, "Go back and refresh the list.")
		c.buttonRow(buttonSpec{"← Back", "back:now", false})
		return
	}
	c.title(pr.Title)
	c.muted(fmt.Sprintf(copyText.text("%s  ·  Pull request #%d  ·  Issue %s#%d"), pr.RepositoryName, pr.Number, pr.RepositoryName, pr.IssueNumber))
	c.buttonRow(buttonSpec{"← Back", "back:now", false}, buttonSpec{"Open linked Issue", "pr-action:issue", false}, buttonSpec{"Open repository", "repo:" + pr.RepositoryName, false})
	c.panel("REVIEW STATUS", p.badge(statusTone(string(pr.Status), false), string(pr.Status)), "Target branch: "+pr.TargetBranchName, "Open: you can still edit it. Accepting it marks the issue done.")
	c.panel("WORK EXPLANATION", fallback(pr.Description, "No description."))
	if pr.Status == domain.PRPending {
		c.buttonRow(buttonSpec{"✎ Edit pull request", "pr-action:edit", true}, buttonSpec{"✓ Accept: issue is done", "pr-action:accept", false}, buttonSpec{"Close without completing", "pr-action:close", false})
	}
	if pr.Status == domain.PRAccepted {
		c.panel("ACCEPTED", "This pull request was accepted. Its issue is done.")
	}
}

func (m *model) renderProjectDetail(c *canvas, p palette) {
	project, err := m.app.GetProject(m.ctx, m.selectedProject)
	if err != nil {
		c.panel("PROJECT NOT FOUND", err.Error())
		c.buttonRow(buttonSpec{"← Back", "back:now", false})
		return
	}
	issues, err := m.app.ListProjectIssues(m.ctx, project.Name)
	if err != nil {
		c.panel("COULD NOT LOAD PROJECT ISSUES", err.Error())
		return
	}
	done := 0
	for _, issue := range issues {
		if issue.Status == domain.IssueDone {
			done++
		}
	}
	c.title(project.Name)
	c.muted(project.Description)
	c.buttonRow(buttonSpec{"← Back", "back:now", false}, buttonSpec{"+ Add Issue", "project-action:add-issue", true})
	c.panel("PROGRESS", fmt.Sprintf(copyText.text("%d / %d Issues done · %s"), done, len(issues), progressText(done, len(issues))))
	for _, issue := range issues {
		c.button(issueRef(issue), "issue:"+issueRef(issue), false)
		c.text("  " + issue.Title)
		c.space(1)
		c.raw(p.badge(statusTone(string(issue.Status), issue.Blocked), string(issue.Status)))
		c.end()
	}
	if len(issues) == 0 {
		c.panel("NO ISSUES YET", "Choose “+ Add Issue” and enter a reference such as CPU#4.")
	}
}

func (m *model) renderConceptDetail(c *canvas, p palette) {
	concept, err := m.app.GetConcept(m.ctx, m.selectedConcept)
	if err != nil {
		c.panel("CONCEPT NOT FOUND", err.Error())
		c.buttonRow(buttonSpec{"← Back", "back:now", false})
		return
	}
	c.title("◇ " + concept.Name)
	c.muted("Concept")
	c.buttonRow(buttonSpec{"← Back", "back:now", false}, buttonSpec{"View connected graph", "graph:focus-concept:" + concept.Name, true})
	c.panel("DESCRIPTION", fallback(concept.Description, "No description yet."))
	c.title("Connections")
	g, err := m.app.Graph(m.ctx, "concept:"+concept.Name, 2)
	if err != nil {
		c.panel("GRAPH UNAVAILABLE", err.Error())
		return
	}
	seen := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Type == "concept" && (n.ID == concept.ID || n.Label == concept.Name) {
			continue
		}
		key := n.Type + ":" + n.Reference
		if seen[key] {
			continue
		}
		seen[key] = true
		c.button(graphNodeRef(n), "graph-node:"+n.ID, false)
		c.text("  " + graphLabel(n))
		c.end()
	}
	if len(seen) == 0 {
		c.muted("Not linked to anything yet. Open an issue and choose “Link concept”.")
	}
}

func (m *model) renderForm(c *canvas, p palette) {
	c.title(m.formTitle)
	c.muted("Tab / Shift+Tab changes fields · Enter adds a line in a multiline field · Ctrl+Enter saves · Esc cancels")
	if m.formError != "" {
		c.lineText(p.danger().Render("! " + m.formError))
	}
	for i, field := range m.formFields {
		active := i == m.formFocus
		action := fmt.Sprintf("form-field:%d", i)
		if field.multiline {
			if active {
				c.raw(p.accent().Render("▸ "))
			} else {
				c.space(2)
			}
			c.button(field.label, action, active)
			c.end()
			if field.helper != "" {
				c.space(2)
				c.muted(field.helper)
				c.end()
			}
			for _, line := range multilineFieldLines(field.value, m.formCaret, active, field.placeholder, 6) {
				c.space(2)
				c.input(line, action, active)
				c.end()
			}
			c.blank()
			continue
		}
		if active {
			c.raw(p.accent().Render("▸ "))
		} else {
			c.space(2)
		}
		c.button(field.label, action, active)
		c.space(1)
		value := field.value
		if active {
			if value == "" {
				value = p.accent().Render("▏") + " " + p.muted().Render(field.placeholder)
			} else {
				value = caretValue(value, m.formCaret, true)
			}
		} else if value == "" {
			value = p.muted().Render(field.placeholder)
		}
		c.input(value, action, active)
		c.end()
		if field.helper != "" {
			c.space(4)
			c.muted(field.helper)
		}
	}
	c.blank()
	submit := "Create"
	switch m.form {
	case formPullRequest:
		submit = "Create pull request"
	case formAcceptPR:
		submit = "Accept & record completion"
	case formEditPR:
		submit = "Save changes"
	case formDependency:
		submit = "Add prerequisite"
	case formConceptLink:
		submit = "Link concept"
	case formRepoSetting:
		submit = "Save setting"
	case formLabel:
		submit = "Create label"
	case formMilestone:
		submit = "Create milestone"
	case formAssignLabel:
		submit = "Assign label"
	case formAssignMilestone:
		submit = "Assign milestone"
	case formProjectIssue:
		submit = "Add Issue to project"
	case formIssue:
		submit = "Create Issue"
	case formRepository:
		submit = "Create repository"
	case formProject:
		submit = "Create project"
	case formConcept:
		submit = "Create concept"
	}
	c.buttonRow(buttonSpec{submit, "form-action:submit", m.formFocus >= len(m.formFields)}, buttonSpec{"Cancel", "form-action:cancel", false})
}

func (m *model) renderHelp(c *canvas, p palette) {
	c.title(copyText.HelpTitle)
	c.muted(copyText.HelpIntro)
	c.panel("BASICS", copyText.HelpRepository, copyText.HelpIssue, copyText.HelpDependency)
	c.panel("GROUPING", copyText.HelpProject, copyText.HelpConcept, copyText.HelpPR)
	c.panel("GRAPH AND NOTES", copyText.HelpGraph, copyText.HelpNotes)
	c.panel("SAMPLE DATA", copyText.HelpSeed)
	c.title("Keyboard shortcuts")
	shortcuts := []struct{ action, description string }{
		{"new", "Open creation hub"}, {"search", "Search / filter this view"}, {"help", "Open this guide"},
		{"overview", "Overview"}, {"repositories", "Repositories"}, {"projects", "Projects"}, {"issues", "Issues"},
		{"pull_requests", "Pull requests"}, {"graph", "Knowledge graph"}, {"notes", "Notes"}, {"quotes", "Quotes"},
		{"jobs", "Scheduled jobs"}, {"notifications", "Notification inbox"}, {"logs", "Logs"}, {"settings", "Settings"},
		{"profile", "Edit profile"}, {"refresh", "Refresh data"}, {"quit", "Quit"},
	}
	for _, shortcut := range shortcuts {
		key := strings.TrimSpace(m.config.Keyboard[shortcut.action])
		if key == "" {
			key = "not assigned"
		}
		c.text(p.accent().Render(key))
		c.space(2)
		c.lineText(shortcut.description)
	}
	c.panel("FORMS", "Tab / Shift+Tab move between fields. Ctrl+Enter saves. Ctrl+U clears the field. Esc cancels.")
	c.panel("BLOCKING VS. LINKING", "Only a prerequisite (depends_on) blocks an issue. Sub-issues, concept links and note links never block it.", "IDIOS is not a Git client: it does not commit, merge or sync anything.")
	c.buttonRow(buttonSpec{"← Back to Overview", "nav:overview", true}, buttonSpec{"Open settings", "nav:settings", false})
}
