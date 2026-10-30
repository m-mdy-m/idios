// Package customrest is a reference adapter for personal-model inference servers.
// It is not linked into the default IDIOS server; import it from a server entry
// point (for example with a blank import) to register custom-rest-v1.
package customrest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"idios/internal/ai"
)

const adapterID = "custom-rest-v1"

func init() {
	if err := ai.RegisterProviderAdapter(ai.ProviderAdapterDescriptor{
		ID:          adapterID,
		Name:        "Custom REST adapter (example)",
		Version:     "1.0.0",
		Description: "Reference adapter for a user-owned inference REST service",
		Protocol:    "custom-rest-v1",
	}, NewProvider); err != nil {
		panic(err)
	}
}

// NewProvider creates the reference adapter. cfg.AdapterConfig may override
// models_path, generate_path, health_path, and stream_path. Set
// supports_streaming=true only if stream_path emits the documented SSE shape.
func NewProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
	client, err := ai.NewProviderHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Provider{cfg: cfg, client: client}, nil
}

type Provider struct {
	cfg    ai.ProviderConfig
	client *http.Client
}

func (p *Provider) ID() string            { return p.cfg.ID }
func (p *Provider) Kind() ai.ProviderKind { return ai.ProviderCustom }
func (p *Provider) Local() bool {
	u, err := url.Parse(p.cfg.BaseURL)
	if err != nil {
		return false
	}
	host := strings.Trim(strings.ToLower(u.Hostname()), "[]")
	if host == "localhost" || p.cfg.TrustedLocal {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (p *Provider) configString(key, fallback string) string {
	if value, ok := p.cfg.AdapterConfig[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func (p *Provider) configBool(key string) bool {
	value, _ := p.cfg.AdapterConfig[key].(bool)
	return value
}

func (p *Provider) endpoint(path string) (string, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(p.cfg.BaseURL), "/"))
	if err != nil || base.Hostname() == "" {
		return "", &ai.Error{Code: "invalid_provider_configuration", Message: "The custom adapter base URL is invalid"}
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", &ai.Error{Code: "invalid_provider_configuration", Message: "The custom adapter endpoint path is empty"}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func (p *Provider) request(ctx context.Context, method, path string, payload any) (*http.Response, error) {
	return p.requestWithAccept(ctx, method, path, payload, "application/json")
}

func (p *Provider) requestWithAccept(ctx context.Context, method, path string, payload any, accept string) (*http.Response, error) {
	target, err := p.endpoint(path)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return nil, &ai.Error{Code: "invalid_request", Message: "The custom adapter could not encode the request"}
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, &ai.Error{Code: "invalid_provider_configuration", Message: "The custom adapter request could not be created"}
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", accept)
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	}
	response, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return nil, &ai.Error{Code: "request_cancelled", Message: "The custom adapter request was cancelled"}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &ai.Error{Code: "request_timed_out", Message: "The custom adapter request timed out"}
		}
		return nil, &ai.Error{Code: "provider_unavailable", Message: "Could not reach the custom inference service"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, &ai.Error{Code: "provider_request_failed", Message: fmt.Sprintf("The custom inference service returned HTTP %d", response.StatusCode)}
	}
	return response, nil
}

func readBounded(response *http.Response, limit int64) ([]byte, error) {
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, &ai.Error{Code: "provider_request_failed", Message: "Could not read the custom inference response"}
	}
	if int64(len(data)) > limit {
		return nil, &ai.Error{Code: "resource_limit_exceeded", Message: "The custom inference response exceeded the 4 MiB limit"}
	}
	return data, nil
}

type modelRecord struct {
	ID                string          `json:"id"`
	Name              string          `json:"name,omitempty"`
	Architecture      string          `json:"architecture,omitempty"`
	Family            string          `json:"family,omitempty"`
	ParameterSize     string          `json:"parameter_size,omitempty"`
	QuantizationLevel string          `json:"quantization_level,omitempty"`
	Format            string          `json:"format,omitempty"`
	SizeBytes         int64           `json:"size_bytes,omitempty"`
	ModifiedAt        *time.Time      `json:"modified_at,omitempty"`
	ContextSize       int             `json:"context_size,omitempty"`
	Capabilities      map[string]bool `json:"capabilities,omitempty"`
	Metadata          map[string]any  `json:"metadata,omitempty"`
}

