package tui

import "strings"

// idiosASCIIAlphabet is a seven-row terminal wordmark. The strokes deliberately
// use plain ASCII so the logo stays aligned in Windows Terminal, PowerShell,
// cmd.exe, SSH sessions and terminals with no Unicode box-drawing support.
var idiosASCIIAlphabet = map[rune][]string{
	'I': {
		"#####",
		"  #  ",
		"  #  ",
		"  #  ",
		"  #  ",
		"  #  ",
		"#####",
	},
	'D': {
		"#### ",
		"#   #",
		"#   #",
		"#   #",
		"#   #",
		"#   #",
		"#### ",
	},
	'O': {
		" ### ",
		"#   #",
		"#   #",
		"#   #",
		"#   #",
		"#   #",
		" ### ",
	},
	'S': {
		" ####",
		"#    ",
		"#    ",
		" ### ",
		"    #",
		"    #",
		"#### ",
	},
}

// idiosASCIILogo returns either a bold or compact version of the wordmark.
// Every row has an identical byte/display width and contains printable ASCII only.
func idiosASCIILogo(doubleWidth bool) []string {
	letters := []rune("IDIOS")
	rows := make([]string, 7)
	cell := "#"
	gap := "  "
	if doubleWidth {
		cell = "##"
		gap = "    "
	}

	for row := range rows {
		parts := make([]string, 0, len(letters))
		for _, letter := range letters {
			glyph := idiosASCIIAlphabet[letter][row]
			var rendered strings.Builder
			for _, pixel := range glyph {
				if pixel == '#' {
					rendered.WriteString(cell)
				} else if doubleWidth {
					rendered.WriteString("  ")
				} else {
					rendered.WriteByte(' ')
				}
			}
			parts = append(parts, rendered.String())
		}
		rows[row] = strings.Join(parts, gap)
	}
	return rows
}

// centerASCII centers plain, monospaced text without relying on terminal-specific
// Unicode width rules. It is used only with the ASCII logo and its separator lines.
func centerASCII(value string, width int) string {
	if width <= 0 || len(value) >= width {
		return value
	}
	return strings.Repeat(" ", (width-len(value))/2) + value
}
