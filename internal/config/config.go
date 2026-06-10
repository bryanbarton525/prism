// Package config centralizes Prism runtime configuration.
package config

import (
	"errors"
	"os"
	"strings"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/spf13/viper"
)

const DefaultOllamaHost = "http://127.0.0.1:11434"

// Settings holds process-level configuration read from defaults, .env, and env.
type Settings struct {
	RootDir      string
	AgentDir     string
	SkillsDir    string
	OllamaHost   string
	GitHubToken  string
	LinearMCPURL string
	ModelRuntime runtime.RuntimeConfig
	EventStore   string
	PolicyFile   string
	StateDir     string
}

// Load reads configuration from defaults, an optional .env file in the current
// working directory, and environment variables.
func Load() (Settings, error) {
	v := newViper()
	if err := v.ReadInConfig(); err != nil && !isConfigNotFound(err) {
		return Settings{}, err
	}
	return settingsFrom(v), nil
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.SetEnvPrefix("prism")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	v.SetDefault("ollama_host", DefaultOllamaHost)
	v.SetDefault("state_dir", defaultStateDir())

	_ = v.BindEnv("github_token", "PRISM_GITHUB_TOKEN", "PRISM_GH_TOKEN", "GITHUB_TOKEN", "GH_TOKEN")
	_ = v.BindEnv("linear_mcp_url", "PRISM_LINEAR_MCP_URL")
	_ = v.BindEnv("model_runtime_engine", "PRISM_MODEL_RUNTIME_ENGINE")
	_ = v.BindEnv("model_runtime_base_url", "PRISM_MODEL_RUNTIME_BASE_URL")
	_ = v.BindEnv("model_runtime_api_key", "PRISM_MODEL_RUNTIME_API_KEY")
	_ = v.BindEnv("model_runtime_model", "PRISM_MODEL_RUNTIME_MODEL")
	_ = v.BindEnv("model_runtime_fallback_engine", "PRISM_MODEL_RUNTIME_FALLBACK_ENGINE")
	_ = v.BindEnv("model_runtime_fallback_base_url", "PRISM_MODEL_RUNTIME_FALLBACK_BASE_URL")
	_ = v.BindEnv("model_runtime_fallback_api_key", "PRISM_MODEL_RUNTIME_FALLBACK_API_KEY")
	_ = v.BindEnv("model_runtime_fallback_model", "PRISM_MODEL_RUNTIME_FALLBACK_MODEL")
	return v
}

func settingsFrom(v *viper.Viper) Settings {
	rootDir := firstNonEmpty(v.GetString("PRISM_ROOT"), v.GetString("root"))
	if rootDir == "" {
		rootDir = defaultRoot()
	}
	stateDir := firstNonEmpty(v.GetString("PRISM_STATE_DIR"), v.GetString("state_dir"))
	return Settings{
		RootDir:      rootDir,
		AgentDir:     firstNonEmpty(v.GetString("agent_dir"), v.GetString("PRISM_AGENT_DIR")),
		SkillsDir:    firstNonEmpty(v.GetString("skills_dir"), v.GetString("PRISM_SKILLS_DIR")),
		OllamaHost:   firstNonEmpty(v.GetString("PRISM_OLLAMA_HOST"), v.GetString("ollama_host")),
		GitHubToken:  firstNonEmpty(v.GetString("github_token"), v.GetString("PRISM_GITHUB_TOKEN"), v.GetString("PRISM_GH_TOKEN"), v.GetString("GITHUB_TOKEN"), v.GetString("GH_TOKEN")),
		LinearMCPURL: firstNonEmpty(v.GetString("linear_mcp_url"), v.GetString("PRISM_LINEAR_MCP_URL")),
		ModelRuntime: modelRuntimeFrom(v),
		EventStore:   firstNonEmpty(v.GetString("PRISM_EVENT_STORE"), v.GetString("event_store")),
		PolicyFile:   firstNonEmpty(v.GetString("PRISM_POLICY_FILE"), v.GetString("policy_file")),
		StateDir:     stateDir,
	}
}

func modelRuntimeFrom(v *viper.Viper) runtime.RuntimeConfig {
	primary := runtime.Config{
		Engine:  runtime.Engine(firstNonEmpty(v.GetString("model_runtime_engine"), v.GetString("PRISM_MODEL_RUNTIME_ENGINE"))),
		BaseURL: firstNonEmpty(v.GetString("model_runtime_base_url"), v.GetString("PRISM_MODEL_RUNTIME_BASE_URL")),
		APIKey:  firstNonEmpty(v.GetString("model_runtime_api_key"), v.GetString("PRISM_MODEL_RUNTIME_API_KEY")),
		Model:   firstNonEmpty(v.GetString("model_runtime_model"), v.GetString("PRISM_MODEL_RUNTIME_MODEL")),
	}
	fallbackEngine := runtime.Engine(firstNonEmpty(v.GetString("model_runtime_fallback_engine"), v.GetString("PRISM_MODEL_RUNTIME_FALLBACK_ENGINE")))
	fallbackBaseURL := firstNonEmpty(v.GetString("model_runtime_fallback_base_url"), v.GetString("PRISM_MODEL_RUNTIME_FALLBACK_BASE_URL"))
	var fallback *runtime.Config
	if fallbackEngine != "" || fallbackBaseURL != "" {
		fallback = &runtime.Config{
			Engine:  fallbackEngine,
			BaseURL: fallbackBaseURL,
			APIKey:  firstNonEmpty(v.GetString("model_runtime_fallback_api_key"), v.GetString("PRISM_MODEL_RUNTIME_FALLBACK_API_KEY")),
			Model:   firstNonEmpty(v.GetString("model_runtime_fallback_model"), v.GetString("PRISM_MODEL_RUNTIME_FALLBACK_MODEL")),
		}
	}
	return runtime.RuntimeConfig{Primary: primary, Fallback: fallback}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultRoot() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func defaultStateDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.prism"
	}
	return ".prism"
}

func isConfigNotFound(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	return errors.As(err, &notFound) || errors.Is(err, os.ErrNotExist)
}
