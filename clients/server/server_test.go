package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"idios/internal/application"
	"idios/internal/config"
	"idios/internal/diagnostics"
	"idios/internal/domain"
	"idios/internal/storage/sqlite"
)

func TestValidateWebThemeConfigAcceptsBuiltInAndCustomThemes(t *testing.T) {
	builtinNames := []string{"system", "dark", "light", "midnight", "forest", "violet", "rose", "oled"}
	for _, name := range builtinNames {
		if got := validateWebThemeConfig(config.WebConfig{Theme: name}); got != "" {
			t.Errorf("built-in theme %q rejected: %s", name, got)
		}
	}

	custom := config.CustomWebTheme{
		Name: "Aurora Drift", Base: "dark", Background: "#0b1220", Surface: "#111d30",
		Text: "#e8f0ff", Muted: "#91a2be", Accent: "#8bd7f0", Border: "#2b3a55",
		Success: "#65d6a7", Warning: "#f3c66d", Danger: "#ff788e",
	}
	web := config.WebConfig{Theme: "aurora-drift", CustomThemes: map[string]config.CustomWebTheme{"aurora-drift": custom}}
	if got := validateWebThemeConfig(web); got != "" {
		t.Fatalf("valid custom theme rejected: %s", got)
	}
	web.Theme = "missing-theme"
	if got := validateWebThemeConfig(web); !strings.Contains(got, "saved custom theme") {
		t.Fatalf("missing custom theme error = %q, want actionable theme selection error", got)
	}
	web.Theme = "aurora-drift"
	custom.Accent = "not-a-color"
	web.CustomThemes["aurora-drift"] = custom
	if got := validateWebThemeConfig(web); !strings.Contains(got, "accent must be a six-digit hex color") {
		t.Fatalf("invalid custom color error = %q", got)
	}
	web.CustomThemes = map[string]config.CustomWebTheme{"Invalid Theme Key": custom}
	if got := validateWebThemeConfig(web); !strings.Contains(got, "lowercase letters") {
		t.Fatalf("invalid custom theme key error = %q", got)
	}
}

