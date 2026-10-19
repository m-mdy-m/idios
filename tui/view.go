package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"strings"
)

func (m *model) View() tea.View {
	m.hits = nil
	p := palette{plain: m.plain, colors: m.config.ResolvedColors(), density: m.config.Appearance.Density}
	if m.width < 48 || m.height < 18 {
		return m.compactView(p)
	}
	width := m.width
	header := newCanvas(width, 0, 0, &m.hits, p)
	header.focusAction = m.focusAction

	// Global application bar: persistent destinations and obvious actions, not a prompt menu.
	brand := "◈ IDIOS"
	if m.demoMode {
		brand += " · DEMO"
	}
	header.button(brand, "nav:intro", m.page == pageIntro && m.repoContext == "")
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
		buttonSpec{fmt.Sprintf(copyText.text("Repositories · %d"), len(m.repositories)), "nav:repositories", m.page == pageRepositories},
		buttonSpec{fmt.Sprintf(copyText.text("Projects · %d"), len(m.projects)), "nav:projects", m.page == pageProjects},
		buttonSpec{fmt.Sprintf(copyText.text("Issues · %d"), len(m.issues)), "nav:issues", m.page == pageIssues},
		buttonSpec{fmt.Sprintf(copyText.text("Pull requests · %d"), len(m.pullRequests)), "nav:pull-requests", m.page == pagePullRequests},
		buttonSpec{"Graph", "nav:graph", m.page == pageGraph},
		buttonSpec{"Concepts", "nav:concepts", m.page == pageConcepts},
		buttonSpec{"Notes", "nav:notes", m.page == pageNotes},
		buttonSpec{"Jobs", "nav:jobs", m.page == pageJobs},
		buttonSpec{"Settings", "nav:settings", m.page == pageSettings},
	)
	if m.page != pageOverview && m.page != pageIntro {
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
	}
	header.blank()
	bodyTop := len(header.lines)

	bodyHeight := max(1, m.height-bodyTop-2)
	m.bodyTop, m.bodyHeight = bodyTop, bodyHeight
	sideWidth := 23
	showSidebar := width >= 88 && m.page != pageOverview && m.page != pageIntro
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
		side.focusAction = m.focusAction
		side.title("WORKSPACE")
		for _, nav := range []struct {
			label string
			page  page
		}{
			{"◫  Overview", pageOverview}, {"▦  Repositories", pageRepositories}, {"▤  Projects", pageProjects},
			{"○  Issues", pageIssues}, {"⑂  Pull requests", pagePullRequests}, {"◎  Knowledge graph", pageGraph}, {"◇  Concepts", pageConcepts}, {"✎  Notes", pageNotes}, {"◷  Jobs", pageJobs}, {"⚙  Settings", pageSettings},
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
					side.muted(fmt.Sprintf(copyText.text("+ %d more"), len(m.repositories)-i))
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
	main.focusAction = m.focusAction
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
	footer.focusAction = m.focusAction
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
	if !m.plain {
		background := lipgloss.NewStyle().Width(width).Background(lipgloss.Color(p.color("background")))
		for i := range lines {
			lines[i] = background.Render(lines[i])
		}
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
	case pageIntro:
		m.renderIntro(c, p)
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
	case pageNotes:
		m.renderNotes(c, p)
	case pageQuotes:
		m.renderQuotes(c, p)
	case pageJobs:
		m.renderJobs(c, p)
	case pageNotifications:
		m.renderNotifications(c, p)
	case pageSettingsNotifications:
		m.renderNotificationSettings(c, p)
	case pageLogs:
		m.renderLogs(c, p)
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

var _ tea.Model = (*model)(nil)
