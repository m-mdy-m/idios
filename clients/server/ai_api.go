package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"idios/internal/ai"
)

func (s *Server) aiManager() (*ai.Manager, error) {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()
	if s.aiRuntime != nil {
		return s.aiRuntime, nil
	}
	if s.aiRuntimeErr != nil {
		return nil, s.aiRuntimeErr
	}
	statePath := ""
	if strings.TrimSpace(s.opts.ConfigPath) != "" {
		statePath = filepath.Join(filepath.Dir(s.opts.ConfigPath), "ai-runtime.json")
	}
	manager, err := ai.NewManager(statePath)
	if err != nil {
		s.aiRuntimeErr = err
		return nil, err
	}
	s.aiRuntime = manager
	return manager, nil
}

func (s *Server) closeAIRuntime() {
	s.aiMu.Lock()
	manager := s.aiRuntime
	s.aiMu.Unlock()
	if manager != nil {
		manager.Close()
	}
}

func (s *Server) getAIRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, manager.Status())
}

func (s *Server) getAIRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, manager.RuntimeConfig())
}

func (s *Server) putAIRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	var input ai.RuntimeConfig
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := manager.UpdateRuntimeConfig(input); err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, manager.RuntimeConfig())
}

func (s *Server) listAIAdapters(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, ai.AvailableProviderAdapters())
}

func (s *Server) listAIProviders(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, manager.Providers())
}

type aiProviderInput struct {
	Config      ai.ProviderConfig `json:"provider"`
	ClearAPIKey bool              `json:"clear_api_key,omitempty"`
}

func (s *Server) createAIProvider(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	var input aiProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	provider, err := manager.UpsertProvider(input.Config, input.ClearAPIKey)
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, provider)
}

func (s *Server) updateAIProvider(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	var input aiProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Config.ID = r.PathValue("id")
	provider, err := manager.UpsertProvider(input.Config, input.ClearAPIKey)
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, provider)
}

func (s *Server) deleteAIProvider(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	if err := manager.DeleteProvider(r.PathValue("id")); err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) testAIProvider(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	health, err := manager.TestProvider(ctx, r.PathValue("id"))
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) listAIModels(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	models := manager.Models()
	providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
	if providerID != "" {
		filtered := make([]ai.Model, 0)
		for _, model := range models {
			if model.ProviderID == providerID {
				filtered = append(filtered, model)
			}
		}
		models = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models, "last_refresh": manager.Status().LastRefresh})
}

type refreshAIModelsRequest struct {
	ProviderID string `json:"provider_id"`
}

func (s *Server) refreshAIModels(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var input refreshAIModelsRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	providerID := strings.TrimSpace(input.ProviderID)
	if providerID == "" {
		writeAIError(w, &ai.Error{Code: "invalid_request", Message: "Choose one provider before refreshing its model catalogue"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	models, err := manager.RefreshProviderModels(ctx, providerID)
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models, "provider_id": providerID, "last_refresh": manager.Status().LastRefresh})
}

type testAIModelRequest struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

func (s *Server) testAIModel(w http.ResponseWriter, r *http.Request) {
	// Per-model probes can take a while on a cold local inference server. The request
	// remains bounded by its context timeout below, so the shared server write deadline
	// should not truncate a valid probe response at the same instant.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var input testAIModelRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	// Local Ollama checks may need to load model weights into RAM/VRAM before
	// returning. TestModel still applies the configured, bounded per-provider timeout.
	ctx, cancel := context.WithTimeout(r.Context(), 11*time.Minute)
	defer cancel()
	model, err := manager.TestModel(ctx, input.ProviderID, input.ModelID)
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (s *Server) aiChat(w http.ResponseWriter, r *http.Request) {
	// AI generation can take longer than the server-wide short write deadline.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var input ai.GenerateRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(runtimeRequestTimeout(input.TimeoutSeconds))*time.Second)
	defer cancel()
	response, err := manager.Generate(ctx, input)
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func responseFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	for w != nil {
		if flusher, ok := w.(http.Flusher); ok {
			return flusher, true
		}
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil, false
		}
		w = unwrapper.Unwrap()
	}
	return nil, false
}

func (s *Server) aiChatStream(w http.ResponseWriter, r *http.Request) {
	// Keep the SSE connection open for the bounded provider request timeout.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var input ai.GenerateRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	flusher, ok := responseFlusher(w)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "Streaming is not supported by this HTTP response writer"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(runtimeRequestTimeout(input.TimeoutSeconds))*time.Second)
	defer cancel()
	err = manager.Stream(ctx, input, func(event ai.StreamEvent) error {
		if _, writeErr := fmt.Fprintf(w, "event: %s\ndata: ", event.Type); writeErr != nil {
			return writeErr
		}
		encoded, marshalErr := jsonMarshal(event)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := fmt.Fprintf(w, "%s\n\n", encoded); writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		apiErr := aiError(err)
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", mustJSON(apiErr))
		flusher.Flush()
	}
}

func runtimeRequestTimeout(seconds int) int {
	if seconds <= 0 {
		return 120
	}
	if seconds > 600 {
		return 600
	}
	return seconds
}

func (s *Server) createAITask(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var input ai.GenerateRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := manager.StartChatTask(input)
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, task)
}

func (s *Server) listAITasks(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, manager.Tasks())
}

