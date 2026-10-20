package tui

import (
	"fmt"
	"strings"

	"idios/internal/domain"

	"charm.land/lipgloss/v2"
)

func (m *model) currentGraphLayout() graphLayout {
	return buildGraphLayout(m.graph, m.graphFrom, m.graphStructure)
}

// renderGraphPage reads top to bottom as: where you are, what is blocked, then every node with
// its links worded as short sentences. Structural plumbing is hidden until asked for.
func (m *model) renderGraphPage(c *canvas, p palette) {
	layout := m.currentGraphLayout()

	c.title("Knowledge graph")
	if layout.hasFocus {
		c.muted(fmt.Sprintf("Showing %s and everything within %d link(s) of it.", graphNodeRef(layout.focus), max(1, m.graphDepth)))
	} else if m.graphFrom != "" {
		c.muted("Focused on " + m.graphFrom + ".")
	} else {
		c.muted("Everything in your workspace. Open a node to focus on just its neighbourhood.")
	}

	buttons := []buttonSpec{}
	if m.graphFrom != "" {
		buttons = append(buttons,
			buttonSpec{"Show whole workspace", "graph:all", false},
			buttonSpec{fmt.Sprintf(copyText.text("Distance: %d → next"), max(1, m.graphDepth)), "graph:depth", false})
	}
	structure := "Ownership links: hidden → show"
	if m.graphStructure {
		structure = "Ownership links: shown → hide"
	}
	buttons = append(buttons, buttonSpec{structure, "graph:structure", false})
	c.buttonRow(buttons...)

	if layout.totalNodeCount == 0 {
		c.panel("NOTHING TO SHOW YET", "Create issues and concepts, then link them with prerequisites or concept links.")
		return
	}

	summary := []string{graphCountSummary(layout.counts), fmt.Sprintf("%d link(s) shown", layout.linkCount)}
	if layout.hiddenLinks > 0 || layout.hiddenNodes > 0 {
		summary = append(summary, fmt.Sprintf("Hidden: %d ownership link(s), %d unlinked branch/label/milestone node(s).", layout.hiddenLinks, layout.hiddenNodes))
	}
	c.panel("SUMMARY", summary...)
	c.panel("HOW TO READ THIS",
		"⛔ waits for  Blocking. The first item cannot start until the second is done.",
		"→  Workflow: sub-issues, pull requests, completions.",
		"◇  Context: concepts, notes, labels, milestones, projects. Never blocks anything.",
		"▪  Ownership (hidden by default): repository ↔ issue, pull request ↔ branch.")

	m.renderGraphBlockers(c, p, layout)
	m.renderGraphGroups(c, p, layout)
	m.renderGraphOthers(c, p, layout)
}

func (m *model) renderGraphBlockers(c *canvas, p palette, layout graphLayout) {
	c.blank()
	c.title("What is blocked right now")
	if len(layout.blockers) == 0 {
		if layout.doneBlocks > 0 {
			c.muted(fmt.Sprintf("Nothing. All %d prerequisite(s) in this view are done.", layout.doneBlocks))
		} else {
			c.muted("Nothing. No issue in this view has a prerequisite.")
		}
		return
	}
	c.muted(fmt.Sprintf("%d issue(s) cannot start until these prerequisites are done:", layout.blockedIssues))
	lastWaiting := ""
	for i, b := range layout.blockers {
		if b.waiting.ID != lastWaiting {
			if lastWaiting != "" {
				c.blank()
			}
			lastWaiting = b.waiting.ID
			m.graphNodeRow(c, p, b.waiting, false, 0)
		}
		last := i == len(layout.blockers)-1 || layout.blockers[i+1].waiting.ID != b.waiting.ID
		m.graphLinkRow(c, p, last, graphLink{phrase: "waits for", kind: linkBlocking, other: b.needed, known: true}, len("waits for"))
	}
}

