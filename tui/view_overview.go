package tui

import (
	"fmt"
	"idios/internal/config"
	"idios/internal/domain"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Databases created by earlier releases were seeded with this placeholder profile text.
// Shipped migrations are immutable, so the TUI treats the placeholder as "not written yet".
const (
	seededProfileBio    = "A local-first workspace for projects, learning and ideas."
	seededProfileReadme = "Welcome to my IDIOS workspace.\n\nI use this space to connect repositories, issues, concepts and the work behind them."
)

func (m *model) renderOverview(c *canvas, p palette) {
	profileName := fallback(m.profile.FullName, "Your workspace")
	username := strings.TrimSpace(m.profile.Username)
	if username == "" {
		username = "workspace-owner"
	}
	c.title(profileName)
	c.muted("@" + username)
	if m.demoMode {
		c.panel(copyText.DemoWorkspaceTitle, copyText.DemoWorkspaceNotice)
	}
	c.buttonRow(
		buttonSpec{"Edit profile", "settings-action:profile", true},
		buttonSpec{"Customize", "nav:settings", false},
		buttonSpec{"+ New", "new:menu", false},
	)
	if bio := strings.TrimSpace(m.profile.Bio); bio != "" && bio != seededProfileBio {
		c.lineText(bio)
	}
	counts := m.issueCounts()
	c.muted(fmt.Sprintf(copyText.text("%d repositories  ·  %d projects  ·  %d issues  ·  %d concepts  ·  %d notes"), len(m.repositories), len(m.projects), len(m.issues), len(m.concepts), len(m.notes)))
	c.rule()

	// Like a profile README, this is an editable, full-width summary rather than a sidebar.
	c.title("README")
	readme := strings.TrimSpace(m.profile.Readme)
	if strings.ReplaceAll(readme, `\n`, "\n") == seededProfileReadme {
		readme = ""
	}
	if readme == "" {
		readme = "This README is empty. Choose “Edit profile” to add your current goals, a short plan or links you want to see every time IDIOS starts."
	}
	c.panel("WORKSPACE OVERVIEW", strings.Split(readme, "\n")...)

	c.title("Active")
	openItems := make([]domain.Issue, 0, 8)
	for _, issue := range m.issues {
		if issue.Status == domain.IssueInProgress || issue.Blocked && issue.Status != domain.IssueDone && issue.Status != domain.IssueCancelled {
			openItems = append(openItems, issue)
		}
	}
	if len(openItems) == 0 {
		c.panel("NOTHING IN PROGRESS", "No issue is in progress or blocked. Open an issue and press “Start issue”.")
	} else {
		for i, issue := range openItems {
			if i >= 5 {
				c.muted(fmt.Sprintf(copyText.text("And %d more active items…"), len(openItems)-i))
				break
			}
			c.button(issueRef(issue), "issue:"+issueRef(issue), false)
			c.space(1)
			c.text(truncateVisible(issue.Title, max(8, c.width-lipgloss.Width(issueRef(issue))-24)))
			c.space(1)
			c.raw(p.badge(statusTone(string(issue.Status), issue.Blocked), string(issue.Status)))
			c.end()
		}
	}
	c.muted(fmt.Sprintf(copyText.text("%d open  ·  %d in progress  ·  %d blocked  ·  %d completed"), counts[domain.IssueOpen], counts[domain.IssueInProgress], m.blockedIssueCount(), counts[domain.IssueDone]))

	c.blank()
	c.title("Important repositories")
	pinnedRepoIDs := map[string]bool{}
	for _, pin := range m.pins {
		if pin.EntityType != "repository" {
			continue
		}
		pinnedRepoIDs[pin.EntityID] = true
		repoName := m.pinLabel(pin)
		var repo *domain.Repository
		for i := range m.repositories {
			if m.repositories[i].ID == pin.EntityID {
				repo = &m.repositories[i]
				break
			}
		}
		if repo == nil {
			continue
		}
		issueCount, doneCount, openCount := 0, 0, 0
		for _, issue := range m.issues {
			if issue.RepositoryName == repo.Name {
				issueCount++
				if issue.Status == domain.IssueDone {
					doneCount++
				} else if issue.Status != domain.IssueCancelled {
					openCount++
				}
			}
		}
		c.panel("◆  "+repoName, fallback(repo.Description, "Repository workspace"), fmt.Sprintf(copyText.text("%d issues · %d open · %d done"), issueCount, openCount, doneCount))
		c.buttonRow(buttonSpec{"Open repository", "repo:" + repo.Name, true}, buttonSpec{"Unpin", "pin:repository:" + repo.Name, false})
	}
	if len(pinnedRepoIDs) == 0 {
		c.panel("NO PINNED REPOSITORIES", "Use “Mark important” on a repository to keep it here.")
		if len(m.repositories) > 0 {
			c.muted("Suggested repositories")
			for i, repo := range m.repositories {
				if i >= 4 {
					break
				}
				c.button("◇ "+repo.Name, "repo:"+repo.Name, false)
				c.space(1)
				c.button("Mark important", "pin:repository:"+repo.Name, false)
				c.end()
			}
		} else {
			c.buttonRow(buttonSpec{"Create first repository", "form:" + string(formRepository), true})
		}
	}

	c.blank()
	c.title("Important items")
	importantCount := 0
	for _, pin := range m.pins {
		if pin.EntityType == "repository" {
			continue
		}
		label := m.pinLabel(pin)
		if label == "" {
			continue
		}
		importantCount++
		badgeKind := pin.Color
		if badgeKind != "success" && badgeKind != "warning" && badgeKind != "danger" {
			badgeKind = "info"
		}
		c.button("◆ "+label, m.pinAction(pin), false)
		c.space(1)
		c.raw(p.badge(badgeKind, pin.EntityType))
		c.space(1)
		c.button("Unpin", "pin:"+pin.EntityType+":"+m.pinReference(pin), false)
		c.end()
		if pin.EntityType == "issue" {
			for _, issue := range m.issues {
				if issue.ID == pin.EntityID && issue.Status != domain.IssueDone && issue.Status != domain.IssueCancelled {
					c.muted("     Still open: " + issueRef(issue) + " · " + string(issue.Status))
				}
			}
		}
	}
	if importantCount == 0 {
		c.muted("No pinned issues, projects, concepts or notes. Use “Mark important” on their pages.")
	}

	if m.quote.ID != "" {
		c.blank()
		c.panel("QUOTE", "“"+m.quote.Text+"”", fallback(m.quote.Author, "Saved quote"), "Add or remove quotes on the Quotes page.")
	}

	c.blank()
	c.title("Activity")
	c.muted("Recent changes, newest first.")
	if len(m.activity) == 0 {
		c.panel("NO ACTIVITY YET", "Changes to issues, pull requests, notes and settings are listed here.")
	} else {
		lastBucket := ""
		now := time.Now()
		for _, event := range m.activity {
			bucket := activityBucket(event.CreatedAt, now)
			if bucket != lastBucket {
				c.blank()
				c.title(bucket)
				lastBucket = bucket
			}
			c.muted(event.CreatedAt.Local().Format("Jan 02 · 15:04"))
			c.raw(p.accent().Render(strings.ReplaceAll(event.EntityType, "_", " ") + "  "))
			c.lineText(activityDescription(event))
		}
	}
	c.blank()
	c.buttonRow(buttonSpec{"Open graph", "nav:graph", false}, buttonSpec{"Write a note", "note-action:new", false}, buttonSpec{"Refresh activity", "refresh:now", false})
}

func (m *model) pinReference(pin domain.WorkspacePin) string {
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
				return issueRef(i)
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
		return pin.EntityID
	}
	return pin.EntityID
}

