package ai

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureCustomProvider struct {
	cfg ProviderConfig
}

func (p *fixtureCustomProvider) ID() string         { return p.cfg.ID }
func (p *fixtureCustomProvider) Kind() ProviderKind { return ProviderCustom }
func (p *fixtureCustomProvider) Local() bool        { return true }
func (p *fixtureCustomProvider) ListModels(context.Context) ([]Model, error) {
	return []Model{{ID: "fixture-model", Name: "Fixture", Family: "custom-family", Metadata: map[string]any{"schema_version": 2}}}, nil
}
func (p *fixtureCustomProvider) Generate(_ context.Context, request GenerateRequest) (GenerateResponse, error) {
	content := "fixture"
	if request.ResponseFormat != nil && request.ResponseFormat.Type == "json_schema" {
		content = `{"ok":true}`
	}
	return GenerateResponse{ProviderID: p.cfg.ID, Model: request.Model, Content: content}, nil
}
func (p *fixtureCustomProvider) Stream(_ context.Context, request GenerateRequest, emit func(string) error) error {
	return emit("fixture stream")
}
func (p *fixtureCustomProvider) Health(context.Context) HealthStatus {
	return HealthStatus{State: "healthy", Message: "fixture adapter is healthy"}
}

func TestCustomProviderAdapterRegistrationAndConfig(t *testing.T) {
	const adapterID = "fixture-custom-adapter-v1"
	err := RegisterProviderAdapter(ProviderAdapterDescriptor{ID: adapterID, Name: "Fixture Adapter", Protocol: "fixture-v1"}, func(cfg ProviderConfig) (Provider, error) {
		if got := cfg.AdapterConfig["checkpoint"]; got != "./models/agent" {
			t.Fatalf("adapter config checkpoint = %#v", got)
		}
		return &fixtureCustomProvider{cfg: cfg}, nil
	})
	if err != nil {
		t.Fatalf("register adapter: %v", err)
	}
	cfg := ProviderConfig{ID: "personal-agent", Name: "Personal Agent", Type: ProviderCustom, AdapterID: adapterID, BaseURL: "http://127.0.0.1:43123", Enabled: true, AdapterConfig: map[string]any{"checkpoint": "./models/agent"}}
	provider, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("create custom provider: %v", err)
	}
	if provider.ID() != "personal-agent" || provider.Kind() != ProviderCustom {
		t.Fatalf("provider identity/kind = %q/%q", provider.ID(), provider.Kind())
	}
	models, err := provider.ListModels(context.Background())
	if err != nil || len(models) != 1 || models[0].Metadata["schema_version"] != 2 {
		t.Fatalf("custom model metadata = %#v, err = %v", models, err)
	}
	found := false
	for _, item := range AvailableProviderAdapters() {
		if item.ID == adapterID && !item.BuiltIn && item.Name == "Fixture Adapter" {
			found = true
		}
	}
	if !found {
		t.Fatal("registered adapter descriptor was not listed")
	}
}

func TestCustomProviderRequiresRegisteredAdapter(t *testing.T) {
	err := ValidateProviderConfig(ProviderConfig{ID: "unknown", Name: "Unknown", Type: ProviderCustom, AdapterID: "not-registered", BaseURL: "http://127.0.0.1:43123"})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("validation error = %v, want unregistered adapter error", err)
	}
}

func TestStructuredOutputAdapterPayloads(t *testing.T) {
	request := GenerateRequest{
		Model:          "personal-agent",
		Messages:       []Message{{Role: "user", Content: "summarize"}},
		ResponseFormat: &ResponseFormat{Type: "json_schema", Name: "study_note", Strict: true, Schema: map[string]any{"type": "object", "properties": map[string]any{"summary": map[string]any{"type": "string"}}}},
	}
	openAI := openAIPayload(request, false)
	responseFormat, ok := openAI["response_format"].(map[string]any)
	if !ok || responseFormat["type"] != "json_schema" {
		t.Fatalf("OpenAI response_format = %#v", openAI["response_format"])
	}
	inner, ok := responseFormat["json_schema"].(map[string]any)
	if !ok || inner["name"] != "study_note" || inner["strict"] != true {
		t.Fatalf("OpenAI json_schema = %#v", responseFormat["json_schema"])
	}
	ollama := ollamaPayload(request, false)
	if _, ok := ollama["format"].(map[string]any); !ok {
		t.Fatalf("Ollama format = %#v, want JSON schema object", ollama["format"])
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded GenerateRequest
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.ResponseFormat == nil || decoded.ResponseFormat.Name != "study_note" {
		t.Fatalf("response format JSON round-trip failed: request=%s decoded=%#v err=%v", encoded, decoded.ResponseFormat, err)
	}
}

func TestFirstPersistentRuntimeSeedsOllamaLocalOnlyOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "ai-runtime.json")
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	providers := manager.Providers()
	if len(providers) != 1 || providers[0].ID != "ollama-local" || providers[0].Name != "Ollama Local" || providers[0].Type != ProviderOllama {
		t.Fatalf("initial persistent providers = %#v", providers)
	}
	if err := manager.DeleteProvider("ollama-local"); err != nil {
		t.Fatal(err)
	}
	manager.Close()

	reopened, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.Providers()) != 0 {
		t.Fatalf("deleted Ollama provider was unexpectedly re-seeded: %#v", reopened.Providers())
	}
}