func (m *model) renderGraphGroups(c *canvas, p palette, layout graphLayout) {
	if len(layout.groups) == 0 {
		return
	}
	c.blank()
	c.title("Nodes and their links")
	for idx, group := range layout.groups {
		c.blank()
		m.selectLines = append(m.selectLines, len(c.lines))
		m.graphNodeRow(c, p, group.node, idx == m.cursor, 0)
		if group.focus {
			c.space(2)
			c.lineText(p.accent().Render("▲ focus: links in both directions"))
		}
		pad := 0
		for _, link := range group.links {
			pad = max(pad, lipgloss.Width(link.phrase))
		}
		pad = min(pad, 24)
		for i, link := range group.links {
			m.graphLinkRow(c, p, i == len(group.links)-1, link, pad)
		}
	}
	if layout.truncGroups > 0 {
		c.blank()
		c.muted(fmt.Sprintf("%d more node(s) are not listed. Open a node and choose “View graph” to focus on its neighbourhood.", layout.truncGroups))
	}
}

func (m *model) renderGraphOthers(c *canvas, p palette, layout graphLayout) {
	if len(layout.others) == 0 && layout.truncOthers == 0 {
		return
	}
	c.blank()
	c.title("Other nodes")
	c.muted("Linked only from the nodes above, or not linked yet.")
	offset := len(layout.groups)
	for idx, node := range layout.others {
		m.selectLines = append(m.selectLines, len(c.lines))
		m.graphNodeRow(c, p, node, offset+idx == m.cursor, 0)
	}
	if layout.truncOthers > 0 {
		c.muted(fmt.Sprintf("%d more node(s) are not listed. Focus on a node to narrow the view.", layout.truncOthers))
	}
}

// graphNodeRow prints "REF  type  title  [STATUS]". The type word and status text are always
// printed so meaning never depends on color alone.
func (m *model) graphNodeRow(c *canvas, p palette, node domain.GraphNode, selected bool, indent int) {
	c.space(indent)
	ref := graphNodeRef(node)
	c.button(ref, "graph-node:"+node.ID, selected)
	c.space(1)
	c.raw(p.muted().Render(graphTypeIcon(node.Type) + " " + graphTypeName(node.Type)))
	m.graphTitleAndStatus(c, p, node)
	c.end()
}

// graphLinkRow prints one link under its node: "  ├─ ⛔ waits for  CPU#4  title  [OPEN]".
func (m *model) graphLinkRow(c *canvas, p palette, last bool, link graphLink, pad int) {
	branch := "├─ "
	if last {
		branch = "└─ "
	}
	c.space(2)
	c.raw(p.border().Render(branch))
	label := graphKindMarker(link.kind) + " " + link.phrase
	if gap := pad - lipgloss.Width(link.phrase); gap > 0 {
		label += strings.Repeat(" ", gap)
	}
	switch link.kind {
	case linkBlocking:
		c.raw(p.danger().Render(label))
	case linkWorkflow:
		c.raw(p.accent().Render(label))
	default:
		c.raw(p.muted().Render(label))
	}
	c.space(2)
	if link.known {
		c.button(graphNodeRef(link.other), "graph-node:"+link.other.ID, false)
		m.graphTitleAndStatus(c, p, link.other)
	} else {
		c.text(link.rawID + " (outside this view)")
	}
	c.end()
}

func (m *model) graphTitleAndStatus(c *canvas, p palette, node domain.GraphNode) {
	badge := ""
	if node.Status != "" {
		badge = p.badge(statusTone(node.Status, strings.Contains(strings.ToLower(node.Status), "block")), node.Status)
	}
	room := c.width - c.x - lipgloss.Width(badge) - 3
	if title := graphNodeTitle(node); title != "" && room >= 6 {
		c.space(2)
		c.text(truncateVisible(title, room))
	}
	if badge != "" {
		c.space(1)
		c.raw(badge)
	}
}
