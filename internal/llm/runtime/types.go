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
	Tools       []Tool            `json:"tools,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
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
