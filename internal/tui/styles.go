package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// palette is presentation-only. No styling decision is visible to the Application Core.
//
// With NO_COLOR every style drops its attributes: the screen then contains no escape
// sequences at all, so its text stays greppable, its measured width is its real width, and
// nothing depends on colour or bold to be understood. Active controls are marked with a rune
// instead (see canvas.button).
type palette struct{ plain bool }

// style combines the only two attributes the palette uses. It is the single place where
// plain mode strips styling, so no call site can accidentally reintroduce an escape sequence.
func (p palette) style(bold bool, hex string) lipgloss.Style {
	if p.plain {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(bold)
}

func (p palette) brand() lipgloss.Style    { return p.style(true, "#bc8cff") }
func (p palette) title() lipgloss.Style    { return p.style(true, "#e6edf3") }
func (p palette) subtitle() lipgloss.Style { return p.style(false, "#8b949e") }
func (p palette) muted() lipgloss.Style    { return p.style(false, "#8b949e") }
func (p palette) accent() lipgloss.Style   { return p.style(false, "#d2a8ff") }
func (p palette) success() lipgloss.Style  { return p.style(true, "#3fb950") }
func (p palette) warning() lipgloss.Style  { return p.style(true, "#d29922") }
func (p palette) danger() lipgloss.Style   { return p.style(true, "#ff7b72") }
func (p palette) border() lipgloss.Style   { return p.style(false, "#3d2f50") }

func (p palette) button(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 1)

	if p.plain {
		return style
	}

	if active {
		return style.
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#8957e5")).
			Bold(true)
	}

	return style.
		Foreground(lipgloss.Color("#c9d1d9")).
		Background(lipgloss.Color("#21262d"))
}

func (p palette) input(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 1)

	if p.plain {
		return style
	}

	if active {
		return style.
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#302044")).
			Bold(true)
	}
	return style.
		Foreground(lipgloss.Color("#c9d1d9")).
		Background(lipgloss.Color("#161b22"))
}

func (p palette) panel(width int) lipgloss.Style {
	style := lipgloss.NewStyle().Width(max(1, width-2)).Padding(0, 1)

	if p.plain {
		return style.Border(lipgloss.NormalBorder())
	}

	return style.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3d2f50"))
}

func (p palette) badge(kind, text string) string {
	text = " " + strings.ToUpper(strings.ReplaceAll(text, "_", " ")) + " "

	if p.plain {
		return "[" + strings.TrimSpace(text) + "]"
	}

	s := lipgloss.NewStyle().Bold(true).Padding(0, 1)

	switch kind {
	case "success":
		s = s.
			Foreground(lipgloss.Color("#7ee787")).
			Background(lipgloss.Color("#173b2a"))
	case "warning":
		s = s.
			Foreground(lipgloss.Color("#e3b341")).
			Background(lipgloss.Color("#3d3e00"))
	case "danger":
		s = s.
			Foreground(lipgloss.Color("#ff7b72")).
			Background(lipgloss.Color("#3d1f24"))
	default:
		s = s.
			Foreground(lipgloss.Color("#d2a8ff")).
			Background(lipgloss.Color("#302044"))
	}

	return s.Render(strings.TrimSpace(text))
}