func TestFeatureCatalogEndpointPublishesSharedClientFeatures(t *testing.T) {
	s := New(nil, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}})
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:43170/api/features", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/features status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		CatalogVersion int `json:"catalog_version"`
		Features       []struct {
			ID            string `json:"id"`
			WebPath       string `json:"web_path"`
			TUIPage       string `json:"tui_page"`
			WebNavigation bool   `json:"web_navigation"`
		} `json:"features"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode feature catalog: %v", err)
	}
	if payload.CatalogVersion != 1 || len(payload.Features) == 0 {
		t.Fatalf("unexpected feature catalog header: version=%d features=%d", payload.CatalogVersion, len(payload.Features))
	}
	found := map[string]bool{}
	for _, item := range payload.Features {
		found[item.ID] = true
		if item.WebNavigation && item.WebPath == "" {
			t.Errorf("feature %q is navigable but has no route", item.ID)
		}
	}
	for _, id := range []string{"profile", "jobs", "remove", "repository-branches", "repository-activity"} {
		if !found[id] {
			t.Errorf("feature catalog is missing %q", id)
		}
	}
}

func TestRoutesServeRootAndSPAWithoutConflict(t *testing.T) {
	s := New(nil, Options{
		Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}},
	})
	handler := s.routes()

	for _, path := range []string{"/", "/issues", "/projects/example", "/repositories/xoron5/"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:43170"+path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d; body: %s", path, rec.Code, http.StatusOK, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("GET %s Content-Type = %q, want text/html; charset=utf-8", path, got)
			}
		})
	}
}

func TestJobPollingIntervalClampsToSupportedRange(t *testing.T) {
	if got := jobPollingInterval(config.AppConfig{Notifications: config.NotificationConfig{PollIntervalMinutes: -1}}); got != time.Minute {
		t.Fatalf("negative interval = %s, want 1m", got)
	}
	if got := jobPollingInterval(config.AppConfig{Notifications: config.NotificationConfig{PollIntervalMinutes: 15}}); got != 15*time.Minute {
		t.Fatalf("configured interval = %s, want 15m", got)
	}
	if got := jobPollingInterval(config.AppConfig{Notifications: config.NotificationConfig{PollIntervalMinutes: 2000}}); got != 24*time.Hour {
		t.Fatalf("large interval = %s, want 24h", got)
	}
}

func TestDuePlanReminderPeriodUsesOrderedLocalCheckIns(t *testing.T) {
	settings := config.PlanReminderConfig{Enabled: true, MorningTime: "08:30", MiddayTime: "13:00", EveningTime: "20:30"}
	cases := []struct{ clock, want string }{
		{"07:59", ""}, {"08:30", "morning"}, {"12:59", "morning"},
		{"13:00", "midday"}, {"19:45", "midday"}, {"20:30", "evening"}, {"23:59", "evening"},
	}
	for _, tc := range cases {
		t.Run(tc.clock, func(t *testing.T) {
			parsed, err := time.Parse("15:04", tc.clock)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, time.October, 22, parsed.Hour(), parsed.Minute(), 0, 0, time.Local)
			if got := duePlanReminderPeriod(now, settings); got != tc.want {
				t.Fatalf("due period at %s = %q, want %q", tc.clock, got, tc.want)
			}
		})
	}
}

func TestValidateLoopbackAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{name: "ipv4 loopback", address: "127.0.0.1:43170"},
		{name: "localhost", address: "localhost:43170"},
		{name: "ipv6 loopback", address: "[::1]:43170"},
		{name: "reject wildcard", address: "0.0.0.0:43170", wantErr: true},
		{name: "reject lan address", address: "192.168.1.12:43170", wantErr: true},
		{name: "reject missing port", address: "127.0.0.1", wantErr: true},
		{name: "reject invalid port", address: "127.0.0.1:70000", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateLoopbackAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateLoopbackAddress(%q) error = %v, wantErr %v", tt.address, err, tt.wantErr)
			}
			if err == nil && got == "" {
				t.Fatal("expected a normalized local address")
			}
		})
	}
}

func TestAPIHostAndOriginValidation(t *testing.T) {
	s := New(nil, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}})

	for _, host := range []string{"127.0.0.1:43170", "localhost:43170", "[::1]:43170"} {
		if !s.validHost(host) {
			t.Errorf("validHost(%q) = false, want true", host)
		}
	}
	for _, host := range []string{"example.com:43170", "127.0.0.1:43171", "127.0.0.1", "0.0.0.0:43170"} {
		if s.validHost(host) {
			t.Errorf("validHost(%q) = true, want false", host)
		}
	}

	origins := []struct {
		origin string
		want   bool
	}{
		{origin: "http://127.0.0.1:43170", want: true},
		{origin: "http://localhost:43170", want: true},
		{origin: "http://127.0.0.1:5173", want: true},
		{origin: "https://127.0.0.1:43170", want: false},
		{origin: "http://example.com:43170", want: false},
		{origin: "http://127.0.0.1:43171", want: false},
		{origin: "http://127.0.0.1:43170.evil.test", want: false},
		{origin: "null", want: false},
	}
	for _, tt := range origins {
		if got := s.validOrigin(tt.origin, "127.0.0.1:43170"); got != tt.want {
			t.Errorf("validOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestSecurityMiddlewareRejectsForeignAPIHost(t *testing.T) {
	s := New(nil, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}})
	handler := s.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign Host status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43170/api/config", nil)
	req.Header.Set("Origin", "http://attacker.test")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestActiveURLRejectsInvalidStateAddress(t *testing.T) {
	// ActiveURL reads under the resolved config directory; use an isolated directory so this test
	// never reads or modifies a developer's real server state.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "idios", "config.json")
	t.Setenv("IDIOS_CONFIG", configPath)
	statePath := filepath.Join(dir, "idios", "server-state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"address":"0.0.0.0:43170","pid":123,"started_at":"2026-10-21T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if url, ok := ActiveURL(); ok || url != "" {
		t.Fatalf("ActiveURL() = (%q, %v), want empty/false for non-loopback address", url, ok)
	}
}

func TestLogsAPIReturnsSharedDiagnostics(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDIOS_CONFIG", filepath.Join(dir, "idios", "config.json"))
	diagnostics.Write("INFO", "server log endpoint fixture", false)

	s := New(nil, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}})
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:43170/api/logs?limit=20", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/logs status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "server log endpoint fixture") || !strings.Contains(rec.Body.String(), "idios.log") {
		t.Fatalf("logs response does not contain shared log data: %s", rec.Body.String())
	}
}

func TestLogsAPICanClearHistoryWithoutReturningPlaceholderAsAnEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDIOS_CONFIG", filepath.Join(dir, "idios", "config.json"))
	diagnostics.Write("INFO", "this record should be removed", false)
	s := New(nil, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}})
	handler := s.routes()

	clearReq := httptest.NewRequest(http.MethodDelete, "http://127.0.0.1:43170/api/logs", nil)
	clearRec := httptest.NewRecorder()
	handler.ServeHTTP(clearRec, clearReq)
	if clearRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/logs status = %d, want 204; body=%s", clearRec.Code, clearRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:43170/api/logs?limit=20", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/logs after clear status = %d, want 200", getRec.Code)
	}
	var payload struct {
		Count int      `json:"count"`
		Lines []string `json:"lines"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode cleared logs response: %v", err)
	}
	if payload.Count != 0 || len(payload.Lines) != 0 {
		t.Fatalf("cleared log response has count=%d lines=%#v", payload.Count, payload.Lines)
	}
}

