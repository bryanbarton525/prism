// Package app wires together the Prism runtime dependencies and exposes a
// single App value consumed by both the CLI and MCP adapters.
package app

import (
	"github.com/bryanbarton525/prism/internal/config"
)

// App holds the resolved configuration and any shared runtime services.
// Additional services (OllamaClient, AgentRegistry, etc.) will be added here
// as the runtime is fleshed out in later milestones.
type App struct {
	Config *config.Config
}

// New builds an App from the resolved configuration.
func New(cfg *config.Config) *App {
	return &App{Config: cfg}
}
