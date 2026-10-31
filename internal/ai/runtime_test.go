package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFreshRuntimeDefaultsToLocalFirst(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if got := manager.RuntimeConfig().Mode; got != ModeLocalFirst {
		t.Fatalf("fresh runtime mode = %q, want %q for local-first Ollama setup", got, ModeLocalFirst)
	}
}

func TestDiscussionAgentOrderGroupsSpecialistModelsAndKeepsChairLast(t *testing.T) {
	ids := []string{"discussion-chair", "evidence-auditor", "reader", "translator", "steelman", "tutor", "scope-keeper"}
	sort.SliceStable(ids, func(i, j int) bool { return discussionAgentPriority(ids[i]) < discussionAgentPriority(ids[j]) })
	want := []string{"reader", "tutor", "steelman", "scope-keeper", "translator", "evidence-auditor", "discussion-chair"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("discussion order = %#v, want %#v", ids, want)
	}
}

func TestEmptyModelCollectionsAreNonNil(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	if models := manager.Models(); models == nil {
		t.Fatal("Models() returned nil; the API would encode the collection as JSON null")
	}
	if status := manager.Status(); status.Models == nil {
		t.Fatal("Status().Models returned nil; clients require an empty JSON array")
	}
	models, err := manager.RefreshModels(context.Background())
	if err != nil {
		t.Fatalf("refresh with no configured providers failed: %v", err)
	}
	if models == nil {
		t.Fatal("RefreshModels() returned nil; the API would encode the collection as JSON null")
	}
	if cached := manager.Models(); cached == nil {
		t.Fatal("cached Models() returned nil after refresh")
	}
}

