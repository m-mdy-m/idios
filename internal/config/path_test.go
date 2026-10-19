package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDemoDBPathDoesNotUseStandardDBOverride(t *testing.T) {
	t.Setenv("IDIOS_DB", filepath.Join(t.TempDir(), "real-workspace.sqlite3"))
	t.Setenv("IDIOS_DEMO_DB", "")

	got := filepath.ToSlash(DefaultDemoDBPath())
	if strings.HasSuffix(got, "real-workspace.sqlite3") {
		t.Fatalf("demo database reused IDIOS_DB path: %q", got)
	}
	if !strings.HasSuffix(got, ".idios/demo/idios-demo.sqlite3") {
		t.Fatalf("unexpected default demo database path: %q", got)
	}
}

func TestDefaultDemoDBPathHonorsDedicatedOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom-demo.sqlite3")
	t.Setenv("IDIOS_DB", filepath.Join(t.TempDir(), "real.sqlite3"))
	t.Setenv("IDIOS_DEMO_DB", want)
	if got := DefaultDemoDBPath(); got != want {
		t.Fatalf("DefaultDemoDBPath() = %q, want %q", got, want)
	}
}
