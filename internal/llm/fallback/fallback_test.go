package fallback

import (
	"context"
	"errors"
	"testing"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

type fakeRuntime struct {
	engine        runtime.Engine
	chatErr       error
	streamErr     error
	structuredErr error
	health        *runtime.HealthStatus
	healthErr     error
	calls         int
}

func (f *fakeRuntime) Engine() runtime.Engine { return f.engine }
func (f *fakeRuntime) Chat(context.Context, runtime.ChatRequest) (*runtime.ChatResponse, error) {
	f.calls++
	return &runtime.ChatResponse{Message: runtime.Message{Content: string(f.engine)}}, f.chatErr
}
func (f *fakeRuntime) Stream(context.Context, runtime.ChatRequest) (<-chan runtime.StreamEvent, error) {
	f.calls++
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	ch := make(chan runtime.StreamEvent, 1)
	ch <- runtime.StreamEvent{Kind: runtime.StreamEventDone}
	close(ch)
	return ch, nil
}
func (f *fakeRuntime) GenerateStructured(context.Context, runtime.StructuredRequest) (*runtime.StructuredResponse, error) {
	f.calls++
	return &runtime.StructuredResponse{Parsed: map[string]any{"engine": string(f.engine)}}, f.structuredErr
}
func (f *fakeRuntime) Health(context.Context) (*runtime.HealthStatus, error) {
	f.calls++
	return f.health, f.healthErr
}

func TestChatFallbackDecisions(t *testing.T) {
	cases := []struct {
		name         string
		err          error
		wantFallback bool
	}{
		{"success", nil, false},
		{"timeout", runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindTimeout, 0, "timeout", nil), true},
		{"unavailable", runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindUnavailable, 0, "down", nil), true},
		{"provider", runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindProvider, 500, "server", nil), true},
		{"invalid", runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindInvalidRequest, 400, "bad", nil), false},
		{"unauthorized", runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindUnauthorized, 401, "no", nil), false},
		{"rate limited", runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindRateLimited, 429, "slow", nil), false},
		{"parse", runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindParse, 0, "bad json", nil), false},
		{"plain", errors.New("plain"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			primary := &fakeRuntime{engine: runtime.EngineSGLang, chatErr: tc.err}
			fb := &fakeRuntime{engine: runtime.EngineVLLM}
			_, _ = New(primary, fb).Chat(context.Background(), runtime.ChatRequest{})
			if got := fb.calls > 0; got != tc.wantFallback {
				t.Fatalf("fallback = %v, want %v", got, tc.wantFallback)
			}
		})
	}
}

func TestStructuredFallbackOnTimeout(t *testing.T) {
	primary := &fakeRuntime{engine: runtime.EngineSGLang, structuredErr: runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindTimeout, 0, "timeout", nil)}
	fb := &fakeRuntime{engine: runtime.EngineVLLM}
	_, _ = New(primary, fb).GenerateStructured(context.Background(), runtime.StructuredRequest{})
	if fb.calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fb.calls)
	}
}

func TestStreamPreStartFallback(t *testing.T) {
	primary := &fakeRuntime{engine: runtime.EngineSGLang, streamErr: runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindUnavailable, 0, "down", nil)}
	fb := &fakeRuntime{engine: runtime.EngineVLLM}
	ch, err := New(primary, fb).Stream(context.Background(), runtime.ChatRequest{})
	if err != nil {
		t.Fatalf("Stream(): %v", err)
	}
	if ch == nil || fb.calls != 1 {
		t.Fatalf("fallback stream not used")
	}
}

func TestHealthFallback(t *testing.T) {
	primary := &fakeRuntime{engine: runtime.EngineSGLang, health: &runtime.HealthStatus{Healthy: false}, healthErr: runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindUnavailable, 0, "down", nil)}
	fb := &fakeRuntime{engine: runtime.EngineVLLM, health: &runtime.HealthStatus{Healthy: true, Engine: runtime.EngineVLLM, Detail: "ok"}}
	status, err := New(primary, fb).Health(context.Background())
	if err != nil {
		t.Fatalf("Health(): %v", err)
	}
	if !status.Healthy || status.Engine != runtime.EngineVLLM {
		t.Fatalf("status = %#v", status)
	}
}

func TestHealthBothUnhealthy(t *testing.T) {
	primary := &fakeRuntime{engine: runtime.EngineSGLang, healthErr: runtime.NewError(runtime.EngineSGLang, runtime.ErrorKindUnavailable, 0, "down", nil)}
	fb := &fakeRuntime{engine: runtime.EngineVLLM, healthErr: runtime.NewError(runtime.EngineVLLM, runtime.ErrorKindProvider, 500, "broken", nil)}
	status, err := New(primary, fb).Health(context.Background())
	if err == nil {
		t.Fatal("expected health error")
	}
	if status == nil || status.Healthy {
		t.Fatalf("status = %#v", status)
	}
}