func (m *model) renderRepositories(c *canvas, p palette) {
	c.title("Repositories")
	c.muted("Each repository has its own issues, pull requests, labels and milestones.")
	c.buttonRow(buttonSpec{"+ New repository", "form:" + string(formRepository), true}, buttonSpec{"Refresh", "refresh:now", false})
	c.muted(fmt.Sprintf(copyText.text("%d repositories · use / to filter by name or description"), len(m.filteredRepositories())))
	if len(m.filteredRepositories()) == 0 {
		c.panel("NO MATCHING REPOSITORIES", "Choose “+ New repository”, or clear the filter: press / then Ctrl+U.")
		return
	}
	for idx, repo := range m.filteredRepositories() {
		m.selectLines = append(m.selectLines, len(c.lines))
		count, done, openPR := 0, 0, 0
		for _, i := range m.issues {
			if i.RepositoryName == repo.Name {
				count++
				if i.Status == domain.IssueDone {
					done++
				}
			}
		}
		for _, pr := range m.pullRequests {
			if pr.RepositoryName == repo.Name && pr.Status == domain.PRPending {
				openPR++
			}
		}
		c.button("◇  "+repo.Name, "repo:"+repo.Name, idx == m.cursor)
		c.space(1)
		c.raw(p.badge("info", "main"))
		c.text("  " + truncateVisible(repo.Description, max(8, c.width-36)))
		c.end()
		c.muted(fmt.Sprintf(copyText.text("     %d issues · %d done · %d open PRs"), count, done, openPR))
		c.buttonRow(buttonSpec{"Open repository →", "repo:" + repo.Name, false}, buttonSpec{"Create Issue", "form:" + string(formIssue), false}, buttonSpec{"Mark important", "pin:repository:" + repo.Name, false})
		c.rule()
	}
}