func TestCustomProviderBaseURLIsStillValidated(t *testing.T) {
	// Adapter registration does not bypass the normal endpoint trust checks.
	const adapterID = "fixture-custom-url-v1"
	if err := RegisterProviderAdapter(ProviderAdapterDescriptor{ID: adapterID, Name: "Fixture URL"}, func(cfg ProviderConfig) (Provider, error) {
		return &fixtureCustomProvider{cfg: cfg}, nil
	}); err != nil {
		t.Fatal(err)
	}
	err := ValidateProviderConfig(ProviderConfig{ID: "unsafe", Name: "Unsafe", Type: ProviderCustom, AdapterID: adapterID, BaseURL: "http://example.com"})
	if err == nil {
		t.Fatal("expected untrusted remote HTTP endpoint to be rejected")
	}
	if _, err := url.Parse("http://127.0.0.1:43123"); err != nil {
		t.Fatal(err)
	}
}

func TestExistingDefaultOllamaProviderIsRenamedWithoutChangingID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "ai-runtime.json")
	state := persistedState{
		Config:    RuntimeConfig{Mode: ModeCloudFirst, MaxConcurrent: 2, MaxQueued: 16, WorkflowTimeoutSeconds: 600, MaxAgents: 4, MaxDiscussionRounds: 2},
		Providers: []ProviderConfig{{ID: "ollama", Name: "Ollama (native)", Type: ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true}},
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	providers := manager.Providers()
	if len(providers) != 1 || providers[0].ID != "ollama" || providers[0].Name != "Ollama Local" {
		t.Fatalf("migrated providers = %#v; existing IDs should be preserved", providers)
	}
}

func TestStructuredOutputRequestValidation(t *testing.T) {
	base := GenerateRequest{Messages: []Message{{Role: "user", Content: "hello"}}}
	base.ResponseFormat = &ResponseFormat{Type: "unexpected"}
	if err := validateGenerateRequest(base); err == nil {
		t.Fatal("unsupported response format should be rejected")
	}
	base.ResponseFormat = &ResponseFormat{Type: "json_schema"}
	if err := validateGenerateRequest(base); err == nil {
		t.Fatal("json_schema without schema should be rejected")
	}
	base.ResponseFormat = &ResponseFormat{Type: "json_schema", Schema: map[string]any{"type": "object"}}
	if err := validateGenerateRequest(base); err != nil {
		t.Fatalf("valid JSON schema should pass validation: %v", err)
	}
}

func TestSwitchingCustomProviderToBuiltinClearsAdapterConfiguration(t *testing.T) {
	const adapterID = "fixture-switch-cleanup-v1"
	if err := RegisterProviderAdapter(ProviderAdapterDescriptor{ID: adapterID, Name: "Fixture Switch Cleanup"}, func(cfg ProviderConfig) (Provider, error) {
		return &fixtureCustomProvider{cfg: cfg}, nil
	}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "switch-me", Name: "Switch me", Type: ProviderCustom, AdapterID: adapterID, AdapterConfig: map[string]any{"private_path": "./checkpoint"}, BaseURL: "http://127.0.0.1:43123", Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "switch-me", Name: "Switch me", Type: ProviderOpenAI, BaseURL: "http://127.0.0.1:43124/v1", Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	for _, cfg := range manager.state.Providers {
		if cfg.ID == "switch-me" {
			if cfg.AdapterID != "" || cfg.AdapterConfig != nil {
				t.Fatalf("built-in provider retained custom extension configuration: %#v", cfg)
			}
			return
		}
	}
	t.Fatal("provider disappeared after type switch")
}
