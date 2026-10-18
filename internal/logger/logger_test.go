package logger

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// Callers never branch on whether monitoring is on, so a nil logger must swallow everything.
func TestNilLoggerIsNoOp(t *testing.T) {
	var l *Logger
	l.Debug("dropped")
	l.Info("dropped", "k", "v")
	l.Warn("dropped")
	l.Error("dropped")
	l.Errorf(nil, "dropped")
	l.Errorf(errors.New("boom"), "dropped")
	if got := l.With("k", "v"); got != nil {
		t.Fatalf("With on nil logger = %v, want nil", got)
	}
}

func TestLevelFiltersRecords(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "warn")
	l.Debug("quiet")
	l.Info("quiet")
	l.Warn("kept")
	if out := buf.String(); !strings.Contains(out, "kept") || strings.Contains(out, "quiet") {
		t.Fatalf("level filter output = %q", out)
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":    slog.LevelDebug,
		"WARN":     slog.LevelWarn,
		"warning":  slog.LevelWarn,
		"error":    slog.LevelError,
		"":         slog.LevelInfo,
		"nonsense": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFromEnvIsOffUnlessSet(t *testing.T) {
	for _, off := range []string{"", "off", "none"} {
		t.Setenv("IDIOS_LOG_LEVEL", off)
		if FromEnv(&bytes.Buffer{}) != nil {
			t.Fatalf("logging should be off for IDIOS_LOG_LEVEL=%q", off)
		}
	}
	t.Setenv("IDIOS_LOG_LEVEL", "debug")
	if FromEnv(&bytes.Buffer{}) == nil {
		t.Fatal("logging should be on for IDIOS_LOG_LEVEL=debug")
	}
}
