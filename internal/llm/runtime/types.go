package runtime

import "context"

type ModelRuntime interface {
	Engine() Engine
	Health(context.Context) (*HealthStatus, error)
	Chat(context.Context, ChatRequest) (*ChatResponse, error)
	Stream(context.Context, ChatRequest) (<-chan StreamEvent, error)
	GenerateStructured(context.Context, StructuredRequest) (*StructuredResponse, error)
}

type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Engine  Engine `json:"engine,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type ChatRequest struct {
	Model       string            `json:"model,omitempty"`
	Messages    []Message         `json:"messages"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type ChatResponse struct {
	Model      string  `json:"model"`
	Message    Message `json:"message"`
	Usage      Usage   `json:"usage"`
	RawContent string  `json:"raw_content,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamEventKind string

const (
	StreamEventDelta StreamEventKind = "delta"
	StreamEventDone  StreamEventKind = "done"
	StreamEventError StreamEventKind = "error"
)

type StreamEvent struct {
	Kind  StreamEventKind
	Delta string
	Err   error
}

type StructuredRequest struct {
	ChatRequest
	Name   string
	Strict bool
	Schema map[string]any
}

type StructuredResponse struct {
	Content string
	Parsed  any
}
