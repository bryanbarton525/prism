package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsDotEnvAndGitHubTokenAlias(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PRISM_GITHUB_TOKEN", "")
	t.Setenv("PRISM_GH_TOKEN", "")
	t.Setenv("PRISM_OLLAMA_HOST", "")
	t.Setenv("PRISM_ROOT", "")
	dir := t.TempDir()
	chdir(t, dir)

	env := "GH_TOKEN=ghp_alias\nPRISM_OLLAMA_HOST=http://ollama.example:11434\nPRISM_ROOT=https://github.com/owner/repo\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.GitHubToken != "ghp_alias" {
		t.Fatalf("GitHubToken: want alias token, got %q", cfg.GitHubToken)
	}
	if cfg.OllamaHost != "http://ollama.example:11434" {
		t.Fatalf("OllamaHost: %q", cfg.OllamaHost)
	}
	if cfg.RootDir != "https://github.com/owner/repo" {
		t.Fatalf("RootDir: %q", cfg.RootDir)
	}
}

func TestLoadPrefersCanonicalGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PRISM_GITHUB_TOKEN", "")
	t.Setenv("PRISM_GH_TOKEN", "")
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GH_TOKEN=alias\nGITHUB_TOKEN=canonical\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.GitHubToken != "canonical" {
		t.Fatalf("GitHubToken: want canonical token, got %q", cfg.GitHubToken)
	}
}

func TestLoadDefaultsWithoutDotEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PRISM_GITHUB_TOKEN", "")
	t.Setenv("PRISM_GH_TOKEN", "")
	t.Setenv("PRISM_OLLAMA_HOST", "")
	t.Setenv("PRISM_ROOT", "")
	dir := t.TempDir()
	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	wantRoot, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	gotRoot, err := filepath.EvalSymlinks(cfg.RootDir)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("RootDir: want cwd, got %q", cfg.RootDir)
	}
	if cfg.OllamaHost != DefaultOllamaHost {
		t.Fatalf("OllamaHost: want default, got %q", cfg.OllamaHost)
	}
}

func TestLoadReadsPrismShortGitHubTokenAlias(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PRISM_GITHUB_TOKEN", "")
	t.Setenv("PRISM_GH_TOKEN", "")

	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PRISM_GH_TOKEN=short-prefixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.GitHubToken != "short-prefixed" {
		t.Fatalf("GitHubToken: %q", cfg.GitHubToken)
	}
}

func TestLoadReadsPrismPrefixedEnvironment(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PRISM_GITHUB_TOKEN", "prefixed")
	t.Setenv("PRISM_GH_TOKEN", "")
	t.Setenv("PRISM_OLLAMA_HOST", "http://prefixed.example:11434")
	t.Setenv("PRISM_ROOT", "https://github.com/prefixed/repo")
	t.Setenv("PRISM_AGENT_DIR", "/tmp/agents")
	t.Setenv("PRISM_SKILLS_DIR", "/tmp/skills")

	dir := t.TempDir()
	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.GitHubToken != "prefixed" {
		t.Fatalf("GitHubToken: %q", cfg.GitHubToken)
	}
	if cfg.OllamaHost != "http://prefixed.example:11434" {
		t.Fatalf("OllamaHost: %q", cfg.OllamaHost)
	}
	if cfg.RootDir != "https://github.com/prefixed/repo" {
		t.Fatalf("RootDir: %q", cfg.RootDir)
	}
	if cfg.AgentDir != "/tmp/agents" {
		t.Fatalf("AgentDir: %q", cfg.AgentDir)
	}
	if cfg.SkillsDir != "/tmp/skills" {
		t.Fatalf("SkillsDir: %q", cfg.SkillsDir)
	}
}

func TestLoadReadsModelRuntimeConfig(t *testing.T) {
	t.Setenv("PRISM_MODEL_RUNTIME_ENGINE", "sglang")
	t.Setenv("PRISM_MODEL_RUNTIME_BASE_URL", "http://localhost:30000/v1")
	t.Setenv("PRISM_MODEL_RUNTIME_MODEL", "Qwen/Qwen3-Coder")
	t.Setenv("PRISM_MODEL_RUNTIME_API_KEY", "EMPTY")
	t.Setenv("PRISM_MODEL_RUNTIME_FALLBACK_ENGINE", "vllm")
	t.Setenv("PRISM_MODEL_RUNTIME_FALLBACK_BASE_URL", "http://localhost:8000/v1")
	t.Setenv("PRISM_MODEL_RUNTIME_FALLBACK_MODEL", "Qwen/Qwen3-Coder")

	dir := t.TempDir()
	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.ModelRuntime.Primary.Engine != "sglang" || cfg.ModelRuntime.Primary.BaseURL != "http://localhost:30000/v1" {
		t.Fatalf("primary runtime = %#v", cfg.ModelRuntime.Primary)
	}
	if cfg.ModelRuntime.Primary.APIKey != "EMPTY" || cfg.ModelRuntime.Primary.Model != "Qwen/Qwen3-Coder" {
		t.Fatalf("primary runtime = %#v", cfg.ModelRuntime.Primary)
	}
	if cfg.ModelRuntime.Fallback == nil || cfg.ModelRuntime.Fallback.Engine != "vllm" {
		t.Fatalf("fallback runtime = %#v", cfg.ModelRuntime.Fallback)
	}
}