func (m *model) renderProjects(c *canvas, p palette) {
	c.title("Projects")
	c.muted("A project groups issues from several repositories under one goal.")
	c.buttonRow(buttonSpec{"+ New project", "form:" + string(formProject), true})
	if len(m.filteredProjects()) == 0 {
		c.panel("NO PROJECTS YET", "Create a project, then add existing issues to it by reference, for example CPU#4.")
		return
	}
	for idx, project := range m.filteredProjects() {
		m.selectLines = append(m.selectLines, len(c.lines))
		issues, _ := m.app.ListProjectIssues(m.ctx, project.Name)
		done := 0
		for _, issue := range issues {
			if issue.Status == domain.IssueDone {
				done++
			}
		}
		c.button("▤  "+project.Name, "project:"+project.Name, idx == m.cursor)
		c.text("  " + truncateVisible(project.Description, max(6, c.width-30)))
		c.end()
		c.muted(fmt.Sprintf(copyText.text("     %d Issues · %d done · %s"), len(issues), done, progressText(done, len(issues))))
		c.buttonRow(buttonSpec{"Open project →", "project:" + project.Name, false}, buttonSpec{"Add Issue", "project-add-issue:" + project.Name, false})
		c.rule()
	}
}

func (m *model) renderIssues(c *canvas, p palette, scoped bool) {
	title := "All Issues"
	if scoped {
		title = m.repoContext + " · Issues"
	}
	c.title(title)
	c.muted("open → in progress → done. An issue becomes done when its pull request is accepted. BLOCKED means it is waiting for a prerequisite.")
	c.buttonRow(buttonSpec{"+ New Issue", "form:" + string(formIssue), true}, buttonSpec{"Refresh", "refresh:now", false})
	items := m.filteredIssues()
	if len(items) == 0 {
		c.panel("NO ISSUES FOUND", "Choose “+ New Issue”. You need a repository first.")
		return
	}
	for idx, issue := range items {
		m.selectLines = append(m.selectLines, len(c.lines))
		if issue.Blocked {
			c.raw(p.danger().Render("●"))
		} else if issue.Status == domain.IssueDone {
			c.raw(p.success().Render("●"))
		} else {
			c.raw(p.accent().Render("○"))
		}
		c.space(1)
		c.button(issueRef(issue), "issue:"+issueRef(issue), idx == m.cursor)
		c.space(1)
		c.text(truncateVisible(issue.Title, max(8, c.width-lipgloss.Width(issueRef(issue))-25)))
		c.space(1)
		c.raw(p.badge(statusTone(string(issue.Status), issue.Blocked), string(issue.Status)))
		c.end()
		if issue.Blocked && len(issue.Blockers) > 0 {
			parts := make([]string, 0, len(issue.Blockers))
			for _, b := range issue.Blockers {
				parts = append(parts, fmt.Sprintf("%s#%d", b.RepositoryName, b.Number))
			}
			c.muted("     Waiting for: " + strings.Join(parts, ", "))
		} else if issue.Priority != "" || issue.DueDate != nil {
			meta := "     " + issue.Priority
			if issue.DueDate != nil {
				meta += " · due " + *issue.DueDate
			}
			c.muted(meta)
		}
		metadata := make([]string, 0, len(m.issueLabels[issue.ID])+len(m.issueMilestones[issue.ID]))
		for _, label := range m.issueLabels[issue.ID] {
			metadata = append(metadata, "#"+label.Name)
		}
		for _, milestone := range m.issueMilestones[issue.ID] {
			metadata = append(metadata, "milestone: "+milestone.Title)
		}
		if len(metadata) > 0 {
			c.muted("     " + strings.Join(metadata, "  ·  "))
		}
		c.rule()
	}
}

