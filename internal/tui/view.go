package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"idios/internal/domain"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type canvas struct {
	width, x, y      int
	originX, originY int
	lines            []string
	line             string
	hits             *[]hitbox
	style            palette
}

type buttonSpec struct {
	label, action string
	active        bool
}

func newCanvas(width, originX, originY int, hits *[]hitbox, p palette) *canvas {
	return &canvas{width: max(1, width), originX: originX, originY: originY, hits: hits, style: p}
}
func (c *canvas) raw(s string) {
	if s == "" {
		return
	}
	remaining := c.width - c.x
	w := lipgloss.Width(s)
	if w > remaining {
		if remaining <= 0 {
			return
		}
		s = truncateVisible(s, remaining)
		w = lipgloss.Width(s)
	}
	c.line += s
	c.x += w
}
func (c *canvas) text(s string) { c.raw(truncateVisible(s, max(0, c.width-c.x))) }
func (c *canvas) space(n int) {
	if n > 0 {
		c.raw(strings.Repeat(" ", min(n, max(0, c.width-c.x))))
	}
}
func (c *canvas) button(label, action string, active bool) {
	maxLabel := max(1, c.width-4)
	if active && c.style.plain {
		// NO_COLOR keeps escape sequences out of the screen, so the current control is
		// marked with a rune instead of bold or an underline.
		label = "▸ " + label
	}
	if lipgloss.Width(label)+4 > c.width {
		label = truncateVisible(label, maxLabel)
	}
	styled := c.style.button(active).Render(label)
	w := lipgloss.Width(styled)
	if c.x+w > c.width {
		return
	}
	if c.hits != nil {
		*c.hits = append(*c.hits, hitbox{x: c.originX + c.x, y: c.originY + c.y, w: w, h: 1, action: action})
	}
	c.raw(styled)
}

// buttonRow behaves like a responsive tab/button group: controls wrap instead of silently
// disappearing when the terminal is narrower than a desktop layout.
// input renders a full-width, visibly focused input area and makes the entire field clickable.
func (c *canvas) input(value, action string, active bool) {
	remaining := max(1, c.width-c.x)
	style := c.style.input(active).Width(max(1, remaining-2))
	rendered := style.Render(value)
	w := min(remaining, lipgloss.Width(rendered))
	if c.hits != nil {
		*c.hits = append(*c.hits, hitbox{x: c.originX + c.x, y: c.originY + c.y, w: remaining, h: 1, action: action})
	}
	c.raw(rendered)
	if w < remaining {
		c.space(remaining - w)
	}
}

func (c *canvas) buttonRow(items ...buttonSpec) {
	for _, item := range items {
		styled := c.style.button(item.active).Render(item.label)
		w := lipgloss.Width(styled)
		if w > c.width {
			item.label = truncateVisible(item.label, max(1, c.width-4))
			w = lipgloss.Width(c.style.button(item.active).Render(item.label))
		}
		if c.x > 0 && c.x+1+w > c.width {
			c.end()
		}
		if c.x > 0 {
			c.space(1)
		}
		c.button(item.label, item.action, item.active)
	}
	c.end()
}
func (c *canvas) end() {
	c.lines = append(c.lines, fitLine(c.line, c.width))
	c.line, c.x = "", 0
	c.y++
}
func (c *canvas) lineText(s string) { c.text(s); c.end() }
func (c *canvas) block(s string) {
	if c.line != "" {
		c.end()
	}
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		c.lines = append(c.lines, fitLine(line, c.width))
		c.y++
	}
}
func (c *canvas) blank()             { c.end() }
func (c *canvas) rule()              { c.lineText(c.style.border().Render(strings.Repeat("─", c.width))) }
func (c *canvas) title(value string) { c.lineText(c.style.title().Render(value)) }
func (c *canvas) muted(value string) { c.lineText(c.style.muted().Render(value)) }
func (c *canvas) panel(title string, lines ...string) {
	c.block(renderPanel(c.width, title, lines, c.style))
}

