package desktopnotify

import (
	"strings"
	"testing"
)

func TestCompactCollapsesWhitespaceAndTruncatesUnicode(t *testing.T) {
	if got := compact("  hello\n\tworld  ", 20); got != "hello world" {
		t.Fatalf("compact whitespace = %q", got)
	}
	got := compact(strings.Repeat("ک", 5), 4)
	if got != "ککک…" {
		t.Fatalf("compact unicode truncation = %q, want %q", got, "ککک…")
	}
}

func TestPowerShellEscapingDoublesSingleQuotes(t *testing.T) {
	if got := escapePowerShell("don't 'break'"); got != "don''t ''break''" {
		t.Fatalf("PowerShell escaping = %q", got)
	}
}

func TestUTF16LEEncoding(t *testing.T) {
	got := utf16LE("A")
	if len(got) != 2 || got[0] != 0x41 || got[1] != 0x00 {
		t.Fatalf("UTF-16LE encoding = %v, want [65 0]", got)
	}
}
