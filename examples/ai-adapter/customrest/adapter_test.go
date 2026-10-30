package customrest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"idios/internal/ai"
)

func TestReferenceAdapterCatalogueAndGeneration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"models":[{"id":"my-agent-q4","name":"My Agent","architecture":"GPT","parameter_size":"160M","quantization_level":"Q4_K_M","size_bytes":180000000,"capabilities":{"text_generation":true,"structured_output":true},"metadata":{"vocab":50257}}]}`))
		case "/v1/generate":
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if request["model"] != "my-agent-q4" {
				t.Errorf("model = %#v", request["model"])
			}
			if _, ok := request["response_format"].(map[string]any); !ok {
				t.Errorf("response_format not passed through: %#v", request["response_format"])
			}
			_, _ = w.Write([]byte(`{"model":"my-agent-q4","content":"{\"ok\":true}"}`))
		case "/v1/generate/stream":
			if r.Header.Get("Accept") != "text/event-stream" {
				t.Errorf("stream Accept = %q", r.Header.Get("Accept"))
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"delta\":\"hello\"}\n\ndata: {\"delta\":\" world\"}\n\ndata: {\"done\":true}\n\n"))
		case "/v1/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := NewProvider(ai.ProviderConfig{
		ID: "personal-agent", Name: "Personal Agent", Type: ai.ProviderCustom, AdapterID: adapterID,
		BaseURL: server.URL + "/v1", Enabled: true,
		AdapterConfig: map[string]any{"supports_streaming": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "my-agent-q4" || models[0].ParameterSize != "160M" || models[0].Metadata["vocab"].(float64) != 50257 {
		t.Fatalf("models = %#v", models)
	}
	request := ai.GenerateRequest{Model: "my-agent-q4", Messages: []ai.Message{{Role: "user", Content: "hello"}}, ResponseFormat: &ai.ResponseFormat{Type: "json_schema", Name: "answer", Schema: map[string]any{"type": "object"}}}
	response, err := provider.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != `{"ok":true}` || response.ProviderID != "personal-agent" {
		t.Fatalf("response = %#v", response)
	}
	var streamed strings.Builder
	if err := provider.Stream(context.Background(), request, func(chunk string) error {
		streamed.WriteString(chunk)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if streamed.String() != "hello world" {
		t.Fatalf("streamed output = %q", streamed.String())
	}
	if status := provider.Health(context.Background()); status.State != "healthy" {
		t.Fatalf("health = %#v", status)
	}
}

func TestReferenceAdapterDoesNotNeedNetworkForUnknownCustomConfig(t *testing.T) {
	if _, err := NewProvider(ai.ProviderConfig{ID: "bad", Name: "Bad", Type: ai.ProviderCustom, AdapterID: adapterID, BaseURL: "http://example.com", AdapterConfig: map[string]any{}}); err == nil {
		t.Fatal("untrusted remote HTTP endpoint should be rejected")
	}
}
