package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"idios/internal/updater"
)

func (m *model) renderUpgrade(c *canvas, p palette) {
	if m.upgradeInstalling {
		frames := []string{"◴", "◷", "◶", "◵"}
		if m.upgradeHandedOff {
			c.title("✦ IDIOS UPGRADE")
			c.muted(fmt.Sprintf("Installing v%s · restarting into the new build…", m.availableUpdate.Version))
			c.blank()
			c.panel("REPLACING IDIOS", frames[m.upgradeFrame%len(frames)]+"  Preparing the updated CLI and terminal workspace.", "The new files have been verified and staged. IDIOS will relaunch automatically.")
			c.lineText(p.success().Render("✓ Update staged successfully"))
			return
		}
		c.title("✦ IDIOS UPGRADE")
		c.muted(fmt.Sprintf("v%s  →  v%s", updater.Version, m.availableUpdate.Version))
		c.blank()
		c.panel("UPGRADING IDIOS", frames[m.upgradeFrame%len(frames)]+"  Connecting to GitHub Releases", "Checking SHA-256 integrity and preparing both executables.", "Your current installation stays in place until verification succeeds.")
		barWidth := min(28, max(12, c.width-12))
		position := m.upgradeFrame % (barWidth + 8)
		start := max(0, position-8)
		end := min(barWidth, position)
		var bar strings.Builder
		for i := 0; i < barWidth; i++ {
			if i >= start && i < end {
				bar.WriteString(p.brand().Render("━"))
			} else {
				bar.WriteString("░")
			}
		}
		c.lineText("  " + bar.String())
		c.muted(fmt.Sprintf("Elapsed %s · Please keep this terminal open", time.Since(m.upgradeStartedAt).Round(100*time.Millisecond)))
		return
	}

	if m.availableUpdate == nil {
		c.title("IDIOS UPDATES")
		c.muted("No release information is available right now.")
		c.buttonRow(buttonSpec{"Back", "upgrade:later", true})
		return
	}

	c.title("✦ A NEW IDIOS IS READY")
	c.muted(fmt.Sprintf("Installed v%s   ·   Latest v%s", updater.Version, m.availableUpdate.Version))
	c.muted("Your workspace database and settings are kept; only the application binaries are replaced.")
	if m.upgradeError != "" {
		c.blank()
		c.panel("UPGRADE NEEDS ATTENTION", m.upgradeError, "For system-wide installs, the install directory may require administrator permission. Use a per-user install for automatic in-app updates.")
	}
	c.blank()
	c.title("CHANGELOG · " + m.availableUpdate.Version)
	lines := wrapReleaseNotes(m.availableUpdate.Notes, max(20, c.width-4))
	if len(lines) == 0 {
		lines = []string{"No changelog was published for this release."}
	}
	c.block(strings.Join(lines, "\n"))
	c.blank()
	c.buttonRow(
		buttonSpec{fmt.Sprintf("⬆ Upgrade to v%s", m.availableUpdate.Version), "upgrade:start", true},
		buttonSpec{"Later", "upgrade:later", false},
	)
	if m.availableUpdate.URL != "" {
		c.muted("Release page: " + m.availableUpdate.URL)
	}
}

func wrapReleaseNotes(notes string, width int) []string {
	var result []string
	for _, original := range strings.Split(strings.TrimSpace(notes), "\n") {
		line := strings.TrimSpace(original)
		if line == "" {
			result = append(result, "")
			continue
		}
		prefix := ""
		if strings.HasPrefix(line, "### ") {
			prefix = "▸ "
			line = strings.TrimSpace(strings.TrimPrefix(line, "### "))
		} else if strings.HasPrefix(line, "## ") {
			prefix = "▸ "
			line = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			prefix = "• "
			line = strings.TrimSpace(line[2:])
		}
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}
		current := prefix + words[0]
		continuationIndent := "  "
		for _, word := range words[1:] {
			candidate := current + " " + word
			if lipgloss.Width(candidate) > width {
				result = append(result, current)
				current = continuationIndent + word
			} else {
				current = candidate
			}
		}
		result = append(result, current)
	}
	return result
}