func (m *model) renderPullRequests(c *canvas, p palette, scoped bool) {
	title := "All pull requests"
	if scoped {
		title = m.repoContext + " · Pull requests"
	}
	c.title(title)
	c.muted("A pull request is your progress report on an issue. Edit it while you work; accept it when the work is finished and the issue becomes done.")
	c.buttonRow(buttonSpec{"Refresh", "refresh:now", false})
	items := m.filteredPRs()
	if len(items) == 0 {
		c.panel("NO PULL REQUESTS", "Open an issue, press “Start issue”, then “Create pull request” to track your progress.")
		return
	}
	for idx, pr := range items {
		m.selectLines = append(m.selectLines, len(c.lines))
		c.button(prRef(pr), "pr:"+prRef(pr), idx == m.cursor)
		c.space(1)
		c.raw(p.badge(statusTone(string(pr.Status), false), string(pr.Status)))
		c.text("  " + truncateVisible(pr.Title, max(8, c.width-lipgloss.Width(prRef(pr))-22)))
		c.end()
		c.muted(fmt.Sprintf(copyText.text("     Issue %s#%d · target %s"), pr.RepositoryName, pr.IssueNumber, pr.TargetBranchName))
		c.buttonRow(buttonSpec{"Review details →", "pr:" + prRef(pr), false}, buttonSpec{"Edit", "pr-edit:" + prRef(pr), pr.Status == domain.PRPending}, buttonSpec{"Accept (issue done)", "pr-accept:" + prRef(pr), false})
		c.rule()
	}
}

