package toolmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLayaUsesOneOrderedChoice(t *testing.T) {
	tools := make([]DecisionTool, 20)
	probabilities := map[string]float64{}
	for i := range tools {
		tools[i] = DecisionTool{Name: fmt.Sprintf("tool%d", i), Description: "Read status"}
		probabilities[fmt.Sprintf("tool_%d", i)] = 0.05
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Model      string `json:"model"`
			MaxLen     int    `json:"max_len"`
			HeadMaxLen int    `json:"head_max_len"`
			Questions  map[string]struct {
				Type     string            `json:"type"`
				Criteria map[string]string `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
		}
		if r.URL.Path != "/v1/systemone" || request.Model != "english" || request.MaxLen != 1024 || request.HeadMaxLen != 512 || len(request.Questions) != 1 || request.Questions["tool"].Type != "choice" || len(request.Questions["tool"].Criteria) != 20 {
			t.Errorf("unexpected request: %s", body)
		}
		if strings.Index(string(body), `"tool_2":`) > strings.Index(string(body), `"tool_10":`) {
			t.Error("candidate order changed")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{"tool": map[string]any{"type": "choice", "probabilities": probabilities}}})
	}))
	defer server.Close()
	client, err := NewDecisionClient(server.URL, "", "laya-english")
	if err != nil {
		t.Fatal(err)
	}
	scores, err := client.Score(context.Background(), "Read service status", tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 20 || scores[19] != 0.05 || client.ScoreKind() != "laya_choice" || client.ModelIdentity() != "laya-english" {
		t.Fatalf("scores=%v", scores)
	}
}

func TestLayaRejectsInvalidChoiceDistributions(t *testing.T) {
	for _, probabilities := range []string{`{}`, `{"tool_0":null}`, `{"tool_0":-0.1}`, `{"tool_0":1.1}`, `{"tool_0":0.5}`, `{"wrong":1}`, `{"tool_0":1,"extra":0}`} {
		t.Run(probabilities, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(w, `{"answers":{"tool":{"type":"choice","probabilities":%s}}}`, probabilities)
			}))
			defer server.Close()
			client, err := NewDecisionClient(server.URL, "", "laya-english")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Score(context.Background(), "Read status", []DecisionTool{{Name: "status"}}); err == nil {
				t.Fatal("invalid distribution accepted")
			}
		})
	}
}
