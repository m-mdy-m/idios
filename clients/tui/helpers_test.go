package tui

import (
	"strings"
	"testing"

	"idios/internal/domain"

	"charm.land/lipgloss/v2"
)

func TestTruncateVisibleUsesDisplayWidth(t *testing.T) {
	if got, want := truncateVisible("abcdefgh", 5), "abcd…"; got != want {
		t.Fatalf("truncateVisible() = %q, want %q", got, want)
	}
	if got := lipgloss.Width(truncateVisible("界界界", 4)); got > 4 || got == 0 {
		t.Fatalf("wide-text display width = %d, want a visible result no wider than 4", got)
	}
}

func TestTruncateVisibleClosesANSIStyles(t *testing.T) {
	styled := "\x1b[31mabcdef\x1b[0m"
	got := truncateVisible(styled, 5)
	if width := lipgloss.Width(got); width != 5 {
		t.Fatalf("styled text display width = %d, want 5 (%q)", width, got)
	}
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("truncated styled text should reset ANSI style: %q", got)
	}
}

func TestFitLinePadsOrTruncatesToExactWidth(t *testing.T) {
	for _, input := range []string{"ok", "too-long-for-terminal", "界面"} {
		if got := lipgloss.Width(fitLine(input, 8)); got != 8 {
			t.Errorf("fitLine(%q) display width = %d, want 8", input, got)
		}
	}
}

func TestGraphNodeReferenceUsesHumanReadableReference(t *testing.T) {
	node := domain.GraphNode{ID: "issue:some-id", Label: "CPU#4 · Learn electricity", Reference: "issue:CPU#4"}
	if got, want := graphNodeRef(node), "CPU#4"; got != want {
		t.Fatalf("graphNodeRef() = %q, want %q", got, want)
	}
	if got, want := graphLabel(node), node.Label; got != want {
		t.Fatalf("graphLabel() = %q, want %q", got, want)
	}
}

func TestRuneEditingHelpersPreserveUnicode(t *testing.T) {
	if got, want := insertAtRune("café", 3, "X"), "cafXé"; got != want {
		t.Fatalf("insertAtRune() = %q, want %q", got, want)
	}
	if got, caret := deleteRuneBefore("café", 4); got != "caf" || caret != 3 {
		t.Fatalf("deleteRuneBefore() = (%q, %d), want (%q, 3)", got, caret, "caf")
	}
	if got, want := deleteRuneAt("café", 3), "caf"; got != want {
		t.Fatalf("deleteRuneAt() = %q, want %q", got, want)
	}
	if got, want := trimLastRune("café"), "caf"; got != want {
		t.Fatalf("trimLastRune() = %q, want %q", got, want)
	}
	if got, want := caretValue("café", 3, true), "caf▏é"; got != want {
		t.Fatalf("caretValue() = %q, want %q", got, want)
	}
}

func TestGraphSortAndStatusHelpersAreStable(t *testing.T) {
	nodes := []domain.GraphNode{
		{ID: "2", Type: "issue", Reference: "issue:CPU#2"},
		{ID: "concept", Type: "concept", Reference: "concept:Energy"},
		{ID: "1", Type: "issue", Reference: "issue:CPU#1"},
	}
	got := sortedGraphNodes(nodes)
	if got[0].Type != "concept" || got[1].ID != "1" || got[2].ID != "2" {
		t.Fatalf("sortedGraphNodes() order = %#v", got)
	}
	if statusTone("done", false) != "success" || statusTone("open", true) != "danger" {
		t.Fatal("statusTone() should distinguish completed and blocked work")
	}
	if progressText(1, 4) != "25%" || progressText(0, 0) != "0%" {
		t.Fatal("progressText() returned an unexpected percentage")
	}
}

func TestPrintableKeyRejectsNamedControlsButAllowsPlus(t *testing.T) {
	if !isPrintableKey("+") || !isPrintableKey("é") {
		t.Fatal("printable characters should be accepted")
	}
	for _, key := range []string{"ctrl+q", "enter", "f1", "up"} {
		if isPrintableKey(key) {
			t.Errorf("control key %q was accepted as text", key)
		}
	}
}

func TestPrettyKey(t *testing.T) {
	cases := map[string]string{"ctrl+n": "Ctrl+N", "/": "/", "q": "Q", "ctrl+shift+q": "Ctrl+Shift+Q", "": "?"}
	for in, want := range cases {
		if got := prettyKey(in, "?"); got != want {
			t.Fatalf("prettyKey(%q) = %q, want %q", in, got, want)
		}
	}
}
