package tui

import (
	"strings"
	"testing"
)

func TestIDIOSASCIILogoIsConsistentAndPrintable(t *testing.T) {
	for _, tc := range []struct {
		name      string
		double    bool
		wantWidth int
	}{
		{name: "compact", wantWidth: 33},
		{name: "bold", double: true, wantWidth: 66},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logo := idiosASCIILogo(tc.double)
			if len(logo) != 7 {
				t.Fatalf("logo has %d rows, want 7", len(logo))
			}
			for row, line := range logo {
				if len(line) != tc.wantWidth {
					t.Errorf("row %d width = %d, want %d: %q", row, len(line), tc.wantWidth, line)
				}
				for _, r := range line {
					if r < 0x20 || r > 0x7e {
						t.Errorf("row %d contains non-printable/non-ASCII rune %q", row, r)
					}
				}
			}
			joined := strings.Join(logo, "\n")
			for _, letter := range []string{"#", " ", "\n"} {
				if !strings.Contains(joined, letter) {
					t.Errorf("logo does not contain expected character %q", letter)
				}
			}
		})
	}
}

func TestCenterASCIIDoesNotExceedRequestedWidth(t *testing.T) {
	for _, tc := range []struct {
		value string
		width int
	}{
		{value: "IDIOS", width: 20},
		{value: "PERSONAL WORK GRAPH", width: 48},
		{value: "already wider", width: 4},
	} {
		got := centerASCII(tc.value, tc.width)
		if len(got) < len(tc.value) {
			t.Fatalf("centerASCII truncated %q to %q", tc.value, got)
		}
		if len(tc.value) < tc.width && len(got) > tc.width {
			t.Errorf("centerASCII width = %d, want <= %d", len(got), tc.width)
		}
	}
}
