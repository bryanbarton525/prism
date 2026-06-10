package llm

import "github.com/bryanbarton525/prism/internal/llm/runtime"

func NewRuntime(cfg runtime.Config) (runtime.ModelRuntime, error) {
	switch cfg.Engine {
	case runtime.EngineSGLang, runtime.EngineVLLM:
		return runtime.NewOpenAICompatibleRuntime(cfg)
	case "":
		return nil, runtime.NewError(cfg.Engine, runtime.ErrorKindInvalidRequest, 0, "engine is required", nil)
	default:
		return nil, runtime.NewError(cfg.Engine, runtime.ErrorKindInvalidRequest, 0, "unsupported model runtime engine "+string(cfg.Engine), nil)
	}
}
