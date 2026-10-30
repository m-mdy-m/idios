package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateShelfAIBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "local 9router", url: "http://127.0.0.1:20128/v1"},
		{name: "localhost router", url: "http://localhost:20128/v1"},
		{name: "remote tls provider", url: "https://openrouter.ai/api/v1"},
		{name: "reject remote plaintext", url: "http://example.com/v1", wantErr: true},
		{name: "reject file scheme", url: "file:///tmp/ai", wantErr: true},
		{name: "reject credentials embedded in URL", url: "https://user:pass@example.com/v1", wantErr: true},
		{name: "reject query string", url: "https://example.com/v1?key=secret", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateShelfAIBaseURL(test.url)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateShelfAIBaseURL(%q) error = %v, wantErr %v", test.url, err, test.wantErr)
			}
			if err == nil && got == nil {
				t.Fatal("expected parsed URL")
			}
		})
	}
}

func TestShelfAIURLPreservesV1BasePath(t *testing.T) {
	base, err := validateShelfAIBaseURL("https://openrouter.ai/api/v1/")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := shelfAIURL(base, "chat/completions"), "https://openrouter.ai/api/v1/chat/completions"; got != want {
		t.Fatalf("shelfAIURL() = %q, want %q", got, want)
	}
}

func TestShelfAILocalProviderCanOmitAPIKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("unexpected Authorization header for keyless local provider: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"local-model"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"local reply"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	server := &Server{}
	models := httptest.NewRecorder()
	modelRequest := httptest.NewRequest(http.MethodPost, "/api/shelf/ai/models", strings.NewReader(`{"base_url":"`+upstream.URL+`/v1"}`))
	server.shelfAIModels(models, modelRequest)
	if models.Code != http.StatusOK {
		t.Fatalf("models status = %d, want %d; body: %s", models.Code, http.StatusOK, models.Body.String())
	}
	if !strings.Contains(models.Body.String(), `"local-model"`) {
		t.Fatalf("local model was not returned: %s", models.Body.String())
	}

	chat := httptest.NewRecorder()
	chatRequest := httptest.NewRequest(http.MethodPost, "/api/shelf/ai/chat", strings.NewReader(`{"base_url":"`+upstream.URL+`/v1","model":"local-model","messages":[{"role":"user","content":"hello"}]}`))
	server.shelfAIChat(chat, chatRequest)
	if chat.Code != http.StatusOK {
		t.Fatalf("chat status = %d, want %d; body: %s", chat.Code, http.StatusOK, chat.Body.String())
	}
	if !strings.Contains(chat.Body.String(), "local reply") {
		t.Fatalf("local chat response missing: %s", chat.Body.String())
	}
}

func TestSetShelfAIHeadersSendsGatewayBearerKey(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:20128/v1/models", nil)
	setShelfAIHeaders(request, "example-router-key")
	if got, want := request.Header.Get("Authorization"), "Bearer example-router-key"; got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
}

func TestShelfAIModelsProxiesGatewayAPIKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer example-router-key"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"provider/model-a"}]}`))
	}))
	defer upstream.Close()

	payload := `{"base_url":"` + upstream.URL + `/v1","api_key":"example-router-key"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/shelf/ai/models", strings.NewReader(payload))
	(&Server{}).shelfAIModels(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"provider/model-a"`) {
		t.Fatalf("model response was not proxied: %s", recorder.Body.String())
	}
}

func TestShelfAIChatProxiesModelAndReply(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer example-router-key"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		var payload struct {
			Model           string `json:"model"`
			ReasoningEffort string `json:"reasoning_effort"`
			Messages        []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode proxy payload: %v", err)
		}
		if payload.Model != "provider/model-a" || payload.ReasoningEffort != "high" || len(payload.Messages) != 1 || payload.Messages[0].Content != "Explain this passage" {
			t.Errorf("unexpected proxy payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"A grounded explanation."}}]}`))
	}))
	defer upstream.Close()

	payload := `{"base_url":"` + upstream.URL + `/v1","api_key":"example-router-key","model":"provider/model-a","reasoning_effort":"high","messages":[{"role":"user","content":"Explain this passage"}]}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/shelf/ai/chat", strings.NewReader(payload))
	(&Server{}).shelfAIChat(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "A grounded explanation.") {
		t.Fatalf("chat response was not proxied: %s", recorder.Body.String())
	}
}

func TestShelfAIModelsProbesEachNineRouterModel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"provider/model-ready","name":"Ready model","capabilities":{"reasoning":true}},{"id":"provider/model-down","name":"Offline model","capabilities":{"reasoning":false}}]}`))
		case "/api/models/test":
			var payload struct {
				Model string `json:"model"`
				Kind  string `json:"kind"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode model probe request: %v", err)
			}
			if payload.Kind != "llm" {
				t.Errorf("probe kind = %q, want llm", payload.Kind)
			}
			if payload.Model == "provider/model-ready" {
				_, _ = w.Write([]byte(`{"ok":true,"latencyMs":17}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":false,"latencyMs":9,"error":"provider unavailable"}`))
		default:
			t.Errorf("unexpected endpoint %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	payload := `{"base_url":"` + upstream.URL + `/v1","api_key":"example-router-key","probe":true}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/shelf/ai/models", strings.NewReader(payload))
	(&Server{}).shelfAIModels(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{`"tested":true`, `"id":"provider/model-ready"`, `"availability":"available"`, `"latency_ms":17`, `"capabilities":{"reasoning":true}`, `"id":"provider/model-down"`, `"availability":"unavailable"`, `"provider unavailable"`, `"capabilities":{"reasoning":false}`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("probe response missing %s: %s", expected, body)
		}
	}
}

func TestShelfAIReasoningCapability(t *testing.T) {
	trueValue := true
	falseValue := false
	tests := []struct {
		name  string
		model shelfAIModelDescriptor
		want  *bool
	}{
		{name: "explicit true capability", model: shelfAIModelDescriptor{Capabilities: map[string]bool{"reasoning": true}}, want: &trueValue},
		{name: "explicit false capability", model: shelfAIModelDescriptor{Capabilities: map[string]bool{"reasoning": false}}, want: &falseValue},
		{name: "thinking alias", model: shelfAIModelDescriptor{Capabilities: map[string]bool{"thinking": true}}, want: &trueValue},
		{name: "parameter support", model: shelfAIModelDescriptor{SupportedParameters: []string{"reasoning_effort"}}, want: &trueValue},
		{name: "unknown should hide control", model: shelfAIModelDescriptor{ID: "model-without-metadata"}, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := shelfAIReasoningCapability(test.model)
			if test.want == nil {
				if got != nil {
					t.Fatalf("got %v, want unknown (nil)", *got)
				}
				return
			}
			if got == nil || *got != *test.want {
				t.Fatalf("got %v, want %v", got, *test.want)
			}
		})
	}
}
