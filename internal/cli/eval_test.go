package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bryanbarton525/prism/internal/eval"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

func TestEvalCommandSurface(t *testing.T) {
	for _, args := range [][]string{{"run", "--help"}, {"test", "--help"}, {"baseline", "create", "--help"}, {"compare", "--help"}, {"export", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newEvalCmd()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if output.Len() == 0 {
				t.Fatal("missing help")
			}
		})
	}
}
func TestEvalRequiresConcreteOutputAndBaseline(t *testing.T) {
	for _, args := range [][]string{{"run"}, {"test", "--output", "candidate.json"}, {"baseline", "create", "run.json", "--model", "candidate"}, {"compare", "baseline.json", "run.json"}, {"export", "run.json"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newEvalCmd()
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("accepted incomplete command")
			}
		})
	}
}

func TestEvalMatrixOverridesConfiguredModelOnProviderWire(t *testing.T) {
	var mu sync.Mutex
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request runtime.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		requested = append(requested, request.Model)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": request.Model, "choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": `{"summary":"No evidence.","findings":[],"confidence":"low"}`}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	previous := cfg
	defer func() { cfg = previous }()
	cfg.ModelRuntime.Primary = runtime.Config{Engine: runtime.EngineSGLang, BaseURL: server.URL + "/v1", Model: "stale-global-model"}
	suite, err := eval.LoadSuite("")
	if err != nil {
		t.Fatal(err)
	}
	suite.Cases = suite.Cases[:1]
	data, _ := json.Marshal(suite)
	dir := t.TempDir()
	suitePath := filepath.Join(dir, "suite.yaml")
	if err := os.WriteFile(suitePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newEvalRunCmd(false)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--suite", suitePath, "--model", "first,second", "--repeat", "1", "--output", filepath.Join(dir, "run.json")})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requested) != 2 || requested[0] != "first" || requested[1] != "second" {
		t.Fatalf("model matrix pinned or changed: %v", requested)
	}
}
