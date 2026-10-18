// Package logger is the single logging surface IDIOS uses for monitoring.
//
// Rules that keep the CLI contract intact:
//   - logs go to stderr only, never to stdout;
//   - a nil *Logger is a valid no-op logger, so callers never branch on "is logging on";
//   - monitoring is opt-in: the adapters stay silent unless a level is configured.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog so the rest of the code never touches logging configuration.
type Logger struct{ l *slog.Logger }

// ParseLevel maps a textual level to slog. An empty or unknown value selects LevelInfo.
func ParseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New writes text logs at or above level to w.
func New(w io.Writer, level string) *Logger {
	return &Logger{l: slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: ParseLevel(level)}))}
}

// FromEnv enables logging only when IDIOS_LOG_LEVEL is set, so normal runs stay quiet.
func FromEnv(w io.Writer) *Logger {
	name := strings.TrimSpace(os.Getenv("IDIOS_LOG_LEVEL"))
	if name == "" || strings.EqualFold(name, "off") || strings.EqualFold(name, "none") {
		return nil
	}
	return New(w, name)
}

// Discard returns the no-op logger.
func Discard() *Logger { return nil }

func (l *Logger) log(level slog.Level, msg string, kv ...any) {
	if l == nil || l.l == nil || !l.l.Enabled(nil, level) {
		return
	}
	l.l.Log(nil, level, msg, kv...)
}

func (l *Logger) Debug(msg string, kv ...any) { l.log(slog.LevelDebug, msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.log(slog.LevelInfo, msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.log(slog.LevelWarn, msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.log(slog.LevelError, msg, kv...) }

// Errorf logs an error value with a short message.
func (l *Logger) Errorf(err error, msg string, kv ...any) {
	if err == nil {
		return
	}
	l.log(slog.LevelError, msg, append(kv, "error", err.Error())...)
}

// With returns a logger that adds kv to every record, or the nil logger unchanged.
func (l *Logger) With(kv ...any) *Logger {
	if l == nil || l.l == nil {
		return l
	}
	return &Logger{l: l.l.With(kv...)}
}
