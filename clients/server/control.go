package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"idios/internal/config"
)

// BaseURL resolves the configured local web address without opening a database.
func BaseURL(cfg config.AppConfig) (string, error) {
	host := strings.TrimSpace(cfg.Server.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Server.Port
	if port < 1 || port > 65535 {
		port = 43170
	}
	addr, err := validateLoopbackAddress(net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return "", err
	}
	return "http://" + addr, nil
}

// ControlURL prefers the listener that is actually running over the newly configured address.
// This keeps stop/status/web commands reliable after a host or port preference has changed but
// before the current server has been restarted.
func ControlURL(cfg config.AppConfig) (string, error) {
	configured, err := BaseURL(cfg)
	if err != nil {
		return "", err
	}
	if active, ok := ActiveURL(); ok {
		if running, _ := Probe(active); running {
			return active, nil
		}
	}
	return configured, nil
}

// Probe returns true if the local server responds with a healthy JSON payload.
func Probe(baseURL string) (bool, map[string]any) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return false, nil
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	response, err := client.Get(strings.TrimRight(baseURL, "/") + "/api/health")
	if err != nil {
		return false, nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, nil
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return false, nil
	}
	return payload["status"] == "ok", payload
}

// RequestShutdown asks an already-running loopback server to stop gracefully.
func RequestShutdown(baseURL string) error {
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/server/shutdown", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("contact IDIOS server: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("IDIOS server rejected shutdown (HTTP %d)", response.StatusCode)
	}
	return nil
}

// OpenBrowser uses the platform's default URL handler and does not wait for the browser process.
func OpenBrowser(url string) error {
	return openBrowser(url)
}
