package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLayaEvaluationRecordsApplicationAndFallback(t *testing.T) {
	for _, failing := range []bool{false, true} {
		t.Run(map[bool]string{false: "applied", true: "fallback"}[failing], func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/v1/systemone" {
					t.Errorf("path=%s", r.URL.Path)
				}
				if failing {
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{"tool": map[string]any{"type": "choice", "probabilities": map[string]float64{"tool_0": 1}}}})
			}))
			defer server.Close()
			report, err := Run(context.Background(), fixtureSuite(t), &scriptedRuntime{}, Options{LayaMode: "advisory", Models: []string{"candidate"}, Repeat: 3, Timeout: 10 * time.Second, Generation: Generation{MaxTokens: 1024}, LayaURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			if requests != 3 {
				t.Fatalf("requests=%d", requests)
			}
			if err := ValidateReport(report); err != nil {
				t.Fatal(err)
			}
			for _, trial := range report.Trials {
				if trial.Recommendation == nil || trial.Grade.Checks["laya_applied"] == failing || trial.Grade.Passed == failing {
					t.Fatalf("trial=%+v", trial)
				}
				found := false
				for _, message := range trial.Calls[0].Request.Messages {
					if strings.Contains(message.Content, "# Suggested Tools") {
						found = true
					}
				}
				if !found {
					t.Fatal("recommendations absent from actual model request")
				}
			}
			baseline, err := CreateBaseline(report, "candidate")
			if failing {
				if err == nil {
					t.Fatal("fallback baseline accepted")
				}
				return
			}
			if err != nil || !baseline.LayaEnabled {
				t.Fatalf("baseline=%+v error=%v", baseline, err)
			}
			off := runScript(t, fixtureSuite(t), &scriptedRuntime{}, "candidate")
			if _, err := Compare(baseline, off, "candidate"); err == nil {
				t.Fatal("mixed modes accepted")
			}
			report.Trials[0].Recommendation = nil
			report.Trials[0].Grade = GradeTrial(report.Suite.Cases[0], report.Trials[0])
			if err := ValidateReport(report); err == nil {
				t.Fatal("missing trace accepted")
			}
		})
	}
}
