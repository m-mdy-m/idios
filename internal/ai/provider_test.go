package ai

import "testing"

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
