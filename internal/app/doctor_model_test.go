package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

func TestDoctorChecksSelectedRuntimeAndEffectiveModel(t *testing.T) {
	for _, model := range []string{"shared-model", "missing-model"} {
		t.Run(model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer private-key" {
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				w.Write([]byte(`{"data":[{"id":"shared-model"}]}`))
			}))
			defer server.Close()
			runner, err := New(Config{BundleFS: prismbundle.BundleFS(), OllamaHost: "http://127.0.0.1:1", ModelRuntimeConfig: runtime.Config{Engine: runtime.EngineSGLang, BaseURL: server.URL + "/v1", Model: model, APIKey: "private-key"}})
			if err != nil {
				t.Fatal(err)
			}
			report, err := runner.Doctor(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if report.RuntimeEngine != "sglang" || report.RuntimeModel != model {
				t.Fatalf("report=%+v", report)
			}
			if model == "shared-model" && report.Status != "ok" {
				t.Fatalf("configured model available but doctor degraded: %+v", report)
			}
			if model == "missing-model" && report.Status != "degraded" {
				t.Fatal("missing model reported healthy")
			}
			for _, check := range report.Checks {
				if strings.HasPrefix(check.Name, "ollama") {
					t.Fatalf("checked inactive Ollama: %+v", check)
				}
			}
		})
	}
}
