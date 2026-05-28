package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/config"
)

func TestDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := config.Load(config.Flags{})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.OllamaHost != config.DefaultOllamaHost {
		t.Errorf("OllamaHost = %q; want %q", cfg.OllamaHost, config.DefaultOllamaHost)
	}
	if cfg.AgentDir != config.DefaultAgentDir {
		t.Errorf("AgentDir = %q; want %q", cfg.AgentDir, config.DefaultAgentDir)
	}
}

func TestEnvOverridesDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv(config.EnvOllamaHost, "http://10.0.0.1:11434")
	t.Setenv(config.EnvDefaultModel, "llama3.1:8b")
	t.Setenv(config.EnvAgentDir, "/custom/agents")

	cfg, err := config.Load(config.Flags{})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.OllamaHost != "http://10.0.0.1:11434" {
		t.Errorf("OllamaHost = %q; want http://10.0.0.1:11434", cfg.OllamaHost)
	}
	if cfg.DefaultModel != "llama3.1:8b" {
		t.Errorf("DefaultModel = %q; want llama3.1:8b", cfg.DefaultModel)
	}
	if cfg.AgentDir != "/custom/agents" {
		t.Errorf("AgentDir = %q; want /custom/agents", cfg.AgentDir)
	}
}

func TestFlagsOverrideEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv(config.EnvOllamaHost, "http://env-host:11434")

	flags := config.Flags{
		OllamaHost: "http://flag-host:11434",
	}
	cfg, err := config.Load(flags)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.OllamaHost != "http://flag-host:11434" {
		t.Errorf("OllamaHost = %q; want http://flag-host:11434", cfg.OllamaHost)
	}
}

func TestConfigFileOverridesDefaults(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.json")
	data, _ := json.Marshal(map[string]string{
		"ollama_host":   "http://file-host:11434",
		"default_model": "phi3:mini",
		"agent_dir":     "/file/agents",
	})
	if err := os.WriteFile(cfgFile, data, 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	flags := config.Flags{ConfigPath: cfgFile}
	cfg, err := config.Load(flags)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.OllamaHost != "http://file-host:11434" {
		t.Errorf("OllamaHost = %q; want http://file-host:11434", cfg.OllamaHost)
	}
	if cfg.DefaultModel != "phi3:mini" {
		t.Errorf("DefaultModel = %q; want phi3:mini", cfg.DefaultModel)
	}
	if cfg.AgentDir != "/file/agents" {
		t.Errorf("AgentDir = %q; want /file/agents", cfg.AgentDir)
	}
	if cfg.ConfigPath != cfgFile {
		t.Errorf("ConfigPath = %q; want %q", cfg.ConfigPath, cfgFile)
	}
}

func TestFlagsOverrideConfigFile(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.json")
	data, _ := json.Marshal(map[string]string{
		"ollama_host": "http://file-host:11434",
	})
	if err := os.WriteFile(cfgFile, data, 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	flags := config.Flags{
		ConfigPath: cfgFile,
		OllamaHost: "http://flag-host:11434",
	}
	cfg, err := config.Load(flags)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.OllamaHost != "http://flag-host:11434" {
		t.Errorf("OllamaHost = %q; want http://flag-host:11434", cfg.OllamaHost)
	}
}

func TestConfigFileMissingIsNotAnError(t *testing.T) {
	clearEnv(t)
	flags := config.Flags{ConfigPath: "/nonexistent/path/config.json"}
	cfg, err := config.Load(flags)
	if err != nil {
		t.Fatalf("Load() unexpected error for missing file: %v", err)
	}
	if cfg.OllamaHost != config.DefaultOllamaHost {
		t.Errorf("OllamaHost = %q; want default %q", cfg.OllamaHost, config.DefaultOllamaHost)
	}
}

func TestEnvConfigPathTakesPrecedenceOverXDGDefault(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "env-config.json")
	data, _ := json.Marshal(map[string]string{"ollama_host": "http://env-cfg:11434"})
	if err := os.WriteFile(cfgFile, data, 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	t.Setenv(config.EnvConfig, cfgFile)

	cfg, err := config.Load(config.Flags{})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.OllamaHost != "http://env-cfg:11434" {
		t.Errorf("OllamaHost = %q; want http://env-cfg:11434", cfg.OllamaHost)
	}
}

func TestInvalidConfigFileReturnsError(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(cfgFile, []byte("not json {{"), 0o644); err != nil {
		t.Fatalf("writing bad config file: %v", err)
	}

	_, err := config.Load(config.Flags{ConfigPath: cfgFile})
	if err == nil {
		t.Error("Load() expected error for invalid JSON, got nil")
	}
}

// clearEnv removes all PRISM_* environment variables to prevent test pollution.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		config.EnvConfig,
		config.EnvOllamaHost,
		config.EnvDefaultModel,
		config.EnvAgentDir,
	} {
		t.Setenv(key, "")
	}
}
