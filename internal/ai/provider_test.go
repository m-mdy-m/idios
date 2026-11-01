package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
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

type ollamaStreamRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip ollamaStreamRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type ollamaStreamErrorBody struct {
	data    []byte
	err     error
	onError func()
}

func (body *ollamaStreamErrorBody) Read(destination []byte) (int, error) {
	if len(body.data) > 0 {
		n := copy(destination, body.data)
		body.data = body.data[n:]
		return n, nil
	}
	if body.onError != nil {
		body.onError()
		body.onError = nil
	}
	return 0, body.err
}

func (body *ollamaStreamErrorBody) Close() error { return nil }

func newOllamaStreamTestProvider(body io.ReadCloser) *ollamaProvider {
	client := &http.Client{Transport: ollamaStreamRoundTripper(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
			Request:    request,
		}, nil
	})}
	return &ollamaProvider{cfg: ProviderConfig{ID: "ollama-test", BaseURL: "http://127.0.0.1:11434"}, client: client}
}

func requireOllamaStreamErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Stream() error = nil, want code %q", want)
	}
	var providerErr *Error
	if !errors.As(err, &providerErr) {
		t.Fatalf("Stream() error = %T (%v), want *ai.Error", err, err)
	}
	if providerErr.Code != want {
		t.Fatalf("Stream() error code = %q, want %q (error: %v)", providerErr.Code, want, err)
	}
}

func TestOllamaStreamRequiresTerminalDoneEvent(t *testing.T) {
	t.Run("complete stream emits chunks in order", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader(
			"{\"message\":{\"content\":\"Hello\"}}\n" +
				"{\"message\":{\"content\":\", world\"}}\n" +
				"{\"done\":true}\n",
		))
		provider := newOllamaStreamTestProvider(body)
		var output strings.Builder
		if err := provider.Stream(context.Background(), GenerateRequest{Model: "qwen2.5:3b"}, func(delta string) error {
			output.WriteString(delta)
			return nil
		}); err != nil {
			t.Fatalf("Stream() error = %v, want nil", err)
		}
		if got, want := output.String(), "Hello, world"; got != want {
			t.Fatalf("stream output = %q, want %q", got, want)
		}
	})

	t.Run("clean EOF before done is incomplete", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader("{\"message\":{\"content\":\"partial\"}}\n"))
		provider := newOllamaStreamTestProvider(body)
		var output strings.Builder
		err := provider.Stream(context.Background(), GenerateRequest{Model: "qwen2.5:3b"}, func(delta string) error {
			output.WriteString(delta)
			return nil
		})
		requireOllamaStreamErrorCode(t, err, "provider_request_failed")
		if got, want := output.String(), "partial"; got != want {
			t.Fatalf("stream preview = %q, want already-emitted partial content %q", got, want)
		}
	})
}

func TestOllamaStreamRejectsMalformedNDJSON(t *testing.T) {
	provider := newOllamaStreamTestProvider(io.NopCloser(strings.NewReader("not-json\n")))
	err := provider.Stream(context.Background(), GenerateRequest{Model: "qwen2.5:3b"}, func(string) error { return nil })
	requireOllamaStreamErrorCode(t, err, "provider_request_failed")
	if !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("Stream() error = %v, want a malformed-event diagnostic", err)
	}
}

func TestOllamaStreamMapsReadCancellationAndDeadline(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code string
	}{
		{name: "cancelled", err: context.Canceled, code: "request_cancelled"},
		{name: "deadline", err: context.DeadlineExceeded, code: "request_timed_out"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := &ollamaStreamErrorBody{
				data: []byte("{\"message\":{\"content\":\"partial\"}}\n"),
				err:  test.err,
			}
			provider := newOllamaStreamTestProvider(body)
			err := provider.Stream(context.Background(), GenerateRequest{Model: "qwen2.5:3b"}, func(string) error { return nil })
			requireOllamaStreamErrorCode(t, err, test.code)
		})
	}
}

func TestOllamaStreamRejectsProviderErrorEvent(t *testing.T) {
	provider := newOllamaStreamTestProvider(io.NopCloser(strings.NewReader("{\"error\":\"model stopped\"}\n")))
	err := provider.Stream(context.Background(), GenerateRequest{Model: "qwen2.5:3b"}, func(string) error { return nil })
	requireOllamaStreamErrorCode(t, err, "provider_request_failed")
}


func TestOllamaStreamCancellationTakesPrecedenceOverGenericReadError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	body := &ollamaStreamErrorBody{
		data: []byte("{\"message\":{\"content\":\"partial\"}}\n"),
		err:  io.ErrUnexpectedEOF,
		onError: cancel,
	}
	provider := newOllamaStreamTestProvider(body)
	err := provider.Stream(ctx, GenerateRequest{Model: "qwen2.5:3b"}, func(string) error { return nil })
	requireOllamaStreamErrorCode(t, err, "request_cancelled")
}
