package tui

import (
	"fmt"
	"strings"

	"idios/internal/features"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	header.button("+ New", "new:menu", m.page == pageNew)
	header.space(1)
	header.button("⌘ Commands  Ctrl+K", "commands:open", m.page == pageCommands)
	header.end()

	// Profile-style tabs. A Repository replaces these with its own tab bar below the hero.
	topTabs := make([]buttonSpec, 0, len(features.TUITopNavigation()))
	for _, item := range features.TUITopNavigation() {
		label := item.Label
		switch item.Metric {
		case "repositories":
			label = fmt.Sprintf(copyText.text("Repositories · %d"), len(m.repositories))
		case "open_issues":
			label = fmt.Sprintf(copyText.text("Issues · %d"), len(m.issues))
		case "jobs":
			label = fmt.Sprintf(copyText.text("Jobs · %d"), len(m.jobs))
		}
		topTabs = append(topTabs, buttonSpec{label, featureAction(item), m.page == page(item.TUIPage) && m.repoContext == ""})
	}
	header.tabRow(topTabs...)
	if m.page != pageOverview && m.page != pageIntro && m.page != pageDeleteConfirm && m.page != pageForm {
		header.rule()
		if m.repoContext != "" {
			header.raw(p.title().Render("◈ " + m.repoContext))
			header.raw(p.muted().Render("  ·  branch main  ·  " + m.repoDescription()))
			header.end()
			header.tabRow(
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

	bodyHeight := max(1, m.height-bodyTop-3)
	m.bodyTop, m.bodyHeight = bodyTop, bodyHeight
	sideWidth := 23
	showSidebar := width >= 88 && m.page != pageOverview && m.page != pageIntro && m.page != pageForm && m.page != pageQuit && m.page != pageBatchProgress && m.page != pageBatchResults && m.page != pageCommands && m.page != pageDeleteConfirm
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
		side.title("MORE")
		// Keep destinations that are not already available in the persistent top tabs.
		// This rail is a useful secondary index, not a second copy of the primary nav.
		for _, item := range features.TUISideNavigation() {
			action := featureAction(item)
			active := item.TUIPage != "" && m.page == page(item.TUIPage) && m.repoContext == ""
			side.buttonRow(buttonSpec{featureTUILabel(item), action, active})
		}
		side.blank()
		side.title("QUICK CREATE")
		side.buttonRow(buttonSpec{"+ Issue", "form:" + string(formIssue), false})
		side.buttonRow(buttonSpec{"+ Issues (bulk)", "form:" + string(formBulkIssues), false})
		side.buttonRow(buttonSpec{"+ Quote", "form:" + string(formQuote), false})
		side.buttonRow(buttonSpec{"+ Quotes (bulk)", "form:" + string(formBulkQuotes), false})
		side.buttonRow(buttonSpec{"+ Job", "job-action:new", false})
		side.buttonRow(buttonSpec{"+ Jobs (bulk)", "form:" + string(formBulkJobs), false})
		side.blank()
		side.muted(fmt.Sprintf("%d repositories · %d open issues", len(m.repositories), len(m.issues)))
		side.buttonRow(buttonSpec{"Exit IDIOS", "quit:confirm", false})
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
	help := newCanvas(width, 0, m.height-1, &m.hits, p)
	if m.availableUpdate != nil && m.page != pageUpgrade && !m.upgradeHandedOff {
		label := "↑ Upgrade"
		buttonWidth := lipgloss.Width(p.button(m.upgradeFrame%8 < 4).Render(label))
		hintsWidth := max(1, width-buttonWidth-2)
		help.raw(truncateVisible(m.footerHints(p, hintsWidth), hintsWidth))
		help.space(max(1, width-help.x-buttonWidth))
		help.button(label, "upgrade:open", m.upgradeFrame%8 < 4)
		help.end()
	} else {
		help.lineText(m.footerHints(p, width))
	}
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
	v.WindowTitle = "IDIOS"
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
		p.brand().Render("◈ IDIOS"),
		"",
		"The terminal window is too small.",
		"",
		"Resize it to at least 48 columns × 18 rows.",
		"",
		"Q / Ctrl+Q  Quit · Ctrl+C emergency",
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
	case pageUpgrade:
		m.renderUpgrade(c, p)
	case pageQuit:
		m.renderQuitConfirmation(c, p)
	case pageBatchResults:
		m.renderBatchResults(c, p)
	case pageBatchProgress:
		m.renderBatchProgress(c, p)
	case pageCommands:
		m.renderCommandPalette(c, p)
	case pageRemove:
		m.renderRemoveCenter(c, p)
	case pageDeleteConfirm:
		m.renderDeleteConfirmation(c, p)
	default:
		c.title("Page not found")
		c.buttonRow(buttonSpec{"Go to overview", "nav:overview", true})
	}
}

var _ tea.Model = (*model)(nil)

// footerHints shows only the keys that work on the current screen. Entries that do not fit
// are dropped from the right, so the most important ones stay visible in narrow terminals.
func (m *model) footerHints(p palette, width int) string {
	type hint struct{ key, label string }
	var hints []hint
	switch {
	case m.page == pageForm:
		if isBulkForm(m.form) && !m.batchConfirm {
			hints = []hint{{"Tab", "next field"}, {"Ctrl+Enter", "review batch"}, {"Ctrl+U", "clear field"}, {"Esc", "cancel"}}
		} else if isBulkForm(m.form) {
			hints = []hint{{"Ctrl+Enter", "create batch"}, {"Tab", "edit fields"}, {"Esc", "cancel"}}
		} else {
			hints = []hint{{"Tab", "next field"}, {"Ctrl+Enter", "save"}, {"Ctrl+U", "clear field"}, {"Esc", "cancel"}}
		}
	case m.page == pageQuit:
		hints = []hint{{"Enter", "exit safely"}, {"Esc", "stay"}}
	case m.page == pageBatchResults:
		hints = []hint{{"↑/↓", "scroll"}, {"PgUp/Dn", "page"}, {"Ctrl+Tab", "focus actions"}, {"Q", "quit"}, {"Esc", "overview"}}
	case m.page == pageBatchProgress:
		hints = []hint{{"Esc", "cancel after current item"}, {"↑/↓", "scroll"}, {"PgUp/Dn", "page"}, {"Ctrl+Tab", "focus cancel"}, {"Q", "quit menu"}}
	case m.page == pageCommands:
		hints = []hint{{"Type", "filter commands"}, {"↑/↓", "select"}, {"Enter", "run"}, {"Esc", "close"}}
	case m.page == pageRemove:
		hints = []hint{{"/", "filter"}, {"Enter", "select"}, {"Esc", "back"}, {"Ctrl+K", "commands"}}
	case m.page == pageDeleteConfirm:
		hints = []hint{{"Enter", "confirm"}, {"Esc", "cancel"}, {"Ctrl+Q", "quit menu"}}
	case m.searchMode:
		hints = []hint{{"Enter", "apply filter"}, {"Esc", "clear filter"}}
	case m.page == pageIntro:
		hints = []hint{{"Enter", "open overview"}, {"?", "guide"}, {"q", "quit"}}
	case m.page == pageUpgrade:
		hints = []hint{{"Enter", "upgrade"}, {"Esc", "back"}, {"q", "quit"}}
	default:
		hints = []hint{
			{"↑/↓", "select"}, {"Enter", "open"}, {"Esc", "back"},
			{prettyKey(m.config.Keyboard["search"], "/"), "filter"},
			{prettyKey(m.config.Keyboard["new"], "Ctrl+N"), "new"},
			{prettyKey(m.config.Keyboard["help"], "?"), "help"},
			{prettyKey(m.config.Keyboard["quit"], "q"), "quit menu"},
			{"Ctrl+Q", "quit menu"},
		}
	}
	var out strings.Builder
	used := 0
	for i, h := range hints {
		size := lipgloss.Width(h.key) + 1 + lipgloss.Width(h.label)
		if i > 0 {
			size += 5
		}
		if used+size > width-1 {
			break
		}
		if i > 0 {
			out.WriteString(p.muted().Render("  ·  "))
		}
		out.WriteString(p.accent().Render(h.key) + " " + p.muted().Render(h.label))
		used += size
	}
	return out.String()
}

// prettyKey turns a configured binding such as "ctrl+n" into "Ctrl+N" for display.
func prettyKey(binding, fallbackKey string) string {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		return fallbackKey
	}
	parts := strings.Split(binding, "+")
	for i, part := range parts {
		switch {
		case len(part) > 1:
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		default:
			parts[i] = strings.ToUpper(part)
		}
	}
	return strings.Join(parts, "+")
}
