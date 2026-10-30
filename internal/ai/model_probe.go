package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TestModel checks whether a discovered model can currently be used. Provider-specific
// probes are preferred where available; otherwise a bounded completion is sent.
// Results are persisted with the model catalogue so every client sees the same status.
func (m *Manager) TestModel(ctx context.Context, providerID, modelID string) (Model, error) {
	providerID = strings.TrimSpace(providerID)
	modelID = strings.TrimSpace(modelID)
	if providerID == "" || modelID == "" {
		return Model{}, &Error{Code: "invalid_request", Message: "Provider and model IDs are required"}
	}

	m.mu.RLock()
	var cfg *ProviderConfig
	for i := range m.state.Providers {
		if m.state.Providers[i].ID == providerID {
			copy := m.state.Providers[i]
			cfg = &copy
			break
		}
	}
	modelIndex := -1
	var model Model
	for i := range m.state.Models {
		if m.state.Models[i].ProviderID == providerID && m.state.Models[i].ID == modelID {
			modelIndex = i
			model = m.state.Models[i]
			break
		}
	}
	runtimeConfig := m.state.Config
	m.mu.RUnlock()
	if cfg == nil {
		return Model{}, &Error{Code: "invalid_provider_configuration", Message: "The selected provider is not configured"}
	}
	if modelIndex < 0 {
		return Model{}, &Error{Code: "model_not_found", Message: "Refresh the model catalogue before testing this model"}
	}

	if !cfg.Enabled {
		return m.storeModelProbe(model, "blocked", "Provider is disabled", 0), nil
	}
	if runtimeConfig.Mode == ModeOfflineOnly && !isConfigLocal(*cfg) {
		return m.storeModelProbe(model, "blocked", "Blocked by Offline-only policy", 0), nil
	}
	if runtimeConfig.Mode == ModeLocalFirst && !isConfigLocal(*cfg) && !runtimeConfig.AllowCloudFallback {
		return m.storeModelProbe(model, "blocked", "Blocked by Local-first policy; enable cloud fallback to use this model", 0), nil
	}

	release, err := m.acquireRuntimeSlot(ctx)
	if err != nil {
		return Model{}, err
	}
	defer release()

	timeout := modelProbeTimeout(*cfg)
	probeCfg := *cfg
	// Provider HTTP clients have their own timeout in addition to the request
	// context. Raise it together with the per-model deadline for cold local Ollama
	// model loads; otherwise the HTTP client would still abort at the old timeout.
	if probeCfg.Type == ProviderOllama && normalizedTimeout(probeCfg.TimeoutSeconds) < int(timeout/time.Second) {
		probeCfg.TimeoutSeconds = int(timeout / time.Second)
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	providerLatency, performance, probeErr := testConfiguredModel(probeCtx, probeCfg, modelID)
	latency := time.Since(started).Milliseconds()
	if providerLatency > 0 {
		latency = providerLatency
	}
	state, message := "available", "Model responded to a test request"
	if performance != nil {
		model.Performance = performance
	}
	if cfg.Type == ProviderOllama && probeErr == nil {
		// /api/show is a metadata-only call; keep its optional fields best-effort so
		// an older Ollama version cannot turn a successful model test into a failure.
		if contextSize, metadata, capabilities := ollamaModelDetails(probeCtx, probeCfg, modelID); contextSize > 0 || len(metadata) > 0 || len(capabilities) > 0 {
			if contextSize > 0 {
				model.ContextSize = contextSize
			}
			if len(model.Metadata) == 0 {
				model.Metadata = make(map[string]any)
			}
			for key, value := range metadata {
				model.Metadata[key] = value
			}
			if model.Capabilities == nil {
				model.Capabilities = make(map[string]bool)
			}
			for key, value := range capabilities {
				model.Capabilities[key] = value
			}
		}
	}
	if probeErr != nil {
		state = "unavailable"
		message = safeMessage(probeErr)
		if errors.Is(probeCtx.Err(), context.Canceled) {
			message = "Model check was cancelled"
		} else if errors.Is(probeCtx.Err(), context.DeadlineExceeded) || errorCode(probeErr) == "request_timed_out" {
			if cfg.Type == ProviderOllama {
				message = "Ollama did not finish loading/responding within the model-check timeout. The model may need more time or available RAM/VRAM; retry after warm-up or increase the provider timeout."
			} else {
				message = "Model check timed out"
			}
		}
	}
	return m.storeModelProbe(model, state, message, latency), nil
}

func (m *Manager) storeModelProbe(model Model, state, message string, latency int64) Model {
	checkedAt := time.Now().UTC()
	model.Availability = state
	model.AvailabilityMessage = message
	model.ProbeCheckedAt = checkedAt
	model.ProbeLatencyMS = latency

	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.state.Models {
		if m.state.Models[i].ProviderID == model.ProviderID && m.state.Models[i].ID == model.ID {
			m.state.Models[i] = model
			break
		}
	}
	_ = m.persistLocked()
	return model
}

func modelProbeTimeout(cfg ProviderConfig) time.Duration {
	timeout := time.Duration(normalizedTimeout(cfg.TimeoutSeconds)) * time.Second
	if cfg.Type == ProviderOllama {
		// Loading weights into RAM/VRAM on the first request can take much longer
		// than a warm request. Never classify a local Ollama model as unavailable
		// after the former global 15-second probe cap.
		if timeout < 180*time.Second {
			return 180 * time.Second
		}
		return timeout
	}
	if timeout > 15*time.Second {
		return 15 * time.Second
	}
	return timeout
}

func testConfiguredModel(ctx context.Context, cfg ProviderConfig, modelID string) (int64, *ModelPerformance, error) {
	// 9Router has a native model-status endpoint. Only use it for providers that
	// speak the OpenAI-compatible protocol; Ollama has its own native API and must
	// not pay the timeout cost of probing an unrelated /api/models/test endpoint.
	if cfg.Type == ProviderOpenAI && (looksLikeNineRouter(cfg) || isLoopbackProviderURL(cfg.BaseURL)) {
		if handled, latency, err := testNineRouterModel(ctx, cfg, modelID); handled {
			return latency, nil, err
		}
	}

	if cfg.Type == ProviderOllama {
		return testOllamaModel(ctx, cfg, modelID)
	}

	provider, err := newProvider(cfg)
	if err != nil {
		return 0, nil, err
	}
	zero := 0.0
	_, err = provider.Generate(ctx, GenerateRequest{
		ProviderID:  cfg.ID,
		Model:       modelID,
		Messages:    []Message{{Role: "user", Content: "Reply with only OK. Do not include an explanation."}},
		Temperature: &zero,
		// Four tokens falsely marked reasoning models offline: they may spend the
		// entire output budget thinking before emitting the short visible answer.
		MaxTokens:      256,
		TimeoutSeconds: 15,
	})
	return 0, nil, err
}

func looksLikeNineRouter(cfg ProviderConfig) bool {
	if strings.Contains(strings.ToLower(cfg.Name), "9router") {
		return true
	}
	u, err := url.Parse(cfg.BaseURL)
	return err == nil && strings.TrimSpace(u.Port()) == "20128" && isLoopbackHost(u.Hostname())
}

// testNineRouterModel uses 9Router's model-specific status endpoint. If the route is
// missing or responds in an unsupported way, callers fall back to the provider adapter.
func testNineRouterModel(ctx context.Context, cfg ProviderConfig, modelID string) (bool, int64, error) {
	u, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || u.Hostname() == "" {
		return false, 0, nil
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(strings.ToLower(path), "/v1") {
		path = path[:len(path)-3]
	}
	u.Path = strings.TrimRight(path, "/") + "/api/models/test"
	u.RawQuery = ""
	u.Fragment = ""

	// Match the previous Shelf implementation: some 9Router versions respond with
	// a non-2xx status or a response shape that cannot be interpreted here. That
	// means the native probe is unsupported, not that the model is offline; fall
	// back to a bounded OpenAI-compatible completion instead.
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client := providerHTTPClient(cfg)
	client.Timeout = 9 * time.Second
	payload, _ := json.Marshal(map[string]string{"model": modelID, "kind": "llm"})
	request, err := http.NewRequestWithContext(probeCtx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return false, 0, nil
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(cfg.APIKey); key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	// These headers were sent by the original Shelf probe too and are harmless for
	// local 9Router instances; they also preserve attribution for compatible gateways.
	request.Header.Set("HTTP-Referer", "http://127.0.0.1")
	request.Header.Set("X-Title", "IDIOS")
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return false, 0, nil
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return false, 0, nil
	}
	var result struct {
		OK        *bool  `json:"ok"`
		LatencyMS int64  `json:"latencyMs"`
		Error     string `json:"error"`
	}
	if json.Unmarshal(body, &result) != nil || result.OK == nil {
		return false, 0, nil
	}
	latency := result.LatencyMS
	if latency <= 0 {
		latency = time.Since(started).Milliseconds()
	}
	if !*result.OK {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = "9Router reports that this model is unavailable"
		}
		return true, latency, &Error{Code: "model_not_found", Message: compactMessage(message)}
	}
	return true, latency, nil
}

func isLoopbackProviderURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && isLoopbackHost(u.Hostname())
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func compactMessage(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 220 {
		return value[:220] + "…"
	}
	return value
}
