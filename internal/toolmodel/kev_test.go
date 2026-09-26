package toolmodel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKevScoresIndependentToolQuestions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			t.Errorf("path=%s", r.URL.Path)
		}
		var request struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if len(request.Questions) != 2 {
			t.Errorf("questions=%d", len(request.Questions))
		}
		_, _ = w.Write([]byte(`{"answers":{"tool_0":{"type":"noul","noul":0.2},"tool_1":{"type":"noul","noul":0.9}}}`))
	}))
	defer server.Close()
	client, err := NewKevClient(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	scores, err := client.Score(context.Background(), "find an issue", []KevTool{{Name: "create_issue", Description: "create"}, {Name: "find_issue", Description: "find"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 2 || scores[0] != 0.2 || scores[1] != 0.9 {
		t.Fatalf("scores=%v", scores)
	}
}

func TestKevRejectsMissingNoulScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"answers":{"tool_0":{"type":"noul"}}}`))
	}))
	defer server.Close()
	client, err := NewKevClient(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Score(context.Background(), "find an issue", []KevTool{{Name: "find_issue"}}); err == nil {
		t.Fatal("missing noul score was accepted as zero")
	}
}

func TestKevAllowsHTTPSJevButNotRemotePlainHTTP(t *testing.T) {
	if _, err := NewKevClient("https://decisions.example.com", "JEV_TOKEN"); err != nil {
		t.Fatalf("HTTPS Jev endpoint rejected: %v", err)
	}
	if _, err := NewKevClient("http://decisions.example.com", "JEV_TOKEN"); err == nil {
		t.Fatal("remote plaintext endpoint accepted")
	}
	if _, err := NewKevClient("https://user:secret@decisions.example.com", "JEV_TOKEN"); err == nil {
		t.Fatal("URL credentials accepted")
	}
}

func TestDecisionClientUsesConfiguredJevModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "jev-latest" {
			t.Errorf("model=%q", body.Model)
		}
		_, _ = w.Write([]byte(`{"answers":{"tool_0":{"type":"noul","noul":0.7}}}`))
	}))
	defer server.Close()
	client, err := NewDecisionClient(server.URL, "", "jev-latest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Score(context.Background(), "Find issue", []KevTool{{Name: "find_issue"}}); err != nil {
		t.Fatal(err)
	}
}
