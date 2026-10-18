package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"idios/internal/domain"
)

// cursorItem is the stable keyboard-selection representation used by list pages.
type cursorItem struct {
	label  string
	action string
}

func issueRef(issue domain.Issue) string {
	return fmt.Sprintf("%s#%d", issue.RepositoryName, issue.Number)
}

func prRef(pr domain.PullRequest) string {
	return fmt.Sprintf("%s!%d", pr.RepositoryName, pr.Number)
}

func graphNodeRef(node domain.GraphNode) string {
	if node.Reference != "" {
		if _, value, ok := strings.Cut(node.Reference, ":"); ok {
			return value
		}
		return node.Reference
	}
	if node.ID != "" {
		return node.ID
	}
	return node.Label
}

func graphLabel(node domain.GraphNode) string {
	if value := strings.TrimSpace(node.Label); value != "" {
		return value
	}
	return graphNodeRef(node)
}

func fallback(value, defaultValue string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return defaultValue
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}

func sliceVisible(lines []string, start, count int) []string {
	if count <= 0 || start >= len(lines) {
		return []string{}
	}
	start = max(0, start)
	end := min(len(lines), start+count)
	return append([]string(nil), lines[start:end]...)
}

func sortedGraphNodes(nodes []domain.GraphNode) []domain.GraphNode {
	out := append([]domain.GraphNode(nil), nodes...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		left, right := graphNodeRef(out[i]), graphNodeRef(out[j])
		if left != right {
			return left < right
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func statusTone(status string, blocked bool) string {
	if blocked {
		return "danger"
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "accepted", "completed", "complete", "success":
		return "success"
	case "cancelled", "canceled", "closed", "failed", "blocked", "error":
		return "danger"
	case "open", "pending", "in_progress", "in progress", "draft", "running", "warning":
		return "warning"
	default:
		return "accent"
	}
}

func progressText(done, total int) string {
	if total <= 0 {
		return "0%"
	}
	percent := done * 100 / total
	percent = max(0, min(100, percent))
	return fmt.Sprintf("%d%%", percent)
}

// caretValue inserts a visible terminal cursor at a rune index without corrupting UTF-8.
func caretValue(value string, caret int, visible bool) string {
	if !visible {
		return value
	}
	runes := []rune(value)
	caret = max(0, min(len(runes), caret))
	return string(runes[:caret]) + "▏" + string(runes[caret:])
}

// multilineFieldLines returns a small editor viewport centered around the current caret.
// The returned lines contain no embedded newlines, so the canvas can attach a hitbox to
// each visible editor row without losing the logical field identity.
func multilineFieldLines(value string, caret int, active bool, placeholder string, maxVisible int) []string {
	if maxVisible < 1 {
		maxVisible = 1
	}
	if value == "" {
		if active {
			return []string{caretValue("", 0, true) + " " + placeholder}
		}
		return []string{placeholder}
	}
	runes := []rune(value)
	caret = max(0, min(len(runes), caret))
	caretLine, caretColumn := 0, 0
	for i := 0; i < caret; i++ {
		if runes[i] == '\n' {
			caretLine++
			caretColumn = 0
		} else {
			caretColumn++
		}
	}
	lines := strings.Split(value, "\n")
	first := max(0, caretLine-maxVisible/2)
	first = min(first, max(0, len(lines)-maxVisible))
	last := min(len(lines), first+maxVisible)
	out := make([]string, 0, maxVisible+2)
	if first > 0 {
		out = append(out, fmt.Sprintf("… %d lines above …", first))
	}
	for i := first; i < last; i++ {
		line := lines[i]
		if active && i == caretLine {
			line = caretValue(line, caretColumn, true)
		}
		if line == "" {
			line = " "
		}
		out = append(out, line)
	}
	if last < len(lines) {
		out = append(out, fmt.Sprintf("… %d lines below …", len(lines)-last))
	}
	return out
}

// multilineLineBoundary returns the start or end rune index of the line that contains caret.
func multilineLineBoundary(value string, caret int, end bool) int {
	runes := []rune(value)
	caret = max(0, min(len(runes), caret))
	start := 0
	for i := 0; i < caret; i++ {
		if runes[i] == '\n' {
			start = i + 1
		}
	}
	if !end {
		return start
	}
	for i := caret; i < len(runes); i++ {
		if runes[i] == '\n' {
			return i
		}
	}
	return len(runes)
}

// moveMultilineCaret moves vertically while preserving the caret's column where possible.
// It returns unchanged when the requested line does not exist, allowing the caller to move
// focus to the adjacent form field instead.
func moveMultilineCaret(value string, caret, direction int) (int, bool) {
	runes := []rune(value)
	caret = max(0, min(len(runes), caret))
	lineStarts := []int{0}
	for i, r := range runes {
		if r == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	line := 0
	for i := 1; i < len(lineStarts); i++ {
		if caret >= lineStarts[i] {
			line = i
		} else {
			break
		}
	}
	column := caret - lineStarts[line]
	target := line + direction
	if target < 0 || target >= len(lineStarts) {
		return caret, false
	}
	end := len(runes)
	if target+1 < len(lineStarts) {
		end = lineStarts[target+1] - 1
	}
	lineLength := max(0, end-lineStarts[target])
	return lineStarts[target] + min(column, lineLength), true
}

func insertAtRune(value string, at int, insertion string) string {
	runes := []rune(value)
	at = max(0, min(len(runes), at))
	result := make([]rune, 0, len(runes)+utf8.RuneCountInString(insertion))
	result = append(result, runes[:at]...)
	result = append(result, []rune(insertion)...)
	result = append(result, runes[at:]...)
	return string(result)
}

func deleteRuneBefore(value string, at int) (string, int) {
	runes := []rune(value)
	at = max(0, min(len(runes), at))
	if at == 0 {
		return value, 0
	}
	runes = append(runes[:at-1], runes[at:]...)
	return string(runes), at - 1
}

func deleteRuneAt(value string, at int) string {
	runes := []rune(value)
	if at < 0 || at >= len(runes) {
		return value
	}
	runes = append(runes[:at], runes[at+1:]...)
	return string(runes)
}

func trimLastRune(value string) string {
	if value == "" {
		return value
	}
	_, size := utf8.DecodeLastRuneInString(value)
	return value[:len(value)-size]
}

func isPrintableKey(key string) bool {
	lower := strings.ToLower(key)
	if key == "" || (strings.Contains(key, "+") && key != "+") || strings.HasPrefix(lower, "ctrl") || strings.HasPrefix(lower, "alt") {
		return false
	}
	switch lower {
	case "enter", "esc", "escape", "tab", "backspace", "delete", "insert", "home", "end",
		"up", "down", "left", "right", "pageup", "pagedown", "pgup", "pgdown", "space":
		return false
	}
	if len(lower) > 1 && lower[0] == 'f' {
		allDigits := true
		for _, r := range lower[1:] {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return false
		}
	}
	for _, r := range key {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// truncateVisible truncates by terminal display columns, not UTF-8 byte count or rune
// count. It preserves ANSI control sequences already emitted by Lip Gloss and resets an
// open style after truncation so styling cannot leak into following terminal content.
func truncateVisible(s string, width int) string {
	if width <= 0 || s == "" {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}

	target := width - 1 // reserve one terminal column for the truncation marker
	var out strings.Builder
	visible := 0
	truncated := false
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			end := ansiSequenceEnd(s, i)
			// Control sequences occupy no terminal columns. Keep them while content is
			// being copied; the final reset below closes any partially copied style.
			out.WriteString(s[i:end])
			i = end
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		runeWidth := lipgloss.Width(string(r))
		if visible+runeWidth > target {
			truncated = true
			break
		}
		out.WriteString(s[i : i+size])
		visible += runeWidth
		i += size
	}
	if !truncated {
		// Display width may differ from the sum of rune widths for emoji clusters;
		// the original string already exceeded the target, so mark the truncation.
		truncated = true
	}
	if truncated {
		out.WriteRune('…')
	}
	if strings.ContainsRune(s, '\x1b') {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

// ansiSequenceEnd returns the first byte after an ANSI escape sequence. The common CSI
// sequences (colors and cursor attributes), OSC hyperlinks, and short ESC forms are covered.
func ansiSequenceEnd(s string, start int) int {
	if start+1 >= len(s) {
		return len(s)
	}
	switch s[start+1] {
	case '[': // CSI: ESC [ ... final-byte
		for i := start + 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return len(s)
	case ']': // OSC: terminated by BEL or ESC backslash
		for i := start + 2; i < len(s); i++ {
			if s[i] == '\x07' {
				return i + 1
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return len(s)
	default:
		return min(len(s), start+2)
	}
}

// fitLine returns a terminal line no wider than width, padding shorter lines to a stable
// width so the full-screen renderer does not shift at the right edge.
func fitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = truncateVisible(s, width)
	visible := lipgloss.Width(s)
	if visible < width {
		s += strings.Repeat(" ", width-visible)
	}
	return s
}

func renderPanel(width int, title string, lines []string, p palette) string {
	content := make([]string, 0, len(lines)+1)
	content = append(content, p.title().Render(truncateVisible(title, max(1, width-6))))
	content = append(content, lines...)
	return p.panel(max(1, width)).Render(strings.Join(content, "\n"))
}
