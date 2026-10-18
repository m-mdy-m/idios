package config

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultDBPath resolves the Engine database override or the user's local-first default.
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
