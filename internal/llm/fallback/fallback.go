package fallback

import (
	"context"
	"fmt"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

type Runtime struct {
	Primary  runtime.ModelRuntime
	Fallback runtime.ModelRuntime
}

func New(primary runtime.ModelRuntime, fallback runtime.ModelRuntime) *Runtime {
	return &Runtime{Primary: primary, Fallback: fallback}
}

func (r *Runtime) Engine() runtime.Engine {
	if r.Primary == nil {
		return ""
	}
	return r.Primary.Engine()
}

func (r *Runtime) Chat(ctx context.Context, req runtime.ChatRequest) (*runtime.ChatResponse, error) {
	res, err := r.Primary.Chat(ctx, req)
	if shouldFallback(err) && r.Fallback != nil {
		return r.Fallback.Chat(ctx, req)
	}
	return res, err
}

func (r *Runtime) GenerateStructured(ctx context.Context, req runtime.StructuredRequest) (*runtime.StructuredResponse, error) {
	res, err := r.Primary.GenerateStructured(ctx, req)
	if shouldFallback(err) && r.Fallback != nil {
		return r.Fallback.GenerateStructured(ctx, req)
	}
	return res, err
}

func (r *Runtime) Stream(ctx context.Context, req runtime.ChatRequest) (<-chan runtime.StreamEvent, error) {
	ch, err := r.Primary.Stream(ctx, req)
	if shouldFallback(err) && r.Fallback != nil {
		return r.Fallback.Stream(ctx, req)
	}
	// If a stream has started, do not transparently restart on fallback after a
	// mid-stream error because partial output may already have reached callers.
	return ch, err
}

func (r *Runtime) Health(ctx context.Context) (*runtime.HealthStatus, error) {
	primaryStatus, primaryErr := r.Primary.Health(ctx)
	if primaryErr == nil && primaryStatus != nil && primaryStatus.Healthy {
		return primaryStatus, nil
	}
	if r.Fallback == nil {
		return primaryStatus, primaryErr
	}
	fallbackStatus, fallbackErr := r.Fallback.Health(ctx)
	if fallbackErr == nil && fallbackStatus != nil && fallbackStatus.Healthy {
		detail := fmt.Sprintf("primary unhealthy: %v; fallback healthy: %s", primaryErr, fallbackStatus.Detail)
		return &runtime.HealthStatus{Healthy: true, Engine: fallbackStatus.Engine, Detail: detail}, nil
	}
	detail := fmt.Sprintf("primary: %v; fallback: %v", primaryErr, fallbackErr)
	return &runtime.HealthStatus{Healthy: false, Detail: detail}, runtime.NewError(r.Engine(), runtime.ErrorKindUnavailable, 0, detail, nil)
}

func shouldFallback(err error) bool {
	if err == nil {
		return false
	}
	kind, ok := runtime.Kind(err)
	if !ok {
		return false
	}
	switch kind {
	case runtime.ErrorKindTimeout, runtime.ErrorKindUnavailable, runtime.ErrorKindProvider:
		return true
	default:
		return false
	}
}
