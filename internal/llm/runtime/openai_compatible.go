package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OpenAICompatibleRuntime struct {
	cfg  Config
	base string
	http *http.Client
}

func NewOpenAICompatibleRuntime(cfg Config) (*OpenAICompatibleRuntime, error) {
	base, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, NewError(cfg.Engine, ErrorKindInvalidRequest, 0, err.Error(), err)
	}
	return &OpenAICompatibleRuntime{cfg: cfg, base: base, http: &http.Client{Timeout: cfg.Timeout()}}, nil
}

func (r *OpenAICompatibleRuntime) Engine() Engine {
	return r.cfg.Engine
}

func (r *OpenAICompatibleRuntime) Health(ctx context.Context) (*HealthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.base+"/health", nil)
	if err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindInvalidRequest, 0, "building health request", err)
	}
	r.setHeaders(req)
	resp, err := r.http.Do(req)
	if err != nil {
		return &HealthStatus{Healthy: false, Engine: r.cfg.Engine, Detail: err.Error()}, NewError(r.cfg.Engine, ErrorKindUnavailable, 0, "health check failed", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := NewError(r.cfg.Engine, KindFromStatus(resp.StatusCode), resp.StatusCode, trimBody(body), nil)
		return &HealthStatus{Healthy: false, Engine: r.cfg.Engine, Detail: err.Error()}, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return &HealthStatus{Healthy: true, Engine: r.cfg.Engine, Detail: "empty health body accepted"}, nil
	}
	var parsed struct {
		Status string `json:"status"`
		OK     *bool  `json:"ok"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindParse, resp.StatusCode, "parsing health response", err)
	}
	status := strings.ToLower(strings.TrimSpace(parsed.Status))
	if parsed.OK != nil && !*parsed.OK {
		err := NewError(r.cfg.Engine, ErrorKindUnavailable, resp.StatusCode, "health response reported ok=false", nil)
		return &HealthStatus{Healthy: false, Engine: r.cfg.Engine, Detail: err.Error()}, err
	}
	if status != "" && status != "ok" && status != "healthy" && status != "ready" {
		err := NewError(r.cfg.Engine, ErrorKindUnavailable, resp.StatusCode, "health response status: "+parsed.Status, nil)
		return &HealthStatus{Healthy: false, Engine: r.cfg.Engine, Detail: err.Error()}, err
	}
	return &HealthStatus{Healthy: true, Engine: r.cfg.Engine, Detail: "healthy"}, nil
}

func (r *OpenAICompatibleRuntime) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return r.chat(ctx, req, nil, false)
}

func (r *OpenAICompatibleRuntime) GenerateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "structured_response"
	}
	format := &responseFormat{
		Type: "json_schema",
		JSONSchema: jsonSchemaFormat{
			Name:   name,
			Strict: req.Strict,
			Schema: req.Schema,
		},
	}
	res, err := r.chat(ctx, req.ChatRequest, format, false)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(res.Message.Content)
	if content == "" {
		return nil, NewError(r.cfg.Engine, ErrorKindParse, 0, "structured response missing assistant content", nil)
	}
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindParse, 0, "structured response content is not valid JSON", err)
	}
	return &StructuredResponse{Content: content, Parsed: parsed}, nil
}

func (r *OpenAICompatibleRuntime) Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	payload := r.openAIRequest(req, nil, true)
	if payload.Model == "" {
		return nil, NewError(r.cfg.Engine, ErrorKindInvalidRequest, 0, "model is required", nil)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindInvalidRequest, 0, "marshalling stream request", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.base+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindInvalidRequest, 0, "building stream request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	r.setHeaders(httpReq)
	resp, err := r.http.Do(httpReq)
	if err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindUnavailable, 0, "starting stream request", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, NewError(r.cfg.Engine, KindFromStatus(resp.StatusCode), resp.StatusCode, trimBody(body), nil)
	}

	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		r.parseSSE(ctx, resp.Body, out)
	}()
	return out, nil
}

func (r *OpenAICompatibleRuntime) chat(ctx context.Context, req ChatRequest, format *responseFormat, stream bool) (*ChatResponse, error) {
	payload := r.openAIRequest(req, format, stream)
	if payload.Model == "" {
		return nil, NewError(r.cfg.Engine, ErrorKindInvalidRequest, 0, "model is required", nil)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindInvalidRequest, 0, "marshalling chat request", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.base+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindInvalidRequest, 0, "building chat request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	r.setHeaders(httpReq)
	resp, err := r.http.Do(httpReq)
	if err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindUnavailable, 0, "sending chat request", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewError(r.cfg.Engine, KindFromStatus(resp.StatusCode), resp.StatusCode, trimBody(body), nil)
	}
	var parsed openAIChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, NewError(r.cfg.Engine, ErrorKindParse, resp.StatusCode, "parsing chat response", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, NewError(r.cfg.Engine, ErrorKindParse, resp.StatusCode, "chat response contained no choices", nil)
	}
	msg := parsed.Choices[0].Message
	return &ChatResponse{
		Model:      parsed.Model,
		Message:    msg,
		RawContent: msg.Content,
		Usage: Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		},
	}, nil
}

func (r *OpenAICompatibleRuntime) openAIRequest(req ChatRequest, format *responseFormat, stream bool) openAIChatRequest {
	return openAIChatRequest{
		Model:          firstNonEmpty(req.Model, r.cfg.Model),
		Messages:       req.Messages,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
		Stream:         stream,
		ResponseFormat: format,
	}
}

func (r *OpenAICompatibleRuntime) parseSSE(ctx context.Context, body io.Reader, out chan<- StreamEvent) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			sendStreamEvent(out, StreamEvent{Kind: StreamEventError, Err: NewError(r.cfg.Engine, ErrorKindTimeout, 0, "stream context canceled", ctx.Err())})
			return
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			sendStreamEvent(out, StreamEvent{Kind: StreamEventDone})
			return
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			sendStreamEvent(out, StreamEvent{Kind: StreamEventError, Err: NewError(r.cfg.Engine, ErrorKindParse, 0, "parsing stream event", err)})
			return
		}
		if chunk.Error != nil {
			sendStreamEvent(out, StreamEvent{Kind: StreamEventError, Err: NewError(r.cfg.Engine, ErrorKindProvider, 0, chunk.Error.Message, nil)})
			return
		}
		for _, choice := range chunk.Choices {
			content := firstNonEmpty(choice.Delta.Content, choice.Message.Content)
			if content != "" {
				sendStreamEvent(out, StreamEvent{Kind: StreamEventDelta, Delta: content})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		sendStreamEvent(out, StreamEvent{Kind: StreamEventError, Err: NewError(r.cfg.Engine, ErrorKindUnavailable, 0, "reading stream", err)})
		return
	}
	sendStreamEvent(out, StreamEvent{Kind: StreamEventDone})
}

func sendStreamEvent(out chan<- StreamEvent, event StreamEvent) {
	out <- event
}

func (r *OpenAICompatibleRuntime) setHeaders(req *http.Request) {
	if r.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	}
	for k, v := range r.cfg.Headers {
		req.Header.Set(k, v)
	}
}

type openAIChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stream         bool            `json:"stream"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string           `json:"type"`
	JSONSchema jsonSchemaFormat `json:"json_schema"`
}

type jsonSchemaFormat struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openAIChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta   Message `json:"delta"`
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", fmt.Errorf("base_url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base_url must be an absolute URL")
	}
	if strings.HasSuffix(parsed.Path, "/v1") {
		raw = strings.TrimSuffix(raw, "/v1")
	}
	return raw, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func trimBody(body []byte) string {
	msg := strings.TrimSpace(string(body))
	if len(msg) > 1000 {
		return msg[:1000] + "..."
	}
	if msg == "" {
		return "empty response body"
	}
	return msg
}