func TestCreateRepositoryRejectsUnknownJSONFields(t *testing.T) {
	s := New(nil, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}})
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43170/api/repositories", strings.NewReader(`{"name":"example","unexpected":true}`))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown field") {
		t.Fatalf("unexpected JSON field status/body = %d/%s, want 400 with a clear field error", rec.Code, rec.Body.String())
	}
}

func TestRequestLoggerIncludesActionableAPIErrorWithoutLoggingRequestBody(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDIOS_CONFIG", filepath.Join(dir, "idios", "config.json"))
	s := New(nil, Options{Config: config.AppConfig{Server: config.ServerConfig{Host: "127.0.0.1", Port: 43170}}})
	handler := s.requestLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, fmt.Errorf("%w: issue title is required", domain.ErrInvalid))
	}))
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43170/api/issues", strings.NewReader("private request content"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	joined := strings.Join(diagnostics.ReadRecent(20), "\n")
	if !strings.Contains(joined, "issue title is required") || !strings.Contains(joined, "WARN") {
		t.Fatalf("log should include bounded error detail and WARN severity: %q", joined)
	}
	if strings.Contains(joined, "private request content") {
		t.Fatalf("request body must never enter diagnostics: %q", joined)
	}
}

func TestUpdateStatusReturnsSharedLifecycleState(t *testing.T) {
	s := New(nil, Options{})
	s.updateState, s.updateVersion = "downloading", "1.2.3"
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:43170/api/update/status", nil)
	rec := httptest.NewRecorder()
	s.getUpdateStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200", rec.Code)
	}
	for _, expected := range []string{`"status":"downloading"`, `"version":"1.2.3"`} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Fatalf("update status body %q does not contain %q", rec.Body.String(), expected)
		}
	}
}

func TestInstallUpdateRejectsConcurrentUpdateBeforeDoingNetworkWork(t *testing.T) {
	s := New(nil, Options{})
	s.updateState, s.updateVersion = "downloading", "1.2.3"
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43170/api/update/install", nil)
	rec := httptest.NewRecorder()
	s.installUpdate(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("install status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "already downloading") {
		t.Fatalf("unexpected concurrent-update response: %s", rec.Body.String())
	}
}

func TestGraphLayoutAPIStoresModeAndResettablePositions(t *testing.T) {
	store, err := sqlite.OpenStore(filepath.Join(t.TempDir(), "graph-layout-api.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := New(application.New(store), Options{})
	handler := server.routes()

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/graph/layout", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET graph layout status=%d body=%s", get.Code, get.Body.String())
	}

	layout := domain.DefaultGraphLayout()
	layout.Mode = "flowchart"
	layout.Positions["flowchart"]["issue:test"] = domain.GraphPosition{X: 600, Y: 300}
	layout.Views["flowchart"] = domain.GraphViewport{X: 12, Y: -8, Scale: 1.2}
	body, err := json.Marshal(layout)
	if err != nil {
		t.Fatal(err)
	}
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/graph/layout", strings.NewReader(string(body))))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT graph layout status=%d body=%s", put.Code, put.Body.String())
	}

	var saved domain.GraphLayout
	if err := json.Unmarshal(put.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Mode != "flowchart" || saved.Positions["flowchart"]["issue:test"].X != 600 {
		t.Fatalf("saved layout did not round-trip: %+v", saved)
	}

	resetBody := strings.NewReader(`{"mode":"flowchart"}`)
	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/api/graph/layout/reset", resetBody))
	if reset.Code != http.StatusOK {
		t.Fatalf("POST graph layout reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	var restored domain.GraphLayout
	if err := json.Unmarshal(reset.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Mode != "flowchart" || len(restored.Positions["flowchart"]) != 0 || restored.Views["flowchart"].Scale != 1 {
		t.Fatalf("layout reset did not restore the active mode: %+v", restored)
	}
}
