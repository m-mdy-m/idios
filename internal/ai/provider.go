package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Provider interface {
	ID() string
	Kind() ProviderKind
	Local() bool
	ListModels(context.Context) ([]Model, error)
	Generate(context.Context, GenerateRequest) (GenerateResponse, error)
	Stream(context.Context, GenerateRequest, func(string) error) error
	Health(context.Context) HealthStatus
}

func newProvider(cfg ProviderConfig) (Provider, error) {
	if err := ValidateProviderConfig(cfg); err != nil {
		return nil, err
	}
	switch cfg.Type {
	case ProviderOpenRouter, ProviderOpenAI:
		return &openAIProvider{cfg: cfg, client: providerHTTPClient(cfg)}, nil
	case ProviderOllama:
		return &ollamaProvider{cfg: cfg, client: providerHTTPClient(cfg)}, nil
	case ProviderCustom:
		factory, ok := registeredProviderFactory(cfg.AdapterID)
		if !ok {
			return nil, &Error{Code: "invalid_provider_configuration", Message: "The configured custom adapter is not registered in this IDIOS build"}
		}
		return factory(cfg)
	default:
		return nil, &Error{Code: "invalid_provider_configuration", Message: "Unsupported AI provider type"}
	}
}

func ValidateProviderConfig(cfg ProviderConfig) error {
	if strings.TrimSpace(cfg.ID) == "" {
		return &Error{Code: "invalid_provider_configuration", Message: "Provider ID is required"}
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return &Error{Code: "invalid_provider_configuration", Message: "Provider name is required"}
	}
	if cfg.Type != ProviderOpenRouter && cfg.Type != ProviderOpenAI && cfg.Type != ProviderOllama && cfg.Type != ProviderCustom {
		return &Error{Code: "invalid_provider_configuration", Message: "Choose OpenRouter, OpenAI-compatible, Ollama, or a registered custom adapter"}
	}
	if cfg.Type == ProviderCustom {
		if strings.TrimSpace(cfg.AdapterID) == "" {
			return &Error{Code: "invalid_provider_configuration", Message: "Custom providers require an adapter ID registered by an IDIOS extension"}
		}
		if !providerAdapterRegistered(cfg.AdapterID) {
			return &Error{Code: "invalid_provider_configuration", Message: "The requested custom adapter is not registered in this IDIOS build"}
		}
	}
	u, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return &Error{Code: "invalid_provider_configuration", Message: "Enter a valid provider base URL without embedded credentials, query parameters, or fragments"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return &Error{Code: "invalid_provider_configuration", Message: "Provider URL must use HTTP or HTTPS"}
	}
	host := strings.Trim(u.Hostname(), "[]")
	ip := net.ParseIP(host)
	loopback := host == "localhost" || (ip != nil && ip.IsLoopback())
	privateIP := ip != nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
	if ip != nil && privateIP && !loopback && !cfg.TrustedLocal {
		return &Error{Code: "invalid_provider_configuration", Message: "Private-network provider addresses must be explicitly trusted before IDIOS can connect"}
	}
	if cfg.TrustedLocal && !loopback {
		trustedName := strings.HasSuffix(strings.ToLower(host), ".local") || strings.HasSuffix(strings.ToLower(host), ".lan") || strings.HasSuffix(strings.ToLower(host), ".internal") || !strings.Contains(host, ".")
		if ip != nil && !privateIP {
			trustedName = false
		}
		if !trustedName && !(ip != nil && privateIP) {
			return &Error{Code: "invalid_provider_configuration", Message: "Trusted-local is only for loopback or private-network endpoints; use a public HTTPS URL without this flag"}
		}
	}
	if u.Scheme == "http" && !loopback && !cfg.TrustedLocal {
		return &Error{Code: "invalid_provider_configuration", Message: "Plain HTTP is allowed only for loopback providers. For a private-network endpoint, explicitly mark it trusted; remote endpoints must use HTTPS"}
	}
	if (cfg.Type == ProviderOllama || cfg.TrustedLocal) && !loopback && ip != nil && !privateIP && !ip.IsGlobalUnicast() {
		return &Error{Code: "invalid_provider_configuration", Message: "The configured trusted-local address is not a valid unicast destination"}
	}
	if cfg.Type == ProviderOllama && !loopback && !cfg.TrustedLocal {
		return &Error{Code: "invalid_provider_configuration", Message: "Ollama must use a loopback URL unless you explicitly trust a private-network endpoint"}
	}
	if cfg.Type == ProviderOpenRouter && !strings.EqualFold(u.Hostname(), "openrouter.ai") && !cfg.TrustedLocal {
		return &Error{Code: "invalid_provider_configuration", Message: "OpenRouter providers must use https://openrouter.ai; use OpenAI-compatible for another endpoint"}
	}
	if cfg.TimeoutSeconds < 0 || cfg.TimeoutSeconds > 600 {
		return &Error{Code: "invalid_provider_configuration", Message: "Provider timeout must be between 1 and 600 seconds"}
	}
	if cfg.MaxConcurrent < 0 || cfg.MaxConcurrent > 64 {
		return &Error{Code: "invalid_provider_configuration", Message: "Provider concurrency must be between 1 and 64"}
	}
	return nil
}

