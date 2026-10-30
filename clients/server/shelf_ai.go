package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"idios/internal/ai"
)

// shelfAIRequest is a compatibility adapter for older Shelf settings that still send connection
// details per request. New integrations should use the central AI Runtime provider registry; these
// legacy credentials are ephemeral and are never persisted by this adapter.
type shelfAIRequest struct {
	BaseURL         string           `json:"base_url"`
	APIKey          string           `json:"api_key"`
	Model           string           `json:"model,omitempty"`
	Messages        []shelfAIMessage `json:"messages,omitempty"`
	Temperature     *float64         `json:"temperature,omitempty"`
	Probe           bool             `json:"probe,omitempty"`
	ReasoningEffort string           `json:"reasoning_effort,omitempty"`
}

type shelfAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func validateShelfAIBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("Enter a valid AI base URL, such as http://127.0.0.1:20128/v1")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("AI base URL must use HTTP or HTTPS")
	}
	// Plain HTTP is permitted only for local routers. Remote providers must use TLS.
	if parsed.Scheme == "http" {
		host := strings.Trim(parsed.Hostname(), "[]")
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("Use HTTPS for remote AI providers; plain HTTP is allowed only for localhost routers")
		}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

func shelfAIURL(base *url.URL, endpoint string) string {
	copy := *base
	copy.Path = strings.TrimRight(copy.Path, "/") + "/" + endpoint
	return copy.String()
}

func shelfAIHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func shelfAIRequestContext(r *http.Request) context.Context { return r.Context() }

type shelfAIModelAvailability struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name,omitempty"`
	Availability string                    `json:"availability"`
	LatencyMS    int64                     `json:"latency_ms,omitempty"`
	Error        string                    `json:"error,omitempty"`
	Capabilities *shelfAIModelCapabilities `json:"capabilities,omitempty"`
}

type shelfAIModelCapabilities struct {
	Reasoning *bool `json:"reasoning,omitempty"`
}

type shelfAIModelDescriptor struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Capabilities        map[string]bool `json:"capabilities"`
	Reasoning           *bool           `json:"reasoning"`
	SupportedParameters []string        `json:"supported_parameters"`
}

// shelfAIReasoningCapability only reports support when the gateway explicitly
// advertises it. Model-name guessing causes unsupported Thinking controls to show up.
func shelfAIReasoningCapability(model shelfAIModelDescriptor) *bool {
	if value, ok := model.Capabilities["reasoning"]; ok {
		return &value
	}
	if value, ok := model.Capabilities["thinking"]; ok {
		return &value
	}
	if model.Reasoning != nil {
		return model.Reasoning
	}
	for _, parameter := range model.SupportedParameters {
		if strings.EqualFold(parameter, "reasoning_effort") || strings.EqualFold(parameter, "thinking") {
			value := true
			return &value
		}
	}
	return nil
}

func (s *Server) shelfAIModels(w http.ResponseWriter, r *http.Request) {
	var input shelfAIRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	base, err := validateShelfAIBaseURL(input.BaseURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	models, err := manager.LegacyListModels(shelfAIRequestContext(r), input.BaseURL, input.APIKey)
	if err != nil {
		writeAIError(w, err)
		return
	}
	if len(models) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": []any{}, "tested": input.Probe})
		return
	}
	descriptors := make([]shelfAIModelDescriptor, len(models))
	for i, model := range models {
		name := model.Name
		if name == "" {
			name = model.ID
		}
		descriptors[i] = shelfAIModelDescriptor{ID: model.ID, Name: name, Capabilities: model.Capabilities, Reasoning: model.Reasoning, SupportedParameters: model.SupportedParameters}
	}
	if !input.Probe {
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": descriptors})
		return
	}

	results := make([]shelfAIModelAvailability, len(descriptors))
	semaphore := make(chan struct{}, 6)
	var workers sync.WaitGroup
	for index, item := range descriptors {
		modelID := strings.TrimSpace(item.ID)
		if modelID == "" {
			modelID = strings.TrimSpace(item.Name)
		}
		if modelID == "" {
			continue
		}
		workers.Add(1)
		go func(index int, item shelfAIModelDescriptor, modelID string) {
			defer workers.Done()
			select {
			case semaphore <- struct{}{}:
			case <-r.Context().Done():
				results[index] = shelfAIModelAvailability{ID: modelID, Name: item.Name, Availability: "unavailable", Error: "Model check was cancelled"}
				return
			}
			defer func() { <-semaphore }()
			available, latency, detail := probeShelfAIModel(r.Context(), base, input.APIKey, modelID, manager)
			status := "unavailable"
			if available {
				status = "available"
			}
			results[index] = shelfAIModelAvailability{ID: modelID, Name: item.Name, Availability: status, LatencyMS: latency, Error: detail}
			if reasoning := shelfAIReasoningCapability(item); reasoning != nil {
				results[index].Capabilities = &shelfAIModelCapabilities{Reasoning: reasoning}
			}
		}(index, item, modelID)
	}
	workers.Wait()
	data := make([]shelfAIModelAvailability, 0, len(results))
	for _, result := range results {
		if result.ID != "" {
			data = append(data, result)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data, "tested": true})
}

