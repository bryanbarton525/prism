// Package ollama provides a minimal HTTP client for the local Ollama API.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultHost = "http://127.0.0.1:11434"

// Client communicates with a local Ollama daemon.
type Client struct {
	host       string
	httpClient *http.Client
}

// New creates a Client targeting the given host. If host is empty the default
// localhost address is used.
func New(host string) *Client {
	if host == "" {
		host = defaultHost
	}
	return &Client{
		host: host,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Model is a single entry returned by the /api/tags endpoint.
type Model struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
}

// ListModels returns the models available in the local Ollama daemon.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.host+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: GET /api/tags: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: GET /api/tags returned status %d", resp.StatusCode)
	}
	var result struct {
		Models []Model `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama: decoding /api/tags response: %w", err)
	}
	return result.Models, nil
}

// Ping verifies that the Ollama daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.host+"/", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable at %s: %w", c.host, err)
	}
	resp.Body.Close()
	return nil
}

// ChatMessage is a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the payload sent to /api/chat.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  chatOptions   `json:"options,omitempty"`
}

type chatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}

// ChatResponse is the (non-streaming) response from /api/chat.
type ChatResponse struct {
	Model     string      `json:"model"`
	Message   ChatMessage `json:"message"`
	DoneReason string     `json:"done_reason"`
	Done      bool        `json:"done"`
	// Token counts (present when done=true)
	PromptEvalCount   int `json:"prompt_eval_count"`
	EvalCount         int `json:"eval_count"`
	TotalDurationNs   int64 `json:"total_duration"`
}

// Chat sends a chat completion request and returns the full response.
// The context deadline is respected; callers should set a deadline matching
// the agent's latency_budget_ms.
func (c *Client) Chat(ctx context.Context, model string, messages []ChatMessage, temperature float64, contextSize int) (*ChatResponse, error) {
	payload := chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options: chatOptions{
			Temperature: temperature,
			NumCtx:      contextSize,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: POST /api/chat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: /api/chat status %d: %s", resp.StatusCode, string(errBody))
	}
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("ollama: decoding /api/chat response: %w", err)
	}
	return &chatResp, nil
}

// Host returns the configured Ollama host URL.
func (c *Client) Host() string {
	return c.host
}
