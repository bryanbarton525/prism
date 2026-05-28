// Package ollama provides a minimal Ollama API client for Prism.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultHost = "http://127.0.0.1:11434"

// Client is a lightweight Ollama REST client.
type Client struct {
	host    string
	http    *http.Client
}

// NewClient creates a Client targeting host (defaults to 127.0.0.1:11434).
func NewClient(host string) *Client {
	if host == "" {
		host = defaultHost
	}
	host = strings.TrimRight(host, "/")
	return &Client{
		host: host,
		http: &http.Client{Timeout: 0}, // per-request context controls timeout
	}
}

// Ping checks that the Ollama server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.host+"/api/version", nil)
	if err != nil {
		return fmt.Errorf("building ping request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable at %s: %w", c.host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama ping returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ListModels returns the tags of all locally available models.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.host+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("building list request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing models: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var parsed struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing model list: %w", err)
	}
	names := make([]string, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// ChatRequest is the payload for POST /api/chat.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  *Options  `json:"options,omitempty"`
}

// Message is one turn in an Ollama chat exchange.
type Message struct {
	Role    string `json:"role"`    // system | user | assistant
	Content string `json:"content"`
}

// Options maps to Ollama model parameters.
type Options struct {
	Temperature   float64 `json:"temperature,omitempty"`
	NumCtx        int     `json:"num_ctx,omitempty"`
	NumPredict    int     `json:"num_predict,omitempty"`
}

// ChatResponse is the non-streaming response from POST /api/chat.
type ChatResponse struct {
	Model     string  `json:"model"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
	// Token counts are provided when Ollama is configured to return them.
	PromptEvalCount int `json:"prompt_eval_count"`
	EvalCount       int `json:"eval_count"`
	// Total generation duration in nanoseconds.
	TotalDuration int64 `json:"total_duration"`
}

// Chat sends a non-streaming chat request and returns the response.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending chat request: %w", err)
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama chat returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("parsing chat response: %w", err)
	}

	// If Ollama did not supply TotalDuration, fall back to wall-clock time.
	if chatResp.TotalDuration == 0 {
		chatResp.TotalDuration = elapsed.Nanoseconds()
	}

	return &chatResp, nil
}