func (m *model) renderRepositoryOverview(c *canvas, p palette) {
	repo, ok := m.currentRepository()
	if !ok {
		c.panel("REPOSITORY NOT FOUND", "Select a repository from the repository list and open it again.")
		c.buttonRow(buttonSpec{"← Repositories", "nav:repositories", true})
		return
	}
	issues := make([]domain.Issue, 0)
	prs := make([]domain.PullRequest, 0)
	for _, issue := range m.issues {
		if strings.EqualFold(issue.RepositoryName, repo.Name) {
			issues = append(issues, issue)
		}
	}
	for _, pr := range m.pullRequests {
		if strings.EqualFold(pr.RepositoryName, repo.Name) {
			prs = append(prs, pr)
		}
	}
	done, open, inProgress, blocked := 0, 0, 0, 0
	for _, issue := range issues {
		switch issue.Status {
		case domain.IssueDone:
			done++
		case domain.IssueOpen:
			open++
		case domain.IssueInProgress:
			inProgress++
		}
		if issue.Blocked && issue.Status != domain.IssueDone && issue.Status != domain.IssueCancelled {
			blocked++
		}
	}
	openPR, acceptedPR, closedPR := 0, 0, 0
	for _, pr := range prs {
		switch pr.Status {
		case domain.PRPending:
			openPR++
		case domain.PRAccepted:
			acceptedPR++
		case domain.PRClosed:
			closedPR++
		}
	}

	c.title("◈ " + repo.Name)
	c.muted(fmt.Sprintf("Created %s · updated %s", repo.CreatedAt.Format("2006-01-02"), repo.UpdatedAt.Format("2006-01-02")))
	c.buttonRow(
		buttonSpec{"+ New Issue", "form:" + string(formIssue), true},
		buttonSpec{"Issues", "repo-tab:" + string(pageRepoIssues), false},
		buttonSpec{"Pull requests", "repo-tab:" + string(pageRepoPRs), false},
		buttonSpec{"Graph", "repo-tab:" + string(pageRepoGraph), false},
		buttonSpec{"Activity", "repo-tab:" + string(pageRepoActivity), false},
		buttonSpec{"Settings", "repo-tab:" + string(pageRepoSettings), false},
		buttonSpec{"Mark important", "pin:repository:" + repo.Name, false},
	)
	c.panel("REPOSITORY AT A GLANCE",
		fmt.Sprintf("%d Issues  ·  %d open  ·  %d in progress  ·  %d done", len(issues), open, inProgress, done),
		fmt.Sprintf("%d blocked Issues  ·  %d open PRs  ·  %d accepted PRs  ·  %d closed PRs", blocked, openPR, acceptedPR, closedPR),
		"Default branch: main")

	readmeLines := strings.Split(strings.TrimSuffix(strings.TrimSpace(repo.Description), "\n"), "\n")
	if strings.TrimSpace(repo.Description) == "" {
		c.panel("README", "No description yet.", "Write what this repository is for, what is in scope, and how to get started.")
	} else {
		c.panel("README", readmeLines...)
	}

	c.blank()
	c.title("Recent Issues")
	c.muted("Newest issues in this repository.")
	c.buttonRow(buttonSpec{"View all Issues →", "repo-tab:" + string(pageRepoIssues), false})
	if len(issues) == 0 {
		c.panel("NO ISSUES YET", "Choose “+ New Issue”. You can start from a task, bug, feature or research template.")
	} else {
		shown := 0
		for _, issue := range issues {
			if shown >= 6 {
				break
			}
			shown++
			c.button(issueRef(issue), "issue:"+issueRef(issue), false)
			c.space(1)
			c.text(truncateVisible(issue.Title, max(8, c.width-lipgloss.Width(issueRef(issue))-24)))
			c.space(1)
			c.raw(p.badge(statusTone(string(issue.Status), issue.Blocked), string(issue.Status)))
			c.end()
			metadata := "Priority: " + fallback(issue.Priority, "normal")
			if issue.ParentReference != "" {
				metadata += " · Sub-issue of " + issue.ParentReference
			}
			if issue.Blocked {
				metadata += fmt.Sprintf(" · blocked by %d prerequisite(s)", len(issue.Blockers))
			}
			c.space(4)
			c.muted(metadata)
		}
	}

	c.blank()
	c.title("Recent Pull Requests")
	c.muted("Your progress reports. Edit one while you work; accept it when finished and its issue becomes done.")
	c.buttonRow(buttonSpec{"View all Pull Requests →", "repo-tab:" + string(pageRepoPRs), false})
	if len(prs) == 0 {
		c.panel("NO PULL REQUESTS YET", "Start an issue, then choose “Create pull request” and write what you have done so far.")
	} else {
		shown := 0
		for _, pr := range prs {
			if shown >= 5 {
				break
			}
			shown++
			c.button(prRef(pr), "pr:"+prRef(pr), false)
			c.space(1)
			c.text(truncateVisible(pr.Title, max(8, c.width-lipgloss.Width(prRef(pr))-25)))
			c.space(1)
			c.raw(p.badge(statusTone(string(pr.Status), false), string(pr.Status)))
			c.end()
			c.space(4)
			c.muted(fmt.Sprintf("Related Issue: %s#%d · target: %s · updated %s", pr.RepositoryName, pr.IssueNumber, pr.TargetBranchName, pr.UpdatedAt.Format("2006-01-02")))
		}
	}

	c.blank()
	c.panel("WORKFLOW", "1. Write an issue (task, bug, feature or research template).", "2. Add sub-issues, prerequisites, labels or milestones if needed.", "3. Start the issue, then create a pull request and keep editing it as you work.", "4. When the work is finished, accept the pull request. The issue is marked done.")
}

