package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"idios/internal/config"
)

func logFilePath() string {
	return filepath.Join(filepath.Dir(config.AppConfigPath()), "logs", "idios-tui.log")
}

func (m *model) logEvent(level, message string) {
	if level == "DEBUG" && !m.config.Debug {
		return
	}
	path := logFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf("%s %-5s %s\n", time.Now().Format(time.RFC3339), level, strings.ReplaceAll(message, "\n", " "))
	_, _ = f.WriteString(line)
}

func readRecentLogs(limit int) []string {
	f, err := os.Open(logFilePath())
	if err != nil {
		return []string{"No log file yet. Enable Debug and perform an action to start collecting UI logs."}
	}
	defer f.Close()
	if limit < 1 {
		limit = 60
	}
	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(f)
	// Long error details should not make the entire log page disappear.
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if len(lines) == 0 {
		return []string{"The log file is empty."}
	}
	return lines
}