func (m *model) View() tea.View {
	m.hits = nil
	p := palette{plain: m.plain}
	if m.width < 48 || m.height < 18 {
		return m.compactView(p)
	}
	width := m.width
	header := newCanvas(width, 0, 0, &m.hits, p)

	// Global application bar: persistent destinations and obvious actions, not a prompt menu.
	header.button("◈ IDIOS", "nav:overview", m.page == pageOverview && m.repoContext == "")
	header.space(2)
	if m.searchMode {
		header.button("⌕ "+m.search+"▏", "search:focus", true)
	} else if m.search != "" {
		header.button("⌕ "+m.search, "search:focus", false)
	} else {
		header.button("⌕ Search  /", "search:focus", false)
	}
	header.space(1)
	header.button("Issues", "nav:issues", m.page == pageIssues)
	header.space(1)
	header.button("Pull requests", "nav:pull-requests", m.page == pagePullRequests)
	header.space(1)
	header.button("+ New", "new:menu", m.page == pageNew)
	header.end()

	// Profile-style tabs. A Repository replaces these with its own tab bar below the hero.
	header.buttonRow(
		buttonSpec{"Overview", "nav:overview", m.page == pageOverview && m.repoContext == ""},
		buttonSpec{fmt.Sprintf("Repositories · %d", len(m.repositories)), "nav:repositories", m.page == pageRepositories},
		buttonSpec{fmt.Sprintf("Projects · %d", len(m.projects)), "nav:projects", m.page == pageProjects},
		buttonSpec{fmt.Sprintf("Issues · %d", len(m.issues)), "nav:issues", m.page == pageIssues},
		buttonSpec{fmt.Sprintf("Pull requests · %d", len(m.pullRequests)), "nav:pull-requests", m.page == pagePullRequests},
		buttonSpec{"Graph", "nav:graph", m.page == pageGraph},
		buttonSpec{"Concepts", "nav:concepts", m.page == pageConcepts},
		buttonSpec{"Settings", "nav:settings", m.page == pageSettings},
	)
	header.rule()
	if m.repoContext == "" {
		header.block(renderPanel(width, "IDIOS WORKSPACE", []string{
			"A personal work graph for projects, knowledge and progress.",
			"Local-first  ·  Dependency-aware  ·  One Engine, multiple interfaces",
		}, p))
	} else {
		header.block(renderPanel(width, "REPOSITORY  /  "+m.repoContext, []string{
			m.repoDescription(), "Default branch  main  ·  Repository-owned Issues, PRs, labels and milestones",
		}, p))
		header.buttonRow(
			buttonSpec{"Overview", "repo-tab:" + string(pageRepository), m.page == pageRepository},
			buttonSpec{"Issues", "repo-tab:" + string(pageRepoIssues), m.page == pageRepoIssues},
			buttonSpec{"Pull requests", "repo-tab:" + string(pageRepoPRs), m.page == pageRepoPRs},
			buttonSpec{"Branches", "repo-tab:" + string(pageRepoBranches), m.page == pageRepoBranches},
			buttonSpec{"Graph", "repo-tab:" + string(pageRepoGraph), m.page == pageRepoGraph},
			buttonSpec{"Labels", "repo-tab:" + string(pageRepoLabels), m.page == pageRepoLabels},
			buttonSpec{"Milestones", "repo-tab:" + string(pageRepoMilestones), m.page == pageRepoMilestones},
			buttonSpec{"Settings", "repo-tab:" + string(pageRepoSettings), m.page == pageRepoSettings},
			buttonSpec{"Activity", "repo-tab:" + string(pageRepoActivity), m.page == pageRepoActivity},
		)
	}
	header.blank()
	bodyTop := len(header.lines)

	bodyHeight := max(1, m.height-bodyTop-2)
	m.bodyTop, m.bodyHeight = bodyTop, bodyHeight
	sideWidth := 23
	showSidebar := width >= 88
	if !showSidebar {
		sideWidth = 0
	}
	mainWidth := width
	if showSidebar {
		mainWidth = width - sideWidth - 2
	}

	var sideLines []string
	if showSidebar {
		side := newCanvas(sideWidth, 0, bodyTop, &m.hits, p)
		side.title("WORKSPACE")
		for _, nav := range []struct {
			label string
			page  page
		}{
			{"◫  Overview", pageOverview}, {"▦  Repositories", pageRepositories}, {"▤  Projects", pageProjects},
			{"○  Issues", pageIssues}, {"⑂  Pull requests", pagePullRequests}, {"◎  Knowledge graph", pageGraph}, {"◇  Concepts", pageConcepts}, {"⚙  Settings", pageSettings},
		} {
			side.buttonRow(buttonSpec{nav.label, "nav:" + string(nav.page), m.page == nav.page && m.repoContext == ""})
		}
		side.blank()
		side.title("YOUR REPOSITORIES")
		if len(m.repositories) == 0 {
			side.muted("No repositories yet")
		} else {
			for i, repo := range m.repositories {
				if i >= 8 {
					side.muted(fmt.Sprintf("+ %d more", len(m.repositories)-i))
					break
				}
				label := "◇ " + repo.Name
				side.buttonRow(buttonSpec{label, "repo:" + repo.Name, m.repoContext == repo.Name})
			}
		}
		side.blank()
		side.buttonRow(buttonSpec{"+ Create repository", "form:" + string(formRepository), false})
		sideLines = side.lines
	}

	main := newCanvas(mainWidth, func() int {
		if showSidebar {
			return sideWidth + 2
		}
		return 0
	}(), bodyTop-m.scroll, &m.hits, p)
	m.selectLines = nil
	m.renderMain(main, p)
	m.contentLines = len(main.lines)
	visibleMain := sliceVisible(main.lines, m.scroll, bodyHeight)
	if len(visibleMain) < bodyHeight {
		for len(visibleMain) < bodyHeight {
			visibleMain = append(visibleMain, strings.Repeat(" ", mainWidth))
		}
	}
	if showSidebar {
		if len(sideLines) < bodyHeight {
			for len(sideLines) < bodyHeight {
				sideLines = append(sideLines, strings.Repeat(" ", sideWidth))
			}
		}
		if len(sideLines) > bodyHeight {
			sideLines = sideLines[:bodyHeight]
		}
	}

	lines := append([]string(nil), header.lines...)
	for i := 0; i < bodyHeight; i++ {
		if showSidebar {
			left := fitLine(sideLines[i], sideWidth)
			lines = append(lines, left+"  "+fitLine(visibleMain[i], mainWidth))
		} else {
			lines = append(lines, fitLine(visibleMain[i], width))
		}
	}

	footer := newCanvas(width, 0, m.height-2, &m.hits, p)
	footer.rule()
	if m.statusError {
		footer.lineText(p.danger().Render("! ") + truncateVisible(m.status, max(1, width-3)))
	} else if m.status != "" {
		footer.lineText(p.muted().Render("● ") + truncateVisible(m.status, max(1, width-3)))
	} else {
		footer.lineText(p.muted().Render("Ready"))
	}
	lines = append(lines, footer.lines...)
	help := newCanvas(width, 0, m.height-1, nil, p)
	helpText := "↑/↓ Navigate  ·  Enter Open  ·  Click buttons  ·  / Search  ·  N New  ·  ? Help  ·  Q Quit"
	if width < 82 {
		helpText = "↑↓ Move  ·  Enter Open  ·  / Search  ·  ? Help  ·  Q Quit"
	}
	help.lineText(p.muted().Render(helpText))
	lines = append(lines, help.lines...)

	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	// Only rendered controls have hit targets. Hidden/scrolled controls never receive clicks.
	visibleHits := m.hits[:0]
	for _, h := range m.hits {
		visible := h.y >= 0 && h.y < m.height && h.x < width && h.x+h.w > 0
		if h.y >= bodyTop && h.y < m.height-2 && h.y >= bodyTop+bodyHeight {
			visible = false // control belongs to content that was clipped from the viewport
		}
		if visible {
			visibleHits = append(visibleHits, h)
		}
	}
	m.hits = visibleHits
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "IDIOS — Personal Work Graph"
	v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
		mouse := msg.Mouse()
		if mouse.Button != tea.MouseLeft {
			return nil
		}
		for _, h := range m.hits {
			if mouse.X >= h.x && mouse.X < h.x+h.w && mouse.Y >= h.y && mouse.Y < h.y+h.h {
				action := h.action
				return func() tea.Msg { return actionClickMsg(action) }
			}
		}
		return nil
	}
	return v
}