// NewProviderHTTPClient returns the standard bounded HTTP client for a provider
// extension. It validates the configured endpoint and retains IDIOS' destination
// address checks, proxy isolation, timeout handling, and redirect rejection.
func NewProviderHTTPClient(cfg ProviderConfig) (*http.Client, error) {
	if err := ValidateProviderConfig(cfg); err != nil {
		return nil, err
	}
	return providerHTTPClient(cfg), nil
}

func providerHTTPClient(cfg ProviderConfig) *http.Client {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	transport := &http.Transport{
		Proxy: nil, // Provider requests must not bypass address validation through an environment proxy.
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, item := range ips {
				if !isAllowedDestination(item.IP, cfg) {
					lastErr = fmt.Errorf("provider destination is not allowed by the configured trust boundary")
					continue
				}
				conn, dialErr := (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, net.JoinHostPort(item.IP.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			if lastErr == nil {
				lastErr = errors.New("provider host resolved to no usable address")
			}
			return nil, lastErr
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       90 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func isAllowedDestination(ip net.IP, cfg ProviderConfig) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	u, _ := url.Parse(cfg.BaseURL)
	host := ""
	if u != nil {
		host = strings.Trim(strings.ToLower(u.Hostname()), "[]")
	}
	configuredIP := net.ParseIP(host)
	configuredLoopback := host == "localhost" || (configuredIP != nil && configuredIP.IsLoopback())
	if configuredLoopback {
		// A local hostname resolving to a public IP is a trust-boundary violation.
		return ip.IsLoopback()
	}
	if ip.IsLoopback() {
		return cfg.TrustedLocal
	}
	if cfg.TrustedLocal {
		// Trusted-local is only for explicitly trusted private endpoints, never a shortcut
		// that permits a .local/.lan name to resolve to an arbitrary public IP.
		return ip.IsPrivate()
	}
	if ip.IsPrivate() || !ip.IsGlobalUnicast() {
		return false
	}
	// Shared carrier space and documentation-only ranges are not public provider destinations.
	for _, cidr := range []string{"100.64.0.0/10", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "198.18.0.0/15"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func endpoint(base, suffix string) string {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(base), "/"))
	if err != nil {
		return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(suffix, "/")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(suffix, "/")
	return u.String()
}

func requestJSON(ctx context.Context, client *http.Client, method, target, apiKey string, payload any) (*http.Response, []byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, &Error{Code: "invalid_request", Message: "Could not encode the AI request"}
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, nil, &Error{Code: "invalid_provider_configuration", Message: "Could not create a provider request"}
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	response, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, nil, &Error{Code: "request_cancelled", Message: "The AI request was cancelled"}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, &Error{Code: "request_timed_out", Message: "The AI provider request timed out"}
		}
		return nil, nil, &Error{Code: "provider_unavailable", Message: "Could not reach the configured AI provider"}
	}
	defer response.Body.Close()
	limit := int64(8 << 20)
	if payload == nil {
		limit = 4 << 20
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return response, nil, &Error{Code: "provider_unavailable", Message: "Could not read the AI provider response"}
	}
	if int64(len(data)) > limit {
		return response, nil, &Error{Code: "resource_limit_exceeded", Message: "The AI provider response exceeded the configured size limit"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, data, normalizedHTTPError(response.StatusCode, data)
	}
	return response, data, nil
}

func normalizedHTTPError(status int, body []byte) *Error {
	message := "The AI provider rejected the request"
	var payload struct {
		Error   any    `json:"error"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if json.Unmarshal(body, &payload) == nil {
		switch {
		case payload.Message != "":
			message = payload.Message
		case payload.Msg != "":
			message = payload.Msg
		case payload.Error != nil:
			message = fmt.Sprint(payload.Error)
		}
	}
	message = compactError(message)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Code: "authentication_failed", Message: "AI provider authentication failed; check the saved API key and provider permissions"}
	case http.StatusNotFound:
		return &Error{Code: "model_not_found", Message: "The requested model or provider API endpoint was not found"}
	case http.StatusTooManyRequests:
		return &Error{Code: "rate_limited", Message: "The AI provider rate limit was reached"}
	case http.StatusRequestEntityTooLarge:
		return &Error{Code: "context_too_large", Message: "The AI provider rejected the request because its context is too large"}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return &Error{Code: "request_timed_out", Message: "The AI provider timed out while processing the request"}
	default:
		if status >= 500 {
			return &Error{Code: "provider_unavailable", Message: "The AI provider is temporarily unavailable"}
		}
		if message == "" {
			message = "The AI provider returned HTTP " + strconv.Itoa(status)
		}
		return &Error{Code: "provider_request_failed", Message: message}
	}
}

func compactError(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	// Never show obvious bearer tokens or long secret-looking values to the UI.
	if len(value) > 220 {
		value = value[:220] + "…"
	}
	return value
}

func (p *openAIProvider) ID() string         { return p.cfg.ID }
func (p *openAIProvider) Kind() ProviderKind { return p.cfg.Type }
func (p *openAIProvider) Local() bool        { return isConfigLocal(p.cfg) }

type openAIProvider struct {
	cfg    ProviderConfig
	client *http.Client
}

type ollamaProvider struct {
	cfg    ProviderConfig
	client *http.Client
}

func (p *openAIProvider) ListModels(ctx context.Context) ([]Model, error) {
	_, data, err := requestJSON(ctx, p.client, http.MethodGet, endpoint(p.cfg.BaseURL, "models"), p.cfg.APIKey, nil)
	if err != nil {
		return nil, err
	}
	return parseOpenAIModelCatalogue(data, p.cfg.ID)
}

// parseOpenAIModelCatalogue intentionally reads only fields needed by IDIOS and
// tolerates vendor-specific metadata. OpenAI-compatible gateways sometimes add
// non-standard capability fields; a single field with a different JSON type must
// not make every otherwise valid model undiscoverable.
func parseOpenAIModelCatalogue(data []byte, providerID string) ([]Model, error) {
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Models json.RawMessage `json:"models"` // tolerated for compatible gateways that use this alias
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, &Error{Code: "provider_request_failed", Message: "The provider returned invalid JSON for its model catalogue"}
	}

	catalogue := bytes.TrimSpace(envelope.Data)
	if len(catalogue) == 0 || bytes.Equal(catalogue, []byte("null")) {
		catalogue = bytes.TrimSpace(envelope.Models)
	}
	if len(catalogue) == 0 || bytes.Equal(catalogue, []byte("null")) {
		return nil, &Error{Code: "provider_request_failed", Message: "The provider returned JSON without a model list. Configure an OpenAI-compatible base URL ending in /v1 and verify GET /v1/models returns a data array."}
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(catalogue, &entries); err != nil {
		return nil, &Error{Code: "provider_request_failed", Message: "The provider returned an unsupported model-catalogue shape. Expected an OpenAI-compatible object containing a data array; verify GET /v1/models."}
	}
	models := make([]Model, 0, len(entries))
	for _, raw := range entries {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil || item == nil {
			// Ignore malformed individual entries rather than losing the whole catalogue.
			continue
		}
		id := firstJSONText(item, "id", "name", "model")
		if id == "" {
			continue
		}
		name := firstJSONText(item, "name")
		if name == "" {
			name = id
		}
		caps := map[string]bool{"text_generation": true}
		for key, value := range readJSONCapabilities(item["capabilities"]) {
			caps[key] = value
		}
		for key, value := range readJSONCapabilities(item["supported_capabilities"]) {
			caps[key] = value
		}
		contextSize := firstJSONInt(item, "context_length", "context_window", "context_size")
		models = append(models, Model{
			ProviderID:          providerID,
			ID:                  id,
			Name:                name,
			Capabilities:        caps,
			Reasoning:           readJSONBool(item["reasoning"]),
			SupportedParameters: readJSONStringList(item["supported_parameters"]),
			ContextSize:         contextSize,
			Discovered:          time.Now().UTC(),
		})
	}
	return models, nil
}

func firstJSONText(fields map[string]json.RawMessage, names ...string) string {
	for _, name := range names {
		var value string
		if len(fields[name]) > 0 && json.Unmarshal(fields[name], &value) == nil {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}

func readJSONCapabilities(raw json.RawMessage) map[string]bool {
	result := make(map[string]bool)
	if len(raw) == 0 {
		return result
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil && object != nil {
		for key, value := range object {
			var enabled bool
			if json.Unmarshal(value, &enabled) == nil {
				result[key] = enabled
			}
		}
		return result
	}
	var values []string
	if json.Unmarshal(raw, &values) == nil {
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				result[value] = true
			}
		}
	}
	return result
}

func readJSONBool(raw json.RawMessage) *bool {
	if len(raw) == 0 {
		return nil
	}
	var value bool
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return &value
}

func readJSONStringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if json.Unmarshal(raw, &values) == nil {
		return values
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil
	}
	values = make([]string, 0, len(object))
	for key, value := range object {
		var enabled bool
		if json.Unmarshal(value, &enabled) == nil && enabled {
			values = append(values, key)
		}
	}
	return values
}

func firstJSONInt(fields map[string]json.RawMessage, names ...string) int {
	for _, name := range names {
		raw := bytes.TrimSpace(fields[name])
		if len(raw) == 0 {
			continue
		}
		var value int
		if json.Unmarshal(raw, &value) == nil && value > 0 {
			return value
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			if parsed, err := strconv.Atoi(strings.TrimSpace(text)); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func (p *openAIProvider) Generate(ctx context.Context, input GenerateRequest) (GenerateResponse, error) {
	started := time.Now()
	payload := openAIPayload(input, false)
	_, data, err := requestJSON(ctx, p.client, http.MethodPost, endpoint(p.cfg.BaseURL, "chat/completions"), p.cfg.APIKey, payload)
	if err != nil {
		return GenerateResponse{}, err
	}
	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return GenerateResponse{}, &Error{Code: "provider_request_failed", Message: "The provider returned an unreadable completion"}
	}
	if parsed.Error != nil {
		return GenerateResponse{}, &Error{Code: "provider_request_failed", Message: "The AI provider reported a generation error"}
	}
	if len(parsed.Choices) == 0 {
		return GenerateResponse{}, &Error{Code: "provider_request_failed", Message: "The AI provider returned no completion choices"}
	}
	content, err := decodeContent(parsed.Choices[0].Message.Content)
	if err != nil {
		return GenerateResponse{}, err
	}
	model := parsed.Model
	if model == "" {
		model = input.Model
	}
	return GenerateResponse{ProviderID: p.cfg.ID, Model: model, Content: content, DurationMS: time.Since(started).Milliseconds()}, nil
}

func openAIPayload(input GenerateRequest, stream bool) map[string]any {
	model := strings.TrimSpace(input.Model)
	payload := map[string]any{"model": model, "messages": input.Messages, "stream": stream}
	if input.MaxTokens > 0 {
		payload["max_tokens"] = input.MaxTokens
	}
	if input.Temperature != nil {
		payload["temperature"] = *input.Temperature
	}
	if input.ReasoningEffort != "" {
		payload["reasoning_effort"] = input.ReasoningEffort
	}
	if input.ResponseFormat != nil {
		switch input.ResponseFormat.Type {
		case "json_object":
			payload["response_format"] = map[string]any{"type": "json_object"}
		case "json_schema":
			name := strings.TrimSpace(input.ResponseFormat.Name)
			if name == "" {
				name = "idios_response"
			}
			payload["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": name, "strict": input.ResponseFormat.Strict, "schema": input.ResponseFormat.Schema}}
		}
	}
	return payload
}

func decodeContent(raw json.RawMessage) (string, error) {
	var content string
	if err := json.Unmarshal(raw, &content); err == nil {
		return content, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var b strings.Builder
		for _, block := range blocks {
			if block.Type == "text" || block.Type == "" {
				b.WriteString(block.Text)
			}
		}
		return b.String(), nil
	}
	return "", &Error{Code: "provider_request_failed", Message: "The model returned an unsupported message content format"}
}

func (p *openAIProvider) Stream(ctx context.Context, input GenerateRequest, emit func(string) error) error {
	payload := openAIPayload(input, true)
	encoded, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(p.cfg.BaseURL, "chat/completions"), bytes.NewReader(encoded))
	if err != nil {
		return &Error{Code: "invalid_provider_configuration", Message: "Could not create a streaming request"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	}
	response, err := p.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return &Error{Code: "request_cancelled", Message: "The AI request was cancelled"}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return &Error{Code: "request_timed_out", Message: "The AI provider request timed out"}
		}
		return &Error{Code: "provider_unavailable", Message: "Could not reach the configured AI provider"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return normalizedHTTPError(response.StatusCode, body)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error any `json:"error"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if chunk.Error != nil {
			return &Error{Code: "provider_request_failed", Message: "The AI provider reported a streaming error"}
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := emit(chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return &Error{Code: "request_cancelled", Message: "The AI request was cancelled"}
		}
		return &Error{Code: "provider_request_failed", Message: "The provider stream ended unexpectedly"}
	}
	return nil
}

func (p *openAIProvider) Health(ctx context.Context) HealthStatus {
	started := time.Now()
	_, _, err := requestJSON(ctx, p.client, http.MethodGet, endpoint(p.cfg.BaseURL, "models"), p.cfg.APIKey, nil)
	status := HealthStatus{CheckedAt: time.Now().UTC(), LatencyMS: time.Since(started).Milliseconds()}
	if err != nil {
		status.State = "unavailable"
		status.ErrorCode = errorCode(err)
		status.Message = safeMessage(err)
		if e, ok := err.(*Error); ok && e.Code == "authentication_failed" {
			status.State = "unauthorized"
		}
		return status
	}
	status.State = "healthy"
	status.Message = "Provider responded successfully"
	return status
}

func (p *ollamaProvider) ID() string         { return p.cfg.ID }
func (p *ollamaProvider) Kind() ProviderKind { return ProviderOllama }
func (p *ollamaProvider) Local() bool        { return true }

func (p *ollamaProvider) ListModels(ctx context.Context) ([]Model, error) {
	_, data, err := requestJSON(ctx, p.client, http.MethodGet, endpoint(p.cfg.BaseURL, "api/tags"), "", nil)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Models []struct {
			Name       string    `json:"name"`
			Model      string    `json:"model"`
			Digest     string    `json:"digest"`
			Size       int64     `json:"size"`
			ModifiedAt time.Time `json:"modified_at"`
			Details    struct {
				Format            string   `json:"format"`
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, &Error{Code: "provider_request_failed", Message: "Ollama returned an unreadable model catalogue"}
	}
	models := make([]Model, 0, len(parsed.Models))
	for _, item := range parsed.Models {
		id := strings.TrimSpace(item.Name)
		if id == "" {
			id = strings.TrimSpace(item.Model)
		}
		if id == "" {
			continue
		}
		metadata := map[string]any{}
		if item.Digest != "" {
			metadata["digest"] = item.Digest
		}
		if len(item.Details.Families) > 0 {
			metadata["families"] = item.Details.Families
		}
		if len(metadata) == 0 {
			metadata = nil
		}
		var modifiedAt *time.Time
		if !item.ModifiedAt.IsZero() {
			value := item.ModifiedAt
			modifiedAt = &value
		}
		models = append(models, Model{
			ProviderID: p.cfg.ID, ID: id, Name: id,
			Architecture: item.Details.Family, Family: item.Details.Family,
			ParameterSize: item.Details.ParameterSize, QuantizationLevel: item.Details.QuantizationLevel,
			Format: item.Details.Format, SizeBytes: item.Size, ModifiedAt: modifiedAt,
			Metadata: metadata, Capabilities: map[string]bool{"text_generation": true, "streaming": true}, Discovered: time.Now().UTC(),
		})
	}
	return models, nil
}

func ollamaMessages(messages []Message) []map[string]string {
	result := make([]map[string]string, 0, len(messages))
	for _, item := range messages {
		result = append(result, map[string]string{"role": item.Role, "content": item.Content})
	}
	return result
}

func ollamaPayload(input GenerateRequest, stream bool) map[string]any {
	payload := map[string]any{"model": input.Model, "messages": ollamaMessages(input.Messages), "stream": stream}
	// Ollama's /api/chat uses a boolean `think` flag. On Qwen3 and DeepSeek-R1,
	// thinking may be enabled by default; leaving the field absent can spend a tiny
	// num_predict budget on hidden reasoning and yield a visibly truncated answer.
	// Only set false when the runtime knows this model supports the flag.
	if input.ReasoningEffort != "" {
		payload["think"] = true
	} else if input.DisableThinking {
		payload["think"] = false
	}
	if input.ResponseFormat != nil {
		switch input.ResponseFormat.Type {
		case "json_object":
			payload["format"] = "json"
		case "json_schema":
			if input.ResponseFormat.Schema != nil {
				payload["format"] = input.ResponseFormat.Schema
			}
		}
	}
	return payload
}

// testOllamaModel runs a small native streaming generation probe. Ollama's final
// NDJSON chunk includes token counts and nanosecond timings, which make these
// measurements substantially more useful than timing an HTTP status alone.
func testOllamaModel(ctx context.Context, cfg ProviderConfig, modelID string) (int64, *ModelPerformance, error) {
	client := providerHTTPClient(cfg)
	zero := 0.0
	payload := map[string]any{
		"model":      modelID,
		"messages":   []map[string]string{{"role": "user", "content": "Write a short, direct greeting. Do not explain."}},
		"stream":     true,
		"think":      false,
		"keep_alive": "0",
		"options": map[string]any{
			"temperature": zero,
			"num_predict": 32,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, &Error{Code: "invalid_request", Message: "Could not encode Ollama model-check request"}
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(cfg.BaseURL, "api/chat"), bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, &Error{Code: "invalid_provider_configuration", Message: "Could not create an Ollama model-check request"}
	}
	req.Header.Set("Accept", "application/x-ndjson")
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		latency := time.Since(started).Milliseconds()
		if errors.Is(err, context.Canceled) {
			return latency, nil, &Error{Code: "request_cancelled", Message: "The AI request was cancelled"}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return latency, nil, &Error{Code: "request_timed_out", Message: "The AI provider request timed out"}
		}
		return latency, nil, &Error{Code: "provider_unavailable", Message: "Could not reach the configured Ollama service"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return time.Since(started).Milliseconds(), nil, normalizedHTTPError(response.StatusCode, body)
	}

	var firstTokenAt time.Time
	var final struct {
		Model   string `json:"model"`
		Done    bool   `json:"done"`
		Error   string `json:"error"`
		Message struct {
			Content  string `json:"content"`
			Thinking string `json:"thinking"`
		} `json:"message"`
		TotalDuration      int64 `json:"total_duration"`
		LoadDuration       int64 `json:"load_duration"`
		PromptEvalCount    int   `json:"prompt_eval_count"`
		PromptEvalDuration int64 `json:"prompt_eval_duration"`
		EvalCount          int   `json:"eval_count"`
		EvalDuration       int64 `json:"eval_duration"`
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		var chunk struct {
			Model   string `json:"model"`
			Done    bool   `json:"done"`
			Error   string `json:"error"`
			Message struct {
				Content  string `json:"content"`
				Thinking string `json:"thinking"`
			} `json:"message"`
			TotalDuration      int64 `json:"total_duration"`
			LoadDuration       int64 `json:"load_duration"`
			PromptEvalCount    int   `json:"prompt_eval_count"`
			PromptEvalDuration int64 `json:"prompt_eval_duration"`
			EvalCount          int   `json:"eval_count"`
			EvalDuration       int64 `json:"eval_duration"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if strings.TrimSpace(chunk.Error) != "" {
			message := compactError(chunk.Error)
			if strings.Contains(strings.ToLower(message), "not found") || strings.Contains(strings.ToLower(message), "does not exist") {
				return time.Since(started).Milliseconds(), nil, &Error{Code: "model_not_found", Message: message}
			}
			return time.Since(started).Milliseconds(), nil, &Error{Code: "provider_request_failed", Message: message}
		}
		if firstTokenAt.IsZero() && (chunk.Message.Content != "" || chunk.Message.Thinking != "") {
			firstTokenAt = time.Now()
		}
		if chunk.Model != "" {
			final.Model = chunk.Model
		}
		if chunk.Done {
			final.Done = true
			final.Error = chunk.Error
			final.TotalDuration = chunk.TotalDuration
			final.LoadDuration = chunk.LoadDuration
			final.PromptEvalCount = chunk.PromptEvalCount
			final.PromptEvalDuration = chunk.PromptEvalDuration
			final.EvalCount = chunk.EvalCount
			final.EvalDuration = chunk.EvalDuration
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return time.Since(started).Milliseconds(), nil, &Error{Code: "request_cancelled", Message: "The AI request was cancelled"}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return time.Since(started).Milliseconds(), nil, &Error{Code: "request_timed_out", Message: "The AI provider request timed out"}
		}
		return time.Since(started).Milliseconds(), nil, &Error{Code: "provider_request_failed", Message: "Ollama's streaming model-check response ended unexpectedly"}
	}
	latency := time.Since(started).Milliseconds()
	if !final.Done || strings.TrimSpace(final.Model) == "" {
		return latency, nil, &Error{Code: "provider_request_failed", Message: "Ollama returned no completed model-check response"}
	}

	performance := &ModelPerformance{
		MeasuredAt: time.Now().UTC(), ResponseDurationMS: latency,
		PromptTokens: final.PromptEvalCount, GeneratedTokens: final.EvalCount,
	}
	if final.TotalDuration > 0 {
		performance.ResponseDurationMS = final.TotalDuration / int64(time.Millisecond)
	}
	if final.LoadDuration > 0 {
		performance.LoadDurationMS = final.LoadDuration / int64(time.Millisecond)
	}
	if !firstTokenAt.IsZero() {
		performance.TimeToFirstTokenMS = firstTokenAt.Sub(started).Milliseconds()
		if performance.TimeToFirstTokenMS < 1 {
			// The API stores whole milliseconds; preserve a measured sub-ms first
			// token as 1 ms rather than making it look like no sample was captured.
			performance.TimeToFirstTokenMS = 1
		}
	}
	if final.PromptEvalDuration > 0 && final.PromptEvalCount > 0 {
		performance.PromptTokensPerSecond = float64(final.PromptEvalCount) * float64(time.Second) / float64(final.PromptEvalDuration)
	}
	if final.EvalDuration > 0 && final.EvalCount > 0 {
		performance.TokensPerSecond = float64(final.EvalCount) * float64(time.Second) / float64(final.EvalDuration)
	}
	return latency, performance, nil
}

// ollamaModelDetails fetches compact architecture facts from /api/show. It is
// best-effort: model availability and measured performance are valid even if an
// older Ollama version does not expose these optional fields.
func ollamaModelDetails(ctx context.Context, cfg ProviderConfig, modelID string) (int, map[string]any, map[string]bool) {
	_, data, err := requestJSON(ctx, providerHTTPClient(cfg), http.MethodPost, endpoint(cfg.BaseURL, "api/show"), "", map[string]any{"model": modelID, "verbose": false})
	if err != nil {
		return 0, nil, nil
	}
	var response struct {
		Details struct {
			Format            string   `json:"format"`
			Family            string   `json:"family"`
			Families          []string `json:"families"`
			ParameterSize     string   `json:"parameter_size"`
			QuantizationLevel string   `json:"quantization_level"`
		} `json:"details"`
		ModelInfo    map[string]any `json:"model_info"`
		Capabilities []string       `json:"capabilities"`
	}
	if json.Unmarshal(data, &response) != nil {
		return 0, nil, nil
	}
	contextSize := 0
	metadata := make(map[string]any)
	for key, value := range response.ModelInfo {
		lower := strings.ToLower(key)
		switch {
		case strings.HasSuffix(lower, ".context_length"):
			if n, ok := numericInt(value); ok && n > 0 {
				contextSize = n
				metadata["context_length"] = n
			}
		case strings.HasSuffix(lower, ".block_count"):
			if n, ok := numericInt(value); ok && n > 0 {
				metadata["block_count"] = n
			}
		case strings.HasSuffix(lower, ".embedding_length"):
			if n, ok := numericInt(value); ok && n > 0 {
				metadata["embedding_length"] = n
			}
		case lower == "general.parameter_count":
			if n, ok := numericInt(value); ok && n > 0 {
				metadata["parameter_count"] = n
			}
		}
	}
	capabilities := make(map[string]bool)
	for _, value := range response.Capabilities {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "completion":
			capabilities["text_generation"] = true
		case "tools":
			capabilities["tools"] = true
		case "thinking":
			capabilities["thinking"] = true
		case "vision":
			capabilities["vision"] = true
		case "embedding":
			capabilities["embeddings"] = true
		}
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	if len(capabilities) == 0 {
		capabilities = nil
	}
	return contextSize, metadata, capabilities
}

func numericInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), number > 0
	case int:
		return number, number > 0
	case int64:
		return int(number), number > 0
	case json.Number:
		parsed, err := number.Int64()
		return int(parsed), err == nil && parsed > 0
	default:
		return 0, false
	}
}

func (p *ollamaProvider) Generate(ctx context.Context, input GenerateRequest) (GenerateResponse, error) {
	started := time.Now()
	options := map[string]any{}
	if input.Temperature != nil {
		options["temperature"] = *input.Temperature
	}
	payload := ollamaPayload(input, false)
	if input.MaxTokens > 0 {
		options["num_predict"] = input.MaxTokens
	}
	if len(options) > 0 {
		payload["options"] = options
	}
	_, data, err := requestJSON(ctx, p.client, http.MethodPost, endpoint(p.cfg.BaseURL, "api/chat"), "", payload)
	if err != nil {
		return GenerateResponse{}, err
	}
	var parsed struct {
		Model   string `json:"model"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.Error != "" {
		return GenerateResponse{}, &Error{Code: "provider_request_failed", Message: "Ollama returned an unreadable or failed completion"}
	}
	model := parsed.Model
	if model == "" {
		model = input.Model
	}
	return GenerateResponse{ProviderID: p.cfg.ID, Model: model, Content: parsed.Message.Content, DurationMS: time.Since(started).Milliseconds()}, nil
}

func (p *ollamaProvider) Stream(ctx context.Context, input GenerateRequest, emit func(string) error) error {
	options := map[string]any{}
	if input.Temperature != nil {
		options["temperature"] = *input.Temperature
	}
	if input.MaxTokens > 0 {
		options["num_predict"] = input.MaxTokens
	}
	payload := ollamaPayload(input, true)
	if len(options) > 0 {
		payload["options"] = options
	}
	encoded, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(p.cfg.BaseURL, "api/chat"), bytes.NewReader(encoded))
	if err != nil {
		return &Error{Code: "invalid_provider_configuration", Message: "Could not create an Ollama streaming request"}
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return &Error{Code: "request_cancelled", Message: "The AI request was cancelled"}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return &Error{Code: "request_timed_out", Message: "The AI provider request timed out"}
		}
		return &Error{Code: "provider_unavailable", Message: "Could not reach the configured Ollama service"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return normalizedHTTPError(response.StatusCode, body)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &chunk) != nil {
			continue
		}
		if chunk.Error != "" {
			return &Error{Code: "provider_request_failed", Message: "Ollama reported a streaming error"}
		}
		if chunk.Message.Content != "" {
			if err := emit(chunk.Message.Content); err != nil {
				return err
			}
		}
		if chunk.Done {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return &Error{Code: "provider_request_failed", Message: "Ollama stream ended unexpectedly"}
	}
	return nil
}

func (p *ollamaProvider) Health(ctx context.Context) HealthStatus {
	started := time.Now()
	_, _, err := requestJSON(ctx, p.client, http.MethodGet, endpoint(p.cfg.BaseURL, "api/tags"), "", nil)
	status := HealthStatus{CheckedAt: time.Now().UTC(), LatencyMS: time.Since(started).Milliseconds()}
	if err != nil {
		status.State = "unavailable"
		status.ErrorCode = errorCode(err)
		status.Message = safeMessage(err)
		return status
	}
	status.State = "healthy"
	status.Message = "Ollama responded successfully"
	return status
}

func isConfigLocal(cfg ProviderConfig) bool {
	if cfg.Type == ProviderOllama {
		return true
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".lan") || strings.HasSuffix(host, ".internal") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip != nil && ip.IsLoopback() {
		return true
	}
	if cfg.TrustedLocal {
		return !strings.Contains(host, ".") || (ip != nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast()))
	}
	return false
}

func errorCode(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return "provider_unavailable"
}

func safeMessage(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Message
	}
	return "The AI provider is unavailable"
}
