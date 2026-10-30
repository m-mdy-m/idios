// Package ai provides IDIOS' provider-independent AI domain and runtime.
package ai

import "time"

type Mode string

const (
	ModeCloudFirst  Mode = "cloud-first"
	ModeLocalFirst  Mode = "local-first"
	ModeOfflineOnly Mode = "offline-only"
)

type ProviderKind string

const (
	ProviderOpenRouter ProviderKind = "openrouter"
	ProviderOpenAI     ProviderKind = "openai-compatible"
	ProviderOllama     ProviderKind = "ollama"
	ProviderCustom     ProviderKind = "custom-adapter"
)

type ProviderConfig struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Type           ProviderKind   `json:"type"`
	BaseURL        string         `json:"base_url"`
	APIKey         string         `json:"api_key,omitempty"`
	DefaultModel   string         `json:"default_model,omitempty"`
	Enabled        bool           `json:"enabled"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
	MaxConcurrent  int            `json:"max_concurrent,omitempty"`
	TrustedLocal   bool           `json:"trusted_local,omitempty"`
	AdapterID      string         `json:"adapter_id,omitempty"`
	AdapterConfig  map[string]any `json:"adapter_config,omitempty"`
}

type ProviderSummary struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Type           ProviderKind `json:"type"`
	BaseURL        string       `json:"base_url"`
	AdapterID      string       `json:"adapter_id,omitempty"`
	DefaultModel   string       `json:"default_model,omitempty"`
	Enabled        bool         `json:"enabled"`
	TimeoutSeconds int          `json:"timeout_seconds"`
	MaxConcurrent  int          `json:"max_concurrent"`
	TrustedLocal   bool         `json:"trusted_local"`
	HasAPIKey      bool         `json:"has_api_key"`
	Local          bool         `json:"local"`
	Health         HealthStatus `json:"health"`
}

type Model struct {
	ProviderID          string            `json:"provider_id"`
	ID                  string            `json:"id"`
	Name                string            `json:"name,omitempty"`
	Architecture        string            `json:"architecture,omitempty"`
	Family              string            `json:"family,omitempty"`
	ParameterSize       string            `json:"parameter_size,omitempty"`
	QuantizationLevel   string            `json:"quantization_level,omitempty"`
	Format              string            `json:"format,omitempty"`
	SizeBytes           int64             `json:"size_bytes,omitempty"`
	ModifiedAt          *time.Time        `json:"modified_at,omitempty"`
	Metadata            map[string]any    `json:"metadata,omitempty"`
	Capabilities        map[string]bool   `json:"capabilities,omitempty"`
	Reasoning           *bool             `json:"reasoning,omitempty"`
	SupportedParameters []string          `json:"supported_parameters,omitempty"`
	ContextSize         int               `json:"context_size,omitempty"`
	Loaded              *bool             `json:"loaded,omitempty"`
	Discovered          time.Time         `json:"discovered_at,omitempty"`
	Availability        string            `json:"availability,omitempty"` // unknown, available, unavailable, blocked
	AvailabilityMessage string            `json:"availability_message,omitempty"`
	ProbeCheckedAt      time.Time         `json:"probe_checked_at,omitempty"`
	ProbeLatencyMS      int64             `json:"probe_latency_ms,omitempty"`
	Performance         *ModelPerformance `json:"performance,omitempty"`
}

// ModelPerformance holds the most recent real generation probe reported by the
// inference runtime. Ollama values come from its final /api/chat stream chunk;
// rates are model/runtime/hardware-specific samples, not universal benchmarks.
type ModelPerformance struct {
	MeasuredAt            time.Time `json:"measured_at,omitempty"`
	ResponseDurationMS    int64     `json:"response_duration_ms,omitempty"`
	TimeToFirstTokenMS    int64     `json:"time_to_first_token_ms,omitempty"`
	LoadDurationMS        int64     `json:"load_duration_ms,omitempty"`
	PromptTokens          int       `json:"prompt_tokens,omitempty"`
	GeneratedTokens       int       `json:"generated_tokens,omitempty"`
	PromptTokensPerSecond float64   `json:"prompt_tokens_per_second,omitempty"`
	TokensPerSecond       float64   `json:"tokens_per_second,omitempty"`
}

type HealthStatus struct {
	State     string    `json:"state"` // unknown, healthy, unavailable, unauthorized, unsupported
	CheckedAt time.Time `json:"checked_at,omitempty"`
	LatencyMS int64     `json:"latency_ms,omitempty"`
	ErrorCode string    `json:"error_code,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseFormat struct {
	// Type is text, json_object, or json_schema.
	Type   string         `json:"type"`
	Name   string         `json:"name,omitempty"`
	Strict bool           `json:"strict,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
}

type GenerateRequest struct {
	ProviderID      string    `json:"provider_id,omitempty"`
	Model           string    `json:"model,omitempty"`
	Task            string    `json:"task,omitempty"`
	Messages        []Message `json:"messages"`
	Temperature     *float64  `json:"temperature,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	TimeoutSeconds  int       `json:"timeout_seconds,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	// DisableThinking explicitly disables native Ollama thinking for capable models
	// when the user chose Thinking Off. It is internal and is not part of the public API.
	DisableThinking  bool            `json:"-"`
	PrivacySensitive bool            `json:"privacy_sensitive,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
}

type GenerateResponse struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	Content    string `json:"content"`
	DurationMS int64  `json:"duration_ms"`
}

type StreamEvent struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	ProviderID string `json:"provider_id,omitempty"`
	Model      string `json:"model,omitempty"`
	Error      *Error `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type RuntimeConfig struct {
	Mode                   Mode              `json:"mode"`
	AllowLocalFallback     bool              `json:"allow_local_fallback"`
	AllowCloudFallback     bool              `json:"allow_cloud_fallback"`
	DefaultModel           string            `json:"default_model,omitempty"` // provider-id/model-id
	TaskModels             map[string]string `json:"task_models,omitempty"`
	MaxConcurrent          int               `json:"max_concurrent"`
	MaxQueued              int               `json:"max_queued"`
	WorkflowTimeoutSeconds int               `json:"workflow_timeout_seconds"`
	MaxAgents              int               `json:"max_agents"`
	MaxDiscussionRounds    int               `json:"max_discussion_rounds"`
}

