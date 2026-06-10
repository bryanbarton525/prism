package cli

import (
	"fmt"

	"github.com/bryanbarton525/prism/internal/llm"
	"github.com/bryanbarton525/prism/internal/llm/fallback"
	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
)

func configuredModelRuntime() (llmruntime.ModelRuntime, error) {
	if cfg.ModelRuntime.Primary.Engine == "" && cfg.ModelRuntime.Primary.BaseURL == "" {
		return nil, nil
	}
	primary, err := llm.NewRuntime(cfg.ModelRuntime.Primary)
	if err != nil {
		return nil, fmt.Errorf("building primary model runtime: %w", err)
	}
	if cfg.ModelRuntime.Fallback == nil {
		return primary, nil
	}
	fallbackRuntime, err := llm.NewRuntime(*cfg.ModelRuntime.Fallback)
	if err != nil {
		return nil, fmt.Errorf("building fallback model runtime: %w", err)
	}
	return fallback.New(primary, fallbackRuntime), nil
}