func (m *model) renderBranches(c *canvas, p palette) {
	c.title("Branches")
	c.muted("Only the main branch is used. Other branches are not supported yet.")
	branches, err := m.app.ListBranches(m.ctx, m.repoContext)
	if err != nil {
		c.panel("COULD NOT LOAD BRANCHES", err.Error())
		return
	}
	if len(branches) == 0 {
		c.panel("NO BRANCHES", "This repository has no branches. Every repository normally starts with main.")
		return
	}
	for _, b := range branches {
		label := b.Name
		if b.IsDefault {
			label += "  ★ default"
		}
		c.panel("⑂  "+label, fmt.Sprintf(copyText.text("Created %s"), b.CreatedAt.Format("2006-01-02 15:04")), "Creating, switching or merging other branches is not supported yet.")
	}
}

func (m *model) renderLabels(c *canvas, p palette) {
	c.title("Labels")
	c.muted("Labels tag issues. A label is either global or limited to one repository.")
	c.buttonRow(buttonSpec{"+ New label", "label-action:new", true})
	labels, _ := m.app.ListLabels(m.ctx, "")
	if len(labels) == 0 {
		c.panel("NO LABELS YET", "Choose “+ New label”. Examples: bug, research, docs.")
		return
	}
	for _, l := range labels {
		if l.ScopeType == "repository" && l.RepositoryName != m.repoContext {
			continue
		}
		c.text("● ")
		c.raw(p.accent().Render(l.Name))
		c.text("   " + p.muted().Render(l.ScopeType))
		if l.Description != "" {
			c.text("   " + l.Description)
		}
		c.end()
	}
}

func (m *model) renderMilestones(c *canvas, p palette) {
	c.title("Milestones")
	c.muted("A milestone is a target with an optional due date. Assign issues to it.")
	c.buttonRow(buttonSpec{"+ New milestone", "milestone-action:new", true})
	if len(m.milestones) == 0 {
		c.panel("NO MILESTONES", "Choose “+ New milestone”. Example: “v1 release”.")
		return
	}
	for _, ms := range m.milestones {
		due := "no due date"
		if ms.DueDate != nil {
			due = "due " + *ms.DueDate
		}
		c.panel(ms.Title, ms.Description, due)
	}
}

func (m *model) renderRepositorySettings(c *canvas, p palette) {
	c.title("Repository settings")
	c.muted("Key/value notes stored with this repository. They do not change how issues and pull requests work.")
	c.buttonRow(buttonSpec{"+ Set key / value", "settings-action:set", true})
	settings, err := m.app.GetRepositorySettings(m.ctx, m.repoContext)
	if err != nil {
		c.panel("COULD NOT LOAD SETTINGS", err.Error())
		return
	}
	if len(settings) == 0 {
		c.panel("NO CUSTOM SETTINGS", "Nothing has been set for this repository.")
	}
	for _, s := range settings {
		c.panel(s.Key, s.Value)
	}
}

func (m *model) renderActivity(c *canvas, p palette) {
	c.title("Activity & completions")
	c.muted("Changes and accepted work in this repository.")
	if len(m.completions) > 0 {
		c.title("Accepted work on main")
		for _, item := range m.completions {
			c.raw(p.success().Render("✓ "))
			c.text(fmt.Sprintf("%s#%d", item.RepositoryName, item.IssueNumber))
			c.text("  " + truncateVisible(item.Summary, max(8, c.width-32)))
			c.text(fmt.Sprintf("  #%d", item.Sequence))
			c.end()
		}
	}
	if len(m.activity) > 0 {
		c.blank()
		c.title("Recent activity")
		for _, item := range m.activity {
			c.text(item.CreatedAt.Format("2006-01-02 15:04"))
			c.space(2)
			c.raw(p.accent().Render(item.Action))
			c.text("  " + item.EntityType)
			c.end()
		}
	} else if len(m.completions) == 0 {
		c.panel("NO ACTIVITY YET", "Nothing has happened in this repository yet.")
	}
}