func probeShelfAIModel(parent context.Context, base *url.URL, apiKey, model string, manager *ai.Manager) (bool, int64, string) {
	// Newer 9Router versions expose a model-specific probe. It is used only for an
	// explicitly configured loopback endpoint; all other probes go through the runtime adapter.
	if isShelfAILoopback(base.Hostname()) {
		root := *base
		path := strings.TrimRight(root.Path, "/")
		if strings.HasSuffix(path, "/v1") {
			path = strings.TrimSuffix(path, "/v1")
		}
		root.Path = strings.TrimRight(path, "/") + "/api/models/test"
		payload, _ := json.Marshal(map[string]string{"model": model, "kind": "llm"})
		ctx, cancel := context.WithTimeout(parent, 8*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, root.String(), bytes.NewReader(payload))
		if err == nil {
			request.Header.Set("Content-Type", "application/json")
			setShelfAIHeaders(request, apiKey)
			started := time.Now()
			response, callErr := shelfAIHTTPClient().Do(request)
			if callErr == nil {
				defer response.Body.Close()
				probeBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
				var probe struct {
					OK        *bool  `json:"ok"`
					LatencyMS int64  `json:"latencyMs"`
					Error     string `json:"error"`
				}
				if json.Unmarshal(probeBody, &probe) == nil && probe.OK != nil && response.StatusCode >= 200 && response.StatusCode < 300 {
					latency := probe.LatencyMS
					if latency <= 0 {
						latency = time.Since(started).Milliseconds()
					}
					if *probe.OK {
						return true, latency, ""
					}
					return false, latency, compactShelfAIError(probe.Error)
				}
			}
		}
	}
	started := time.Now()
	zero := 0.0
	_, err := manager.LegacyGenerate(parent, base.String(), apiKey, ai.GenerateRequest{Model: model, Messages: []ai.Message{{Role: "user", Content: "Reply with only OK."}}, MaxTokens: 48, Temperature: &zero, TimeoutSeconds: 12})
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return false, latency, compactShelfAIError(err.Error())
	}
	return true, latency, ""
}

func isShelfAILoopback(host string) bool {
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func shelfAIResponseError(body []byte, status int) string {
	var parsed struct {
		Error   any    `json:"error"`
		Msg     string `json:"msg"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Error != nil {
			return compactShelfAIError(fmt.Sprint(parsed.Error))
		}
		if parsed.Msg != "" {
			return compactShelfAIError(parsed.Msg)
		}
		if parsed.Message != "" {
			return compactShelfAIError(parsed.Message)
		}
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Sprintf("HTTP %d", status)
	}
	return fmt.Sprintf("HTTP %d: %s", status, compactShelfAIError(text))
}

func compactShelfAIError(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 220 {
		value = value[:220] + "…"
	}
	return value
}

func (s *Server) shelfAIChat(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var input shelfAIRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if _, err := validateShelfAIBaseURL(input.BaseURL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(input.Model) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Choose an AI model first"})
		return
	}
	if len(input.Messages) == 0 || len(input.Messages) > 40 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Provide between 1 and 40 chat messages"})
		return
	}
	messages := make([]ai.Message, 0, len(input.Messages))
	for _, message := range input.Messages {
		if message.Role != "system" && message.Role != "user" && message.Role != "assistant" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Messages may use system, user or assistant roles"})
			return
		}
		if len(message.Content) > 60000 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "A message is too long (maximum 60,000 characters)"})
			return
		}
		messages = append(messages, ai.Message{Role: message.Role, Content: message.Content})
	}
	if input.Temperature != nil && (*input.Temperature < 0 || *input.Temperature > 2) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Temperature must be between 0 and 2"})
		return
	}
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	ctx, cancel := context.WithTimeout(shelfAIRequestContext(r), 25*time.Second)
	defer cancel()
	response, err := manager.LegacyGenerate(ctx, input.BaseURL, input.APIKey, ai.GenerateRequest{Model: input.Model, Messages: messages, Temperature: input.Temperature, MaxTokens: 2048, ReasoningEffort: strings.ToLower(strings.TrimSpace(input.ReasoningEffort)), TimeoutSeconds: 22})
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"model": response.Model, "provider_id": response.ProviderID, "choices": []any{map[string]any{"message": map[string]any{"content": response.Content}}}})
}

func setShelfAIHeaders(req *http.Request, apiKey string) {
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Accept", "application/json")
	// OpenRouter uses these optional metadata headers for app attribution.
	req.Header.Set("HTTP-Referer", "http://127.0.0.1")
	req.Header.Set("X-Title", "IDIOS Shelf Study Assistant")
}
