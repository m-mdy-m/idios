package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"idios/internal/ai"
)

func TestAIRuntimeStatusAndProviderSecretsStayPrivate(t *testing.T) {
	server := &Server{}
	status := httptest.NewRecorder()
	server.getAIRuntimeStatus(status, httptest.NewRequest(http.MethodGet, "/api/ai/status", nil))
	if status.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d: %s", status.Code, http.StatusOK, status.Body.String())
	}
	if !strings.Contains(status.Body.String(), `"mode":"local-first"`) {
		t.Fatalf("default execution mode missing: %s", status.Body.String())
	}

	testProviderSecret := strings.Join([]string{"test", "credential", "not-real"}, "-")
	input, err := json.Marshal(map[string]any{"provider": ai.ProviderConfig{ID: "test-local", Name: "Test local", Type: ai.ProviderOpenAI, BaseURL: "http://127.0.0.1:1234/v1", APIKey: testProviderSecret, DefaultModel: "test-model", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	created := httptest.NewRecorder()
	server.createAIProvider(created, httptest.NewRequest(http.MethodPost, "/api/ai/providers", strings.NewReader(string(input))))
	if created.Code != http.StatusCreated {
		t.Fatalf("create provider status = %d, want %d: %s", created.Code, http.StatusCreated, created.Body.String())
	}
	if strings.Contains(created.Body.String(), testProviderSecret) || strings.Contains(created.Body.String(), `"api_key"`) {
		t.Fatalf("provider response leaked secret: %s", created.Body.String())
	}

	listed := httptest.NewRecorder()
	server.listAIProviders(listed, httptest.NewRequest(http.MethodGet, "/api/ai/providers", nil))
	if listed.Code != http.StatusOK {
		t.Fatalf("list provider status = %d: %s", listed.Code, listed.Body.String())
	}
	if strings.Contains(listed.Body.String(), testProviderSecret) {
		t.Fatalf("provider list leaked secret: %s", listed.Body.String())
	}
}

func TestAIModelsEndpointReturnsEmptyArrayInsteadOfNull(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	server.listAIModels(response, httptest.NewRequest(http.MethodGet, "/api/ai/models", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"models":[]`) {
		t.Fatalf("empty model collection must be JSON [], not null: %s", response.Body.String())
	}
}

func TestRefreshAIModelsContactsOnlySelectedProvider(t *testing.T) {
	var firstRequests atomic.Int32
	var secondRequests atomic.Int32
	newCatalogueServer := func(counter *atomic.Int32, modelID string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			if r.URL.Path != "/v1/models" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"` + modelID + `"}]}`))
		}))
	}
	first := newCatalogueServer(&firstRequests, "first-model")
	defer first.Close()
	second := newCatalogueServer(&secondRequests, "second-model")
	defer second.Close()

	server := &Server{}
	manager, err := server.aiManager()
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	for _, provider := range []ai.ProviderConfig{
		{ID: "first", Name: "First provider", Type: ai.ProviderOpenAI, BaseURL: first.URL + "/v1", Enabled: true, TrustedLocal: true},
		{ID: "second", Name: "Second provider", Type: ai.ProviderOpenAI, BaseURL: second.URL + "/v1", Enabled: true, TrustedLocal: true},
	} {
		if _, err := manager.UpsertProvider(provider, false); err != nil {
			t.Fatalf("UpsertProvider(%s): %v", provider.ID, err)
		}
	}
	// Seed the unselected provider's cache, then reset its counter. A targeted refresh
	// must preserve this cached model without sending another request to that provider.
	if _, err := manager.RefreshProviderModels(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "second"); err != nil {
		t.Fatalf("seed second provider catalogue: %v", err)
	}
	secondRequests.Store(0)

	request := httptest.NewRequest(http.MethodPost, "/api/ai/models/refresh", strings.NewReader(`{"provider_id":"first"}`))
	response := httptest.NewRecorder()
	server.refreshAIModels(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if firstRequests.Load() != 1 {
		t.Fatalf("selected provider requests = %d, want 1", firstRequests.Load())
	}
	if secondRequests.Load() != 0 {
		t.Fatalf("unselected provider requests = %d, want 0", secondRequests.Load())
	}
	if !strings.Contains(response.Body.String(), `"provider_id":"first"`) || !strings.Contains(response.Body.String(), `"first-model"`) {
		t.Fatalf("response does not identify selected provider and its model: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"second-model"`) {
		t.Fatalf("refresh discarded the unselected provider's cached catalogue: %s", response.Body.String())
	}
}

func TestRefreshAIModelsRejectsRemoteProviderInOfflineOnly(t *testing.T) {
	server := &Server{}
	manager, err := server.aiManager()
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	config := manager.RuntimeConfig()
	config.Mode = ai.ModeOfflineOnly
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.UpsertProvider(ai.ProviderConfig{ID: "remote", Name: "Remote", Type: ai.ProviderOpenAI, BaseURL: "https://example.invalid/v1", Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/ai/models/refresh", strings.NewReader(`{"provider_id":"remote"}`))
	response := httptest.NewRecorder()
	server.refreshAIModels(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 in Offline-only: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "offline_policy_blocked") {
		t.Fatalf("expected server-side offline policy error: %s", response.Body.String())
	}
}

func TestRefreshAIModelsRequiresProviderSelection(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	server.refreshAIModels(response, httptest.NewRequest(http.MethodPost, "/api/ai/models/refresh", strings.NewReader(`{}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when no provider is selected: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Choose one provider") {
		t.Fatalf("missing provider-selection guidance: %s", response.Body.String())
	}
}

func TestOfflineOnlyBlocksLegacyShelfCloudRequest(t *testing.T) {
	server := &Server{}
	manager, err := server.aiManager()
	if err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ai.ModeOfflineOnly
	config.AllowCloudFallback = true // even a stale/contradictory fallback flag cannot bypass Offline-only
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/shelf/ai/chat", strings.NewReader(`{"base_url":"https://openrouter.ai/api/v1","model":"provider/model","messages":[{"role":"user","content":"must not go online"}]}`))
	response := httptest.NewRecorder()
	server.shelfAIChat(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "offline_policy_blocked") {
		t.Fatalf("expected explicit Offline-only policy error: %s", response.Body.String())
	}
}

func TestAIRuntimeRoutesRegisterWithoutPatternConflict(t *testing.T) {
	// Go's ServeMux panics on conflicting patterns. Constructing the real router verifies
	// the complete AI route surface remains compatible with the rest of the application.
	_ = (&Server{}).routes()
}

func TestAIModelTestEndpointReturnsPerModelAvailability(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"local-check-model"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"model":"local-check-model","choices":[{"message":{"content":"OK"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()

	server := &Server{}
	manager, err := server.aiManager()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.UpsertProvider(ai.ProviderConfig{ID: "check-local", Name: "Check local", Type: ai.ProviderOpenAI, BaseURL: providerServer.URL + "/v1", Enabled: true, TrustedLocal: true}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RefreshModels(httptest.NewRequest(http.MethodPost, "/", nil).Context()); err != nil {
		t.Fatal(err)
	}
	body := `{"provider_id":"check-local","model_id":"local-check-model"}`
	request := httptest.NewRequest(http.MethodPost, "/api/ai/models/test", strings.NewReader(body))
	response := httptest.NewRecorder()
	server.testAIModel(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("model test status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"availability":"available"`) {
		t.Fatalf("model test response does not expose availability: %s", response.Body.String())
	}
}

func TestAIAdaptersEndpointListsBuiltInAdapters(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	server.listAIAdapters(response, httptest.NewRequest(http.MethodGet, "/api/ai/adapters", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	for _, adapterID := range []string{"openrouter", "openai-compatible", "ollama-native"} {
		if !strings.Contains(response.Body.String(), `"id":"`+adapterID+`"`) {
			t.Fatalf("adapter %q missing from catalogue: %s", adapterID, response.Body.String())
		}
	}
}