type AgentDefinition struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Role              string   `json:"role"`
	Instructions      string   `json:"instructions"`
	Tasks             []string `json:"tasks,omitempty"`
	PreferredModel    string   `json:"preferred_model,omitempty"` // provider-id/model-id
	SkillIDs          []string `json:"skill_ids,omitempty"`
	SkillInstructions string   `json:"skill_instructions,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	MaxTokens         int      `json:"max_tokens,omitempty"`
	Enabled           bool     `json:"enabled"`
}

type ContextPassage struct {
	ID     string `json:"id"`
	BookID string `json:"book_id,omitempty"`
	Page   int    `json:"page,omitempty"`
	Start  int    `json:"start,omitempty"`
	End    int    `json:"end,omitempty"`
	Text   string `json:"text"`
}

type DiscussionRequest struct {
	BookID                string              `json:"book_id"`
	BookTitle             string              `json:"book_title"`
	PageStart             int                 `json:"page_start,omitempty"`
	PageEnd               int                 `json:"page_end,omitempty"`
	Question              string              `json:"question"`
	Context               []ContextPassage    `json:"context"`
	AgentIDs              []string            `json:"agent_ids,omitempty"`
	Parallel              bool                `json:"parallel,omitempty"`
	Rounds                int                 `json:"rounds,omitempty"`
	ReasoningEffort       string              `json:"reasoning_effort,omitempty"`
	SkipSynthesis         bool                `json:"skip_synthesis,omitempty"`
	PreviousContributions []AgentContribution `json:"previous_contributions,omitempty"`
}

type AgentContribution struct {
	Round      int      `json:"round,omitempty"`
	AgentID    string   `json:"agent_id"`
	AgentName  string   `json:"agent_name"`
	ProviderID string   `json:"provider_id,omitempty"`
	Model      string   `json:"model,omitempty"`
	Local      bool     `json:"local"`
	Status     string   `json:"status"`
	Content    string   `json:"content,omitempty"`
	SourceIDs  []string `json:"source_ids,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	Error      *Error   `json:"error,omitempty"`
}

type WorkflowResult struct {
	BookID        string              `json:"book_id"`
	Question      string              `json:"question"`
	Contributions []AgentContribution `json:"contributions"`
	Synthesis     string              `json:"synthesis,omitempty"`
	Disagreements []string            `json:"disagreements,omitempty"`
	Warnings      []string            `json:"warnings,omitempty"`
}

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskPartial   TaskStatus = "partially_completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Status     TaskStatus        `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	ProviderID string            `json:"provider_id,omitempty"`
	Model      string            `json:"model,omitempty"`
	Result     *GenerateResponse `json:"result,omitempty"`
	Workflow   *WorkflowResult   `json:"workflow,omitempty"`
	Error      *Error            `json:"error,omitempty"`
}

type Status struct {
	Mode               Mode              `json:"mode"`
	AllowLocalFallback bool              `json:"allow_local_fallback"`
	AllowCloudFallback bool              `json:"allow_cloud_fallback"`
	Providers          []ProviderSummary `json:"providers"`
	Models             []Model           `json:"models"`
	DefaultModel       string            `json:"default_model,omitempty"`
	RunningTasks       int               `json:"running_tasks"`
	QueuedTasks        int               `json:"queued_tasks"`
	LastRefresh        time.Time         `json:"last_refresh,omitempty"`
	ResourceLimits     RuntimeConfig     `json:"resource_limits"`
}
