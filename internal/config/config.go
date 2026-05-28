// Package config handles Prism configuration resolution.
//
// Resolution order: command flags > environment variables > config file > defaults.
package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultOllamaHost = "http://127.0.0.1:11434"
)

// Config holds resolved Prism configuration values.
type Config struct {
	// OllamaHost is the base URL for the local Ollama instance.
	OllamaHost string

	// DefaultModel is the fallback model tag when an agent does not specify one.
	DefaultModel string

	// AgentDir is the directory that contains agent spec Markdown files.
	AgentDir string

	// ConstitutionDir is the directory for standalone constitution files.
	ConstitutionDir string

	// SkillDir is the directory that contains Agent Skills directories.
	SkillDir string

	// ConfigFile is the path to the resolved config file (may be empty).
	ConfigFile string
}

// Load resolves configuration by merging environment variables with defaults.
// Flag overrides are applied by the caller after Load returns.
func Load(repoRoot string) *Config {
	c := &Config{
		OllamaHost:      DefaultOllamaHost,
		AgentDir:        filepath.Join(repoRoot, "agents"),
		ConstitutionDir: filepath.Join(repoRoot, "constitutions"),
		SkillDir:        filepath.Join(repoRoot, "skills"),
	}

	if v := os.Getenv("PRISM_OLLAMA_HOST"); v != "" {
		c.OllamaHost = v
	}
	if v := os.Getenv("PRISM_DEFAULT_MODEL"); v != "" {
		c.DefaultModel = v
	}
	if v := os.Getenv("PRISM_AGENT_DIR"); v != "" {
		c.AgentDir = v
	}
	if v := os.Getenv("PRISM_CONFIG"); v != "" {
		c.ConfigFile = v
	}

	return c
}
