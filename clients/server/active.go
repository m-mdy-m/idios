package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"idios/internal/config"
)

// ActiveState describes the currently bound local listener. It lets CLI control commands reach
// the process even if the user has edited the desired host or port since it started.
type ActiveState struct {
	Address   string    `json:"address"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

func activeStatePath() string {
	return filepath.Join(config.ConfigDir(), "server-state.json")
}

// ActiveURL returns the last recorded listener URL. Callers must Probe it before treating the
// process as running because a crash or forced termination can leave a stale state file behind.
func ActiveURL() (string, bool) {
	data, err := os.ReadFile(activeStatePath())
	if err != nil {
		return "", false
	}
	var state ActiveState
	if err := json.Unmarshal(data, &state); err != nil || state.PID < 1 {
		return "", false
	}
	addr, err := validateLoopbackAddress(state.Address)
	if err != nil {
		return "", false
	}
	return "http://" + addr, true
}

// ProbeActive reports whether the currently recorded local server responds to its health check.
// It deliberately has a short timeout because it is also used by the background service scheduler.
func ProbeActive(parent context.Context) bool {
	base, ok := ActiveURL()
	if !ok {
		return false
	}
	ctx, cancel := context.WithTimeout(parent, 450*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/health", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 450 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func writeActiveState(address string) error {
	addr, err := validateLoopbackAddress(address)
	if err != nil {
		return err
	}
	path := activeStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create server state directory: %w", err)
	}
	data, err := json.MarshalIndent(ActiveState{Address: addr, PID: os.Getpid(), StartedAt: time.Now().UTC()}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode server state: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".idios-server-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary server state: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure server state file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write server state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close server state: %w", err)
	}
	// os.Rename does not replace an existing destination on every supported Windows version.
	// The listener is already bound, so no second IDIOS server can own the same address.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace old server state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish server state: %w", err)
	}
	return nil
}

func removeActiveState(address string) {
	path := activeStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state ActiveState
	if json.Unmarshal(data, &state) != nil || state.PID != os.Getpid() {
		return
	}
	addr, err := validateLoopbackAddress(address)
	if err != nil || state.Address != addr {
		return
	}
	_ = os.Remove(path)
}
