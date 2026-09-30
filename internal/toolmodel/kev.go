package toolmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type KevTool struct{ Name, Description string }

// KevClient calls an explicitly started local Kev server. A timed-out server
// exchange disables the adapter until Prism restarts because server inference
// may still be queued after HTTP cancellation.
type KevClient struct {
	url       string
	apiKeyEnv string
	model     string
	http      *http.Client
	slot      chan struct{}
	mu        sync.Mutex
	disabled  bool
}

func NewKevClient(rawURL, apiKeyEnv string) (*KevClient, error) {
	return NewDecisionClient(rawURL, apiKeyEnv, "kev-latest")
}

// NewDecisionClient supports a local Kev server or a compatible HTTPS Jev endpoint.
func NewDecisionClient(rawURL, apiKeyEnv, model string) (*KevClient, error) {
	if model != "kev-latest" && model != "jev-latest" {
		return nil, fmt.Errorf("decision model must be kev-latest or jev-latest")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.Hostname() == "" {
		return nil, fmt.Errorf("decision service URL must be an HTTPS origin or HTTP loopback origin")
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && host != "localhost" && !net.ParseIP(host).IsLoopback() {
		return nil, fmt.Errorf("plaintext decision service URL must use a loopback host")
	}
	return &KevClient{url: strings.TrimSuffix(rawURL, "/"), apiKeyEnv: apiKeyEnv, model: model, slot: make(chan struct{}, 1), http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *KevClient) Score(ctx context.Context, task string, tools []KevTool) ([]float64, error) {
	c.mu.Lock()
	disabled := c.disabled
	c.mu.Unlock()
	if disabled {
		return nil, fmt.Errorf("Kev is disabled after an ambiguous timeout; restart Prism to reset")
	}
	if len(tools) < 1 || len(tools) > 20 {
		return nil, fmt.Errorf("Kev accepts 1 to 20 tool candidates")
	}
	select {
	case c.slot <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	type response struct {
		scores []float64
		err    error
	}
	done := make(chan response, 1)
	go func() {
		defer func() { <-c.slot }()
		hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		scores, err := c.scoreHTTP(hardCtx, task, tools)
		var networkErr net.Error
		if hardCtx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkErr) && networkErr.Timeout()) {
			c.mu.Lock()
			c.disabled = true
			c.mu.Unlock()
		}
		done <- response{scores, err}
	}()
	select {
	case out := <-done:
		return out.scores, out.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *KevClient) scoreHTTP(ctx context.Context, task string, tools []KevTool) ([]float64, error) {
	questions := map[string]any{}
	for i, tool := range tools {
		questions[fmt.Sprintf("tool_%d", i)] = map[string]any{"type": "noul", "instructions": "Would this specific tool help complete the task?", "criteria": map[string]string{"true": tool.Name + ": " + tool.Description, "false": "This tool does not help."}}
	}
	body, _ := json.Marshal(map[string]any{"state": task, "model": c.model, "questions": questions})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/v1/systemone", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKeyEnv != "" {
		key := os.Getenv(c.apiKeyEnv)
		if key == "" {
			return nil, fmt.Errorf("Kev API key environment variable is unset")
		}
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Kev returned HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Answers map[string]struct {
			Type string   `json:"type"`
			Noul *float64 `json:"noul"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		return nil, err
	}
	scores := make([]float64, len(tools))
	for i := range tools {
		answer, ok := parsed.Answers[fmt.Sprintf("tool_%d", i)]
		if !ok || answer.Type != "noul" || answer.Noul == nil || math.IsNaN(*answer.Noul) || math.IsInf(*answer.Noul, 0) || *answer.Noul < 0 || *answer.Noul > 1 {
			return nil, fmt.Errorf("Kev returned invalid tool score")
		}
		scores[i] = *answer.Noul
	}
	return scores, nil
}
