package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/bryanbarton525/prism/internal/ollama"
)

type OllamaRuntime struct {
	cfg    runtime.Config
	client *ollama.Client
}

func NewOllamaRuntime(cfg runtime.Config) (runtime.ModelRuntime, error) {
	return &OllamaRuntime{cfg: cfg, client: ollama.NewClient(cfg.BaseURL)}, nil
}

func (r *OllamaRuntime) Engine() runtime.Engine {
	return runtime.EngineOllama
}

func (r *OllamaRuntime) Health(ctx context.Context) (*runtime.HealthStatus, error) {
	if err := r.client.Ping(ctx); err != nil {
		return &runtime.HealthStatus{Healthy: false, Engine: r.Engine(), Detail: err.Error()}, runtime.NewError(r.Engine(), runtime.ErrorKindUnavailable, 0, "health check failed", err)
	}
	return &runtime.HealthStatus{Healthy: true, Engine: r.Engine(), Detail: "healthy"}, nil
}

func (r *OllamaRuntime) Chat(ctx context.Context, req runtime.ChatRequest) (*runtime.ChatResponse, error) {
	messages := make([]ollama.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, ollama.Message{Role: msg.Role, Content: msg.Content, ToolName: msg.ToolCallID})
	}
	opts := &ollama.Options{NumPredict: req.MaxTokens}
	if req.Temperature != nil {
		opts.Temperature = *req.Temperature
	}
	resp, err := r.client.Chat(ctx, ollama.ChatRequest{
		Model:    firstNonEmpty(r.cfg.Model, req.Model),
		Messages: messages,
		Options:  opts,
	})
	if err != nil {
		return nil, runtime.NewError(r.Engine(), runtime.ErrorKindProvider, 0, "ollama chat failed", err)
	}
	return &runtime.ChatResponse{
		Model:      resp.Model,
		Message:    runtime.Message{Role: resp.Message.Role, Content: resp.Message.Content},
		RawContent: resp.Message.Content,
		Usage: runtime.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}, nil
}

func (r *OllamaRuntime) Stream(context.Context, runtime.ChatRequest) (<-chan runtime.StreamEvent, error) {
	return nil, runtime.NewError(r.Engine(), runtime.ErrorKindInvalidRequest, 0, "ollama runtime streaming is not implemented", nil)
}

func (r *OllamaRuntime) GenerateStructured(ctx context.Context, req runtime.StructuredRequest) (*runtime.StructuredResponse, error) {
	userSchema, err := json.Marshal(req.Schema)
	if err != nil {
		return nil, runtime.NewError(r.Engine(), runtime.ErrorKindInvalidRequest, 0, "marshalling structured schema", err)
	}
	structuredInstruction := runtime.Message{
		Role: "system",
		Content: fmt.Sprintf("Return only valid JSON matching this schema named %q. Do not include markdown fences.\n%s",
			req.Name, string(userSchema)),
	}
	req.Messages = append([]runtime.Message{structuredInstruction}, req.Messages...)
	resp, err := r.Chat(ctx, req.ChatRequest)
	if err != nil {
		return nil, err
	}
	var parsed any
	if err := json.Unmarshal([]byte(resp.Message.Content), &parsed); err != nil {
		return nil, runtime.NewError(r.Engine(), runtime.ErrorKindParse, 0, "parsing structured ollama response", err)
	}
	return &runtime.StructuredResponse{Content: resp.Message.Content, Parsed: parsed}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
