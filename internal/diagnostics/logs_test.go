package diagnostics

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSharedLogFiltersDebugAndReturnsNewestLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDIOS_CONFIG", filepath.Join(dir, "config", "config.json"))

	Write("DEBUG", "hidden debug", false)
	Write("INFO", "first line", false)
	Write("WARN", "second line", false)

	lines := ReadRecent(1)
	if len(lines) != 1 || !strings.Contains(lines[0], "second line") {
		t.Fatalf("ReadRecent(1) = %#v, want the newest line", lines)
	}
	all := ReadRecent(10)
	joined := strings.Join(all, "\n")
	if !strings.Contains(joined, "first line") || !strings.Contains(joined, "second line") {
		t.Fatalf("expected regular log records, got %q", joined)
	}
	if strings.Contains(joined, "hidden debug") {
		t.Fatalf("debug log should be filtered while debug is disabled: %q", joined)
	}
	if _, err := os.Stat(Path()); err != nil {
		t.Fatalf("shared log file was not created: %v", err)
	}
}

func TestLegacyTUILogMigratesToSharedPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDIOS_CONFIG", filepath.Join(dir, "idios", "config.json"))
	legacy := legacyPath()
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("2026-10-22T00:00:00Z INFO  legacy record\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	Write("INFO", "new record", false)
	lines := ReadRecent(10)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "legacy record") || !strings.Contains(joined, "new record") {
		t.Fatalf("legacy history was not preserved: %q", joined)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy log path still exists after migration: %v", err)
	}
}

func TestClearTruncatesHistoryAndKeepsPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDIOS_CONFIG", filepath.Join(dir, "idios", "config.json"))
	Write("INFO", "history to remove", false)
	if err := os.WriteFile(legacyPath(), []byte("legacy record that must also be removed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	info, err := os.Stat(Path())
	if err != nil {
		t.Fatalf("stat cleared log: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("cleared log size = %d, want 0", info.Size())
	}
	// POSIX file permissions are not emulated on Windows; keep asserting them on Unix/CI.
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("cleared log permissions = %o, want 600", got)
		}
	}
	if _, err := os.Stat(legacyPath()); !os.IsNotExist(err) {
		t.Fatalf("legacy log still exists after clear: %v", err)
	}
	if lines := ReadRecent(10); len(lines) != 1 || lines[0] != "The log file is empty." {
		t.Fatalf("ReadRecent after Clear() = %#v", lines)
	}
	Write("INFO", "fresh history", false)
	if lines := ReadRecent(10); len(lines) != 1 || !strings.Contains(lines[0], "fresh history") {
		t.Fatalf("new history did not record after Clear(): %#v", lines)
	}
}