func (m *model) renderConcepts(c *canvas, p palette) {
	c.title("Concepts")
	c.muted("A concept is a topic you can link from many issues, for example “caching” or “authentication”.")
	c.buttonRow(buttonSpec{"+ New concept", "form:" + string(formConcept), true}, buttonSpec{"Open graph", "nav:graph", false})
	if len(m.filteredConcepts()) == 0 {
		c.panel("NO CONCEPTS YET", "Choose “+ New concept”, then link it from an issue.")
		return
	}
	for idx, concept := range m.filteredConcepts() {
		m.selectLines = append(m.selectLines, len(c.lines))
		c.button("◇ "+concept.Name, "concept:"+concept.Name, idx == m.cursor)
		c.text("  " + truncateVisible(concept.Description, max(8, c.width-lipgloss.Width(concept.Name)-8)))
		c.end()
		c.buttonRow(buttonSpec{"Open concept →", "concept:" + concept.Name, false}, buttonSpec{"View graph", "graph:focus-concept:" + concept.Name, false})
		c.rule()
	}
}

func (m *model) renderSettings(c *canvas, p palette) {
	c.title("Settings")
	c.muted("Changes are saved immediately.")
	c.title("Appearance")
	c.panel("ACTIVE THEME", "Theme: "+m.config.Appearance.Theme, "Layout density: "+m.config.Appearance.Density, "Font family and size are set in your terminal, not in IDIOS.")
	c.buttonRow(buttonSpec{"Next theme", "settings-action:theme", true}, buttonSpec{"Toggle density", "settings-action:density", false})
	c.panel("CUSTOM COLORS", "Override single colors in appearance.colors in: "+config.AppConfigPath(), `Example: "appearance": {"colors": {"accent": "#ff78a9", "danger": "#ff4b4b"}}`, "Available tokens: background, foreground, muted, accent, accent_background, on_accent, surface, surface_alt, border, success, success_background, warning, warning_background, danger, danger_background.")
	c.blank()
	c.title("Repository, Issue & Pull Request templates")
	c.panel("TEMPLATES",
		"Repository README scaffold: REPOSITORY_TEMPLATE",
		"Default Issue template: "+m.config.IssueTemplate.Default,
		"Available Issue templates: "+strings.Join(issueTemplateKeys(m.config), ", "),
		"Pull Request description template: PULL_REQUEST_TEMPLATE",
		"Edit these sections in "+config.AppConfigPath()+". Template text is copied into new items; existing items are not changed.",
		"Restart IDIOS after changing the config. The CLI does not use these templates.")
	c.buttonRow(buttonSpec{"In-app help", "nav:help", false})
	c.blank()
	c.title("Startup & profile")
	c.panel("INTRO SCREEN", "Show intro on next launch: "+onOff(m.config.Startup.ShowIntro))
	c.buttonRow(buttonSpec{"Toggle startup intro", "settings-action:intro", false}, buttonSpec{"Edit Overview profile", "settings-action:profile", false})
	c.blank()
	c.title("Notifications & scheduled jobs")
	c.panel("REMINDERS", fmt.Sprintf(copyText.text("Notifications: %s · check interval: every %d minute(s)"), onOff(m.config.Notifications.Enabled), m.config.Notifications.PollIntervalMinutes), "Reminders run only while IDIOS is open. There is no background service.")
	c.buttonRow(buttonSpec{"Notification settings", "settings-action:notifications-page", true}, buttonSpec{"Scheduled jobs", "nav:jobs", false}, buttonSpec{"Notification inbox", "nav:notifications", false})
	c.blank()
	c.title("Diagnostics")
	c.panel("LOGGING", "Debug logging: "+onOff(m.config.Debug), "Log file: "+logFilePath(), "Errors are always logged. DEBUG lines are recorded only while Debug is on.")
	c.buttonRow(buttonSpec{"Toggle debug", "settings-action:debug", false}, buttonSpec{"Open logs", "settings-action:logs", false})
	c.blank()
	c.title("Data & workflow")
	c.panel("FILES", "Database (SQLite): "+m.databasePath, "Settings file: "+config.AppConfigPath())
	c.panel("HOW ISSUES FINISH", "An issue cannot start until its prerequisites are done.", "Accepting its pull request marks it done. Git and your files are not touched.")
	c.buttonRow(buttonSpec{"Reload data", "refresh:now", false}, buttonSpec{"Quick guide", "nav:help", false})
	c.blank()
	c.title("Keyboard shortcuts")
	shortcuts := []struct{ action, label string }{
		{"new", "New item"}, {"search", "Search current list"}, {"help", "Help"}, {"overview", "Overview"},
		{"repositories", "Repositories"}, {"projects", "Projects"}, {"issues", "Issues"}, {"pull_requests", "Pull requests"},
		{"graph", "Knowledge graph"}, {"concepts", "Concepts"}, {"notes", "Notes"}, {"quotes", "Quotes"}, {"jobs", "Scheduled jobs"},
		{"notifications", "Notifications"}, {"logs", "Logs"}, {"settings", "Settings"}, {"profile", "Edit profile"},
		{"theme", "Next theme"}, {"density", "Toggle density"}, {"debug", "Toggle debug logs"}, {"toggle_notifications", "Toggle reminders"},
		{"intro", "Toggle startup intro"}, {"poll_interval", "Change check interval"}, {"new_note", "New note"}, {"new_quote", "Save quote"}, {"new_job", "Schedule reminder"}, {"mark_notifications_read", "Mark notifications read"},
		{"refresh", "Refresh"}, {"quit", "Quit"},
	}
	for _, item := range shortcuts {
		key := m.config.Keyboard[item.action]
		if key == "" {
			key = "not set"
		}
		c.text(p.accent().Render(key))
		c.space(2)
		c.lineText(item.label)
	}
}

