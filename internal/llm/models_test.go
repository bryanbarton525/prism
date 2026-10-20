package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

func TestListModelsProtocolsAndAuthentication(t *testing.T) {
	for _, engine := range []runtime.Engine{runtime.EngineOllama, runtime.EngineSGLang, runtime.EngineVLLM} {
		t.Run(string(engine), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				path := "/v1/models"
				body := `{"data":[{"id":"coder"}]}`
				if engine == runtime.EngineOllama {
					path = "/api/tags"
					body = `{"models":[{"name":"coder:latest"}]}`
				}
				if req.Method != "GET" || req.URL.Path != path || req.Header.Get("Authorization") != "Bearer private-key" {
					t.Errorf("unexpected discovery request: %s %s", req.Method, req.URL.Path)
				}
				w.Write([]byte(body))
			}))
			defer server.Close()
			for _, suffix := range []string{"", "/"} {
				base := server.URL
				if engine != runtime.EngineOllama {
					base += "/v1"
				}
				cfg := runtime.Config{Engine: engine, BaseURL: base + suffix, APIKey: "private-key", Model: "coder"}
				models, err := ListModels(context.Background(), cfg)
				if err != nil || len(models) != 1 || !ModelAvailable(cfg, models) {
					t.Fatalf("models=%v err=%v", models, err)
				}
			}
		})
	}
}

func TestListModelsRejectsFailureResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"unauthorized", 401, "secret-provider-body"}, {"redirect", 302, ""}, {"malformed", 200, "not-json"}, {"wrong-shape", 200, `{"ok":true}`}, {"null-array", 200, `{"data":null}`}, {"oversized", 200, strings.Repeat("x", (4<<20)+1)}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "http://should-not-be-contacted.invalid")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := ListModels(context.Background(), runtime.Config{Engine: runtime.EngineSGLang, BaseURL: server.URL})
			if err == nil || strings.Contains(err.Error(), "secret-provider-body") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestListModelsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := ListModels(ctx, runtime.Config{Engine: runtime.EngineOllama, BaseURL: server.URL}); err == nil {
		t.Fatal("expected deadline failure")
	}
}

func TestValidateModelTarget(t *testing.T) {
	for _, url := range []string{"", "file:///tmp/models", "http://user:secret@host/v1", "http://host/v1?key=secret", "http://host/#fragment"} {
		if err := ValidateModelTarget(runtime.Config{Engine: runtime.EngineSGLang, BaseURL: url}); err == nil {
			t.Errorf("accepted %q", url)
		}
	}
	if ModelAvailable(runtime.Config{Engine: runtime.EngineSGLang, Model: "coder"}, []string{"coder:latest"}) {
		t.Fatal("OpenAI-compatible IDs must match exactly")
	}
	if !ModelAvailable(runtime.Config{Engine: runtime.EngineOllama, Model: "registry.local:5000/coder"}, []string{"registry.local:5000/coder:latest"}) {
		t.Fatal("registry port was mistaken for an Ollama model tag")
	}
	public := PublicConfig(runtime.Config{Engine: runtime.EngineSGLang, BaseURL: "https://user:secret@host/v1?api_key=secret#secret", APIKey: "secret", Headers: map[string]string{"X-Key": "secret"}})
	if public.APIKey != "" || public.Headers != nil || strings.Contains(public.BaseURL, "secret") || public.BaseURL != "https://host/v1" {
		t.Fatalf("public config=%+v", public)
	}
}
