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

type DecisionTool struct{ Name, Description string }

// KevTool preserves compatibility with existing callers.
type KevTool = DecisionTool

// DecisionClient calls an explicitly started decision server. A timed-out server
// exchange disables the adapter until Prism restarts because server inference
// may still be queued after HTTP cancellation.
type DecisionClient struct {
	url       string
	apiKeyEnv string
	model     string
	http      *http.Client
	slot      chan struct{}
	mu        sync.Mutex
	disabled  bool
}

// KevClient preserves compatibility with existing callers.
type KevClient = DecisionClient

const LayaRevision = "55cf4c4ebb4ebe31b2550e8bdf3bd21b99753851"

func NewKevClient(rawURL, apiKeyEnv string) (*DecisionClient, error) {
	return NewDecisionClient(rawURL, apiKeyEnv, "kev-latest")
}

// NewDecisionClient supports Kev/Jev binary scoring and Laya categorical choices.
func NewDecisionClient(rawURL, apiKeyEnv, model string) (*DecisionClient, error) {
	if model != "kev-latest" && model != "jev-latest" && model != "laya-english" {
		return nil, fmt.Errorf("decision model must be kev-latest, jev-latest, or laya-english")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.Hostname() == "" {
		return nil, fmt.Errorf("decision service URL must be an HTTPS origin or HTTP loopback origin")
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && host != "localhost" && !net.ParseIP(host).IsLoopback() {
		return nil, fmt.Errorf("plaintext decision service URL must use a loopback host")
	}
	return &DecisionClient{url: strings.TrimSuffix(rawURL, "/"), apiKeyEnv: apiKeyEnv, model: model, slot: make(chan struct{}, 1), http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *DecisionClient) Score(ctx context.Context, task string, tools []KevTool) ([]float64, error) {
	c.mu.Lock()
	disabled := c.disabled
	c.mu.Unlock()
	if disabled {
		return nil, fmt.Errorf("Decision service is disabled after an ambiguous timeout; restart Prism to reset")
	}
	if len(tools) < 1 || len(tools) > 20 {
		return nil, fmt.Errorf("Decision service accepts 1 to 20 tool candidates")
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

func (c *DecisionClient) scoreHTTP(ctx context.Context, task string, tools []KevTool) ([]float64, error) {
	questions := map[string]any{}
	for i, tool := range tools {
		questions[fmt.Sprintf("tool_%d", i)] = map[string]any{"type": "noul", "instructions": "Would this specific tool help complete the task?", "criteria": map[string]string{"true": tool.Name + ": " + tool.Description, "false": "This tool does not help."}}
	}
	payload := map[string]any{"state": task, "model": c.model, "questions": questions}
	if c.model == "laya-english" {
		criteria := choiceCriteria(tools)
		payload["model"] = "english"
		payload["max_len"] = 1024
		payload["head_max_len"] = 512
		payload["questions"] = map[string]any{"tool": map[string]any{
			"type":         "choice",
			"instructions": "Select the single tool that best directly carries out the user's task. Respect requests to read or inspect without making changes.",
			"criteria":     criteria,
		}}
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/v1/systemone", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKeyEnv != "" {
		key := os.Getenv(c.apiKeyEnv)
		if key == "" {
			return nil, fmt.Errorf("Decision service API key environment variable is unset")
		}
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Decision service returned HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Answers map[string]struct {
			Type          string              `json:"type"`
			Noul          *float64            `json:"noul"`
			Probabilities map[string]*float64 `json:"probabilities"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		return nil, err
	}
	scores := make([]float64, len(tools))
	if c.model == "laya-english" {
		answer, ok := parsed.Answers["tool"]
		if !ok || answer.Type != "choice" || len(answer.Probabilities) != len(tools) {
			return nil, fmt.Errorf("Laya returned an incomplete choice distribution")
		}
		total := 0.0
		for i := range tools {
			value, ok := answer.Probabilities[fmt.Sprintf("tool_%d", i)]
			if !ok || value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 1 {
				return nil, fmt.Errorf("Laya returned an invalid choice probability")
			}
			scores[i] = *value
			total += *value
		}
		if math.Abs(total-1) > 0.01 {
			return nil, fmt.Errorf("Laya returned an unnormalized choice distribution")
		}
		return scores, nil
	}
	for i := range tools {
		answer, ok := parsed.Answers[fmt.Sprintf("tool_%d", i)]
		if !ok || answer.Type != "noul" || answer.Noul == nil || math.IsNaN(*answer.Noul) || math.IsInf(*answer.Noul, 0) || *answer.Noul < 0 || *answer.Noul > 1 {
			return nil, fmt.Errorf("Decision service returned invalid tool score")
		}
		scores[i] = *answer.Noul
	}
	return scores, nil
}

// ScoreKind distinguishes relative choice probabilities from independent binary scores.
func (c *DecisionClient) ScoreKind() string {
	if c.model == "laya-english" {
		return "laya_choice"
	}
	return "kev_noul"
}

func (c *DecisionClient) ModelIdentity() string {
	if c.model == "laya-english" {
		return "laya-english"
	}
	return c.model
}

// choiceCriteria preserves retrieval order: ordinary JSON maps sort tool_10
// before tool_2, changing the option positions seen by an order-sensitive model.
type choiceCriteria []DecisionTool

func (tools choiceCriteria) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for i, tool := range tools {
		if i > 0 {
			out.WriteByte(',')
		}
		key, err := json.Marshal(fmt.Sprintf("tool_%d", i))
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(tool.Name + ": " + tool.Description)
		if err != nil {
			return nil, err
		}
		out.Write(key)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}
