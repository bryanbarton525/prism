package eval

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLexicalControlUsesProductionHintsAndCannotMixBaselines(t *testing.T) {
	suite := fixtureSuite(t)
	opts := Options{LexicalAdvice: true, Models: []string{"candidate"}, Repeat: 3, Timeout: 10 * time.Second, Generation: Generation{Temperature: 0.1, MaxTokens: 1024}}
	report, err := Run(context.Background(), suite, &scriptedRuntime{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if report.ControlMode != "lexical" || report.Version == Version || report.LayaURL != "" || report.LayaMode != "" {
		t.Fatalf("incorrect control provenance: %+v", report)
	}
	for _, trial := range report.Trials {
		if !trial.Grade.Passed || !trial.Grade.Checks["lexical_applied"] || trial.Recommendation == nil || trial.Recommendation.Result.ScoreKind != "lexical_overlap" {
			t.Fatalf("lexical production path did not run: %+v", trial)
		}
		if _, exists := trial.Grade.Checks["laya_applied"]; exists {
			t.Fatal("lexical control mislabeled as Laya")
		}
		messages := trial.Calls[0].Request.Messages
		if !strings.HasSuffix(strings.TrimSpace(messages[len(messages)-1].Content), "}") || !strings.Contains(messages[len(messages)-1].Content, "# Suggested Tools") {
			t.Fatal("production recommendation was not appended to the actual model request")
		}
	}
	if err := ValidateReport(report); err != nil {
		t.Fatal(err)
	}
	if err := Export(report, filepath.Join(t.TempDir(), "export")); err != nil {
		t.Fatal(err)
	}
	base, err := CreateBaseline(report, "candidate")
	if err != nil || base.ControlMode != "lexical" || base.Version == Version {
		t.Fatalf("control baseline provenance lost: %v", err)
	}
	plain := runScript(t, suite, &scriptedRuntime{}, "candidate")
	plainRequest := plain.Trials[0].Calls[0].Request
	controlRequest := report.Trials[0].Calls[0].Request
	last := len(plainRequest.Messages) - 1
	if !strings.HasPrefix(controlRequest.Messages[last].Content, plainRequest.Messages[last].Content+"\n\n# Suggested Tools") || Hash(controlRequest.Messages[:last]) != Hash(plainRequest.Messages[:last]) || Hash(controlRequest.Tools) != Hash(plainRequest.Tools) {
		t.Fatal("control changed the model context beyond the production advisory block")
	}
	if _, err := Compare(base, plain, "candidate"); err == nil {
		t.Fatal("lexical baseline accepted a plain candidate")
	}
	plainBase, err := CreateBaseline(plain, "candidate")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compare(plainBase, report, "candidate"); err == nil {
		t.Fatal("plain baseline accepted a lexical candidate")
	}
	report.Trials[0].Recommendation = nil
	report.Trials[0].Grade = GradeTrial(suite.Cases[0], report.Trials[0])
	if err := ValidateReport(report); err == nil {
		t.Fatal("missing control trace was accepted")
	}
	opts.LayaURL = "http://127.0.0.1:1"
	if _, err := Run(context.Background(), suite, &scriptedRuntime{}, opts); err == nil {
		t.Fatal("combined lexical/Laya mode accepted")
	}
}
