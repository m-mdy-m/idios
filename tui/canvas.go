package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type canvas struct {
	width, x, y      int
	originX, originY int
	lines            []string
	line             string
	hits             *[]hitbox
	style            palette
	focusAction      string
	tabMode          bool
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

func (c *canvas) text(s string) {
	s = copyText.text(s)
	c.raw(truncateVisible(s, max(0, c.width-c.x)))
}

func (c *canvas) space(n int) {
	if n > 0 {
		c.raw(strings.Repeat(" ", min(n, max(0, c.width-c.x))))
	}
}

func (c *canvas) button(label, action string, active bool) {
	label = copyText.text(label)
	active = active || (c.focusAction != "" && c.focusAction == action)
	maxLabel := max(1, c.width-4)
	if active && c.style.plain {
		// NO_COLOR keeps escape sequences out of the screen, so the current control is
		// marked with a rune instead of bold or an underline.
		label = "▸ " + label
	}
	if lipgloss.Width(label)+4 > c.width {
		label = truncateVisible(label, maxLabel)
	}
	style := c.style.button(active)
	if c.tabMode {
		style = c.style.tab(active)
	}
	styled := style.Render(label)
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
	active = active || (c.focusAction != "" && c.focusAction == action)
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
		item.label = copyText.text(item.label)
		active := item.active || (c.focusAction != "" && c.focusAction == item.action)
		styled := c.style.button(active).Render(item.label)
		w := lipgloss.Width(styled)
		if w > c.width {
			item.label = truncateVisible(item.label, max(1, c.width-4))
			w = lipgloss.Width(c.style.button(active).Render(item.label))
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

// tabRow renders navigation items as tabs: the current page is filled, the others stay quiet.
func (c *canvas) tabRow(items ...buttonSpec) {
	c.tabMode = true
	c.buttonRow(items...)
	c.tabMode = false
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

func (c *canvas) blank() {
	if c.style.density == "compact" {
		return
	}
	c.end()
}

func (c *canvas) rule() { c.lineText(c.style.border().Render(strings.Repeat("─", c.width))) }

func (c *canvas) title(value string) { c.lineText(c.style.title().Render(copyText.text(value))) }

func (c *canvas) muted(value string) { c.lineText(c.style.muted().Render(copyText.text(value))) }

func (c *canvas) panel(title string, lines ...string) {
	title = copyText.text(title)
	for i := range lines {
		lines[i] = copyText.text(lines[i])
	}
	c.block(renderPanel(c.width, title, lines, c.style))
}
