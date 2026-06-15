package llm

import (
	"context"
	"testing"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

func TestNewRuntimeCreatesOpenAICompatibleRuntime(t *testing.T) {
	for _, engine := range []runtime.Engine{runtime.EngineSGLang, runtime.EngineVLLM} {
		rt, err := NewRuntime(runtime.Config{Engine: engine, BaseURL: "http://127.0.0.1:8000", Model: "m"})
		if err != nil {
			t.Fatalf("NewRuntime(%s): %v", engine, err)
		}
		if rt.Engine() != engine {
			t.Fatalf("engine = %q, want %q", rt.Engine(), engine)
		}
	}
}

func TestNewRuntimeCreatesOllamaRuntime(t *testing.T) {
	rt, err := NewRuntime(runtime.Config{Engine: runtime.EngineOllama, BaseURL: "http://127.0.0.1:11434", Model: "m"})
	if err != nil {
		t.Fatalf("NewRuntime(%s): %v", runtime.EngineOllama, err)
	}
	if rt.Engine() != runtime.EngineOllama {
		t.Fatalf("engine = %q, want %q", rt.Engine(), runtime.EngineOllama)
	}
}

func TestRegistryBuildsCustomRuntime(t *testing.T) {
	reg := NewRegistry()
	reg.Register("custom", func(cfg runtime.Config) (runtime.ModelRuntime, error) {
		return &fakeRuntime{engine: cfg.Engine}, nil
	})
	rt, err := NewRuntimeWithRegistry(reg, runtime.Config{Engine: "custom"})
	if err != nil {
		t.Fatal(err)
	}
	if rt.Engine() != "custom" {
		t.Fatalf("engine = %q", rt.Engine())
	}
}

func TestNewRuntimeRejectsUnsupportedEngine(t *testing.T) {
	_, err := NewRuntime(runtime.Config{Engine: "other", BaseURL: "http://127.0.0.1:8000"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !runtime.IsKind(err, runtime.ErrorKindInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
}

type fakeRuntime struct {
	engine runtime.Engine
}

func (f *fakeRuntime) Engine() runtime.Engine { return f.engine }
func (f *fakeRuntime) Health(context.Context) (*runtime.HealthStatus, error) {
	return &runtime.HealthStatus{Healthy: true, Engine: f.engine}, nil
}
func (f *fakeRuntime) Chat(context.Context, runtime.ChatRequest) (*runtime.ChatResponse, error) {
	return &runtime.ChatResponse{Message: runtime.Message{Content: "ok"}}, nil
}
func (f *fakeRuntime) Stream(context.Context, runtime.ChatRequest) (<-chan runtime.StreamEvent, error) {
	ch := make(chan runtime.StreamEvent)
	close(ch)
	return ch, nil
}
func (f *fakeRuntime) GenerateStructured(context.Context, runtime.StructuredRequest) (*runtime.StructuredResponse, error) {
	return &runtime.StructuredResponse{Parsed: map[string]any{"ok": true}}, nil
}
