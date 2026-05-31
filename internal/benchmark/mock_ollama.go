package benchmark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// mockOllamaServer returns canned responses keyed by skill name found in the chat request.
func mockOllamaServer(responses map[string]string) *httptest.Server {
	var mu sync.Mutex
	callIndex := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"}]}`))
		case "/api/chat":
			var req struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			body := strings.Builder{}
			for _, m := range req.Messages {
				body.WriteString(m.Content)
			}
			combined := body.String()
			text := "benchmark mock response"
			for skill, resp := range responses {
				if strings.Contains(combined, skill) {
					text = resp
					break
				}
			}
			// Synthesis fallback when no skill marker matches.
			if text == "benchmark mock response" {
				if syn, ok := responses["__synthesis__"]; ok {
					text = syn
				}
			}
			promptTokens := EstimateTokens(combined)
			out := map[string]interface{}{
				"model":             "llama3.1:8b",
				"message":           map[string]string{"role": "assistant", "content": text},
				"done":              true,
				"prompt_eval_count": promptTokens,
				"eval_count":        EstimateTokens(text),
			}
			mu.Lock()
			callIndex++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.NotFound(w, r)
		}
	}))
}

func skillResponseMap(scenario *Scenario) (map[string]string, error) {
	m := make(map[string]string)
	for _, d := range scenario.Delegations {
		resp, err := d.MockResponse(scenario.Dir())
		if err != nil {
			return nil, fmt.Errorf("delegation %s: %w", d.ID, err)
		}
		for _, sk := range d.SkillNames {
			m[sk] = resp
		}
	}
	syn, err := scenario.SynthesisResponse()
	if err != nil {
		return nil, err
	}
	m["__synthesis__"] = syn
	return m, nil
}
