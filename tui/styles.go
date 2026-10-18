package tui

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"idios/internal/config"
)

// palette is a data-driven presentation adapter. Every color resolves from the selected
// preset/user override in ~/.idios/config.json; page renderers never carry hex codes.
type palette struct {
	plain   bool
	colors  map[string]string
	density string
}

func (p palette) color(key string) string {
	if value := strings.TrimSpace(p.colors[key]); value != "" {
		return value
	}
	// "default" delegates to the terminal palette. Actual app colors belong in defaults.json.
	return "default"
}

func (p palette) style(bold bool, key, fallback string) lipgloss.Style {
	if p.plain {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(p.color(key))).Bold(bold)
}

func (p palette) brand() lipgloss.Style    { return p.style(true, "accent", "default") }
func (p palette) title() lipgloss.Style    { return p.style(true, "foreground", "default") }
func (p palette) subtitle() lipgloss.Style { return p.style(false, "muted", "default") }
func (p palette) muted() lipgloss.Style    { return p.style(false, "muted", "default") }
func (p palette) accent() lipgloss.Style   { return p.style(false, "accent", "default") }
func (p palette) success() lipgloss.Style  { return p.style(true, "success", "default") }
func (p palette) warning() lipgloss.Style  { return p.style(true, "warning", "default") }
func (p palette) danger() lipgloss.Style   { return p.style(true, "danger", "default") }
func (p palette) border() lipgloss.Style   { return p.style(false, "border", "default") }

func (p palette) button(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 1)
	if p.plain {
		return style
	}
	if active {
		return style.Foreground(lipgloss.Color(p.color("foreground"))).Background(lipgloss.Color(p.color("accent_background"))).Bold(true)
	}
	return style.Foreground(lipgloss.Color(p.color("foreground"))).Background(lipgloss.Color(p.color("surface_alt")))
}

func (p palette) input(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 1)
	if p.plain {
		return style
	}
	if active {
		return style.Foreground(lipgloss.Color(p.color("foreground"))).Background(lipgloss.Color(p.color("accent_background"))).Bold(true)
	}
	return style.Foreground(lipgloss.Color(p.color("foreground"))).Background(lipgloss.Color(p.color("surface")))
}

func (p palette) panel(width int) lipgloss.Style {
	style := lipgloss.NewStyle().Width(max(1, width-4)).Padding(0, 1)
	if p.plain {
		return style.Border(lipgloss.NormalBorder())
	}
	return style.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(p.color("border")))
}

func (p palette) badge(kind, text string) string {
	text = " " + strings.ToUpper(strings.ReplaceAll(text, "_", " ")) + " "
	if p.plain {
		return "[" + strings.TrimSpace(text) + "]"
	}
	foreground, background := "accent", "surface_alt"
	switch kind {
	case "success":
		foreground, background = "success", "success_background"
	case "warning":
		foreground, background = "warning", "warning_background"
	case "danger":
		foreground, background = "danger", "danger_background"
	}
	s := lipgloss.NewStyle().Bold(true).Padding(0, 1).
		Foreground(lipgloss.Color(p.color(foreground))).
		Background(lipgloss.Color(p.color(background)))
	return s.Render(strings.TrimSpace(text))
}

// themeNames is kept sorted so the appearance picker and docs show a stable order.
func themeNames(cfg config.AppConfig) []string {
	out := make([]string, 0, len(cfg.Themes))
	for name := range cfg.Themes {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
