package llm

import (
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

func TestNewRuntimeRejectsUnsupportedEngine(t *testing.T) {
	_, err := NewRuntime(runtime.Config{Engine: "other", BaseURL: "http://127.0.0.1:8000"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !runtime.IsKind(err, runtime.ErrorKindInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
}