func TestOpenAICompatibleLocalWithoutAPIKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"local-model","name":"Local model"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"model":"local-model","choices":[{"message":{"content":"hello from local"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	provider, err := newProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL + "/v1", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "local-model" {
		t.Fatalf("unexpected model list: %#v", models)
	}
	response, err := provider.Generate(context.Background(), GenerateRequest{Model: "local-model", Messages: []Message{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "hello from local" {
		t.Fatalf("content = %q", response.Content)
	}
	if gotAuth != "" {
		t.Fatalf("unexpected auth header for keyless local provider: %q", gotAuth)
	}
}

func TestOpenAIModelCatalogueToleratesVendorSpecificMetadata(t *testing.T) {
	payload := []byte(`{
		"object":"list",
		"data":[
			{"id":"cc/claude-sonnet-4-5","object":"model","owned_by":"anthropic","capabilities":["text_generation","vision"],"reasoning":{"enabled":true},"supported_parameters":{"reasoning_effort":true},"context_length":"200000"},
			{"id":"cx/deepseek-chat","capabilities":{"reasoning":true,"vision":false},"context_window":65536}
		]
	}`)
	models, err := parseOpenAIModelCatalogue(payload, "nine-router")
	if err != nil {
		t.Fatalf("parseOpenAIModelCatalogue returned error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("discovered %d models, want 2: %#v", len(models), models)
	}
	if models[0].ID != "cc/claude-sonnet-4-5" || models[0].ProviderID != "nine-router" {
		t.Fatalf("unexpected first model identity: %#v", models[0])
	}
	if !models[0].Capabilities["vision"] || models[0].ContextSize != 200000 {
		t.Fatalf("vendor-specific metadata was not normalized: %#v", models[0])
	}
	if len(models[0].SupportedParameters) != 1 || models[0].SupportedParameters[0] != "reasoning_effort" {
		t.Fatalf("supported parameters were not normalized: %#v", models[0].SupportedParameters)
	}
	if models[0].Reasoning != nil {
		t.Fatalf("non-boolean reasoning field must remain unknown, got %v", *models[0].Reasoning)
	}
	if models[1].Capabilities["reasoning"] != true || models[1].Capabilities["vision"] != false || models[1].ContextSize != 65536 {
		t.Fatalf("standard metadata was not preserved: %#v", models[1])
	}
}

func TestOpenAIModelCatalogueAcceptsModelsAliasAndEmptyData(t *testing.T) {
	models, err := parseOpenAIModelCatalogue([]byte(`{"models":[{"id":"local-model"}]}`), "compatible")
	if err != nil || len(models) != 1 || models[0].ID != "local-model" {
		t.Fatalf("models alias parse = %#v, %v", models, err)
	}
	models, err = parseOpenAIModelCatalogue([]byte(`{"object":"list","data":[]}`), "compatible")
	if err != nil || models == nil || len(models) != 0 {
		t.Fatalf("empty data catalogue = %#v, %v; want a non-nil empty slice", models, err)
	}
}

func TestOpenAIModelCatalogueExplainsUnexpectedShape(t *testing.T) {
	_, err := parseOpenAIModelCatalogue([]byte(`{"status":"ok","items":[]}`), "compatible")
	if err == nil {
		t.Fatal("expected an actionable error when the provider returns no data/models array")
	}
	if !strings.Contains(err.Error(), "GET /v1/models") {
		t.Fatalf("error should include endpoint troubleshooting guidance, got: %v", err)
	}
}

func TestRuntimeBoundsDirectGenerationConcurrency(t *testing.T) {
	var active atomic.Int32
	var peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		for observed := peak.Load(); current > observed && !peak.CompareAndSwap(observed, current); observed = peak.Load() {
		}
		time.Sleep(35 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"message":{"content":"ok"}}]}`))
		active.Add(-1)
	}))
	defer server.Close()

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	_, err = manager.UpsertProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL + "/v1", DefaultModel: "test-model", Enabled: true, MaxConcurrent: 4}, false)
	if err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeLocalFirst
	config.DefaultModel = "local/test-model"
	config.MaxConcurrent = 1
	config.MaxQueued = 4
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}

	const requestCount = 4
	var wait sync.WaitGroup
	errorsOut := make(chan error, requestCount)
	for i := 0; i < requestCount; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, generateErr := manager.Generate(context.Background(), GenerateRequest{Messages: []Message{{Role: "user", Content: "hello"}}})
			errorsOut <- generateErr
		}()
	}
	wait.Wait()
	close(errorsOut)
	for generateErr := range errorsOut {
		if generateErr != nil {
			t.Fatalf("generation unexpectedly failed: %v", generateErr)
		}
	}
	if got := peak.Load(); got != 1 {
		t.Fatalf("peak simultaneous provider requests = %d, want 1", got)
	}
}

func TestOfflineOnlyNeverCallsCloudProvider(t *testing.T) {
	manager, err := NewManager("")
	offlineCredential := strings.Join([]string{"fixture", "value", "offline"}, "-")
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.UpsertProvider(ProviderConfig{ID: "cloud", Name: "Cloud", Type: ProviderOpenRouter, BaseURL: "https://openrouter.ai/api/v1", APIKey: offlineCredential, DefaultModel: "test/model", Enabled: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeOfflineOnly
	config.DefaultModel = "cloud/test/model"
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	_, err = manager.Generate(context.Background(), GenerateRequest{Messages: []Message{{Role: "user", Content: "do not go online"}}})
	if err == nil {
		t.Fatal("expected Offline-only policy to reject cloud-only configuration")
	}
	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "offline_policy_blocked" {
		t.Fatalf("error = %#v, want offline_policy_blocked", err)
	}
}

func TestProviderSecretsAreNotReturnedAndPersistPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "ai-runtime.json")
	providerCredential := strings.Join([]string{"fixture", "value", "private"}, "-")
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.UpsertProvider(ProviderConfig{ID: "cloud", Name: "Cloud", Type: ProviderOpenRouter, BaseURL: "https://openrouter.ai/api/v1", APIKey: providerCredential, DefaultModel: "test/model", Enabled: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	public, err := json.Marshal(manager.Providers())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(public), providerCredential) {
		t.Fatalf("secret leaked in public provider list: %s", public)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), providerCredential) {
		t.Fatal("expected secret to be persisted in the protected runtime state")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// POSIX permission bits are observable on Unix. Windows exposes file access
	// through ACLs rather than FileMode.Perm(), which reports a generic 0666 here.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
}

func TestProviderValidationRejectsRemoteHTTPAndURLCredentials(t *testing.T) {
	cases := []ProviderConfig{
		{ID: "remote-http", Name: "Remote HTTP", Type: ProviderOpenAI, BaseURL: "http://example.com/v1", Enabled: true},
		{ID: "credentials", Name: "Credentials", Type: ProviderOpenAI, BaseURL: "https://user:password@example.com/v1", Enabled: true},
		{ID: "query", Name: "Query", Type: ProviderOpenAI, BaseURL: "https://example.com/v1?api_key=secret", Enabled: true},
		{ID: "cloud-private", Name: "Cloud Private", Type: ProviderOpenAI, BaseURL: "http://192.168.1.5:1234/v1", Enabled: true},
	}
	for _, cfg := range cases {
		t.Run(cfg.ID, func(t *testing.T) {
			if err := ValidateProviderConfig(cfg); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestOllamaNativeAPIWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"qwen2.5:3b","model":"qwen2.5:3b","digest":"sha256:fixture","size":1234,"modified_at":"2026-10-24T10:00:00Z","details":{"format":"gguf","family":"qwen2","families":["qwen2"],"parameter_size":"3B","quantization_level":"Q4_K_M"}}]}`))
		case "/api/chat":
			_, _ = w.Write([]byte(`{"model":"qwen2.5:3b","message":{"role":"assistant","content":"local ollama reply"},"done":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	provider, err := newProvider(ProviderConfig{ID: "ollama", Name: "Ollama", Type: ProviderOllama, BaseURL: server.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "qwen2.5:3b" {
		t.Fatalf("unexpected models: %#v", models)
	}
	if models[0].Family != "qwen2" || models[0].ParameterSize != "3B" || models[0].QuantizationLevel != "Q4_K_M" || models[0].SizeBytes != 1234 || models[0].Format != "gguf" || models[0].Metadata["digest"] != "sha256:fixture" {
		t.Fatalf("Ollama model metadata was not preserved: %#v", models[0])
	}
	response, err := provider.Generate(context.Background(), GenerateRequest{Model: "qwen2.5:3b", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "local ollama reply" {
		t.Fatalf("content = %q", response.Content)
	}
}

func TestTaskSchedulerCompletesChatTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"message":{"content":"task completed"}}]}`))
	}))
	defer server.Close()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	_, err = manager.UpsertProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL, DefaultModel: "test-model", Enabled: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := manager.RuntimeConfig()
	cfg.Mode = ModeLocalFirst
	cfg.DefaultModel = "local/test-model"
	if err := manager.UpdateRuntimeConfig(cfg); err != nil {
		t.Fatal(err)
	}
	task, err := manager.StartChatTask(GenerateRequest{Messages: []Message{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		current, ok := manager.Task(task.ID)
		if ok && current.Status == TaskCompleted {
			if current.Result == nil || current.Result.Content != "task completed" {
				t.Fatalf("unexpected task result: %#v", current)
			}
			return
		}
		if ok && current.Status == TaskFailed {
			t.Fatalf("task failed: %#v", current.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("task did not complete before the test deadline")
}

func TestBookDiscussionEnforcesAgentTaskPermissions(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	_, err = manager.UpsertAgent(AgentDefinition{ID: "chat-only", Name: "Chat only", Role: "general assistant", Instructions: "Answer general questions.", Tasks: []string{"chat"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.runDiscussion(context.Background(), DiscussionRequest{BookTitle: "Example", Question: "Discuss this book", AgentIDs: []string{"chat-only"}}, "")
	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "invalid_request" || !strings.Contains(runtimeErr.Message, "not permitted") {
		t.Fatalf("discussion permission error = %#v, want explicit task-permission rejection", err)
	}
}

func TestBookDiscussionRunsBoundedReviewRoundsAndValidatesSources(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		content := ""
		switch call {
		case 1:
			content = "Reader identifies one supported claim in the supplied illustrative passage. [source:page-1]"
		case 2:
			content = "Tutor responds to Reader's claim: the example illustrates the principle, but the passage does not establish wider intent. [source:page-1]"
		case 3:
			content = "Reader refines Tutor's scope point: the example illustrates the supplied principle without establishing wider intent. [source:page-1]"
		case 4:
			content = "Tutor responds to Reader's refinement: the example supports a narrow reading, not the author's wider intent. [source:page-1]"
		default:
			content = "## Synthesis\nThe passage supports a cautious reading. [source:page-1]\n\n## Material disagreements\nThe agents differ on whether the example is central or illustrative.\n\n## Evidence gaps and uncertainty\nThe passage does not establish the author's wider intent."
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "test-model", "choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	_, err = manager.UpsertProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL, DefaultModel: "test-model", Enabled: true, MaxConcurrent: 2}, false)
	if err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeLocalFirst
	config.DefaultModel = "local/test-model"
	config.MaxConcurrent = 2
	config.MaxAgents = 2
	config.MaxDiscussionRounds = 2
	config.TaskModels["book_discussion"] = "local/test-model"
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	task, err := manager.StartDiscussion(DiscussionRequest{
		BookID: "book-1", BookTitle: "Example", Question: "What is the point?", Parallel: true, Rounds: 2,
		AgentIDs: []string{"reader", "tutor"},
		Context:  []ContextPassage{{ID: "page-1", BookID: "book-1", Page: 1, Text: "An illustrative passage from the book."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		current, ok := manager.Task(task.ID)
		if ok && current.Status == TaskCompleted {
			if current.Workflow == nil {
				t.Fatal("completed discussion has no workflow result")
			}
			if got := len(current.Workflow.Contributions); got != 4 {
				t.Fatalf("contribution count = %d, want 4 across two rounds", got)
			}
			if current.Workflow.Contributions[0].Round != 1 || current.Workflow.Contributions[2].Round != 2 {
				t.Fatalf("round labels incorrect: %#v", current.Workflow.Contributions)
			}
			if len(current.Workflow.Disagreements) != 1 || !strings.Contains(current.Workflow.Disagreements[0], "differ") {
				t.Fatalf("material disagreements were not extracted: %#v", current.Workflow.Disagreements)
			}
			if len(current.Workflow.Contributions[0].SourceIDs) != 1 || current.Workflow.Contributions[0].SourceIDs[0] != "page-1" {
				t.Fatalf("known source marker was not validated: %#v", current.Workflow.Contributions[0])
			}
			if got := calls.Load(); got != 5 {
				t.Fatalf("provider call count = %d, want four agent calls plus synthesis", got)
			}
			return
		}
		if ok && current.Status == TaskFailed {
			t.Fatalf("discussion failed: %#v", current.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("discussion did not complete before the test deadline")
}

func TestDiscussionAgentContextUsesProviderAwareDeadline(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{
		ID: "ollama-timeout", Name: "Ollama timeout", Type: ProviderOllama,
		BaseURL: "http://127.0.0.1:11434", DefaultModel: "qwen2.5:3b",
		Enabled: true, TimeoutSeconds: 60, MaxConcurrent: 1,
	}, false); err != nil {
		t.Fatal(err)
	}

	workflowCtx, cancelWorkflow := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelWorkflow()
	agentCtx, cancelAgent := manager.newDiscussionAgentContext(workflowCtx, "ollama-timeout")
	defer cancelAgent()

	agentDeadline, ok := agentCtx.Deadline()
	if !ok {
		t.Fatal("agent context must have its own deadline")
	}
	workflowDeadline, _ := workflowCtx.Deadline()
	remaining := time.Until(agentDeadline)
	if !agentDeadline.Before(workflowDeadline) {
		t.Fatalf("agent deadline %v must be earlier than workflow deadline %v", agentDeadline, workflowDeadline)
	}
	if remaining <= 0 || remaining > 4*time.Minute {
		t.Fatalf("agent deadline remaining = %s, want a positive duration no greater than the local cold-start floor", remaining)
	}
}

func TestBookDiscussionRunsSequentiallySharesEarlierTurnsAndSkipsSynthesis(t *testing.T) {
	var calls atomic.Int32
	var mu sync.Mutex
	var captured []struct {
		Messages  []Message `json:"messages"`
		MaxTokens int       `json:"max_tokens"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Messages  []Message `json:"messages"`
			MaxTokens int       `json:"max_tokens"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode completion request: %v", err)
		}
		mu.Lock()
		captured = append(captured, request)
		mu.Unlock()
		call := calls.Add(1)
		content := "Reader found one central claim supported by the passage. [source:page-1]"
		if call == 2 {
			content = "Tutor explains one implication of Reader's claim without changing its scope. [source:page-1]"
		} else if call == 3 {
			content = "MIMI compares Reader's supported claim with Tutor's scope clarification and preserves the remaining qualification. [source:page-1]"
		} else if call > 3 {
			content = "Unexpected extra call."
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "test-model", "choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL, DefaultModel: "test-model", Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeLocalFirst
	config.DefaultModel = "local/test-model"
	// Keep book_discussion unrouted to verify agents inherit the configured default model.
	delete(config.TaskModels, "book_discussion")
	config.MaxAgents = 3
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	result, err := manager.runDiscussion(context.Background(), DiscussionRequest{
		BookID: "book-1", BookTitle: "Example", Question: "What is the key claim?",
		AgentIDs: []string{"reader", "tutor", "discussion-chair"}, Parallel: false, Rounds: 1, SkipSynthesis: true,
		Context: []ContextPassage{{ID: "page-1", BookID: "book-1", Page: 1, Text: "The passage presents a principle and a short example."}},
	}, "")
	if err != nil {
		t.Fatalf("sequential discussion failed: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("provider calls = %d, want three sequential turns including Chair and no separate synthesis", got)
	}
	if len(result.Contributions) != 3 || result.Synthesis != "" {
		t.Fatalf("unexpected workflow result: %#v", result)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 3 {
		t.Fatalf("captured requests = %d, want 3", len(captured))
	}
	if captured[1].MaxTokens > 220 || captured[1].MaxTokens <= 0 {
		t.Fatalf("discussion max_tokens = %d, want 1..220", captured[1].MaxTokens)
	}
	secondPrompt := ""
	for _, message := range captured[1].Messages {
		secondPrompt += message.Content + "\n"
	}
	if strings.Contains(secondPrompt, "Reader found one central claim") {
		t.Fatalf("Round 1 specialist analysis must be independent; Tutor saw Reader's answer: %s", secondPrompt)
	}
	chairPrompt := ""
	for _, message := range captured[2].Messages {
		chairPrompt += message.Content + "\n"
	}
	for _, expected := range []string{"Reader found one central claim", "Tutor explains one implication", "final Chair"} {
		if !strings.Contains(chairPrompt, expected) {
			t.Errorf("Chair prompt does not preserve earlier turn %q: %s", expected, chairPrompt)
		}
	}
}

func TestBookDiscussionChairSpeaksLastAndLaterRoundsSeeFullHistory(t *testing.T) {
	var calls atomic.Int32
	var mu sync.Mutex
	var captured []struct {
		Messages []Message `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Messages []Message `json:"messages"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		call := calls.Add(1)
		mu.Lock()
		captured = append(captured, request)
		mu.Unlock()
		content := "Reader rechecks MIMI's earlier qualification and narrows the central claim. [source:page-1]"
		switch call {
		case 1:
			content = "Reader's central claim is narrowly supported by the supplied passage. [source:page-1]"
		case 2:
			content = "Tutor responds to Reader's central claim and narrows its scope. [source:page-1]"
		case 3:
			content = "MIMI compares Reader's claim with Tutor's qualification and preserves the remaining disagreement. [source:page-1]"
		case 4:
			content = "Reader revisits MIMI's qualification and refines the central claim. [source:page-1]"
		case 5:
			content = "Tutor develops Reader's refinement without widening its scope. [source:page-1]"
		case 6:
			content = "MIMI compares Reader's revised claim with Tutor's qualification and states what remains unresolved. [source:page-1]"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "test-model", "choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL, DefaultModel: "test-model", Enabled: true, MaxConcurrent: 2}, false); err != nil {
		t.Fatal(err)
	}
	cfg := manager.RuntimeConfig()
	cfg.Mode = ModeLocalFirst
	cfg.DefaultModel = "local/test-model"
	cfg.MaxAgents = 3
	cfg.MaxDiscussionRounds = 2
	if err := manager.UpdateRuntimeConfig(cfg); err != nil {
		t.Fatal(err)
	}
	result, err := manager.runDiscussion(context.Background(), DiscussionRequest{
		BookID: "book-1", BookTitle: "Example", Question: "Test continuity", AgentIDs: []string{"reader", "discussion-chair", "tutor"}, Rounds: 2,
		SkipSynthesis: true, Context: []ContextPassage{{ID: "page-1", BookID: "book-1", Page: 1, Text: "A narrow claim from the current page."}},
	}, "")
	if err != nil {
		t.Fatalf("run discussion: %v", err)
	}
	if len(result.Contributions) != 6 {
		t.Fatalf("contributions = %d, want six turns across two rounds", len(result.Contributions))
	}
	if result.Contributions[2].AgentID != "discussion-chair" || result.Contributions[5].AgentID != "discussion-chair" {
		t.Fatalf("chair did not speak last in each round: order = %q, %q", result.Contributions[2].AgentID, result.Contributions[5].AgentID)
	}
	if result.Contributions[3].Round != 2 {
		t.Fatalf("fourth contribution round = %d, want 2", result.Contributions[3].Round)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 6 {
		t.Fatalf("captured requests = %d, want 6", len(captured))
	}
	var roundTwoReaderPrompt strings.Builder
	for _, message := range captured[3].Messages {
		roundTwoReaderPrompt.WriteString(message.Content)
		roundTwoReaderPrompt.WriteByte('\n')
	}
	if !strings.Contains(roundTwoReaderPrompt.String(), "Reader's central claim is narrowly supported") || !strings.Contains(roundTwoReaderPrompt.String(), "Tutor responds to Reader's central claim") {
		t.Fatalf("round two did not receive the complete prior-round history: %s", roundTwoReaderPrompt.String())
	}
}

func TestDiscussionValidationCanRetainSafeCompleteAnswerWithMissingCitationWarning(t *testing.T) {
	allowed := map[string]bool{"page-20": true}
	passages := []ContextPassage{{ID: "page-20", Page: 20, Text: "The passage discusses a theory of justice through a hypothetical social contract."}}
	answer := "The passage discusses justice through a hypothetical social contract, though one page is not enough to assess the whole book."
	if problem := validateDiscussionOutput(answer, allowed, passages, false); problem != missingCitationProblem {
		t.Fatalf("strict validation problem = %q, want missing-citation warning", problem)
	}
	if problem := validateDiscussionOutputWithoutCitationRequirement(answer, allowed, passages, false); problem != "" {
		t.Fatalf("lenient-after-repair validation rejected otherwise safe answer: %s", problem)
	}
	for _, unsafe := range []string{
		"The passage discusses justice. [source:invented-page]",
		"Chapter IX resolves the argument. [source:page-20]",
		"Reading the pa",
	} {
		if problem := validateDiscussionOutputWithoutCitationRequirement(unsafe, allowed, passages, false); problem == "" {
			t.Errorf("lenient validation incorrectly accepted unsafe answer: %q", unsafe)
		}
	}
}

func TestBookDiscussionKeepsRepairedUncitedContributionWithWarning(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		content := "Reading the pa"
		if call == 2 {
			content = "The passage discusses justice through a hypothetical social contract, though one page is not enough to assess the whole book."
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "test-model", "choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL, DefaultModel: "test-model", Enabled: true, MaxConcurrent: 1}, false); err != nil {
		t.Fatal(err)
	}
	cfg := manager.RuntimeConfig()
	cfg.Mode = ModeLocalFirst
	cfg.DefaultModel = "local/test-model"
	cfg.TaskModels["book_discussion"] = "local/test-model"
	cfg.MaxAgents = 1
	cfg.MaxDiscussionRounds = 1
	if err := manager.UpdateRuntimeConfig(cfg); err != nil {
		t.Fatal(err)
	}
	task, err := manager.StartDiscussion(DiscussionRequest{
		BookID: "book-1", BookTitle: "Example", Question: "What do you think about this book?", AgentIDs: []string{"tutor"},
		Rounds: 1, SkipSynthesis: true,
		Context: []ContextPassage{{ID: "page-20", BookID: "book-1", Page: 20, Text: "The passage discusses a theory of justice through a hypothetical social contract."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		current, ok := manager.Task(task.ID)
		if ok && current.Status == TaskCompleted {
			if current.Workflow == nil || len(current.Workflow.Contributions) != 1 {
				t.Fatalf("unexpected workflow: %#v", current.Workflow)
			}
			got := current.Workflow.Contributions[0]
			if got.Status != "completed" {
				t.Fatalf("repaired contribution status = %q, want completed: %#v", got.Status, got.Error)
			}
			if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "lacks an exact source marker") {
				t.Fatalf("missing transparent citation warning: %#v", got.Warnings)
			}
			if got.Content == "" || calls.Load() != 2 {
				t.Fatalf("content/call count not preserved: %#v, calls=%d", got, calls.Load())
			}
			return
		}
		if ok && current.Status == TaskFailed {
			t.Fatalf("discussion failed: %#v", current.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("discussion did not complete before the test deadline")
}

func TestIncompleteDiscussionResponseDetectsDanglingListItem(t *testing.T) {
	if !incompleteDiscussionResponse("Claim: fairness matters.\n1. First item\n3.") {
		t.Fatal("expected a dangling numbered item to be treated as incomplete")
	}
	if incompleteDiscussionResponse("Claim: fairness matters.\nEvidence: [source:page-1] supports this.\nReading: This is a limited interpretation.") {
		t.Fatal("complete three-line reading should not be treated as incomplete")
	}
}

func TestDiscussionStyleBudgetIsWarningRatherThanHardFailure(t *testing.T) {
	allowed := map[string]bool{"page-21": true}
	content := "The passage supports this interpretation. [source:page-21] " + strings.Repeat("A concise explanatory sentence. ", 32)
	if problem := discussionResponseProblem(content, allowed, true, false); problem != "" {
		t.Fatalf("a complete source-grounded response should not fail solely for length: %s", problem)
	}
	warnings := discussionQualityWarnings(content, allowed, []ContextPassage{{ID: "page-21", Text: "A theory of justice."}}, nil, "reader", false)
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "exceeds the 100-word style budget") {
		t.Fatalf("over-budget response should be retained with a style warning: %#v", warnings)
	}
}

func TestRequiredDiscussionSourceMarkerUsesOnlyOneSuppliedPassage(t *testing.T) {
	passage := []ContextPassage{{ID: "rawls-page-21", Page: 21, Text: "Chapters IV–VIII are not peripheral."}}
	content := "The author explicitly rejects calling Chapters IV–VIII peripheral."
	got := ensureRequiredDiscussionSourceMarker(content, passage, "evidence-auditor", false)
	if !strings.HasSuffix(got, "[source:rawls-page-21]") {
		t.Fatalf("single supplied passage was not attached as an exact source marker: %q", got)
	}
	if got := ensureRequiredDiscussionSourceMarker(content, append(passage, ContextPassage{ID: "page-22", Text: "Other text."}), "evidence-auditor", false); got != content {
		t.Fatalf("multiple passages must not receive an arbitrary citation: %q", got)
	}
	if got := ensureRequiredDiscussionSourceMarker("I cannot verify that claim from the supplied page.", passage, "reader", false); got != "I cannot verify that claim from the supplied page." {
		t.Fatalf("transparent limitation should not receive an automatic citation: %q", got)
	}
}

func TestFinalChairPeerAttributionLedgerQuotesActualCompletedPositions(t *testing.T) {
	previous := []AgentContribution{
		{AgentID: "reader", AgentName: "Orion · Reader", Status: "completed", Content: "The chapters test whether a theory can introduce order and system into considered judgments. [source:rawls-page-21]"},
		{AgentID: "steelman", AgentName: "Atlas · Steelman", Status: "completed", Content: "The chapters are not peripheral applications; their conclusions modify the proposed view. [source:rawls-page-21]"},
	}
	content := "The page rejects the peripheral reading and distinguishes reader choice from the importance of the chapters. [source:rawls-page-21]"
	if problem := discussionPeerResponseProblem(content, previous, "discussion-chair", true); problem == "" {
		t.Fatal("test setup must require the two named peer positions")
	}
	got := ensureFinalChairPeerAttributions(content, previous, "discussion-chair", true)
	if problem := discussionPeerResponseProblem(got, previous, "discussion-chair", true); problem != "" {
		t.Fatalf("actual named peer claims were not preserved in the Chair ledger: %s; output=%q", problem, got)
	}
	for _, name := range []string{"Orion · Reader", "Atlas · Steelman"} {
		if !strings.Contains(got, name) {
			t.Errorf("Chair ledger omitted %s: %q", name, got)
		}
	}
	if strings.Contains(ensureFinalChairPeerAttributions(content, previous, "tutor", true), "Peer positions reviewed:") {
		t.Fatal("peer ledger should be exclusive to the final Chair")
	}
}

func TestDiscussionValidationAllowsNonEvidencePeerCritiqueWithoutCitation(t *testing.T) {
	allowed := map[string]bool{"page-20": true}
	for _, content := range []string{
		"I cannot verify that connection from the supplied passage, so I would not treat it as established.",
		"Orion's inference is too broad because a mention does not establish central emphasis.",
		"I see no grounded connection to the supplied earlier concepts, so this point remains open.",
	} {
		if problem := discussionResponseProblem(content, allowed, true, false); problem != "" {
			t.Errorf("non-source claim was unexpectedly rejected: %q -> %s", content, problem)
		}
	}
}

func TestDiscussionValidationStillRequiresCitationForTextualClaims(t *testing.T) {
	allowed := map[string]bool{"page-20": true}
	if problem := discussionResponseProblem("The passage emphasizes autonomy as its central value.", allowed, true, false); problem == "" {
		t.Fatal("specific claim about the passage was accepted without a source marker")
	}
	if problem := discussionResponseProblem("The passage emphasizes autonomy as its central value. [source:page-20]", allowed, true, false); problem != "" {
		t.Fatalf("valid source marker was rejected: %s", problem)
	}
}

func TestDiscussionValidationRejectsUnseenSectionAndChapterNameDropping(t *testing.T) {
	allowed := map[string]bool{"page-20": true}
	passages := []ContextPassage{{ID: "page-20", Page: 20, Text: "The passage discusses social contract theory and principles of justice."}}
	for _, content := range []string{
		"The best place to read next is §§66–67 on moral worth. [source:page-20]",
		"Chapter IX resolves this point. [source:page-20]",
		"Later chapters make the theory clearer. [source:page-20]",
	} {
		if problem := validateDiscussionOutput(content, allowed, passages, false); problem == "" {
			t.Errorf("unsupported wider-book reference was accepted: %q", content)
		}
	}
	grounded := []ContextPassage{{ID: "page-20", Page: 20, Text: "The author cross-references §§66–67 and Chapter IX in this passage."}}
	if problem := validateDiscussionOutput("The page mentions §§66–67 and Chapter IX. [source:page-20]", allowed, grounded, false); problem != "" {
		t.Fatalf("explicitly supplied section/chapter reference was rejected: %s", problem)
	}
}

func TestIncompleteDiscussionResponseDetectsTruncatedTail(t *testing.T) {
	if !incompleteDiscussionResponse("Reading the pa") {
		t.Fatal("a stream-truncated phrase should be treated as unfinished")
	}
	if incompleteDiscussionResponse("The passage gives a limited account of justice. [source:page-20]") {
		t.Fatal("complete sentence with a source marker was treated as unfinished")
	}
}

func TestDiscussionRepairInstructionDoesNotRequireOpeningSpeakerToAddressPeer(t *testing.T) {
	markers := map[string]bool{"page-20": true, "annotation-abc": true}
	opening := discussionRepairInstruction("the answer is empty", markers, false, false)
	if !strings.Contains(opening, "opening contribution") || !strings.Contains(opening, "[source:page-20]") || !strings.Contains(opening, "[source:annotation-abc]") {
		t.Fatalf("repair instruction doesn't describe opening turn or exact markers: %s", opening)
	}
	if strings.Contains(opening, "Address one actually completed peer") {
		t.Fatal("opening repair incorrectly requires a peer that does not exist")
	}
	peer := discussionRepairInstruction("missing citation", markers, false, true)
	if !strings.Contains(peer, "Address one actually completed peer") {
		t.Fatal("repair should address an actual peer after one completed turn")
	}
}

func TestDiscussionReviewExcludesFailedEmptyTurns(t *testing.T) {
	messages := []Message{{Role: "system", Content: "base"}, {Role: "user", Content: "page"}}
	turns := []AgentContribution{
		{AgentID: "tutor", AgentName: "Tutor", Status: "failed", Error: &Error{Code: "rejected", Message: "empty"}},
		{AgentID: "reader", AgentName: "Reader", Status: "completed", Content: "The passage makes a narrow claim.", Warnings: []string{"This contribution lacks an exact source marker; treat its textual claim as unverified."}},
	}
	got := discussionReviewMessages(messages, AgentDefinition{Name: "Reviewer"}, 1, turns, false)
	joined := strings.Join([]string{got[len(got)-1].Content}, "\n")
	if strings.Contains(joined, "Agent Tutor") || !strings.Contains(joined, "Agent Reader") {
		t.Fatalf("review context should include valid peer only, got: %s", joined)
	}
	if !strings.Contains(joined, "treat its textual claim as unverified") {
		t.Fatalf("peer review did not preserve the contribution's evidence warning: %s", joined)
	}
}

func TestBookDiscussionDefersReaderQuestionUntilPeersReviewIt(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		content := "Reader identifies one ambiguity about fairness in the supplied passage. [source:page-1]"
		if call == 1 {
			content += "\nQuestion for you: Do you read this as a rule for institutions or for individual choices?"
		} else if call == 2 {
			content = "Tutor responds to Reader's ambiguity: the passage supports an institutional rule, but does not settle individual scope. [source:page-1]"
		} else {
			content = "MIMI weighs Reader's question and Tutor's qualification: the institutional reading is supported, while individual scope remains open. [source:page-1]\nQuestion for you: Do you read this as a rule for institutions or for individual choices?"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "test-model", "choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "local", Name: "Local", Type: ProviderOpenAI, BaseURL: server.URL, DefaultModel: "test-model", Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeLocalFirst
	config.DefaultModel = "local/test-model"
	config.TaskModels["book_discussion"] = "local/test-model"
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	result, err := manager.runDiscussion(context.Background(), DiscussionRequest{
		BookID: "book-1", BookTitle: "Example", Question: "Discuss this passage",
		AgentIDs: []string{"discussion-chair", "tutor", "reader"}, Parallel: false, Rounds: 1, SkipSynthesis: true,
		Context: []ContextPassage{{ID: "page-1", BookID: "book-1", Page: 1, Text: "A rule about fair institutions."}},
	}, "")
	if err != nil {
		t.Fatalf("discussion with reader question failed: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("provider calls = %d, want all three agents to review the question before asking the reader", got)
	}
	if len(result.Contributions) != 3 {
		t.Fatalf("contributions = %d, want all three agents to speak", len(result.Contributions))
	}
	if result.Contributions[0].Status != "completed" || agentQuestionMarker.MatchString(result.Contributions[0].Content) || !strings.Contains(result.Contributions[0].Content, "Issue for peer review") {
		t.Fatalf("first contribution should defer its reader question for peer review: %#v", result.Contributions[0])
	}
	if result.Contributions[1].Status != "completed" || agentQuestionMarker.MatchString(result.Contributions[1].Content) {
		t.Fatalf("second agent should review without interrupting the reader: %#v", result.Contributions[1])
	}
	if result.Contributions[2].Status != "completed" || !agentQuestionMarker.MatchString(result.Contributions[2].Content) {
		t.Fatalf("final chair should ask the consolidated reader question: %#v", result.Contributions[2])
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[len(result.Warnings)-1], "paused for the reader") {
		t.Fatalf("missing reader pause notice after the final chair turn: %#v", result.Warnings)
	}
}

func TestExistingBuiltinAgentsReceiveCompactDiscussionSkillDefaultsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-runtime.json")
	initial := `{"runtime":{"mode":"local-first","max_concurrent":1,"max_queued":4,"workflow_timeout_seconds":60,"max_agents":3,"max_discussion_rounds":1},"providers":[],"agents":[{"id":"reader","name":"Reader","role":"close reader","instructions":"Keep this custom instruction.","tasks":["book_discussion"],"enabled":true}]}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	agents := manager.Agents()
	if len(agents) != len(builtinAgents()) {
		t.Fatalf("agent count = %d, want every built-in profile plus no duplicate reader; built-ins=%d", len(agents), len(builtinAgents()))
	}
	var reader *AgentDefinition
	present := map[string]bool{}
	for i := range agents {
		present[agents[i].ID] = true
		if agents[i].ID == "reader" {
			reader = &agents[i]
		}
	}
	if reader == nil {
		t.Fatal("existing reader agent was lost during migration")
	}
	if reader.Instructions != "Keep this custom instruction." {
		t.Fatalf("migration overwrote the user's custom instruction: %q", reader.Instructions)
	}
	if len(reader.SkillIDs) == 0 || !strings.Contains(strings.Join(reader.SkillIDs, ","), "sequential-agent-dialogue") {
		t.Fatalf("reader defaults did not get compact dialogue skills: %#v", reader.SkillIDs)
	}
	if !strings.Contains(reader.SkillInstructions, "exact supplied evidence") {
		t.Fatalf("compact role skill instructions were not attached: %q", reader.SkillInstructions)
	}
	for _, id := range []string{"concept-connector", "socratic-questioner", "counterexample-tester", "context-keeper", "example-builder", "discussion-chair", "evidence-auditor", "ambiguity-scout", "steelman", "scope-keeper", "code-cartographer", "signal-scout"} {
		if !present[id] {
			t.Errorf("new built-in discussion persona %q was not seeded", id)
		}
	}
	if !manager.state.AgentSkillsInitialized {
		t.Fatal("agent skill-default migration flag was not persisted")
	}
	if !manager.state.BuiltinDiscussionAgentsInitialized {
		t.Fatal("new discussion-agent migration flag was not persisted")
	}
	if !manager.state.BuiltinRoundtableV2Initialized {
		t.Fatal("round-table v2 profile migration flag was not persisted")
	}
}

func TestBuiltinRoleProfileV3MigrationRefreshesDefaultsAndPreservesUserEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-runtime.json")
	initial := `{"runtime":{"mode":"local-first","max_concurrent":1,"max_queued":4,"workflow_timeout_seconds":60,"max_agents":5,"max_discussion_rounds":5},"providers":[],"agent_skills_initialized":true,"builtin_discussion_agents_initialized":true,"builtin_roundtable_v2_initialized":true,"agents":[{"id":"reader","name":"Reader","description":"Finds one central claim and its strongest textual support.","role":"close reader","instructions":"State the main claim and one exact piece of evidence. Separate text from interpretation. Flag at most one useful ambiguity. Cite only supplied [source:ID] markers.","tasks":["reading_analysis","book_discussion","explanation"],"skill_ids":["core-reliability","active-reading"],"enabled":true},{"id":"tutor","name":"My custom Tutor","description":"User-authored profile","role":"personal tutor","instructions":"Keep this custom instruction exactly.","tasks":["book_discussion","explanation"],"skill_ids":["learning-coach"],"enabled":true},{"id":"evidence-auditor","name":"Vera · Evidence Auditor","description":"Checks whether claims are actually supported by the page and supplied references.","role":"evidence auditor","instructions":"Audit one named speaker's exact claim. Separate what the page says from inference. Never invent quotes, citations, page references, or claims about unseen chapters.","tasks":["book_discussion","review","reading_analysis"],"enabled":true}]}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	var reader, tutor, auditor *AgentDefinition
	present := map[string]bool{}
	for _, agent := range manager.Agents() {
		present[agent.ID] = true
		switch agent.ID {
		case "reader":
			copy := agent
			reader = &copy
		case "tutor":
			copy := agent
			tutor = &copy
		case "evidence-auditor":
			copy := agent
			auditor = &copy
		}
	}
	if reader == nil || tutor == nil || auditor == nil {
		t.Fatal("migration lost built-in agents")
	}
	if reader.Name != "Orion · Reader" {
		t.Errorf("reader name = %q; expected migrated built-in display name", reader.Name)
	}
	if !strings.Contains(reader.Instructions, "strongest exact textual support") {
		t.Errorf("reader instructions were not migrated: %q", reader.Instructions)
	}
	if !containsString(reader.SkillIDs, "claim-evidence-audit") {
		t.Errorf("reader missing evidence-audit skill: %#v", reader.SkillIDs)
	}
	if !containsString(reader.Tasks, "book_discussion") {
		t.Errorf("reader lost book-discussion permission: %#v", reader.Tasks)
	}
	if tutor.Name != "My custom Tutor" || tutor.Instructions != "Keep this custom instruction exactly." || tutor.Role != "personal tutor" {
		t.Errorf("migration overwrote user-customized tutor profile: %#v", tutor)
	}
	if !strings.Contains(auditor.Instructions, "Audit one exact claim") {
		t.Errorf("legacy evidence-auditor instructions were not upgraded: %q", auditor.Instructions)
	}
	if !containsString(tutor.SkillIDs, "concept-explanation") {
		t.Errorf("custom tutor did not receive role skill additions: %#v", tutor.SkillIDs)
	}
	for _, id := range []string{"reader", "tutor", "translator", "reviewer", "note-writer", "concept-connector", "socratic-questioner", "counterexample-tester", "context-keeper", "example-builder", "discussion-chair", "evidence-auditor", "ambiguity-scout", "steelman", "scope-keeper", "code-cartographer", "signal-scout"} {
		if !present[id] {
			t.Errorf("built-in profile %q missing after migration", id)
		}
	}
	if !manager.state.BuiltinRoleProfilesV3Initialized {
		t.Fatal("role-profile migration flag was not set")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestDiscussionReviewMessagesRetainEarlyClaimsInCompactLedger(t *testing.T) {
	previous := make([]AgentContribution, 12)
	for i := range previous {
		previous[i] = AgentContribution{Round: i/5 + 1, AgentID: fmt.Sprintf("agent-%d", i), AgentName: fmt.Sprintf("Agent %d", i), Status: "completed", Content: fmt.Sprintf("Contribution number %d offers a distinct reason.", i)}
	}
	previous[0].AgentName = "Orion · Reader"
	previous[0].Content = "ORIGINAL_CLAIM_CANARY: the passage distinguishes institutional rules from personal motives."
	previous[11].Content = "LATEST_TURN_CANARY: the reviewer challenges the scope of the inference."
	messages := []Message{{Role: "user", Content: "Review the live discussion."}}
	got := discussionReviewMessages(messages, AgentDefinition{ID: "tutor", Name: "Sage · Tutor"}, 3, previous, false)
	if len(got) != 2 {
		t.Fatalf("message count = %d, want original source plus separate bounded review context", len(got))
	}
	if !strings.Contains(got[1].Content, "EARLIER DISCUSSION LEDGER") || !strings.Contains(got[1].Content, "ORIGINAL_CLAIM_CANARY") {
		t.Fatalf("early claim was dropped from the compressed discussion history: %s", got[1].Content)
	}
	if !strings.Contains(got[1].Content, "LATEST_TURN_CANARY") || !strings.Contains(got[1].Content, "RECENT PEER TURNS") {
		t.Fatalf("latest peer turn was not retained at full priority: %s", got[1].Content)
	}
	if got[0].Content != "Review the live discussion." {
		t.Fatalf("original source/question message was unexpectedly modified: %q", got[0].Content)
	}
}

func TestDiscussionModelRoutingUsesRoleAndPreservesExplicitPreference(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "ollama-local", Name: "Ollama Local", Type: ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.state.Models = []Model{
		{ProviderID: "ollama-local", ID: "qwen2.5:14b", Availability: "available"},
		{ProviderID: "ollama-local", ID: "qwen3:8b", Availability: "available"},
		{ProviderID: "ollama-local", ID: "qwen2.5:7b", Availability: "available"},
		{ProviderID: "ollama-local", ID: "qwen2.5:3b", Availability: "available"},
		{ProviderID: "ollama-local", ID: "translategemma:4b", Availability: "available"},
		{ProviderID: "ollama-local", ID: "deepseek-r1:1.5b", Availability: "available"},
		{ProviderID: "ollama-local", ID: "qwen2.5-coder:latest", Availability: "available"},
		{ProviderID: "ollama-local", ID: "qwen3:1.7b", Availability: "checking"},
		{ProviderID: "ollama-local", ID: "qwen3:0.6b", Availability: "available"},
	}
	manager.mu.Unlock()
	tests := []struct {
		agent    AgentDefinition
		fallback string
		want     string
	}{
		{AgentDefinition{ID: "discussion-chair"}, "global/model", "ollama-local/qwen2.5:14b"},
		{AgentDefinition{ID: "translator"}, "global/model", "ollama-local/translategemma:4b"},
		{AgentDefinition{ID: "tutor"}, "global/model", "ollama-local/qwen2.5:7b"},
		{AgentDefinition{ID: "socratic-questioner"}, "global/model", "ollama-local/qwen2.5:7b"},
		{AgentDefinition{ID: "code-cartographer"}, "global/model", "ollama-local/qwen2.5-coder:latest"},
		{AgentDefinition{ID: "signal-scout"}, "global/model", "ollama-local/qwen2.5:3b"},
		{AgentDefinition{ID: "scope-keeper"}, "global/model", "ollama-local/qwen2.5:7b"},
		{AgentDefinition{ID: "discussion-chair", PreferredModel: "ollama-local/custom-model"}, "global/model", "ollama-local/custom-model"},
		{AgentDefinition{ID: "unknown-role"}, "global/model", "global/model"},
	}
	for _, tc := range tests {
		if got := manager.discussionModelForAgent(tc.agent, tc.fallback); got != tc.want {
			t.Errorf("discussionModelForAgent(%q) = %q, want %q", tc.agent.ID, got, tc.want)
		}
	}
	if got := manager.discussionModelForAgent(AgentDefinition{ID: "tutor"}, "global/model"); got != "ollama-local/qwen2.5:7b" {
		t.Fatalf("tutor did not preserve the stronger available general-purpose model: %q", got)
	}
}

func TestBookDiscussionDisablesNativeThinkingUnlessRequested(t *testing.T) {
	var mu sync.Mutex
	var payloads []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode Ollama payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		payloads = append(payloads, payload)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "qwen3:1.7b",
			"message": map[string]any{"role": "assistant", "content": "The passage makes a limited claim about justice. [source:page-20]"},
			"done":    true,
		})
	}))
	defer server.Close()

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "ollama-local", Name: "Ollama Local", Type: ProviderOllama, BaseURL: server.URL, Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeLocalFirst
	config.DefaultModel = "ollama-local/qwen3:1.7b"
	config.TaskModels["book_discussion"] = "ollama-local/qwen3:1.7b"
	config.MaxAgents = 1
	config.MaxDiscussionRounds = 1
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.state.Models = []Model{{ProviderID: "ollama-local", ID: "qwen3:1.7b", Availability: "available"}}
	manager.mu.Unlock()

	for _, effort := range []string{"", "medium"} {
		_, err := manager.runDiscussion(context.Background(), DiscussionRequest{
			BookID: "book-1", BookTitle: "Example", Question: "What do you think of this page?",
			AgentIDs: []string{"reader"}, Rounds: 1, SkipSynthesis: true, ReasoningEffort: effort,
			Context: []ContextPassage{{ID: "page-20", Page: 20, Text: "The passage discusses a principle of justice."}},
		}, "")
		if err != nil {
			t.Fatalf("discussion with reasoning effort %q failed: %v", effort, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 2 {
		t.Fatalf("Ollama request count = %d, want 2", len(payloads))
	}
	if got, ok := payloads[0]["think"].(bool); !ok || got {
		t.Fatalf("Thinking Off payload think = %#v, want false", payloads[0]["think"])
	}
	if got, ok := payloads[1]["think"].(bool); !ok || !got {
		t.Fatalf("Thinking On payload think = %#v, want true", payloads[1]["think"])
	}
	for i, payload := range payloads {
		options, ok := payload["options"].(map[string]any)
		if !ok {
			t.Fatalf("request %d has no generation options: %#v", i, payload)
		}
		if numPredict, ok := options["num_predict"].(float64); !ok || numPredict < 200 {
			t.Fatalf("request %d num_predict = %#v, want at least 200 tokens", i, options["num_predict"])
		}
	}
}

func TestDiscussionPromptListsExactSourcesAndLimitsWholeBookClaims(t *testing.T) {
	messages := discussionMessages(DiscussionRequest{
		BookTitle: "Example", Question: "What do you think about this book?",
		Context: []ContextPassage{{ID: "page-20", Page: 20, Text: "A theory of justice."}},
	}, AgentDefinition{ID: "reader", Name: "Orion", Role: "Reader", Instructions: "Read closely."}, false, nil)
	prompt := messages[len(messages)-1].Content
	for _, required := range []string{"Exact source markers available: [source:page-20]", "You do not automatically have the whole book", "Do not recommend specific sections/chapters unless that exact reference appears"} {
		if !strings.Contains(prompt, required) {
			t.Errorf("discussion prompt is missing %q: %s", required, prompt)
		}
	}
}

func TestReasoningModeRequiresAdvertisedModelCapability(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "ollama-local", Name: "Ollama Local", Type: ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	trueValue := true
	falseValue := false
	manager.mu.Lock()
	manager.state.Models = []Model{
		{ProviderID: "ollama-local", ID: "reasoner", Reasoning: &trueValue},
		{ProviderID: "ollama-local", ID: "ordinary", Reasoning: &falseValue},
		{ProviderID: "ollama-local", ID: "parameter-model", SupportedParameters: []string{"think"}},
		{ProviderID: "ollama-local", ID: "qwen3:0.6b"},
		{ProviderID: "ollama-local", ID: "deepseek-r1:1.5b"},
		{ProviderID: "ollama-local", ID: "qwen2.5:7b"},
	}
	manager.mu.Unlock()
	for _, tc := range []struct {
		provider, model string
		want            bool
	}{
		{"ollama-local", "reasoner", true},
		{"ollama-local", "ordinary", false},
		{"ollama-local", "parameter-model", true},
		{"ollama-local", "qwen3:0.6b", true},
		{"ollama-local", "deepseek-r1:1.5b", true},
		{"ollama-local", "qwen2.5:7b", false},
		{"ollama-local", "missing", false},
		{"other-provider", "reasoner", false},
	} {
		if got := manager.modelSupportsReasoning(tc.provider, tc.model); got != tc.want {
			t.Errorf("modelSupportsReasoning(%q, %q) = %v, want %v", tc.provider, tc.model, got, tc.want)
		}
	}
}

func TestConfiguredAgentCatalogSupportsAtLeastOneHundredCustomRoles(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("custom-role-%03d", i)
		_, err := manager.UpsertAgent(AgentDefinition{
			ID: id, Name: fmt.Sprintf("Custom Role %03d", i), Role: "custom discussion role",
			Instructions: "Make one concise, source-grounded contribution.",
			Tasks:        []string{"book_discussion"}, Enabled: true, MaxTokens: 120,
		})
		if err != nil {
			t.Fatalf("adding custom agent %s failed: %v", id, err)
		}
	}

	agents := manager.Agents()
	if len(agents) < 100 {
		t.Fatalf("configured agent catalog contains %d agents after adding 100; want at least 100", len(agents))
	}
	if len(agents) != len(builtinAgents())+100 {
		t.Fatalf("configured agent count = %d, want %d built-ins plus 100 custom roles", len(agents), len(builtinAgents()))
	}
}

func TestOfflineOnlyBlocksCloudHealthCheckAndModelDiscovery(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	_, err = manager.UpsertProvider(ProviderConfig{ID: "cloud", Name: "Cloud", Type: ProviderOpenRouter, BaseURL: "https://openrouter.ai/api/v1", APIKey: "secret", DefaultModel: "vendor/model", Enabled: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeOfflineOnly
	config.AllowCloudFallback = true
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	if manager.RuntimeConfig().AllowCloudFallback {
		t.Fatal("Offline-only must clear cloud fallback on the backend")
	}
	_, err = manager.TestProvider(context.Background(), "cloud")
	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "offline_policy_blocked" {
		t.Fatalf("cloud health check error = %#v, want offline_policy_blocked", err)
	}
	models, err := manager.RefreshModels(context.Background())
	if err != nil {
		t.Fatalf("Offline-only discovery should skip configured cloud providers without contacting them: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("cloud models unexpectedly discovered while offline-only: %#v", models)
	}
}

type catalogueOnlyTestProvider struct{ models []Model }

func (p *catalogueOnlyTestProvider) ID() string         { return "cloud" }
func (p *catalogueOnlyTestProvider) Kind() ProviderKind { return ProviderOpenRouter }
func (p *catalogueOnlyTestProvider) Local() bool        { return false }
func (p *catalogueOnlyTestProvider) ListModels(context.Context) ([]Model, error) {
	return append([]Model(nil), p.models...), nil
}
func (p *catalogueOnlyTestProvider) Generate(context.Context, GenerateRequest) (GenerateResponse, error) {
	return GenerateResponse{}, errors.New("inference should be blocked")
}
func (p *catalogueOnlyTestProvider) Stream(context.Context, GenerateRequest, func(string) error) error {
	return errors.New("inference should be blocked")
}
func (p *catalogueOnlyTestProvider) Health(context.Context) HealthStatus {
	return HealthStatus{State: "healthy"}
}

func TestLocalFirstWithoutCloudFallbackDiscoversCloudCatalogueButBlocksInference(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	manager.mu.Lock()
	manager.state.Providers = append(manager.state.Providers, ProviderConfig{
		ID: "cloud", Name: "Cloud Router", Type: ProviderOpenRouter,
		BaseURL: "https://openrouter.ai/api/v1", APIKey: "test-key", Enabled: true, DefaultModel: "router/model-a",
	})
	manager.state.Config.Mode = ModeLocalFirst
	manager.state.Config.AllowCloudFallback = false
	manager.rebuildProvidersLocked()
	manager.mu.Unlock()
	manager.providerFactory = func(ProviderConfig) (Provider, error) {
		return &catalogueOnlyTestProvider{models: []Model{
			{ID: "router/model-a", Name: "Router model A", Capabilities: map[string]bool{"text_generation": true}},
			{ID: "router/model-b", Name: "Router model B", Capabilities: map[string]bool{"text_generation": true}},
		}}, nil
	}

	models, err := manager.RefreshModels(context.Background())
	if err != nil {
		t.Fatalf("Local-first catalogue refresh should discover cloud models: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v, want both router models discoverable", models)
	}
	for _, model := range models {
		if model.ProviderID != "cloud" {
			t.Errorf("provider id = %q, want cloud", model.ProviderID)
		}
		if model.Availability != "blocked" || !strings.Contains(model.AvailabilityMessage, "Local-first") {
			t.Errorf("model state = %#v, want catalogue-visible but inference-blocked by Local-first", model)
		}
	}
}

func TestRefreshModelsKeepsSettingsUsableWhenProviderIsUnavailable(t *testing.T) {
	providerDown := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if providerDown {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"kept-model","name":"Kept model"}]}`))
	}))
	defer server.Close()

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{
		ID: "local-test", Name: "Local test", Type: ProviderOpenAI,
		BaseURL: server.URL + "/v1", Enabled: true, TrustedLocal: true,
	}, false); err != nil {
		t.Fatal(err)
	}

	models, err := manager.RefreshModels(context.Background())
	if err != nil {
		t.Fatalf("model discovery should be best-effort when a provider is down: %v", err)
	}
	if len(models) != 1 || models[0].ID != "kept-model" {
		t.Fatalf("initial models = %#v, want the discovered model", models)
	}
	checkedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	manager.mu.Lock()
	manager.state.Models[0].Availability = "available"
	manager.state.Models[0].AvailabilityMessage = "Model passed a direct probe"
	manager.state.Models[0].ProbeCheckedAt = checkedAt
	manager.state.Models[0].ProbeLatencyMS = 123
	manager.mu.Unlock()

	providerDown = true
	models, err = manager.RefreshModels(context.Background())
	if err != nil {
		t.Fatalf("refresh should be best-effort when a provider later goes down: %v", err)
	}
	if len(models) != 1 || models[0].ID != "kept-model" {
		t.Fatalf("models = %#v, want last-known catalogue preserved", models)
	}
	if models[0].Availability != "available" || !models[0].ProbeCheckedAt.Equal(checkedAt) || models[0].ProbeLatencyMS != 123 {
		t.Fatalf("provider outage overwrote the model-level probe cache: %#v", models[0])
	}
	providers := manager.Providers()
	if len(providers) != 1 || providers[0].Health.State != "unavailable" {
		t.Fatalf("provider health = %#v, want unavailable", providers)
	}

	providerDown = false
	models, err = manager.RefreshModels(context.Background())
	if err != nil {
		t.Fatalf("provider recovery refresh failed: %v", err)
	}
	if len(models) != 1 || models[0].Availability != "available" || !models[0].ProbeCheckedAt.Equal(checkedAt) || models[0].ProbeLatencyMS != 123 {
		t.Fatalf("successful catalogue refresh should preserve matching model probe results: %#v", models)
	}
}

func TestTestModelRunsModelSpecificCheckAndPersistsAvailability(t *testing.T) {
	var completions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"probe-model","name":"Probe model"}]}`))
		case "/v1/chat/completions":
			completions.Add(1)
			_, _ = w.Write([]byte(`{"model":"probe-model","choices":[{"message":{"content":"OK"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "probe", Name: "Probe", Type: ProviderOpenAI, BaseURL: server.URL + "/v1", Enabled: true, TrustedLocal: true}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RefreshModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	model, err := manager.TestModel(context.Background(), "probe", "probe-model")
	if err != nil {
		t.Fatalf("TestModel() error = %v", err)
	}
	if model.Availability != "available" || model.ProbeCheckedAt.IsZero() {
		t.Fatalf("model probe status = %#v, want a persisted available result", model)
	}
	if completions.Load() != 1 {
		t.Fatalf("completion probes = %d, want 1", completions.Load())
	}
	cached := manager.Models()
	if len(cached) != 1 || cached[0].Availability != "available" {
		t.Fatalf("cached model status = %#v, want available", cached)
	}
}

func TestModelProbeTimeoutAllowsColdOllamaLoads(t *testing.T) {
	if got := modelProbeTimeout(ProviderConfig{Type: ProviderOllama, TimeoutSeconds: 15}); got != 180*time.Second {
		t.Fatalf("Ollama probe timeout = %s, want 180s minimum for cold loads", got)
	}
	if got := modelProbeTimeout(ProviderConfig{Type: ProviderOllama, TimeoutSeconds: 240}); got != 240*time.Second {
		t.Fatalf("configured Ollama probe timeout = %s, want 240s", got)
	}
	if got := modelProbeTimeout(ProviderConfig{Type: ProviderOpenAI, TimeoutSeconds: 120}); got != 15*time.Second {
		t.Fatalf("cloud/OpenAI-compatible probe timeout = %s, want existing 15s bound", got)
	}
}

func TestTestOllamaModelCapturesPerformanceAndArchitectureMetadata(t *testing.T) {
	var probeCalls atomic.Int32
	var sawWrongLegacyProbe atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"deepseek-r1:1.5b","model":"deepseek-r1:1.5b","size":1500000000,"details":{"format":"gguf","family":"deepseek2","parameter_size":"1.5B","quantization_level":"Q4_K_M"}}]}`))
		case "/api/chat":
			probeCalls.Add(1)
			var input struct {
				Model     string         `json:"model"`
				Stream    bool           `json:"stream"`
				Think     bool           `json:"think"`
				KeepAlive string         `json:"keep_alive"`
				Messages  []Message      `json:"messages"`
				Options   map[string]any `json:"options"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Errorf("decode Ollama probe: %v", err)
			}
			if input.Model != "deepseek-r1:1.5b" || !input.Stream || input.Think || input.KeepAlive != "0" {
				t.Errorf("unexpected Ollama probe fields: %#v", input)
			}
			if input.Options["num_predict"] != float64(32) || input.Options["temperature"] != float64(0) {
				t.Errorf("unexpected Ollama probe options: %#v", input.Options)
			}
			_, _ = w.Write([]byte(`{"model":"deepseek-r1:1.5b","message":{"role":"assistant","content":"Hello"},"done":false}` + "\n"))
			_, _ = w.Write([]byte(`{"model":"deepseek-r1:1.5b","done":true,"total_duration":5000000000,"load_duration":1500000000,"prompt_eval_count":9,"prompt_eval_duration":500000000,"eval_count":6,"eval_duration":2000000000}` + "\n"))
		case "/api/show":
			_, _ = w.Write([]byte(`{"model_info":{"deepseek2.context_length":32768,"deepseek2.block_count":28,"deepseek2.embedding_length":2048,"general.parameter_count":1500000000},"capabilities":["completion","tools","thinking"]}`))
		case "/api/models/test":
			sawWrongLegacyProbe.Store(true)
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "ollama-local-test", Name: "Ollama Local test", Type: ProviderOllama, BaseURL: server.URL, Enabled: true, TrustedLocal: true, TimeoutSeconds: 1}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RefreshModels(context.Background()); err != nil {
		t.Fatalf("RefreshModels() error = %v", err)
	}
	model, err := manager.TestModel(context.Background(), "ollama-local-test", "deepseek-r1:1.5b")
	if err != nil {
		t.Fatalf("TestModel() error = %v", err)
	}
	if model.Availability != "available" || model.ProbeLatencyMS < 0 {
		t.Fatalf("model probe = %#v, want available", model)
	}
	if model.Performance == nil {
		t.Fatal("model performance metadata is nil")
	}
	if model.Performance.PromptTokens != 9 || model.Performance.GeneratedTokens != 6 || model.Performance.TokensPerSecond != 3 || model.Performance.PromptTokensPerSecond != 18 || model.Performance.LoadDurationMS != 1500 || model.Performance.ResponseDurationMS != 5000 {
		t.Fatalf("unexpected model performance: %#v", model.Performance)
	}
	if model.ContextSize != 32768 || model.Metadata["block_count"] != 28 || model.Capabilities["tools"] != true || model.Capabilities["thinking"] != true {
		t.Fatalf("Ollama /api/show metadata not captured: %#v", model)
	}
	if probeCalls.Load() != 1 {
		t.Fatalf("Ollama native probe calls = %d, want 1", probeCalls.Load())
	}
	if sawWrongLegacyProbe.Load() {
		t.Fatal("Ollama model test hit the unrelated 9Router /api/models/test endpoint")
	}
	cached := manager.Models()
	if len(cached) != 1 || cached[0].Performance == nil || cached[0].Performance.GeneratedTokens != 6 {
		t.Fatalf("performance metadata was not persisted: %#v", cached)
	}
}

func TestTestModelUsesNineRouterNativeProbeWithoutCompletion(t *testing.T) {
	var completions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"cc/example-model","name":"Example"}]}`))
		case "/api/models/test":
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input["model"] != "cc/example-model" || input["kind"] != "llm" {
				t.Errorf("unexpected 9Router probe body: %#v, err=%v", input, err)
			}
			_, _ = w.Write([]byte(`{"ok":true,"latencyMs":17}`))
		case "/v1/chat/completions":
			completions.Add(1)
			_, _ = w.Write([]byte(`{"model":"cc/example-model","choices":[{"message":{"content":"OK"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "nine", Name: "Local gateway", Type: ProviderOpenAI, BaseURL: server.URL + "/v1", Enabled: true, TrustedLocal: true}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RefreshModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	model, err := manager.TestModel(context.Background(), "nine", "cc/example-model")
	if err != nil {
		t.Fatalf("TestModel() error = %v", err)
	}
	if model.Availability != "available" || model.ProbeLatencyMS != 17 {
		t.Fatalf("model probe = %#v, want native available probe", model)
	}
	if completions.Load() != 0 {
		t.Fatalf("completion calls = %d, want zero for the native 9Router probe", completions.Load())
	}
}

func TestTestModelDoesNotContactCloudInOfflineOnly(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"cloud-model"}]}`))
	}))
	defer server.Close()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	// Register a cloud provider with HTTPS-style URL semantics is validated by the
	// provider manager; use a normal configured cloud endpoint in the state directly
	// to exercise the policy boundary without performing network requests.
	manager.mu.Lock()
	manager.state.Providers = append(manager.state.Providers, ProviderConfig{ID: "cloud", Name: "Cloud", Type: ProviderOpenAI, BaseURL: "https://example.invalid/v1", Enabled: true, DefaultModel: "cloud-model"})
	manager.state.Models = append(manager.state.Models, Model{ProviderID: "cloud", ID: "cloud-model"})
	manager.state.Config.Mode = ModeOfflineOnly
	manager.rebuildProvidersLocked()
	manager.mu.Unlock()
	model, err := manager.TestModel(context.Background(), "cloud", "cloud-model")
	if err != nil {
		t.Fatalf("TestModel() error = %v", err)
	}
	if model.Availability != "blocked" {
		t.Fatalf("model availability = %q, want blocked", model.Availability)
	}
	if requests.Load() != 0 {
		t.Fatalf("cloud requests = %d, want zero in Offline-only", requests.Load())
	}
}

func TestTestModelFallsBackWhenNativeProbeEndpointFails(t *testing.T) {
	var nativeCalls atomic.Int32
	var completionBudget int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"ollama/gpt-oss:120b","name":"GPT OSS 120B"}]}`))
		case "/api/models/test":
			nativeCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"native test route unavailable"}`))
		case "/v1/chat/completions":
			var payload struct {
				MaxTokens int `json:"max_tokens"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode completion probe: %v", err)
			}
			completionBudget = payload.MaxTokens
			_, _ = w.Write([]byte(`{"model":"ollama/gpt-oss:120b","choices":[{"message":{"content":"OK"}}]}`))
		default:
			t.Errorf("unexpected endpoint %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{ID: "gateway", Name: "Local gateway", Type: ProviderOpenAI, BaseURL: server.URL + "/v1", Enabled: true, TrustedLocal: true}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RefreshModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	model, err := manager.TestModel(context.Background(), "gateway", "ollama/gpt-oss:120b")
	if err != nil {
		t.Fatalf("TestModel() error = %v", err)
	}
	if model.Availability != "available" {
		t.Fatalf("model availability = %q, want available; message: %s", model.Availability, model.AvailabilityMessage)
	}
	if nativeCalls.Load() != 1 {
		t.Fatalf("native probe calls = %d, want 1", nativeCalls.Load())
	}
	if completionBudget < 256 {
		t.Fatalf("fallback max_tokens = %d, want >= 256 for reasoning models", completionBudget)
	}
}

func TestRefreshProviderModelsContactsOnlySelectedProviderAndPreservesOtherCache(t *testing.T) {
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

	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	for _, provider := range []ProviderConfig{
		{ID: "first", Name: "First provider", Type: ProviderOpenAI, BaseURL: first.URL + "/v1", Enabled: true, TrustedLocal: true},
		{ID: "second", Name: "Second provider", Type: ProviderOpenAI, BaseURL: second.URL + "/v1", Enabled: true, TrustedLocal: true},
	} {
		if _, err := manager.UpsertProvider(provider, false); err != nil {
			t.Fatalf("UpsertProvider(%s): %v", provider.ID, err)
		}
	}

	if _, err := manager.RefreshProviderModels(context.Background(), "second"); err != nil {
		t.Fatalf("seed second provider cache: %v", err)
	}
	secondRequests.Store(0)

	models, err := manager.RefreshProviderModels(context.Background(), "first")
	if err != nil {
		t.Fatalf("refresh selected provider: %v", err)
	}
	if firstRequests.Load() != 1 {
		t.Fatalf("selected provider requests = %d, want 1", firstRequests.Load())
	}
	if secondRequests.Load() != 0 {
		t.Fatalf("unselected provider requests = %d, want 0", secondRequests.Load())
	}
	foundFirst, foundSecond := false, false
	for _, model := range models {
		foundFirst = foundFirst || (model.ProviderID == "first" && model.ID == "first-model")
		foundSecond = foundSecond || (model.ProviderID == "second" && model.ID == "second-model")
	}
	if !foundFirst || !foundSecond {
		t.Fatalf("targeted refresh must return the selected result and preserve other cached models; models = %#v", models)
	}
}

func TestDiscussionChapterRangesAreValidatedAsInclusiveRanges(t *testing.T) {
	passages := []ContextPassage{{
		ID: "page-21", Page: 21,
		Text: "By not including most of Chapters IV–VIII in the more basic parts of the book, I do not mean to suggest that these chapters are peripheral, or merely applications.",
	}}
	allowed := map[string]bool{"page-21": true}
	for _, answer := range []string{
		"The author explicitly rejects treating Chapters IV–VIII as peripheral or merely applications. [source:page-21]",
		"Chapter VIII is included in the range the author explicitly discusses. [source:page-21]",
		"Chapter VI falls within the stated range, though this page does not describe its full contents. [source:page-21]",
	} {
		if problem := validateDiscussionOutput(answer, allowed, passages, false); problem != "" {
			t.Errorf("valid source-backed chapter reference rejected (%q): %s", answer, problem)
		}
	}
	for _, answer := range []string{
		"Chapter IX is described as peripheral. [source:page-21]",
		"Chapters IV–IX are discussed in this passage. [source:page-21]",
	} {
		if problem := validateDiscussionOutput(answer, allowed, passages, false); problem == "" {
			t.Errorf("unsupported chapter reference accepted: %q", answer)
		}
	}
}

func TestCompactDiscussionExcerptPreservesSourceMarkers(t *testing.T) {
	content := strings.Repeat("A careful textual observation with a qualification. ", 18) + "[source:page-21]"
	got := compactDiscussionExcerpt(content, 500)
	if len([]rune(got)) > 500 {
		t.Fatalf("excerpt contains %d runes, want at most 500", len([]rune(got)))
	}
	if !strings.Contains(got, "[source:page-21]") {
		t.Fatalf("bounded excerpt dropped its source marker: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("expected a visible truncation marker: %q", got)
	}

	manyMarkers := strings.Repeat("context ", 120)
	for i := 0; i < 80; i++ {
		manyMarkers += fmt.Sprintf(" [source:page-%d]", i)
	}
	large := compactDiscussionExcerpt(manyMarkers, 50)
	if len([]rune(large)) > 50 {
		t.Fatalf("excerpt with many markers contains %d runes, want at most 50", len([]rune(large)))
	}
}

func TestNormalizeReasoningForModelNeverThinksDuringTranslation(t *testing.T) {
	translation := GenerateRequest{Task: "translation", ReasoningEffort: "low", DisableThinking: true}
	got := normalizeReasoningForModel(translation, true, true)
	if got.ReasoningEffort != "" || !got.DisableThinking {
		t.Fatalf("translation request must disable native thinking when supported: %#v", got)
	}
	unsupported := GenerateRequest{Task: "book_discussion", ReasoningEffort: "medium", DisableThinking: true}
	got = normalizeReasoningForModel(unsupported, false, true)
	if got.ReasoningEffort != "" || got.DisableThinking {
		t.Fatalf("known unsupported model retained reasoning controls: %#v", got)
	}
	unknown := GenerateRequest{Task: "chat", ReasoningEffort: "low"}
	got = normalizeReasoningForModel(unknown, false, false)
	if got.ReasoningEffort != "low" {
		t.Fatalf("unknown model capability must not erase an explicit request: %#v", got)
	}
	supported := GenerateRequest{Task: "book_discussion", ReasoningEffort: "high"}
	got = normalizeReasoningForModel(supported, true, true)
	if got.ReasoningEffort != "high" {
		t.Fatalf("capable model lost explicit reasoning preference: %#v", got)
	}
}

func TestDiscussionPeerTargetsFollowAgentSpecialty(t *testing.T) {
	previous := []AgentContribution{
		{AgentID: "reader", AgentName: "Orion · Reader", Status: "completed", Content: "Reader claim. [source:page-1]"},
		{AgentID: "tutor", AgentName: "Sage · Tutor", Status: "completed", Content: "Tutor explanation. [source:page-1]"},
		{AgentID: "steelman", AgentName: "Atlas · Steelman", Status: "completed", Content: "Steelman challenge. [source:page-1]"},
		{AgentID: "ambiguity-scout", AgentName: "Lex · Ambiguity Scout", Status: "completed", Content: "Lex ambiguity. [source:page-1]"},
	}
	connector := discussionTargetPeers(previous, "concept-connector", 1)
	if len(connector) != 1 || connector[0].AgentID != "reader" {
		t.Fatalf("Concept Connector target = %#v, want Reader rather than the latest speaker", connector)
	}
	chair := discussionTargetPeers(previous, "discussion-chair", 2)
	if len(chair) != 2 || chair[0].AgentID != "reader" || chair[1].AgentID != "steelman" {
		t.Fatalf("Chair targets = %#v, want the central reading and strongest challenge", chair)
	}
}

func TestDiscussionQualityWarningsDetectParaphrasedArgumentEcho(t *testing.T) {
	peer := AgentContribution{
		AgentID: "reader", AgentName: "Orion · Reader", Status: "completed",
		Content: "The author says Chapters IV–VIII are not peripheral; they test how a theory of justice orders considered judgments across many questions. [source:page-21]",
	}
	answer := "Chapters IV–VIII matter because they test a theory's ability to bring order and system to considered judgments across different questions. [source:page-21]"
	similarity, shared := discussionEchoSimilarity(answer, peer.Content)
	if shared < 4 || similarity < 0.40 {
		t.Fatalf("paraphrased echo similarity = %.2f with %d shared terms, want advisory overlap signal", similarity, shared)
	}
	warnings := discussionQualityWarnings(answer, map[string]bool{"page-21": true}, []ContextPassage{{ID: "page-21", Text: "A theory of justice."}}, []AgentContribution{peer}, "tutor", false)
	found := false
	for _, warning := range warnings {
		found = found || strings.Contains(warning, "Possible argument echo")
	}
	if !found {
		t.Fatalf("paraphrased echo did not produce an advisory quality warning: %#v", warnings)
	}
}

func TestDiscussionModelRoutingUsesRoleQualityAndMeasuredProviderPerformance(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.mu.Lock()
	manager.state.Providers = []ProviderConfig{
		{ID: "ollama-slow", Name: "Ollama slow", Type: ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true},
		{ID: "ollama-fast", Name: "Ollama fast", Type: ProviderOllama, BaseURL: "http://127.0.0.1:11435", Enabled: true},
	}
	manager.state.Models = []Model{
		{ProviderID: "ollama-slow", ID: "qwen2.5:14b", Availability: "available", Performance: &ModelPerformance{TokensPerSecond: 1.2, TimeToFirstTokenMS: 4000}},
		{ProviderID: "ollama-fast", ID: "qwen2.5:14b", Availability: "available", Performance: &ModelPerformance{TokensPerSecond: 7.5, TimeToFirstTokenMS: 500}},
		{ProviderID: "ollama-fast", ID: "qwen2.5:7b", Availability: "available", Performance: &ModelPerformance{TokensPerSecond: 22}},
	}
	manager.mu.Unlock()
	if got := manager.discussionModelForAgent(AgentDefinition{ID: "discussion-chair"}, "global/model"); got != "ollama-fast/qwen2.5:14b" {
		t.Fatalf("Chair route = %q, want fastest measured provider for its preferred model", got)
	}
	if got := manager.discussionModelForAgent(AgentDefinition{ID: "tutor"}, "global/model"); got != "ollama-fast/qwen2.5:7b" {
		t.Fatalf("Tutor route = %q, want its first available preferred model", got)
	}
	if got := manager.discussionModelForAgent(AgentDefinition{ID: "discussion-chair", PreferredModel: "ollama-custom/model"}, "global/model"); got != "ollama-custom/model" {
		t.Fatalf("explicit preferred model was not preserved: %q", got)
	}
}

func TestDiscussionOpeningPromptUsesRoleSpecificDeliverableWithoutForcingAnInterpretation(t *testing.T) {
	agent := AgentDefinition{
		ID:           "concept-connector",
		Name:         "Concept Connector",
		Role:         "concept connector",
		Instructions: "Find one grounded relationship or say none is available.",
	}
	messages := discussionMessages(DiscussionRequest{
		BookTitle: "Example",
		Question:  "Find a connection to another concept.",
		Context: []ContextPassage{{
			ID: "page-1", Page: 1, Text: "The passage discusses a theory of justice.",
		}},
	}, agent, false, nil)
	if len(messages) == 0 {
		t.Fatal("opening discussion prompt was empty")
	}
	prompt := messages[0].Content
	if strings.Contains(prompt, "establish one provisional, source-grounded interpretation") {
		t.Fatal("generic opening instruction still forces every specialist to make the same kind of interpretation")
	}
	if !strings.Contains(prompt, "fulfil your role-specific deliverable independently") {
		t.Fatal("opening prompt does not prioritize the agent's own role deliverable")
	}
	if !strings.Contains(prompt, "no grounded cross-page connection") {
		t.Fatal("Concept Connector must be allowed to report that no grounded link exists")
	}
}

func TestDiscussionLogicalForceProblemPreservesTheImportancePreferenceDistinction(t *testing.T) {
	source := `By not including most of Chapters IV–VIII in the more basic parts of the book, I do not mean to suggest that these chapters are peripheral, or merely applications. Rather, I believe that an important test of a theory of justice is how well it introduces order and system into our considered judgments over a wide range of questions. Therefore the topics of these chapters need to be taken up, and the conclusions reached modify in turn the view proposed. But in this regard the reader is more free to follow his preferences and to look at the problems which most concern him.`
	passages := []ContextPassage{{ID: "rawls-page-21", Page: 21, Text: source}}
	bad := "Rawls treats Chapters IV–VIII as optional reading because readers may follow their preferences. [source:rawls-page-21]"
	if problem := discussionLogicalForceProblem(bad, passages); problem == "" {
		t.Fatal("reader preference was incorrectly allowed to downgrade chapters the source explicitly says are not peripheral")
	}
	contradictory := "Rawls says Chapters IV–VIII are not peripheral, but they are optional reading. [source:rawls-page-21]"
	if problem := discussionLogicalForceProblem(contradictory, passages); problem == "" {
		t.Fatal("a contrast that reverses the source's explicit qualification was accepted")
	}
	alsoBad := "Reader freedom means these topics are not important to the theory, even though the reader may choose which issues to study. [source:rawls-page-21]"
	if problem := discussionLogicalForceProblem(alsoBad, passages); problem == "" {
		t.Fatal("an importance downgrade expressed as 'not important' was accepted")
	}
	faithful := "Rawls says Chapters IV–VIII are not peripheral, or merely applications; the reader's freedom concerns which problems to approach first, not the chapters' importance. [source:rawls-page-21]"
	if problem := discussionLogicalForceProblem(faithful, passages); problem != "" {
		t.Fatalf("the source's own negated alternatives were incorrectly flagged: %s", problem)
	}
	rejectedReading := "Rawls explicitly rejects describing Chapters IV–VIII as peripheral or merely applications; reader preference concerns which questions to approach first, not their importance. [source:rawls-page-21]"
	if problem := discussionLogicalForceProblem(rejectedReading, passages); problem != "" {
		t.Fatalf("an explicit rejection of the downgrade was incorrectly flagged: %s", problem)
	}
	negated := "The passage does not call Chapters IV–VIII optional; it explicitly rejects a peripheral reading. [source:rawls-page-21]"
	if problem := discussionLogicalForceProblem(negated, passages); problem != "" {
		t.Fatalf("a correctly negated downgrade was rejected: %s", problem)
	}
	unrelated := []ContextPassage{{ID: "page-1", Text: "The reader may follow personal preferences when choosing which examples to examine."}}
	if problem := discussionLogicalForceProblem(bad, unrelated); problem != "" {
		t.Fatalf("the page-specific logical-force guard leaked into an unrelated passage: %s", problem)
	}
}

func TestDiscussionLogicalForceRepairInstructionExplainsTheExactDistinction(t *testing.T) {
	reason := "logical-force contradiction: the source rejects a peripheral reading while separately granting reader preference"
	instruction := discussionRepairInstruction(reason, map[string]bool{"rawls-page-21": true}, false, false)
	for _, required := range []string{"distinguish the importance of material", "reader's freedom to choose a reading path", "Do not call the material optional", "Rawls explicitly rejects calling Chapters IV–VIII peripheral or merely applications"} {
		if !strings.Contains(instruction, required) {
			t.Errorf("logical-force repair instruction is missing %q: %s", required, instruction)
		}
	}
}

func TestReaderClaimsRequireAnExactSuppliedSourceMarker(t *testing.T) {
	passages := []ContextPassage{{ID: "page-21", Page: 21, Text: "A theory of justice should introduce order and system into considered judgments."}}
	allowed := map[string]bool{"page-21": true}
	claim := "Rawls's central test is whether a theory can organize considered judgments across many questions."
	if problem := validateDiscussionContribution(claim, allowed, passages, nil, "reader", false); problem != missingCitationProblem {
		t.Fatalf("Reader's uncited page claim = %q, want missing-citation repair", problem)
	}
	grounded := claim + " [source:page-21]"
	if problem := validateDiscussionContribution(grounded, allowed, passages, nil, "reader", false); problem != "" {
		t.Fatalf("Reader's correctly cited claim was rejected: %s", problem)
	}
	limitation := "The supplied context does not establish what those topics contain beyond this page."
	if problem := validateDiscussionContribution(limitation, allowed, passages, nil, "reader", false); problem != "" {
		t.Fatalf("a transparent evidence limitation should not require a citation: %s", problem)
	}
}
func TestEvidenceAuditorAndFinalChairRequireExactSuppliedSourceMarker(t *testing.T) {
	passages := []ContextPassage{{ID: "rawls-page-21", Page: 21, Text: "The author rejects calling Chapters IV–VIII peripheral or merely applications."}}
	allowed := map[string]bool{"rawls-page-21": true}
	uncited := "A focused audit distinguishes the author's claim from the reader's choice of reading order."
	for _, tc := range []struct {
		name      string
		agentID   string
		finalTurn bool
	}{
		{name: "Evidence Auditor opening", agentID: "evidence-auditor"},
		{name: "MIMI final adjudication", agentID: "discussion-chair", finalTurn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if problem := validateDiscussionContribution(uncited, allowed, passages, nil, tc.agentID, tc.finalTurn); problem != missingCitationProblem {
				t.Fatalf("uncited %s = %q, want missing-citation repair", tc.name, problem)
			}
			grounded := uncited + " [source:rawls-page-21]"
			if problem := validateDiscussionContribution(grounded, allowed, passages, nil, tc.agentID, tc.finalTurn); problem != "" {
				t.Fatalf("correctly cited %s was rejected: %s", tc.name, problem)
			}
		})
	}
	if problem := discussionRequiredCitationProblem(uncited, passages, "discussion-chair", false); problem != "" {
		t.Fatalf("a non-final Chair turn should not be forced to adjudicate the page: %s", problem)
	}
}

func TestDiscussionOpeningIndependenceBoundary(t *testing.T) {
	tests := []struct {
		name            string
		round           int
		hasPriorDebate  bool
		finalTurn       bool
		wantIndependent bool
	}{
		{name: "fresh first-round specialist", round: 1, wantIndependent: true},
		{name: "final Chair sees opening positions", round: 1, finalTurn: true},
		{name: "resumed discussion sees prior history", round: 1, hasPriorDebate: true},
		{name: "later-round specialist sees debate history", round: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := discussionIsIndependentOpening(test.round, test.hasPriorDebate, test.finalTurn)
			if got != test.wantIndependent {
				t.Fatalf("discussionIsIndependentOpening(%d, %t, %t) = %t, want %t", test.round, test.hasPriorDebate, test.finalTurn, got, test.wantIndependent)
			}
		})
	}
}

func TestRawlsDiscussionRolesPreserveNegationAndAuditEvidence(t *testing.T) {
	steelman := discussionRoleFocus("steelman")
	auditor := discussionRoleFocus("evidence-auditor")
	chair := discussionRoleFocus("discussion-chair")
	for _, required := range []struct {
		role string
		text string
		want []string
	}{
		{role: "Steelman", text: steelman, want: []string{"do not infer that the material is optional", "Preserve explicit negations"}},
		{role: "Evidence Auditor", text: auditor, want: []string{"On an opening turn with no peer positions", "against the supplied passage", "reader's freedom to follow preferences concerns which problems to approach first, not the chapters' importance."}},
		{role: "MIMI Chair", text: chair, want: []string{"decisive source sentences", "reading discretion is not evidence", "optional, peripheral, or unimportant"}},
	} {
		for _, phrase := range required.want {
			if !strings.Contains(required.text, phrase) {
				t.Errorf("%s role contract is missing %q: %s", required.role, phrase, required.text)
			}
		}
	}
}

func TestDiscussionRoleFocusIsDistinctAndInjected(t *testing.T) {
	readerFocus := discussionRoleFocus("reader")
	tutorFocus := discussionRoleFocus("tutor")
	chairFocus := discussionRoleFocus("discussion-chair")
	if readerFocus == "" || tutorFocus == "" || chairFocus == "" {
		t.Fatal("built-in discussion roles must have a role-specific deliverable")
	}
	if readerFocus == tutorFocus || tutorFocus == chairFocus || readerFocus == chairFocus {
		t.Fatal("Reader, Tutor, and Chair must not share the same role contract")
	}

	var tutor AgentDefinition
	for _, item := range builtinAgents() {
		if item.ID == "tutor" {
			tutor = item
			break
		}
	}
	messages := discussionMessages(DiscussionRequest{
		BookTitle: "Example",
		Question:  "What is this page about?",
		Context:   []ContextPassage{{ID: "page-21", Page: 21, Text: "The author explicitly rejects a reading."}},
	}, tutor, false, nil)
	if len(messages) != 2 {
		t.Fatalf("discussion prompt has %d messages, want system and source messages", len(messages))
	}
	for _, required := range []string{"IDENTITY AND ORIGINALITY", "ROLE-SPECIFIC DELIVERABLE", tutorFocus, "distinct role-specific implication"} {
		if !strings.Contains(messages[0].Content, required) {
			t.Errorf("Tutor system prompt does not contain %q", required)
		}
	}
}

func TestDiscussionReviewDoesNotEchoPriorChairConclusion(t *testing.T) {
	var tutor AgentDefinition
	for _, item := range builtinAgents() {
		if item.ID == "tutor" {
			tutor = item
			break
		}
	}
	req := DiscussionRequest{
		BookTitle: "Example", Question: "Continue the page discussion",
		Context: []ContextPassage{{ID: "page-21", Page: 21, Text: "The author rejects the claim that the chapters are merely applications."}},
	}
	messages := discussionMessages(req, tutor, false, nil)
	previous := []AgentContribution{{
		AgentID: "discussion-chair", AgentName: "MIMI · Chair", Status: "completed",
		Content: "The page explicitly rejects the reading that these chapters are merely applications. [source:page-21]",
	}}
	messages = discussionReviewMessages(messages, tutor, 2, previous, false)
	var prompt strings.Builder
	for _, message := range messages {
		prompt.WriteString(message.Content)
		prompt.WriteByte('\n')
	}
	if !strings.Contains(prompt.String(), "CHAIR-REVIEW RULE") || !strings.Contains(prompt.String(), "not wording to echo") {
		t.Fatalf("later Tutor did not receive the anti-echo Chair review contract: %s", prompt.String())
	}
}

func TestDiscussionRequestAcceptsFortyPriorTurnsWithinByteBudget(t *testing.T) {
	previous := make([]AgentContribution, 40)
	for i := range previous {
		previous[i] = AgentContribution{
			AgentID: fmt.Sprintf("agent-%d", i), AgentName: fmt.Sprintf("Agent %d", i),
			Status: "completed", Content: strings.Repeat("x", 1200),
		}
	}
	req := DiscussionRequest{BookTitle: "Example", Question: "Continue the discussion", PreviousContributions: previous}
	if err := validateDiscussionRequest(req); err != nil {
		t.Fatalf("valid 40-turn history within 48,000 bytes rejected: %v", err)
	}
	req.PreviousContributions[0].Content += "x"
	if err := validateDiscussionRequest(req); err == nil {
		t.Fatal("history above the 48,000-byte aggregate limit should be rejected")
	}
	req.PreviousContributions = append(req.PreviousContributions, AgentContribution{AgentID: "extra", Content: "one more", Status: "completed"})
	if err := validateDiscussionRequest(req); err == nil {
		t.Fatal("41 prior turns should exceed the bounded history limit")
	}
}

func TestSkipSynthesisRequiresChairForMultiAgentDiscussion(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	_, err = manager.runDiscussion(context.Background(), DiscussionRequest{
		BookID: "book-1", BookTitle: "Example", Question: "What is the page's central claim?",
		AgentIDs: []string{"reader", "tutor"}, Rounds: 1, SkipSynthesis: true,
		Context: []ContextPassage{{ID: "page-1", Page: 1, Text: "A short source passage."}},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "MIMI · Chair is required") {
		t.Fatalf("multi-agent discussion without Chair error = %v, want clear Chair requirement", err)
	}
}

func TestDiscussionPeerResponseRequiresNamesAndRejectsExactEcho(t *testing.T) {
	previous := []AgentContribution{{
		AgentID: "reader", AgentName: "Orion · Reader", Status: "completed",
		Content: "The author prioritizes equal basic liberties. [source:page-1]",
	}}
	if problem := discussionPeerResponseProblem("This is supported by the page. [source:page-1]", previous, "tutor", false); problem == "" {
		t.Fatal("peer response without a named participant was accepted")
	}
	valid := "Reader's claim about basic liberties is supported, but the page also makes their priority explicit. [source:page-1]"
	if problem := discussionPeerResponseProblem(valid, previous, "tutor", false); problem != "" {
		t.Fatalf("named peer response rejected: %s", problem)
	}
	if problem := discussionPeerResponseProblem(previous[0].Content, previous, "tutor", false); problem == "" {
		t.Fatal("verbatim peer echo was accepted as an independent contribution")
	}
}

func TestDiscussionRepairPromptIncludesOnlyCompletedPeerInventory(t *testing.T) {
	previous := []AgentContribution{
		{AgentID: "concept-connector", AgentName: "Concept Connector", Status: "completed", Content: "Chapters IV–VIII matter because the theory must organize considered judgments. [source:rawls-page-21]"},
		{AgentID: "lex", AgentName: "Lex · Ambiguity Scout", Status: "failed", Error: &Error{Code: "provider_unavailable", Message: "timeout"}},
	}
	prompt := discussionRepairInstructionWithPeers("the response does not identify an actual completed peer", map[string]bool{"rawls-page-21": true}, false, true, previous, "steelman")
	if !strings.Contains(prompt, "Concept Connector") || !strings.Contains(prompt, "organize considered judgments") {
		t.Fatalf("repair prompt omitted the exact completed peer and its claim: %s", prompt)
	}
	if !strings.Contains(prompt, "Regarding Concept Connector's point,") {
		t.Fatalf("repair prompt did not require the exact named peer opening: %s", prompt)
	}
	if strings.Contains(prompt, "Ambiguity Scout") {
		t.Fatalf("repair prompt included a failed/non-peer contribution: %s", prompt)
	}
}

func TestDiscussionQualityDefectsAreWarningsButUntrustedChapterClaimsRemainHardFailures(t *testing.T) {
	allowed := map[string]bool{"rawls-page-21": true}
	passages := []ContextPassage{{ID: "rawls-page-21", Page: 21, Text: "Chapters IV–VIII are not peripheral; they test how a theory orders considered judgments."}}
	previous := []AgentContribution{{
		AgentID: "concept-connector", AgentName: "Concept Connector", Status: "completed",
		Content: "The chapters test a theory across many questions. [source:rawls-page-21]",
	}}
	answer := "The passage says the theory should introduce order and system into considered judgments."
	if problem := discussionHardResponseProblem(answer, allowed, passages); problem != "" {
		t.Fatalf("a complete answer with a soft citation/attribution defect was treated as hard-invalid: %s", problem)
	}
	warnings := discussionQualityWarnings(answer, allowed, passages, previous, "steelman", false)
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "lacks an exact source marker") {
		t.Fatalf("missing citation was not surfaced as a warning: %#v", warnings)
	}
	if !strings.Contains(joined, "participant name") {
		t.Fatalf("missing peer attribution was not surfaced as a warning: %#v", warnings)
	}
	unseen := "Later Chapter IX resolves the argument. [source:rawls-page-21]"
	if problem := discussionHardResponseProblem(unseen, allowed, passages); !strings.Contains(problem, "chapter") {
		t.Fatalf("an unsupported chapter reference was not rejected: %q", problem)
	}
}

func TestNewRuntimeDefaultsSupportSixAgentShelfRoundtable(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	config := manager.RuntimeConfig()
	if config.MaxAgents != 6 {
		t.Fatalf("new runtime max agents = %d, want six roles", config.MaxAgents)
	}
	if config.MaxDiscussionRounds < 2 {
		t.Fatalf("new runtime max discussion rounds = %d, want at least two", config.MaxDiscussionRounds)
	}
}
func TestDiscussionWorkflowDeadlineScalesWithSequentialAgentRounds(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{
		ID: "ollama-budget", Name: "Ollama budget", Type: ProviderOllama,
		BaseURL: "http://127.0.0.1:11434", DefaultModel: "qwen2.5:3b",
		Enabled: true, TimeoutSeconds: 120, MaxConcurrent: 1,
	}, false); err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.WorkflowTimeoutSeconds = 600
	config.MaxAgents = 5
	config.MaxDiscussionRounds = 5
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	agentIDs := []string{"reader", "tutor", "steelman", "concept-connector", "discussion-chair"}
	assigned := map[string]bool{}
	for _, agent := range manager.Agents() {
		if !containsString(agentIDs, agent.ID) {
			continue
		}
		agent.PreferredModel = "ollama-budget/qwen2.5:3b"
		if _, err := manager.UpsertAgent(agent); err != nil {
			t.Fatal(err)
		}
		assigned[agent.ID] = true
	}
	for _, id := range agentIDs {
		if !assigned[id] {
			t.Fatalf("missing integration agent %q", id)
		}
	}
	request := &DiscussionRequest{AgentIDs: agentIDs, Rounds: 2}
	manager.mu.RLock()
	got := manager.discussionWorkflowTimeoutLocked(request)
	manager.mu.RUnlock()
	want := 5*2*240*time.Second + 2*30*time.Second
	if got != want {
		t.Fatalf("two-round local discussion budget = %s, want %s", got, want)
	}
	if manager.RuntimeConfig().WorkflowTimeoutSeconds != 600 {
		t.Fatal("adaptive discussion budget must not mutate the user's global workflow timeout setting")
	}
	request.Rounds = 1
	manager.mu.RLock()
	oneRound := manager.discussionWorkflowTimeoutLocked(request)
	manager.mu.RUnlock()
	if oneRound != 5*240*time.Second+30*time.Second {
		t.Fatalf("one-round local discussion budget = %s, want %s", oneRound, 5*240*time.Second+30*time.Second)
	}
}

func TestDiscussionWorkflowDeadlineUsesSelectedProviderTimeoutAndCapsAtOneHour(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{
		ID: "cloud-budget", Name: "Cloud budget", Type: ProviderOpenAI,
		BaseURL: "https://example.invalid/v1", APIKey: "test-key", DefaultModel: "test-model",
		Enabled: true, TimeoutSeconds: 90, MaxConcurrent: 1,
	}, false); err != nil {
		t.Fatal(err)
	}
	config := manager.RuntimeConfig()
	config.WorkflowTimeoutSeconds = 600
	config.MaxAgents = 8
	config.MaxDiscussionRounds = 5
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	agentIDs := []string{"reader", "tutor", "steelman", "concept-connector", "discussion-chair"}
	for _, agent := range manager.Agents() {
		if !containsString(agentIDs, agent.ID) {
			continue
		}
		agent.PreferredModel = "cloud-budget/test-model"
		if _, err := manager.UpsertAgent(agent); err != nil {
			t.Fatal(err)
		}
	}
	request := &DiscussionRequest{AgentIDs: agentIDs, Rounds: 2}
	manager.mu.RLock()
	got := manager.discussionWorkflowTimeoutLocked(request)
	manager.mu.RUnlock()
	want := 5*2*90*time.Second + 2*30*time.Second
	if got != want {
		t.Fatalf("cloud discussion budget = %s, want %s", got, want)
	}

	request.AgentIDs = []string{"reader", "tutor", "steelman", "concept-connector", "evidence-auditor", "discussion-chair", "ambiguity-scout", "scope-keeper"}
	request.Rounds = 5
	for _, agent := range manager.Agents() {
		if !containsString(request.AgentIDs, agent.ID) {
			continue
		}
		agent.PreferredModel = "cloud-budget/test-model"
		if _, err := manager.UpsertAgent(agent); err != nil {
			t.Fatal(err)
		}
	}
	manager.mu.RLock()
	got = manager.discussionWorkflowTimeoutLocked(request)
	manager.mu.RUnlock()
	if got != time.Hour {
		t.Fatalf("maximum discussion budget = %s, want %s", got, time.Hour)
	}
}

func TestDiscussionTimeoutUsesProviderSettingsWithLocalWarmupFloor(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpsertProvider(ProviderConfig{
		ID: "ollama-short", Name: "Ollama short", Type: ProviderOllama,
		BaseURL: "http://127.0.0.1:11434", DefaultModel: "qwen2.5:3b",
		Enabled: true, TimeoutSeconds: 60, MaxConcurrent: 1,
	}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.UpsertProvider(ProviderConfig{
		ID: "cloud", Name: "Cloud", Type: ProviderOpenAI,
		BaseURL: "https://example.invalid", DefaultModel: "test",
		Enabled: true, TimeoutSeconds: 90, MaxConcurrent: 1,
	}, false); err != nil {
		t.Fatal(err)
	}
	if got := manager.discussionTimeoutForProvider("ollama-short"); got != 240*time.Second {
		t.Fatalf("local discussion timeout = %s, want 4-minute cold-start floor", got)
	}
	if got := manager.discussionTimeoutForProvider("cloud"); got != 90*time.Second {
		t.Fatalf("cloud discussion timeout = %s, want provider-configured 90s", got)
	}
}

func TestOllamaBookDiscussionIntegration(t *testing.T) {
	if os.Getenv("IDIOS_OLLAMA_INTEGRATION") != "1" {
		t.Skip("set IDIOS_OLLAMA_INTEGRATION=1 to run the downloaded local-model integration test")
	}
	model := strings.TrimSpace(os.Getenv("IDIOS_OLLAMA_MODEL"))
	if model == "" {
		model = "qwen2.5:3b"
	}
	baseURL := strings.TrimSpace(os.Getenv("IDIOS_OLLAMA_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	page, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "rawls-a-theory-of-justice-page-21.txt"))
	if err != nil {
		t.Fatalf("read Rawls page-21 fixture: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 23*time.Minute)
	defer cancel()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	const providerID = "ollama-ci"
	if _, err := manager.UpsertProvider(ProviderConfig{
		ID: providerID, Name: "Ollama CI", Type: ProviderOllama, BaseURL: baseURL,
		DefaultModel: model, Enabled: true, TimeoutSeconds: 240, MaxConcurrent: 1,
	}, false); err != nil {
		t.Fatalf("configure local Ollama provider: %v", err)
	}
	config := manager.RuntimeConfig()
	config.Mode = ModeLocalFirst
	config.AllowCloudFallback = false
	config.MaxConcurrent = 1
	config.MaxQueued = 4
	config.WorkflowTimeoutSeconds = 600 // Deliberately retain the normal default; the dispatcher must budget the full round table.
	config.MaxAgents = 6
	config.MaxDiscussionRounds = 2
	config.DefaultModel = providerID + "/" + model
	config.TaskModels["book_discussion"] = providerID + "/" + model
	if err := manager.UpdateRuntimeConfig(config); err != nil {
		t.Fatalf("configure local-only discussion runtime: %v", err)
	}
	agentIDs := []string{"reader", "tutor", "steelman", "concept-connector", "evidence-auditor", "discussion-chair"}
	assigned := map[string]bool{}
	for _, agent := range manager.Agents() {
		if !containsString(agentIDs, agent.ID) {
			continue
		}
		agent.PreferredModel = providerID + "/" + model
		if _, err := manager.UpsertAgent(agent); err != nil {
			t.Fatalf("assign %s to %s: %v", agent.Name, model, err)
		}
		assigned[agent.ID] = true
	}
	for _, id := range agentIDs {
		if !assigned[id] {
			t.Fatalf("required integration-test agent %q was not configured", id)
		}
	}
	discussionTask, err := manager.StartDiscussion(DiscussionRequest{
		BookID:    "rawls-a-theory-of-justice",
		BookTitle: "A Theory of Justice: Revised Edition",
		Question:  "Assess why the author says Chapters IV–VIII are not peripheral. Distinguish the page's stated claim from interpretation, and preserve the qualification that readers may follow their preferences without treating reading choice as a judgement of importance. Each specialist should provide an independent role-specific opening analysis; Reader and Evidence Auditor must cite the exact supplied source marker; MIMI must check every position against the decisive wording. In Round 2, every specialist must review a real peer claim by name, and Concept Connector must complete its cross-concept analysis.",
		PageStart: 21, PageEnd: 21,
		AgentIDs: agentIDs, Parallel: false, Rounds: 2, SkipSynthesis: true,
		Context: []ContextPassage{{ID: "rawls-page-21", BookID: "rawls-a-theory-of-justice", Page: 21, Text: string(page)}},
	})
	if err != nil {
		t.Fatalf("start Qwen discussion: %v", err)
	}
	// Poll the real task dispatcher, not runDiscussion directly, so this integration
	// exercises the adaptive workflow deadline that prevents later-round speakers
	// from being cut off by the generic ten-minute task timeout.
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	task := discussionTask
	for task.Status == TaskPending || task.Status == TaskRunning {
		select {
		case <-ctx.Done():
			t.Fatalf("Qwen two-round task exceeded the integration test deadline: status=%s workflow=%#v", task.Status, task.Workflow)
		case <-ticker.C:
			current, ok := manager.Task(discussionTask.ID)
			if !ok {
				t.Fatal("Qwen discussion task disappeared before completion")
			}
			task = current
		}
	}
	if task.Status != TaskCompleted || task.Workflow == nil {
		t.Fatalf("Qwen two-round discussion task finished with status=%s error=%#v workflow=%#v", task.Status, task.Error, task.Workflow)
	}
	result := *task.Workflow
	wantTurns := len(agentIDs) * 2
	if len(result.Contributions) != wantTurns {
		t.Fatalf("discussion returned %d turns, want %d: %#v", len(result.Contributions), wantTurns, result.Contributions)
	}
	conceptRoundTwoCompleted := false
	for i, contribution := range result.Contributions {
		if contribution.Status != "completed" || strings.TrimSpace(contribution.Content) == "" {
			t.Errorf("agent %s did not return a usable contribution: status=%s error=%#v", contribution.AgentName, contribution.Status, contribution.Error)
		}
		t.Logf("round=%d agent=%s model=%s/%s status=%s warnings=%v\n%s",
			contribution.Round, contribution.AgentName, contribution.ProviderID, contribution.Model,
			contribution.Status, contribution.Warnings, contribution.Content)
		if problem := discussionLogicalForceProblem(contribution.Content, []ContextPassage{{ID: "rawls-page-21", Page: 21, Text: string(page)}}); problem != "" {
			t.Errorf("agent %s reversed an explicit source qualification: %s; output=%q", contribution.AgentName, problem, contribution.Content)
		}
		if (contribution.AgentID == "reader" || contribution.AgentID == "evidence-auditor") && !sourceMarker.MatchString(contribution.Content) {
			t.Errorf("%s did not ground the page claim with an exact supplied source marker: %q warnings=%v", contribution.AgentName, contribution.Content, contribution.Warnings)
		}
		finalChairTurn := contribution.Round == 2 && contribution.AgentID == "discussion-chair"
		if finalChairTurn && !sourceMarker.MatchString(contribution.Content) {
			t.Errorf("MIMI did not cite the source while adjudicating the page claims: %q warnings=%v", contribution.Content, contribution.Warnings)
		}
		if contribution.AgentID == "concept-connector" && contribution.Round == 2 && contribution.Status == "completed" {
			conceptRoundTwoCompleted = true
		}
		// Fresh Round 1 specialists work independently; Round 2 must review real
		// completed peers. The final MIMI turn must compare the actual positions.
		needsPeerReview := !discussionIsIndependentOpening(contribution.Round, false, finalChairTurn)
		if needsPeerReview && contribution.Status == "completed" {
			prior := result.Contributions[:i]
			peerProblem := discussionPeerResponseProblem(contribution.Content, prior, contribution.AgentID, finalChairTurn)
			if peerProblem != "" {
				t.Errorf("agent %s did not address the required actual peer by name: %s; output=%q warnings=%v", contribution.AgentName, peerProblem, contribution.Content, contribution.Warnings)
			}
		}
	}
	if !conceptRoundTwoCompleted {
		t.Fatal("Concept Connector did not complete its second-round peer review; the round-table must not stop at this speaker")
	}
	last := result.Contributions[len(result.Contributions)-1]
	if last.AgentID != "discussion-chair" || last.Round != 2 || last.Status != "completed" {
		t.Fatalf("MIMI did not complete the final moderation turn: %#v", last)
	}

	// Exercise MIMI's separate Shelf-note proposal with the same downloaded model.
	// The selected highlight phrase is reader-provided and remains the exact anchor;
	// the model is responsible for writing a useful, standalone note.
	const selectedQuote = "order and system"
	type actionItem struct {
		Type    string
		Page    int
		Quote   string
		Title   string
		Content string
		Labels  []string
	}
	var noteEnvelope struct {
		Actions []actionItem
	}
	actionSystem := "You are MIMI, the careful reading companion for A Theory of Justice. Write one useful standalone book note grounded only in the supplied page. Return only a JSON object with an actions array containing exactly one note action: {\"actions\":[{\"type\":\"note\",\"title\":\"...\",\"content\":\"...\",\"labels\":[\"reading-companion\"]}]}. Do not invent chapters, claims, quotations, or page references. The application will create the highlight separately from the reader's exact selected phrase."
	actionUser := "The reader asks: “Please write a concise standalone note about the central argument on this page, and highlight the selected phrase ‘order and system’. Add a short margin explanation to the highlight. Save both the highlight and the separate note to this book.”\n\nReader-selected exact highlight phrase (do not alter): “" + selectedQuote + "”\n\nPage 21 source text:\n" + string(page)
	noteRequest := GenerateRequest{
		ProviderID: providerID, Model: model, Task: "book_discussion",
		Messages:  []Message{{Role: "system", Content: actionSystem}, {Role: "user", Content: actionUser}},
		MaxTokens: 520, TimeoutSeconds: 240, PrivacySensitive: false,
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}
	noteResponse, noteErr := manager.Generate(ctx, noteRequest)
	if noteErr != nil {
		t.Fatalf("MIMI local note generation failed: %v", noteErr)
	}
	decodeNote := func(raw string) error {
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "\x60\x60\x60json")
		raw = strings.TrimSuffix(raw, "\x60\x60\x60")
		raw = strings.TrimSpace(raw)
		if start := strings.Index(raw, "<idios_book_actions>"); start >= 0 {
			raw = raw[start+len("<idios_book_actions>"):]
			if end := strings.Index(raw, "</idios_book_actions>"); end >= 0 {
				raw = raw[:end]
			}
		}
		return json.Unmarshal([]byte(strings.TrimSpace(raw)), &noteEnvelope)
	}
	noteProblem := func() string {
		if len(noteEnvelope.Actions) != 1 {
			return fmt.Sprintf("got %d actions, want exactly one separate note action", len(noteEnvelope.Actions))
		}
		action := noteEnvelope.Actions[0]
		if action.Type != "note" || strings.TrimSpace(action.Title) == "" || len(strings.Fields(action.Content)) < 20 {
			return "note action must have type=note, a title, and at least 20 useful content words"
		}
		if !strings.Contains(strings.ToLower(action.Content), "judgment") && !strings.Contains(strings.ToLower(action.Content), "judgement") {
			return "note does not preserve the page's central idea of considered judgments"
		}
		return ""
	}
	parseErr := decodeNote(noteResponse.Content)
	problem := "invalid JSON: " + fmt.Sprint(parseErr)
	if parseErr == nil {
		problem = noteProblem()
	}
	if problem != "" {
		repairRequest := noteRequest
		repairRequest.Messages = append(append([]Message(nil), noteRequest.Messages...),
			Message{Role: "assistant", Content: noteResponse.Content},
			Message{Role: "user", Content: "Repair the action output. " + problem + ". Return only valid JSON with exactly one standalone note action, title, and at least 20 words in content. Ground the note in the supplied page, explicitly explain the idea of bringing order/system to considered judgments, and do not include a highlight action because the application adds the reader-selected exact phrase separately."},
		)
		repaired, repairErr := manager.Generate(ctx, repairRequest)
		if repairErr != nil {
			t.Fatalf("MIMI note repair failed after %s: %v", problem, repairErr)
		}
		noteEnvelope = struct{ Actions []actionItem }{}
		if err := decodeNote(repaired.Content); err != nil {
			t.Fatalf("MIMI repaired note is not valid JSON: %v; response=%s", err, repaired.Content)
		}
		if problem := noteProblem(); problem != "" {
			t.Fatalf("MIMI did not produce a usable standalone note after one repair: %s; response=%s", problem, repaired.Content)
		}
		noteResponse = repaired
	}
	if !strings.Contains(string(page), selectedQuote) || strings.Count(string(page), selectedQuote) != 1 {
		t.Fatal("reader-selected highlight anchor must occur exactly once on the supplied page")
	}
	t.Logf("MIMI shelf note model=%s/%s title=%q content=%s", noteResponse.ProviderID, noteResponse.Model, noteEnvelope.Actions[0].Title, noteEnvelope.Actions[0].Content)
	t.Logf("MIMI exact separate highlight anchor: page=21 quote=%q; client contract tests verify independent highlight and Shelf-note persistence paths.", selectedQuote)
}

func TestFinalChairMustNameActualPositions(t *testing.T) {
	previous := []AgentContribution{
		{AgentID: "reader", AgentName: "Orion · Reader", Status: "completed", Content: "Reader's claim is supported. [source:page-1]"},
		{AgentID: "tutor", AgentName: "Sage · Tutor", Status: "completed", Content: "Tutor narrows the scope. [source:page-1]"},
	}
	oneName := "MIMI agrees with Reader and refines the supported reading. [source:page-1]"
	if problem := discussionPeerResponseProblem(oneName, previous, "discussion-chair", true); problem == "" {
		t.Fatal("final Chair was allowed to omit a second actual peer position")
	}
	bothNames := "MIMI compares Reader's claim with Tutor's qualification and states the remaining limit. [source:page-1]"
	if problem := discussionPeerResponseProblem(bothNames, previous, "discussion-chair", true); problem != "" {
		t.Fatalf("final Chair with both actual positions was rejected: %s", problem)
	}
}

func TestDiscussionPeerResponseCannotPassByNamingItself(t *testing.T) {
	previous := []AgentContribution{
		{AgentID: "reader", AgentName: "Orion · Reader", Status: "completed", Content: "The page supports a narrow interpretation. [source:page-1]"},
		{AgentID: "tutor", AgentName: "Sage · Tutor", Status: "completed", Content: "The example clarifies the principle. [source:page-1]"},
	}
	answer := "Reader confirms the page supports a narrow interpretation. [source:page-1]"
	if problem := discussionPeerResponseProblem(answer, previous, "reader", false); problem == "" {
		t.Fatal("an agent was allowed to satisfy peer attribution by naming its own prior turn")
	}
}

func TestFinalChairRequiresTwoDistinctPeersNotTwoTurnsFromOnePeer(t *testing.T) {
	previous := []AgentContribution{
		{AgentID: "reader", AgentName: "Orion · Reader", Round: 1, Status: "completed", Content: "Reader identifies the first supported claim. [source:page-1]"},
		{AgentID: "reader", AgentName: "Orion · Reader", Round: 2, Status: "completed", Content: "Reader refines that claim after reviewing the qualification. [source:page-1]"},
	}
	answer := "MIMI compares Reader's first claim with Reader's later refinement and preserves the remaining limit. [source:page-1]"
	if problem := discussionPeerResponseProblem(answer, previous, "discussion-chair", true); problem != "" {
		t.Fatalf("one distinct peer with two completed turns should require one named position, got: %s", problem)
	}
}

func TestBuiltinMimiChairV4MigrationPreservesUserCustomization(t *testing.T) {
	oldSkill := "You are Mira, final Chair. In non-final rounds, contribute a specific peer critique without premature synthesis. On the final turn, compare the actual named positions, weigh evidence asymmetrically when warranted, state what is resolved/narrowed/still open, and refine the conclusion. Ask at most one final reader question only if the answer could change the interpretation."
	newSkill := builtinAgentSkillDefaults()["discussion-chair"].Instructions
	type migrationCase struct{ name, description, role, instructions, skillInstructions, wantName, wantDescription, wantRole, wantInstructions, wantSkill string }
	cases := []migrationCase{
		{name: "Mira · Chair", description: "The final moderator: compares positions, reports unresolved conflict, and asks one consequential question only when needed.", role: "round-table chair and synthesis moderator", instructions: "My customized Chair instructions.", skillInstructions: oldSkill, wantName: "MIMI · Chair", wantDescription: "IDIOS's primary reading companion and final moderator; independently checks peer claims and preserves unresolved disagreement.", wantRole: "primary reading companion and evidence-led round-table chair", wantInstructions: "My customized Chair instructions.", wantSkill: newSkill},
		{name: "My Custom Chair", description: "A user-authored Chair description.", role: "user-defined moderation role", instructions: "Keep my custom chair instructions.", skillInstructions: "Keep my custom chair skill.", wantName: "My Custom Chair", wantDescription: "A user-authored Chair description.", wantRole: "user-defined moderation role", wantInstructions: "Keep my custom chair instructions.", wantSkill: "Keep my custom chair skill."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ai-runtime.json")
			state := persistedState{}
			state.Config = RuntimeConfig{Mode: ModeLocalFirst, MaxConcurrent: 2, MaxQueued: 16, WorkflowTimeoutSeconds: 60, MaxAgents: 5, MaxDiscussionRounds: 5, TaskModels: map[string]string{}}
			state.Agents = []AgentDefinition{{ID: "discussion-chair", Name: tc.name, Description: tc.description, Role: tc.role, Instructions: tc.instructions, PreferredModel: "ollama-local/custom-model", Tasks: []string{"book_discussion", "reading_analysis", "review"}, SkillIDs: []string{"core-reliability"}, SkillInstructions: tc.skillInstructions, Enabled: true}}
			state.AgentSkillsInitialized = true
			state.BuiltinDiscussionAgentsInitialized = true
			state.BuiltinRoundtableV2Initialized = true
			state.BuiltinRoleProfilesV3Initialized = true
			data, err := json.Marshal(state)
			if err != nil {
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

			var chair *AgentDefinition
			for _, agent := range manager.Agents() {
				if agent.ID == "discussion-chair" {
					copy := agent
					chair = &copy
					break
				}
			}
			if chair == nil {
				t.Fatal("discussion Chair profile was removed by migration")
			}
			if chair.Name != tc.wantName || chair.Description != tc.wantDescription || chair.Role != tc.wantRole {
				t.Errorf("unexpected migrated Chair identity: %#v", chair)
			}
			if chair.Instructions != tc.wantInstructions || chair.SkillInstructions != tc.wantSkill {
				t.Errorf("migration overwrote customized instructions or failed to upgrade the shipped skill: %#v", chair)
			}
			if chair.PreferredModel != "ollama-local/custom-model" {
				t.Errorf("migration overwrote the explicitly selected model: %q", chair.PreferredModel)
			}
			if !manager.state.BuiltinMimiChairV4Initialized {
				t.Fatal("MIMI Chair v4 migration flag was not set")
			}
		})
	}
}
