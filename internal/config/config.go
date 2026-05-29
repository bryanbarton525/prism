// Package config resolves Prism runtime configuration from four ordered sources:
// command flags, environment variables, a config file, and compiled-in defaults.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultOllamaHost = "http://127.0.0.1:11434"
	DefaultAgentDir   = "agents"
	DefaultModel      = ""

	EnvConfig      = "PRISM_CONFIG"
	EnvOllamaHost  = "PRISM_OLLAMA_HOST"
	EnvDefaultModel = "PRISM_DEFAULT_MODEL"
	EnvAgentDir    = "PRISM_AGENT_DIR"
)

// Config holds all Prism runtime configuration.
type Config struct {
	// OllamaHost is the base URL of the local Ollama API.
	OllamaHost string `json:"ollama_host"`
	// DefaultModel is the fallback Ollama model tag when an agent spec omits one.
	DefaultModel string `json:"default_model"`
	// AgentDir is the directory containing Markdown+frontmatter agent specs.
	AgentDir string `json:"agent_dir"`
	// ConfigPath is the resolved path of the config file that was loaded (if any).
	ConfigPath string `json:"-"`
}

// fileConfig is the JSON structure read from a config file.
type fileConfig struct {
	OllamaHost   string `json:"ollama_host"`
	DefaultModel string `json:"default_model"`
	AgentDir     string `json:"agent_dir"`
}

// Flags carries values explicitly set via CLI flags. Empty string means "not set".
type Flags struct {
	OllamaHost   string
	DefaultModel string
	AgentDir     string
	ConfigPath   string
}

// Load resolves configuration in priority order:
//  1. flags (highest priority)
//  2. environment variables
//  3. config file
//  4. built-in defaults (lowest priority)
func Load(flags Flags) (*Config, error) {
	cfg := defaults()

	// Layer 3: config file
	cfgPath, err := resolveConfigPath(flags.ConfigPath)
	if err != nil {
		return nil, err
	}
	if cfgPath != "" {
		if err := applyFile(cfg, cfgPath); err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", cfgPath, err)
		}
		cfg.ConfigPath = cfgPath
	}

	// Layer 2: environment variables
	applyEnv(cfg)

	// Layer 1: flags
	applyFlags(cfg, flags)

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		OllamaHost:   DefaultOllamaHost,
		DefaultModel: DefaultModel,
		AgentDir:     DefaultAgentDir,
	}
}

// resolveConfigPath determines which config file to load.
// Resolution order: explicit flag/env PRISM_CONFIG → XDG default → none.
func resolveConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if env := os.Getenv(EnvConfig); env != "" {
		return env, nil
	}
	// XDG default: $HOME/.config/prism/config.json
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil // best-effort; no home dir means no default file
	}
	candidate := filepath.Join(home, ".config", "prism", "config.json")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", nil
}

func applyFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return err
	}
	if fc.OllamaHost != "" {
		cfg.OllamaHost = fc.OllamaHost
	}
	if fc.DefaultModel != "" {
		cfg.DefaultModel = fc.DefaultModel
	}
	if fc.AgentDir != "" {
		cfg.AgentDir = fc.AgentDir
	}
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv(EnvOllamaHost); v != "" {
		cfg.OllamaHost = v
	}
	if v := os.Getenv(EnvDefaultModel); v != "" {
		cfg.DefaultModel = v
	}
	if v := os.Getenv(EnvAgentDir); v != "" {
		cfg.AgentDir = v
	}
}

func applyFlags(cfg *Config, flags Flags) {
	if flags.OllamaHost != "" {
		cfg.OllamaHost = flags.OllamaHost
	}
	if flags.DefaultModel != "" {
		cfg.DefaultModel = flags.DefaultModel
	}
	if flags.AgentDir != "" {
		cfg.AgentDir = flags.AgentDir
	}
}
