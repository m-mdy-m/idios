package ai

import (
	"net/http"
	"testing"
	"time"
)

func TestOllamaPayloadEnablesNativeThinkingOnlyWhenRequested(t *testing.T) {
	base := GenerateRequest{Model: "qwen3:1.7b", Messages: []Message{{Role: "user", Content: "Analyze this passage."}}}
	withoutThinking := ollamaPayload(base, false)
	if _, exists := withoutThinking["think"]; exists {
		t.Fatalf("think option unexpectedly present when neither disabling nor requesting it: %#v", withoutThinking)
	}
	base.ReasoningEffort = "high"
	withThinking := ollamaPayload(base, false)
	if got, ok := withThinking["think"].(bool); !ok || !got {
		t.Fatalf("think option = %#v, want true for advertised native reasoning request", withThinking["think"])
	}
}

func TestOllamaPayloadCanExplicitlyDisableNativeThinking(t *testing.T) {
	base := GenerateRequest{Model: "qwen3:1.7b", Messages: []Message{{Role: "user", Content: "Give a concise answer."}}, DisableThinking: true}
	payload := ollamaPayload(base, false)
	if got, ok := payload["think"].(bool); !ok || got {
		t.Fatalf("think option = %#v, want explicit false when Thinking is Off", payload["think"])
	}
	base.ReasoningEffort = "medium"
	payload = ollamaPayload(base, false)
	if got, ok := payload["think"].(bool); !ok || !got {
		t.Fatalf("think option = %#v, want true when Thinking is enabled", payload["think"])
	}
}

func TestProviderHTTPClientAllowsRuntimeBoundedLongRequests(t *testing.T) {
	client := providerHTTPClient(ProviderConfig{TimeoutSeconds: 120})
	if client.Timeout != 10*time.Minute {
		t.Fatalf("HTTP client timeout = %s, want the Runtime safety ceiling of 10m", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.ResponseHeaderTimeout != 10*time.Minute {
		t.Fatalf("response header timeout = %s, want 10m", transport.ResponseHeaderTimeout)
	}
}


func TestOllamaTranslationPayloadDisablesOnlyAdvertisedThinking(t *testing.T) {
	capable := normalizeReasoningForModel(GenerateRequest{
		Model: "qwen3:1.7b", Task: "translation", ReasoningEffort: "low",
		Messages: []Message{{Role: "user", Content: "Translate this passage."}},
	}, true, true)
	capablePayload := ollamaPayload(capable, false)
	if got, ok := capablePayload["think"].(bool); !ok || got {
		t.Fatalf("capable translation payload think = %#v, want false", capablePayload["think"])
	}

	ordinary := normalizeReasoningForModel(GenerateRequest{
		Model: "qwen2.5:3b", Task: "translation", ReasoningEffort: "low",
		Messages: []Message{{Role: "user", Content: "Translate this passage."}},
	}, false, true)
	ordinaryPayload := ollamaPayload(ordinary, false)
	if _, exists := ordinaryPayload["think"]; exists {
		t.Fatalf("unsupported model translation payload must omit think, got %#v", ordinaryPayload["think"])
	}
}