func TestLoadReadsPrismConfigFileFallback(t *testing.T) {
	t.Setenv("PRISM_MODEL_RUNTIME_ENGINE", "")
	t.Setenv("PRISM_MODEL_RUNTIME_BASE_URL", "")
	t.Setenv("PRISM_MODEL_RUNTIME_MODEL", "")
	t.Setenv("PRISM_MODEL_RUNTIME_API_KEY", "")
	t.Setenv("PRISM_CONFIG_FILE", "")

	dir := t.TempDir()
	chdir(t, dir)
	stateDir := filepath.Join(dir, "state")
	t.Setenv("PRISM_STATE_DIR", stateDir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := `
PRISM_MODEL_RUNTIME_MODEL = "openai/gpt-oss-20b"
PRISM_MODEL_RUNTIME_BASE_URL = "http://sglang.barton.local/v1"
PRISM_MODEL_RUNTIME_ENGINE = "sglang"
`
	if err := os.WriteFile(filepath.Join(stateDir, "config.env"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.ModelRuntime.Primary.Engine != "sglang" {
		t.Fatalf("engine = %q", cfg.ModelRuntime.Primary.Engine)
	}
	if cfg.ModelRuntime.Primary.BaseURL != "http://sglang.barton.local/v1" {
		t.Fatalf("base url = %q", cfg.ModelRuntime.Primary.BaseURL)
	}
	if cfg.ModelRuntime.Primary.Model != "openai/gpt-oss-20b" {
		t.Fatalf("model = %q", cfg.ModelRuntime.Primary.Model)
	}
}

func TestLoadReadsExplicitPrismConfigFile(t *testing.T) {
	t.Setenv("PRISM_MODEL_RUNTIME_ENGINE", "")
	t.Setenv("PRISM_MODEL_RUNTIME_BASE_URL", "")
	t.Setenv("PRISM_MODEL_RUNTIME_MODEL", "")

	dir := t.TempDir()
	chdir(t, dir)
	configPath := filepath.Join(dir, "runtime.env")
	t.Setenv("PRISM_CONFIG_FILE", configPath)
	config := `
export PRISM_MODEL_RUNTIME_MODEL = "openai/gpt-oss-20b"
PRISM_MODEL_RUNTIME_BASE_URL = "http://sglang.barton.local/v1"
PRISM_MODEL_RUNTIME_ENGINE = "sglang"
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.ModelRuntime.Primary.Engine != "sglang" {
		t.Fatalf("engine = %q", cfg.ModelRuntime.Primary.Engine)
	}
	if cfg.ModelRuntime.Primary.BaseURL != "http://sglang.barton.local/v1" {
		t.Fatalf("base url = %q", cfg.ModelRuntime.Primary.BaseURL)
	}
	if cfg.ModelRuntime.Primary.Model != "openai/gpt-oss-20b" {
		t.Fatalf("model = %q", cfg.ModelRuntime.Primary.Model)
	}
}

func TestLoadPrefersPrismEnvOverConfigFile(t *testing.T) {
	t.Setenv("PRISM_MODEL_RUNTIME_ENGINE", "vllm")
	t.Setenv("PRISM_MODEL_RUNTIME_BASE_URL", "http://vllm.local/v1")
	t.Setenv("PRISM_MODEL_RUNTIME_MODEL", "local/model")

	dir := t.TempDir()
	chdir(t, dir)
	configPath := filepath.Join(dir, "runtime.env")
	t.Setenv("PRISM_CONFIG_FILE", configPath)
	config := `
PRISM_MODEL_RUNTIME_MODEL = "openai/gpt-oss-20b"
PRISM_MODEL_RUNTIME_BASE_URL = "http://sglang.barton.local/v1"
PRISM_MODEL_RUNTIME_ENGINE = "sglang"
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.ModelRuntime.Primary.Engine != "vllm" {
		t.Fatalf("engine = %q", cfg.ModelRuntime.Primary.Engine)
	}
	if cfg.ModelRuntime.Primary.BaseURL != "http://vllm.local/v1" {
		t.Fatalf("base url = %q", cfg.ModelRuntime.Primary.BaseURL)
	}
	if cfg.ModelRuntime.Primary.Model != "local/model" {
		t.Fatalf("model = %q", cfg.ModelRuntime.Primary.Model)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}
