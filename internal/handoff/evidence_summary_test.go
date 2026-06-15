package handoff

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

type fakeStructuredRuntime struct {
	req runtime.StructuredRequest
	res *runtime.StructuredResponse
	err error
}

func (f *fakeStructuredRuntime) Engine() runtime.Engine { return runtime.EngineSGLang }
func (f *fakeStructuredRuntime) Health(context.Context) (*runtime.HealthStatus, error) {
	return &runtime.HealthStatus{Healthy: true}, nil
}
func (f *fakeStructuredRuntime) Chat(context.Context, runtime.ChatRequest) (*runtime.ChatResponse, error) {
	return nil, nil
}
func (f *fakeStructuredRuntime) Stream(context.Context, runtime.ChatRequest) (<-chan runtime.StreamEvent, error) {
	return nil, nil
}
func (f *fakeStructuredRuntime) GenerateStructured(_ context.Context, req runtime.StructuredRequest) (*runtime.StructuredResponse, error) {
	f.req = req
	return f.res, f.err
}

func TestGenerateRepoScanSummary(t *testing.T) {
	rt := &fakeStructuredRuntime{res: &runtime.StructuredResponse{Parsed: map[string]any{
		"findings": []any{map[string]any{"title": "Runtime added", "summary": "Adapter exists", "confidence": "high"}},
		"risks":    []any{map[string]any{"title": "None", "severity": "low", "mitigation": "Keep tests"}},
		"sources":  []any{map[string]any{"type": "file", "path": "internal/llm/runtime"}},
	}}}
	summary, err := GenerateRepoScanSummary(context.Background(), rt, RepoScanRequest{Task: "scan", Evidence: "files"})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Findings[0].Title != "Runtime added" {
		t.Fatalf("summary = %#v", summary)
	}
	if rt.req.Name != "evidence_summary" || !rt.req.Strict {
		t.Fatalf("structured request = %#v", rt.req)
	}
	if !strings.Contains(rt.req.Messages[0].Content, "## System") || !strings.Contains(rt.req.Messages[0].Content, "## Evidence") {
		t.Fatalf("prompt missing sections:\n%s", rt.req.Messages[0].Content)
	}
}

func TestGenerateRepoScanSummaryRuntimeFailure(t *testing.T) {
	rt := &fakeStructuredRuntime{err: errors.New("runtime down")}
	_, err := GenerateRepoScanSummary(context.Background(), rt, RepoScanRequest{Task: "scan"})
	if err == nil || !strings.Contains(err.Error(), "generating structured repo scan summary") {
		t.Fatalf("err = %v", err)
	}
}

func TestGenerateRepoScanSummaryDecodeFailure(t *testing.T) {
	rt := &fakeStructuredRuntime{res: &runtime.StructuredResponse{Parsed: map[string]any{"findings": "wrong"}}}
	_, err := GenerateRepoScanSummary(context.Background(), rt, RepoScanRequest{Task: "scan"})
	if err == nil || !strings.Contains(err.Error(), "decoding structured repo scan summary") {
		t.Fatalf("err = %v", err)
	}
}
