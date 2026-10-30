package server

import (
	"context"
	"errors"
	"fmt"
	"idios/internal/diagnostics"
	"idios/internal/updater"
	"net/http"
	"os"
	"strings"
	"time"
)

func (s *Server) currentVersion() string {
	if version := strings.TrimSpace(s.opts.Version); version != "" {
		return version
	}
	return updater.Version
}

// checkUpdate uses the same release and semantic-version logic as the CLI updater.
func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	release, err := updater.Check(requestContext(r), s.currentVersion())
	if err != nil {
		writeError(w, fmt.Errorf("check for IDIOS updates: %w", err))
		return
	}
	response := map[string]any{"current_version": s.currentVersion(), "update_available": release != nil}
	if release != nil {
		response["release"] = updateReleaseResponse{Version: release.Version, Tag: release.Tag, URL: release.URL, Notes: release.Notes}
	}
	writeJSON(w, http.StatusOK, response)
}

// getUpdateStatus exposes the local update workflow state so a Web client can observe
// verification and restart without holding an HTTP request open during downloads.
func (s *Server) getUpdateStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	state, detail, version := s.updateState, s.updateError, s.updateVersion
	s.mu.RUnlock()
	if state == "" {
		state = "idle"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": state, "error": detail, "version": version})
}

// installUpdate starts the same verified release updater as CLI. Work continues in a Go
// goroutine so the browser gets an immediate acknowledgement instead of timing out while a
// large release archive is downloaded. The process restarts in web mode after staging.
func (s *Server) installUpdate(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	if s.updateState == "checking" || s.updateState == "downloading" || s.updateState == "scheduled" {
		state, version := s.updateState, s.updateVersion
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "an IDIOS update is already " + state, "status": state, "version": version})
		return
	}
	s.updateState, s.updateError, s.updateVersion = "checking", "", ""
	s.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "checking"})
	go s.prepareAndScheduleUpdate()
}

func (s *Server) setUpdateStatus(state, version, detail string) {
	s.mu.Lock()
	s.updateState, s.updateVersion, s.updateError = state, version, detail
	s.mu.Unlock()
}

func (s *Server) prepareAndScheduleUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	fail := func(err error) {
		detail := "prepare IDIOS update: " + err.Error()
		s.setUpdateStatus("failed", "", detail)
		diagnostics.Write("ERROR", detail, s.debugEnabled())
	}
	release, err := updater.Check(ctx, s.currentVersion())
	if err != nil {
		fail(err)
		return
	}
	if release == nil {
		fail(errors.New("no newer stable release is available"))
		return
	}
	s.setUpdateStatus("downloading", release.Version, "")
	executable, err := os.Executable()
	if err != nil {
		fail(fmt.Errorf("locate installed IDIOS executable: %w", err))
		return
	}
	prepared, err := updater.Prepare(ctx, *release, executable)
	if err != nil {
		fail(err)
		return
	}
	if err := updater.ScheduleWithArgs(prepared, os.Getpid(), []string{"web", "--no-open"}); err != nil {
		prepared.Cleanup()
		fail(fmt.Errorf("schedule replacement binaries: %w", err))
		return
	}
	s.setUpdateStatus("scheduled", release.Version, "")
	diagnostics.Write("INFO", "verified IDIOS update scheduled for version "+release.Version, s.debugEnabled())
	s.stopOnce.Do(func() {
		go func() {
			time.Sleep(120 * time.Millisecond)
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelShutdown()
			_ = s.http.Shutdown(shutdownCtx)
		}()
	})
}
