package config

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultDBPath resolves the Engine database override or the default path in the user's home directory.
func DefaultDBPath() string {
	if v := strings.TrimSpace(os.Getenv("IDIOS_DB")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".idios", "idios-engine.sqlite3")
	}
	return filepath.Join(home, ".idios", "idios-engine.sqlite3")
}

// DefaultDemoDBPath resolves the demo-only database override or a separate local default.
// It intentionally ignores IDIOS_DB so launching the demo app cannot seed the user's normal DB.
func DefaultDemoDBPath() string {
	if v := strings.TrimSpace(os.Getenv("IDIOS_DEMO_DB")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".idios", "demo", "idios-demo.sqlite3")
	}
	return filepath.Join(home, ".idios", "demo", "idios-demo.sqlite3")
}
