package tui

import (
	"idios/internal/diagnostics"
)

func logFilePath() string { return diagnostics.Path() }

func (m *model) logEvent(level, message string) {
	diagnostics.Write(level, message, m.config.Debug)
}

func readRecentLogs(limit int) []string { return diagnostics.ReadRecent(limit) }
