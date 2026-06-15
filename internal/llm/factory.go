package llm

import (
	"fmt"
	"sync"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

type Builder func(runtime.Config) (runtime.ModelRuntime, error)

type Registry struct {
	mu       sync.RWMutex
	builders map[runtime.Engine]Builder
}

func NewRegistry() *Registry {
	return &Registry{builders: map[runtime.Engine]Builder{}}
}

func NewDefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(runtime.EngineOllama, NewOllamaRuntime)
	reg.Register(runtime.EngineSGLang, newOpenAICompatibleRuntime)
	reg.Register(runtime.EngineVLLM, newOpenAICompatibleRuntime)
	return reg
}

func (r *Registry) Register(engine runtime.Engine, builder Builder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builders[engine] = builder
}

func (r *Registry) Build(cfg runtime.Config) (runtime.ModelRuntime, error) {
	if cfg.Engine == "" {
		return nil, runtime.NewError(cfg.Engine, runtime.ErrorKindInvalidRequest, 0, "engine is required", nil)
	}
	r.mu.RLock()
	builder := r.builders[cfg.Engine]
	r.mu.RUnlock()
	if builder == nil {
		return nil, runtime.NewError(cfg.Engine, runtime.ErrorKindInvalidRequest, 0, fmt.Sprintf("unsupported model runtime engine %s", cfg.Engine), nil)
	}
	return builder(cfg)
}

var defaultRegistry = NewDefaultRegistry()

func NewRuntime(cfg runtime.Config) (runtime.ModelRuntime, error) {
	return defaultRegistry.Build(cfg)
}

func NewRuntimeWithRegistry(reg *Registry, cfg runtime.Config) (runtime.ModelRuntime, error) {
	if reg == nil {
		reg = defaultRegistry
	}
	return reg.Build(cfg)
}

func newOpenAICompatibleRuntime(cfg runtime.Config) (runtime.ModelRuntime, error) {
	return runtime.NewOpenAICompatibleRuntime(cfg)
}
