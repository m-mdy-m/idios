package application

import (
	"path/filepath"
	"testing"
)

func TestExpandShelfHomePathUsesPlatformSeparators(t *testing.T) {
	home := filepath.Join("root", "home", "reader")
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "home only", path: "~", want: home},
		{name: "slash separator", path: "~/Books", want: filepath.Join(home, "Books")},
		{name: "backslash separator", path: `~\Books\Shelf`, want: filepath.Join(home, "Books", "Shelf")},
		{name: "other user notation stays untouched", path: "~reader/Books", want: "~reader/Books"},
		{name: "ordinary path stays untouched", path: filepath.Join("tmp", "Books"), want: filepath.Join("tmp", "Books")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandShelfHomePath(tt.path, home); got != tt.want {
				t.Fatalf("expandShelfHomePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