func (m *model) renderNotificationSettings(c *canvas, p palette) {
	c.title("Settings / Notifications")
	c.muted("Turn reminders on or off and set how often IDIOS checks for due jobs.")
	c.panel("DELIVERY", "Notifications appear in the inbox inside IDIOS.", "Jobs run only while IDIOS is open.")
	c.panel("GLOBAL SWITCH", "Scheduled notifications: "+onOff(m.config.Notifications.Enabled), "When off, due jobs still run and move to their next time, but no message is added to the inbox.")
	c.buttonRow(buttonSpec{"Toggle notifications", "settings-action:notifications", true}, buttonSpec{fmt.Sprintf(copyText.text("Check interval: %d minute(s) → next"), m.config.Notifications.PollIntervalMinutes), "settings-action:poll-interval", false}, buttonSpec{"Open inbox", "nav:notifications", false}, buttonSpec{"Manage scheduled jobs", "nav:jobs", false})
	c.blank()
	c.title("Current scheduled jobs")
	if len(m.jobs) == 0 {
		c.panel("NO JOBS", "Open Scheduled jobs and add a daily plan, a weekly review or a blocked-issues reminder.")
	} else {
		for _, job := range m.jobs {
			state := "paused"
			if job.Enabled {
				state = "enabled"
			}
			c.text(job.Name)
			c.space(2)
			c.raw(p.badge(func() string {
				if job.Enabled {
					return "success"
				}
				return "warning"
			}(), state))
			c.space(2)
			c.lineText(scheduleSummary(job) + " · " + onOff(job.Notify) + " notification")
		}
	}
}

func (m *model) renderNew(c *canvas, p palette) {
	c.title("Create something new")
	c.muted("Choose what to create.")
	c.buttonRow(buttonSpec{"Repository", "form:" + string(formRepository), true}, buttonSpec{"Project", "form:" + string(formProject), false})
	c.buttonRow(buttonSpec{"Issue", "form:" + string(formIssue), false}, buttonSpec{"Concept", "form:" + string(formConcept), false})
	c.panel("WHICH ONE?", "Repository: a separate area of work, such as CPU or ARLIZ.", "Project: a goal that collects issues from several repositories.", "Issue: one task. A sub-issue belongs to a parent issue.", "Concept: a topic you can link from many issues.")
}