func (p *Provider) ListModels(ctx context.Context) ([]ai.Model, error) {
	response, err := p.request(ctx, http.MethodGet, p.configString("models_path", "/models"), nil)
	if err != nil {
		return nil, err
	}
	data, err := readBounded(response, 4<<20)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Models []modelRecord `json:"models"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, &ai.Error{Code: "provider_request_failed", Message: "The custom model catalogue was not valid JSON"}
	}
	models := make([]ai.Model, 0, len(parsed.Models))
	for _, item := range parsed.Models {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		capabilities := item.Capabilities
		if capabilities == nil {
			capabilities = map[string]bool{"text_generation": true}
		}
		models = append(models, ai.Model{
			ID: id, Name: name, Architecture: item.Architecture, Family: item.Family,
			ParameterSize: item.ParameterSize, QuantizationLevel: item.QuantizationLevel,
			Format: item.Format, SizeBytes: item.SizeBytes, ModifiedAt: item.ModifiedAt,
			ContextSize: item.ContextSize, Capabilities: capabilities, Metadata: item.Metadata,
			Discovered: time.Now().UTC(),
		})
	}
	return models, nil
}

type generationRequest struct {
	Model          string             `json:"model"`
	Messages       []ai.Message       `json:"messages"`
	Temperature    *float64           `json:"temperature,omitempty"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	ResponseFormat *ai.ResponseFormat `json:"response_format,omitempty"`
}

type generationResponse struct {
	Model   string `json:"model,omitempty"`
	Content string `json:"content"`
}

func (p *Provider) Generate(ctx context.Context, input ai.GenerateRequest) (ai.GenerateResponse, error) {
	started := time.Now()
	payload := generationRequest{Model: input.Model, Messages: input.Messages, Temperature: input.Temperature, MaxTokens: input.MaxTokens, ResponseFormat: input.ResponseFormat}
	response, err := p.request(ctx, http.MethodPost, p.configString("generate_path", "/generate"), payload)
	if err != nil {
		return ai.GenerateResponse{}, err
	}
	data, err := readBounded(response, 4<<20)
	if err != nil {
		return ai.GenerateResponse{}, err
	}
	var parsed generationResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ai.GenerateResponse{}, &ai.Error{Code: "provider_request_failed", Message: "The custom inference response was not valid JSON"}
	}
	if parsed.Model == "" {
		parsed.Model = input.Model
	}
	return ai.GenerateResponse{ProviderID: p.cfg.ID, Model: parsed.Model, Content: parsed.Content, DurationMS: time.Since(started).Milliseconds()}, nil
}

// Stream uses native SSE when supports_streaming=true; otherwise it emits the
// completed response as one chunk so callers still receive a terminal event.
// A model should only advertise the streaming capability when its service really
// sends incremental SSE chunks.
func (p *Provider) Stream(ctx context.Context, input ai.GenerateRequest, emit func(string) error) error {
	if !p.configBool("supports_streaming") {
		response, err := p.Generate(ctx, input)
		if err != nil {
			return err
		}
		return emit(response.Content)
	}
	payload := generationRequest{Model: input.Model, Messages: input.Messages, Temperature: input.Temperature, MaxTokens: input.MaxTokens, ResponseFormat: input.ResponseFormat}
	response, err := p.requestWithAccept(ctx, http.MethodPost, p.configString("stream_path", "/generate/stream"), payload, "text/event-stream")
	if err != nil {
		return err
	}
	defer response.Body.Close()
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
			Content string `json:"content"`
			Delta   string `json:"delta"`
			Done    bool   `json:"done"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return &ai.Error{Code: "provider_request_failed", Message: "The custom inference stream reported an error"}
		}
		text := chunk.Delta
		if text == "" {
			text = chunk.Content
		}
		if text != "" {
			if err := emit(text); err != nil {
				return err
			}
		}
		if chunk.Done {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return &ai.Error{Code: "provider_request_failed", Message: "The custom inference stream ended unexpectedly"}
	}
	return nil
}

func (p *Provider) Health(ctx context.Context) ai.HealthStatus {
	started := time.Now()
	response, err := p.request(ctx, http.MethodGet, p.configString("health_path", "/health"), nil)
	status := ai.HealthStatus{CheckedAt: time.Now().UTC(), LatencyMS: time.Since(started).Milliseconds()}
	if err != nil {
		status.State = "unavailable"
		if typed, ok := err.(*ai.Error); ok {
			status.ErrorCode = typed.Code
			status.Message = typed.Message
		} else {
			status.ErrorCode = "provider_unavailable"
			status.Message = "Could not reach the custom inference service"
		}
		return status
	}
	response.Body.Close()
	status.State = "healthy"
	status.Message = "Custom inference service responded successfully"
	return status
}