// actionClickMsg keeps mouse routing in the event loop, rather than executing UI actions in View.
type actionClickMsg string

// compactView avoids drawing beyond the physical terminal when a user shrinks the window.
func (m *model) compactView(p palette) tea.View {
	width, height := max(1, m.width), max(1, m.height)
	lines := []string{
		p.brand().Render("◈ IDIOS · Workspace"),
		"",
		"The terminal window is too small for the full-screen UI.",
		"",
		"Resize to at least 48 columns × 18 rows.",
		"",
		"Q / Ctrl+C  Quit",
	}
	for i, line := range lines {
		lines[i] = fitLine(line, width)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.WindowTitle = "IDIOS — Resize terminal"
	return v
}

func (m *model) renderMain(c *canvas, p palette) {
	switch m.page {
	case pageOverview:
		m.renderOverview(c, p)
	case pageRepositories:
		m.renderRepositories(c, p)
	case pageProjects:
		m.renderProjects(c, p)
	case pageIssues:
		m.renderIssues(c, p, false)
	case pagePullRequests:
		m.renderPullRequests(c, p, false)
	case pageGraph:
		m.renderGraphPage(c, p)
	case pageConcepts:
		m.renderConcepts(c, p)
	case pageSettings:
		m.renderSettings(c, p)
	case pageNew:
		m.renderNew(c, p)
	case pageRepository:
		m.renderRepositoryOverview(c, p)
	case pageRepoIssues:
		m.renderIssues(c, p, true)
	case pageRepoPRs:
		m.renderPullRequests(c, p, true)
	case pageRepoBranches:
		m.renderBranches(c, p)
	case pageRepoGraph:
		m.renderGraphPage(c, p)
	case pageRepoLabels:
		m.renderLabels(c, p)
	case pageRepoMilestones:
		m.renderMilestones(c, p)
	case pageRepoSettings:
		m.renderRepositorySettings(c, p)
	case pageRepoActivity:
		m.renderActivity(c, p)
	case pageIssueDetail:
		m.renderIssueDetail(c, p)
	case pagePRDetail:
		m.renderPRDetail(c, p)
	case pageProjectDetail:
		m.renderProjectDetail(c, p)
	case pageConceptDetail:
		m.renderConceptDetail(c, p)
	case pageForm:
		m.renderForm(c, p)
	case pageHelp:
		m.renderHelp(c, p)
	default:
		c.title("Page not found")
		c.buttonRow(buttonSpec{"Go to overview", "nav:overview", true})
	}
}

func (m *model) renderOverview(c *canvas, p palette) {
	c.title("Your workspace, at a glance")
	c.muted("Your work, projects and ideas — organized in one connected workspace.")
	c.buttonRow(buttonSpec{"+ New repository", "form:" + string(formRepository), true}, buttonSpec{"+ New issue", "form:" + string(formIssue), false}, buttonSpec{"Explore graph", "nav:graph", false})
	counts := m.issueCounts()
	c.panel("WORKSPACE HEALTH",
		fmt.Sprintf("%d repositories   ·   %d projects   ·   %d concepts", len(m.repositories), len(m.projects), len(m.concepts)),
		fmt.Sprintf("%d Issues total   ·   %d open   ·   %d in progress   ·   %d done", len(m.issues), counts[domain.IssueOpen], counts[domain.IssueInProgress], counts[domain.IssueDone]),
		fmt.Sprintf("%d blocked by prerequisites   ·   %d open PRs   ·   %d recorded completions", m.blockedIssueCount(), m.openPRCount(), len(m.completions)),
	)
	c.blank()
	c.title("Pinned repositories")
	if len(m.repositories) == 0 {
		c.panel("GET STARTED", "Create a Repository to start organizing work.", "Example: CPU for learning tasks, ARLIZ for writing and research.")
	} else {
		for i, repo := range m.repositories {
			if i >= 4 {
				break
			}
			issues := 0
			done := 0
			for _, issue := range m.issues {
				if issue.RepositoryName == repo.Name {
					issues++
					if issue.Status == domain.IssueDone {
						done++
					}
				}
			}
			c.button("◇ "+repo.Name, "repo:"+repo.Name, false)
			c.text("   " + truncateVisible(repo.Description, max(8, c.width-lipgloss.Width(repo.Name)-28)))
			c.space(1)
			c.raw(p.badge("info", "main"))
			c.space(1)
			c.text(fmt.Sprintf("%d Issues · %d done", issues, done))
			c.end()
			c.muted(strings.Repeat("─", max(1, c.width-2)))
		}
	}
	c.blank()
	c.title("Needs attention")
	blocked := 0
	for _, issue := range m.issues {
		if issue.Blocked && issue.Status != domain.IssueDone && issue.Status != domain.IssueCancelled {
			blocked++
			if blocked > 4 {
				break
			}
			c.button("⛔ "+issueRef(issue), "issue:"+issueRef(issue), false)
			c.text("  " + issue.Title)
			c.end()
		}
	}
	if blocked == 0 {
		c.muted("Nothing is blocked. Your dependency graph is clear.")
	}
}

func (m *model) renderRepositories(c *canvas, p palette) {
	c.title("Repositories")
	c.muted("Independent workspaces with their own Issues, PRs, labels, milestones and settings.")
	c.buttonRow(buttonSpec{"+ New repository", "form:" + string(formRepository), true}, buttonSpec{"Refresh", "refresh:now", false})
	c.muted(fmt.Sprintf("%d repositories · use / to filter by name or description", len(m.filteredRepositories())))
	if len(m.filteredRepositories()) == 0 {
		c.panel("NO MATCHING REPOSITORIES", "Create a repository or clear the search filter with / then Ctrl+U.")
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
		c.muted(fmt.Sprintf("     %d Issues · %d done · %d open PRs · local-first", count, done, openPR))
		c.buttonRow(buttonSpec{"Open repository →", "repo:" + repo.Name, false}, buttonSpec{"Create Issue", "form:" + string(formIssue), false})
		c.rule()
	}
}

func (m *model) renderProjects(c *canvas, p palette) {
	c.title("Projects")
	c.muted("Combine Issues from multiple repositories around an outcome or milestone.")
	c.buttonRow(buttonSpec{"+ New project", "form:" + string(formProject), true})
	if len(m.filteredProjects()) == 0 {
		c.panel("NO PROJECTS YET", "Projects are cross-repository collections, not repositories themselves.", "Create one, then add Issues like CPU#4 or ARLIZ#2.")
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
		c.muted(fmt.Sprintf("     %d Issues · %d done · %s", len(issues), done, progressText(done, len(issues))))
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
	c.muted("An Issue is a unit of work. Blocking prerequisites are distinct from knowledge links.")
	c.buttonRow(buttonSpec{"+ New Issue", "form:" + string(formIssue), true}, buttonSpec{"Refresh", "refresh:now", false})
	items := m.filteredIssues()
	if len(items) == 0 {
		c.panel("NO ISSUES FOUND", "Create an Issue in a Repository, then connect prerequisites and Concepts.")
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
	c.muted("Review the explanation and outcome. Accepting records a Completion on main; it does not merge files.")
	c.buttonRow(buttonSpec{"Refresh", "refresh:now", false})
	items := m.filteredPRs()
	if len(items) == 0 {
		c.panel("NO PULL REQUESTS", "Start an eligible Issue, then create a PR with the work explanation.")
		return
	}
	for idx, pr := range items {
		m.selectLines = append(m.selectLines, len(c.lines))
		c.button(prRef(pr), "pr:"+prRef(pr), idx == m.cursor)
		c.space(1)
		c.raw(p.badge(statusTone(string(pr.Status), false), string(pr.Status)))
		c.text("  " + truncateVisible(pr.Title, max(8, c.width-lipgloss.Width(prRef(pr))-22)))
		c.end()
		c.muted(fmt.Sprintf("     Issue %s#%d · target %s", pr.RepositoryName, pr.IssueNumber, pr.TargetBranchName))
		c.buttonRow(buttonSpec{"Review details →", "pr:" + prRef(pr), false}, buttonSpec{"Accept", "pr-accept:" + prRef(pr), pr.Status == domain.PRPending})
		c.rule()
	}
}

func (m *model) renderRepositoryOverview(c *canvas, p palette) {
	repo, ok := m.currentRepository()
	if !ok {
		c.panel("REPOSITORY NOT FOUND", "Select a repository from the left sidebar.")
		return
	}
	issues := []domain.Issue{}
	prs := []domain.PullRequest{}
	for _, i := range m.issues {
		if i.RepositoryName == repo.Name {
			issues = append(issues, i)
		}
	}
	for _, pr := range m.pullRequests {
		if pr.RepositoryName == repo.Name {
			prs = append(prs, pr)
		}
	}
	done, open, inProgress, blocked := 0, 0, 0, 0
	for _, i := range issues {
		switch i.Status {
		case domain.IssueDone:
			done++
		case domain.IssueOpen:
			open++
		case domain.IssueInProgress:
			inProgress++
		}
		if i.Blocked {
			blocked++
		}
	}
	openPR := 0
	for _, pr := range prs {
		if pr.Status == domain.PRPending {
			openPR++
		}
	}
	c.title("Repository overview")
	c.muted(repo.Description)
	c.buttonRow(buttonSpec{"+ New Issue", "form:" + string(formIssue), true}, buttonSpec{"View graph", "repo-tab:" + string(pageRepoGraph), false}, buttonSpec{"Settings", "repo-tab:" + string(pageRepoSettings), false})
	c.panel("AT A GLANCE", fmt.Sprintf("%d Issues  ·  %d open  ·  %d in progress  ·  %d done", len(issues), open, inProgress, done), fmt.Sprintf("%d blocked prerequisites  ·  %d open PRs / %d total  ·  default branch main", blocked, openPR, len(prs)))
	c.blank()
	c.title("Recent Issues")
	if len(issues) == 0 {
		c.muted("No Issues yet. Create the first unit of work.")
	}
	for i, issue := range issues {
		if i >= 6 {
			break
		}
		c.button(issueRef(issue), "issue:"+issueRef(issue), false)
		c.space(1)
		c.text(truncateVisible(issue.Title, max(6, c.width-25)))
		c.space(1)
		c.raw(p.badge(statusTone(string(issue.Status), issue.Blocked), string(issue.Status)))
		c.end()
	}
	c.blank()
	c.title("Current branch")
	c.panel("main", "Default branch for accepted work history.", "Other Branch operations are reserved for a future release.")
}

func (m *model) renderBranches(c *canvas, p palette) {
	c.title("Branches")
	c.muted("Branch is modeled independently of Issues. Only main is operational in this release.")
	branches, err := m.app.ListBranches(m.ctx, m.repoContext)
	if err != nil {
		c.panel("COULD NOT LOAD BRANCHES", err.Error())
		return
	}
	if len(branches) == 0 {
		c.panel("NO BRANCHES", "Every Repository should have a default main Branch.")
		return
	}
	for _, b := range branches {
		label := b.Name
		if b.IsDefault {
			label += "  ★ default"
		}
		c.panel("⑂  "+label, fmt.Sprintf("Created %s", b.CreatedAt.Format("2006-01-02 15:04")), "Branch creation, switching and merge for non-main branches are not active yet.")
	}
}

func (m *model) renderLabels(c *canvas, p palette) {
	c.title("Labels")
	c.muted("Use global labels for workspace-wide meaning, or repository labels for local workflow.")
	c.buttonRow(buttonSpec{"+ New label", "label-action:new", true})
	labels, _ := m.app.ListLabels(m.ctx, "")
	if len(labels) == 0 {
		c.panel("NO LABELS YET", "Examples: lang, research, architecture, blocked, writing.")
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
	c.muted("Checkpoints group work toward a defined outcome.")
	c.buttonRow(buttonSpec{"+ New milestone", "milestone-action:new", true})
	if len(m.milestones) == 0 {
		c.panel("NO MILESTONES", "Create a milestone such as 'Chapter 3 draft' or 'CPU fundamentals'.")
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
	c.muted("Settings are local metadata. Workflow rules remain enforced by the Engine.")
	c.buttonRow(buttonSpec{"+ Set key / value", "settings-action:set", true})
	settings, err := m.app.GetRepositorySettings(m.ctx, m.repoContext)
	if err != nil {
		c.panel("COULD NOT LOAD SETTINGS", err.Error())
		return
	}
	if len(settings) == 0 {
		c.panel("NO CUSTOM SETTINGS", "The repository uses defaults and its standard main Branch.")
	}
	for _, s := range settings {
		c.panel(s.Key, s.Value)
	}
}

func (m *model) renderActivity(c *canvas, p palette) {
	c.title("Activity & completions")
	c.muted("An audit-style timeline of changes and accepted work in this Repository.")
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
		c.panel("NO ACTIVITY YET", "Activity will appear as Issues, PRs, relations and settings change.")
	}
}

func (m *model) renderGraphPage(c *canvas, p palette) {
	c.title("Knowledge graph")
	c.muted("Click a node to inspect it. Blocking dependencies are shown separately from semantic links.")
	if m.graphFrom != "" {
		c.lineText("Focused on: " + m.graphFrom)
		c.buttonRow(buttonSpec{"Show whole workspace", "graph:all", false})
	}
	c.panel("GRAPH SUMMARY", fmt.Sprintf("%d nodes · %d typed relationships", len(m.graph.Nodes), len(m.graph.Edges)), "Solid dependency edges block starting an Issue. Concept links are contextual only.")
	if len(m.graph.Nodes) == 0 {
		c.panel("YOUR GRAPH STARTS HERE", "Create repositories, Issues and Concepts, then connect them.", "A graph becomes useful when it explains why work is blocked or how ideas connect.")
		return
	}
	byID := map[string]domain.GraphNode{}
	for _, n := range m.graph.Nodes {
		byID[n.ID] = n
	}
	outgoing := map[string][]domain.GraphEdge{}
	for _, e := range m.graph.Edges {
		outgoing[e.Source] = append(outgoing[e.Source], e)
	}
	c.blank()
	c.title("Relationship map")
	count := 0
	for _, node := range sortedGraphNodes(m.graph.Nodes) {
		edges := outgoing[node.ID]
		if len(edges) == 0 {
			continue
		}
		if count >= 40 {
			c.muted("Graph view capped at 40 relationship groups. Narrow the graph by opening a node.")
			break
		}
		count++
		c.button(graphNodeRef(node), "graph-node:"+node.ID, false)
		c.text("  " + truncateVisible(graphLabel(node), max(8, c.width-lipgloss.Width(graphNodeRef(node))-4)))
		c.end()
		for i, e := range edges {
			target, ok := byID[e.Target]
			branch := "├─"
			if i == len(edges)-1 {
				branch = "└─"
			}
			c.space(2)
			c.raw(p.border().Render(branch + " "))
			relation := e.Type
			if relation == "depends_on" {
				relation = "⛔ depends_on · BLOCKING"
				c.raw(p.danger().Render(relation))
			} else {
				c.raw(p.muted().Render("◇ " + relation))
			}
			c.text("  →  ")
			if ok {
				c.button(graphNodeRef(target), "graph-node:"+target.ID, false)
				c.text(" " + truncateVisible(graphLabel(target), max(3, c.width-lipgloss.Width(graphNodeRef(target))-18)))
			} else {
				c.text(e.Target)
			}
			c.end()
		}
		c.rule()
	}
	c.blank()
	c.title("All nodes")
	for idx, node := range sortedGraphNodes(m.graph.Nodes) {
		m.selectLines = append(m.selectLines, len(c.lines))
		c.button("["+strings.ToUpper(node.Type)+"] "+graphNodeRef(node), "graph-node:"+node.ID, idx == m.cursor)
		c.space(1)
		c.text(truncateVisible(graphLabel(node), max(5, c.width-lipgloss.Width(graphNodeRef(node))-15)))
		if node.Status != "" {
			c.space(1)
			c.raw(p.badge(statusTone(node.Status, strings.Contains(strings.ToLower(node.Status), "block")), node.Status))
		}
		c.end()
	}
}

func (m *model) renderConcepts(c *canvas, p palette) {
	c.title("Concepts")
	c.muted("Reusable pieces of knowledge connected to Issues and repositories.")
	c.buttonRow(buttonSpec{"+ New concept", "form:" + string(formConcept), true}, buttonSpec{"Open graph", "nav:graph", false})
	if len(m.filteredConcepts()) == 0 {
		c.panel("NO CONCEPTS YET", "Create Concepts like CPU, electricity, mathematics or virtual memory.")
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
	c.title("Workspace settings")
	c.muted("Local-first configuration and UI preferences for this installation.")
	c.panel("DATA & PRIVACY", "All workspace data lives in the local SQLite database.", "Database path: "+m.databasePath, "No account or network service is required for normal use.")
	c.panel("WORKFLOW POLICY", "Every Issue is completed through an accepted Pull Request.", "Accepting a PR records a Completion on main; it does not perform a Git merge.", "Starting an Issue with unfinished prerequisites is rejected by the Core.")
	c.buttonRow(buttonSpec{"Choose repository settings", "nav:repositories", false}, buttonSpec{"Refresh data", "refresh:now", false}, buttonSpec{"Keyboard & mouse guide", "help:keys", false})
}

func (m *model) renderNew(c *canvas, p palette) {
	c.title("Create something new")
	c.muted("Choose what you want to add. Each action opens a proper form in this UI.")
	c.buttonRow(buttonSpec{"Repository", "form:" + string(formRepository), true}, buttonSpec{"Project", "form:" + string(formProject), false})
	c.buttonRow(buttonSpec{"Issue", "form:" + string(formIssue), false}, buttonSpec{"Concept", "form:" + string(formConcept), false})
	c.panel("WHICH ONE SHOULD I USE?", "Repository: an independent work area such as CPU or ARLIZ.", "Project: one goal that combines Issues from different repositories.", "Issue: one unit of work; a Sub-issue belongs to a parent Issue.", "Concept: reusable knowledge that can connect many pieces of work.")
}

func (m *model) renderIssueDetail(c *canvas, p palette) {
	issue := m.findIssue(m.selectedIssueRef)
	if issue == nil {
		c.panel("ISSUE NOT FOUND", m.selectedIssueRef, "The Issue may have been removed. Refresh and return to the list.")
		c.buttonRow(buttonSpec{"← Back", "back:now", false})
		return
	}
	c.title(issue.Title)
	c.muted(fmt.Sprintf("%s  ·  Issue #%d  ·  created %s", issue.RepositoryName, issue.Number, issue.CreatedAt.Format("2006-01-02")))
	c.buttonRow(buttonSpec{"← Back", "back:now", false}, buttonSpec{"Open repository", "repo:" + issue.RepositoryName, false}, buttonSpec{"View graph", "issue-action:graph", false})
	statusToneValue := statusTone(string(issue.Status), issue.Blocked)
	c.panel("STATUS", p.badge(statusToneValue, string(issue.Status)), fmt.Sprintf("Priority: %s", fallback(issue.Priority, "normal")), fmt.Sprintf("Blocked: %t", issue.Blocked))
	c.panel("DESCRIPTION", fallback(issue.Description, "No description. Add context when creating the Issue; record final outcomes in its PR."))
	if labels := m.issueLabels[issue.ID]; len(labels) > 0 {
		c.title("Labels")
		items := make([]buttonSpec, 0, len(labels))
		for _, label := range labels {
			items = append(items, buttonSpec{"◆ " + label.Name, "issue-labels:" + issue.RepositoryName, false})
		}
		c.buttonRow(items...)
	} else {
		c.muted("Labels: none assigned")
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
		c.muted("Milestone: none assigned")
	}
	if issue.Blocked {
		c.title("Prerequisites blocking this Issue")
		for _, b := range issue.Blockers {
			prerequisite := fmt.Sprintf("%s#%d", b.RepositoryName, b.Number)
			c.button(fmt.Sprintf("⛔ %s", prerequisite), "issue:"+prerequisite, false)
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
		c.muted("Start is disabled until all blocking prerequisites are complete.")
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
		c.panel("PULL REQUEST NOT FOUND", m.selectedPRRef, "Return to the PR list and refresh.")
		c.buttonRow(buttonSpec{"← Back", "back:now", false})
		return
	}
	c.title(pr.Title)
	c.muted(fmt.Sprintf("%s  ·  Pull request #%d  ·  Issue %s#%d", pr.RepositoryName, pr.Number, pr.RepositoryName, pr.IssueNumber))
	c.buttonRow(buttonSpec{"← Back", "back:now", false}, buttonSpec{"Open linked Issue", "pr-action:issue", false}, buttonSpec{"Open repository", "repo:" + pr.RepositoryName, false})
	c.panel("REVIEW STATUS", p.badge(statusTone(string(pr.Status), false), string(pr.Status)), "Target branch: "+pr.TargetBranchName, "Completion on main is recorded only after acceptance.")
	c.panel("WORK EXPLANATION", fallback(pr.Description, "No explanation supplied."))
	if pr.Status == domain.PRPending {
		c.buttonRow(buttonSpec{"✓ Accept & record completion", "pr-action:accept", true}, buttonSpec{"Close without completion", "pr-action:close", false})
	}
	if pr.Status == domain.PRAccepted {
		c.panel("ACCEPTED", "This PR already has a Completion Record.")
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
	c.panel("PROGRESS", fmt.Sprintf("%d / %d Issues done · %s", done, len(issues), progressText(done, len(issues))))
	for _, issue := range issues {
		c.button(issueRef(issue), "issue:"+issueRef(issue), false)
		c.text("  " + issue.Title)
		c.space(1)
		c.raw(p.badge(statusTone(string(issue.Status), issue.Blocked), string(issue.Status)))
		c.end()
	}
	if len(issues) == 0 {
		c.panel("EMPTY PROJECT", "Add an Issue from any repository using a reference such as CPU#4.")
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
	c.muted("Shared knowledge node · concept detail")
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
		c.muted("No linked nodes yet. Link the Concept to an Issue to build context.")
	}
}

func (m *model) renderForm(c *canvas, p palette) {
	c.title(m.formTitle)
	c.muted("Fill the fields with keyboard input. Tab moves focus; Ctrl+Enter saves. Click a field or button at any time.")
	if m.formError != "" {
		c.lineText(p.danger().Render("! " + m.formError))
	}
	for i, field := range m.formFields {
		active := i == m.formFocus
		if active {
			c.raw(p.accent().Render("▸ "))
		} else {
			c.space(2)
		}
		c.button(field.label, fmt.Sprintf("form-field:%d", i), active)
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
		c.input(value, fmt.Sprintf("form-field:%d", i), active)
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
	c.panel("FORM TIPS", "Press Ctrl+U to clear the focused field. Backspace deletes the previous character.", "Required-field and workflow validation is performed by the Application Core.")
}

func (m *model) renderHelp(c *canvas, p palette) {
	c.title("How to use IDIOS")
	c.muted("A full-screen terminal UI: click buttons with the mouse or use keyboard navigation.")
	c.panel("NAVIGATION", "Click any tab or sidebar destination. ↑/↓ or j/k moves through lists. Enter opens the selected item.", "Tab / Shift+Tab cycles global destinations. Esc goes back. Q or Ctrl+C quits.", "/ activates search; type to filter the current list; Enter finishes; Esc clears it.")
	c.panel("ACTIONS", "N opens the creation hub. The New button opens forms for repository, project, Issue and Concept.", "Forms use Tab to move focus and Ctrl+Enter to submit. Click a field or action button directly.", "Press R to refresh data from SQLite.")
	c.panel("WORKFLOW", "Start an Issue only when its explicit depends_on prerequisites are complete.", "Create a PR with a meaningful explanation; accept it to record a Completion on main.", "Semantic Concept links provide context, not blocking. No Git file merge occurs in V1.")
	c.buttonRow(buttonSpec{"← Back to workspace", "nav:overview", true})
}

func (m *model) currentRepository() (domain.Repository, bool) {
	for _, r := range m.repositories {
		if r.Name == m.repoContext {
			return r, true
		}
	}
	return domain.Repository{}, false
}
func (m *model) repoDescription() string {
	if r, ok := m.currentRepository(); ok {
		return fallback(r.Description, "No repository description.")
	}
	return "Repository metadata"
}
func (m *model) issueCounts() map[domain.IssueStatus]int {
	out := map[domain.IssueStatus]int{}
	for _, i := range m.issues {
		out[i.Status]++
	}
	return out
}
func (m *model) blockedIssueCount() int {
	n := 0
	for _, i := range m.issues {
		if i.Blocked && i.Status != domain.IssueDone && i.Status != domain.IssueCancelled {
			n++
		}
	}
	return n
}
func (m *model) openPRCount() int {
	n := 0
	for _, pr := range m.pullRequests {
		if pr.Status == domain.PRPending {
			n++
		}
	}
	return n
}
func (m *model) findIssue(ref string) *domain.Issue {
	for i := range m.issues {
		if issueRef(m.issues[i]) == ref {
			return &m.issues[i]
		}
	}
	return nil
}
func (m *model) findPR(ref string) *domain.PullRequest {
	for i := range m.pullRequests {
		if prRef(m.pullRequests[i]) == ref {
			return &m.pullRequests[i]
		}
	}
	return nil
}

func (m *model) handleActionClick(action string) tea.Cmd { return m.handleAction(action) }

func statusTone(status string, blocked bool) string {
	if blocked {
		return "danger"
	}
	switch strings.ToLower(status) {
	case "done", "accepted", "success":
		return "success"
	case "in_progress", "pending", "open":
		return "info"
	case "blocked", "cancelled", "closed", "error":
		return "danger"
	default:
		return "warning"
	}
}
func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
func progressText(done, total int) string {
	if total == 0 {
		return "no work linked yet"
	}
	return fmt.Sprintf("%d%% complete", done*100/total)
}
func graphNodeRef(n domain.GraphNode) string {
	if n.Reference != "" {
		return strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(n.Reference, "issue:"), "repo:"), "concept:")
	}
	return n.Type + ":" + n.Label
}
func graphLabel(n domain.GraphNode) string {
	label := strings.TrimSpace(n.Label)
	ref := graphNodeRef(n)
	label = strings.TrimPrefix(label, ref+" · ")
	if label == "" {
		return n.ID
	}
	return label
}
func sortedGraphNodes(nodes []domain.GraphNode) []domain.GraphNode {
	out := append([]domain.GraphNode(nil), nodes...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			left, right := out[j-1], out[j]
			if left.Type < right.Type || (left.Type == right.Type && graphNodeRef(left) <= graphNodeRef(right)) {
				break
			}
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
func renderPanel(width int, title string, body []string, p palette) string {
	width = max(20, width)
	title = truncateVisible(title, width-7)
	topPrefix, topSuffix := "╭─ ", " "
	top := p.border().Render(topPrefix) + p.title().Render(title) + p.border().Render(topSuffix+strings.Repeat("─", max(1, width-lipgloss.Width(title)-5))+"╮")
	lines := []string{fitLine(top, width)}
	inner := width - 4
	for _, raw := range body {
		line := truncateVisible(raw, inner)
		pad := max(0, inner-lipgloss.Width(line))
		lines = append(lines, p.border().Render("│ ")+line+strings.Repeat(" ", pad)+p.border().Render(" │"))
	}
	lines = append(lines, p.border().Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(lines, "\n")
}
func fitLine(value string, width int) string {
	w := lipgloss.Width(value)
	if w > width {
		value = truncateVisible(value, width)
		w = lipgloss.Width(value)
	}
	if w < width {
		value += strings.Repeat(" ", width-w)
	}
	return value
}
func truncateVisible(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}

	var out strings.Builder
	visibleWidth := 0
	hadANSI := strings.Contains(value, "\x1b[")
	for i := 0; i < len(value); {
		if value[i] == 0x1b && i+1 < len(value) {
			start := i
			switch value[i+1] {
			case '[': // CSI sequence, including SGR styling
				i += 2
				for i < len(value) {
					b := value[i]
					i++
					if b >= 0x40 && b <= 0x7e {
						break
					}
				}
				out.WriteString(value[start:i])
				continue
			case ']': // OSC sequence, terminated by BEL or ST
				i += 2
				for i < len(value) {
					if value[i] == 0x07 {
						i++
						break
					}
					if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
				out.WriteString(value[start:i])
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		runeWidth := lipgloss.Width(string(r))
		if visibleWidth+runeWidth > width-1 {
			break
		}
		out.WriteString(value[i : i+size])
		visibleWidth += runeWidth
		i += size
	}
	out.WriteRune('…')
	if hadANSI {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func sliceVisible(lines []string, start, count int) []string {
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := min(len(lines), start+count)
	return append([]string(nil), lines[start:end]...)
}

var _ tea.Model = (*model)(nil)