func (s *Server) getAITask(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	task, ok := manager.Task(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "AI task not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) cancelAITask(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	if err := manager.CancelTask(r.PathValue("id")); err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancellation_requested"})
}

func (s *Server) listAIAgents(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, manager.Agents())
}

func (s *Server) upsertAIAgent(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	var input ai.AgentDefinition
	if !decodeJSON(w, r, &input) {
		return
	}
	if id := r.PathValue("id"); id != "" {
		input.ID = id
	}
	created := input.ID == ""
	agent, err := manager.UpsertAgent(input)
	if err != nil {
		writeAIError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, agent)
}

func (s *Server) deleteAIAgent(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	if err := manager.DeleteAgent(r.PathValue("id")); err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) startAIAgentRun(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	var input ai.DiscussionRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := manager.StartDiscussion(input)
	if err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, task)
}

func (s *Server) getAIAgentRun(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	task, ok := manager.Task(r.PathValue("id"))
	if !ok || task.Kind != "book_discussion" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "AI agent run not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) cancelAIAgentRun(w http.ResponseWriter, r *http.Request) {
	manager, err := s.aiManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI runtime configuration could not be loaded"})
		return
	}
	task, ok := manager.Task(r.PathValue("id"))
	if !ok || task.Kind != "book_discussion" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "AI agent run not found"})
		return
	}
	if err := manager.CancelTask(task.ID); err != nil {
		writeAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancellation_requested"})
}

func aiError(err error) *ai.Error {
	var typed *ai.Error
	if errors.As(err, &typed) {
		return &ai.Error{Code: typed.Code, Message: typed.Message}
	}
	return &ai.Error{Code: "provider_unavailable", Message: "The AI operation failed; check provider configuration and diagnostics"}
}

func writeAIError(w http.ResponseWriter, err error) {
	item := aiError(err)
	status := http.StatusBadGateway
	switch item.Code {
	case "invalid_request", "invalid_provider_configuration", "model_not_found", "unsupported_capability":
		status = http.StatusBadRequest
	case "offline_policy_blocked":
		status = http.StatusForbidden
	case "resource_limit_exceeded", "rate_limited":
		status = http.StatusTooManyRequests
	case "request_cancelled":
		status = http.StatusConflict
	case "authentication_failed":
		status = http.StatusUnauthorized
	case "task_not_found":
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]any{"error": item})
}

// Helper functions live here so streaming and ordinary response handling use the same JSON shape.
func jsonMarshal(value any) ([]byte, error) { return json.Marshal(value) }
func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `{"code":"internal_error","message":"Could not encode event"}`
	}
	return string(data)
}
