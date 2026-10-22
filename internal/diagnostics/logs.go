// Package diagnostics provides the shared local log stream used by the TUI and Web clients.
package diagnostics

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"idios/internal/config"
)

const legacyLogName = "idios-tui.log"
const sharedLogName = "idios.log"

var logMu sync.Mutex

// Path returns the location of the shared, user-local IDIOS log file.
func Path() string {
	return filepath.Join(filepath.Dir(config.AppConfigPath()), "logs", sharedLogName)
}

func legacyPath() string {
	return filepath.Join(filepath.Dir(config.AppConfigPath()), "logs", legacyLogName)
}

// FileStats describes the local diagnostic file without reading its contents.
type FileStats struct {
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at,omitempty"`
}

// Stats reports the current log file's size and modification time. A missing log is a valid
// first-run state and is returned with size zero.
func Stats() FileStats {
	path := Path()
	result := FileStats{Path: path}
	info, err := os.Stat(path)
	if err == nil {
		result.SizeBytes = info.Size()
		result.ModifiedAt = info.ModTime().UTC()
	}
	return result
}

// Clear truncates the shared log while preserving its private permissions and directory.
// It also migrates a legacy TUI log first so a clear action cannot leave old history behind.
func Clear() error {
	logMu.Lock()
	defer logMu.Unlock()
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create local log directory: %w", err)
	}
	migrateLegacy(path)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("clear local log file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure cleared log file permissions: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close cleared log file: %w", err)
	}
	if err := os.Remove(legacyPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy local log file: %w", err)
	}
	return nil
}

// Write appends one sanitized log event. DEBUG is recorded only when debugEnabled is true.
func Write(level, message string, debugEnabled bool) {
	logMu.Lock()
	defer logMu.Unlock()
	level = strings.ToUpper(strings.TrimSpace(level))
	if level == "" {
		level = "INFO"
	}
	if level == "DEBUG" && !debugEnabled {
		return
	}
	message = strings.NewReplacer("\r", " ", "\n", " ").Replace(message)
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	migrateLegacy(path)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s %-5s %s\n", time.Now().Format(time.RFC3339), level, message)
}

// ReadRecent returns the newest log lines, retaining the legacy TUI log if no shared log
// has been created yet. Missing or unreadable logs become useful empty-state messages.
func ReadRecent(limit int) []string {
	logMu.Lock()
	defer logMu.Unlock()
	if limit < 1 {
		limit = 60
	}
	path := Path()
	migrateLegacy(path)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"No log file yet. Enable Debug and perform an action to start collecting UI diagnostics."}
		}
		return []string{"Unable to read the local log file: " + err.Error()}
	}
	defer file.Close()

	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return append(lines, "Unable to read the rest of the log: "+err.Error())
	}
	if len(lines) == 0 {
		return []string{"The log file is empty."}
	}
	return lines
}

func migrateLegacy(path string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	legacy := legacyPath()
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	_ = os.Rename(legacy, path)
}
