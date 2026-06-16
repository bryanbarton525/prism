// Package config centralizes Prism runtime configuration.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
// working directory, an optional Prism config env file, and environment
// variables.
func Load() (Settings, error) {
	v := newViper()
	if err := v.ReadInConfig(); err != nil && !isConfigNotFound(err) {
		return Settings{}, err
	}
	fileEnv, err := loadPrismConfigEnv(v)
	if err != nil {
		return Settings{}, err
	}
	return settingsFrom(v, fileEnv), nil
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
	_ = v.BindEnv("config_file", "PRISM_CONFIG_FILE")
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

func settingsFrom(v *viper.Viper, fileEnv map[string]string) Settings {
	rootDir := configValue(v, fileEnv, "root", "PRISM_ROOT")
	if rootDir == "" {
		rootDir = defaultRoot()
	}
	stateDir := configValue(v, fileEnv, "state_dir", "PRISM_STATE_DIR")
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	return Settings{
		RootDir:      rootDir,
		AgentDir:     configValue(v, fileEnv, "agent_dir", "PRISM_AGENT_DIR"),
		SkillsDir:    configValue(v, fileEnv, "skills_dir", "PRISM_SKILLS_DIR"),
		OllamaHost:   firstNonEmpty(configValue(v, fileEnv, "ollama_host", "PRISM_OLLAMA_HOST"), DefaultOllamaHost),
		GitHubToken:  configValue(v, fileEnv, "github_token", "PRISM_GITHUB_TOKEN", "PRISM_GH_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"),
		LinearMCPURL: configValue(v, fileEnv, "linear_mcp_url", "PRISM_LINEAR_MCP_URL"),
		ModelRuntime: modelRuntimeFrom(v, fileEnv),
		EventStore:   configValue(v, fileEnv, "event_store", "PRISM_EVENT_STORE"),
		PolicyFile:   configValue(v, fileEnv, "policy_file", "PRISM_POLICY_FILE"),
		StateDir:     stateDir,
	}
}

func modelRuntimeFrom(v *viper.Viper, fileEnv map[string]string) runtime.RuntimeConfig {
	primary := runtime.Config{
		Engine:  runtime.Engine(configValue(v, fileEnv, "model_runtime_engine", "PRISM_MODEL_RUNTIME_ENGINE")),
		BaseURL: configValue(v, fileEnv, "model_runtime_base_url", "PRISM_MODEL_RUNTIME_BASE_URL"),
		APIKey:  configValue(v, fileEnv, "model_runtime_api_key", "PRISM_MODEL_RUNTIME_API_KEY"),
		Model:   configValue(v, fileEnv, "model_runtime_model", "PRISM_MODEL_RUNTIME_MODEL"),
	}
	fallbackEngine := runtime.Engine(configValue(v, fileEnv, "model_runtime_fallback_engine", "PRISM_MODEL_RUNTIME_FALLBACK_ENGINE"))
	fallbackBaseURL := configValue(v, fileEnv, "model_runtime_fallback_base_url", "PRISM_MODEL_RUNTIME_FALLBACK_BASE_URL")
	var fallback *runtime.Config
	if fallbackEngine != "" || fallbackBaseURL != "" {
		fallback = &runtime.Config{
			Engine:  fallbackEngine,
			BaseURL: fallbackBaseURL,
			APIKey:  configValue(v, fileEnv, "model_runtime_fallback_api_key", "PRISM_MODEL_RUNTIME_FALLBACK_API_KEY"),
			Model:   configValue(v, fileEnv, "model_runtime_fallback_model", "PRISM_MODEL_RUNTIME_FALLBACK_MODEL"),
		}
	}
	return runtime.RuntimeConfig{Primary: primary, Fallback: fallback}
}

func configValue(v *viper.Viper, fileEnv map[string]string, key string, envNames ...string) string {
	for _, name := range envNames {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	for _, name := range envNames {
		if value := v.GetString(name); value != "" {
			return value
		}
	}
	if value := v.GetString(key); value != "" && !isDefaultOnly(key, value) {
		return value
	}
	for _, name := range envNames {
		if value := fileEnv[name]; value != "" {
			return value
		}
	}
	return v.GetString(key)
}

func isDefaultOnly(key, value string) bool {
	switch key {
	case "ollama_host":
		return value == DefaultOllamaHost && os.Getenv("PRISM_OLLAMA_HOST") == ""
	case "state_dir":
		return value == defaultStateDir() && os.Getenv("PRISM_STATE_DIR") == ""
	default:
		return false
	}
}

func loadPrismConfigEnv(v *viper.Viper) (map[string]string, error) {
	path := prismConfigPath(v)
	if path == "" {
		return nil, nil
	}
	env, err := parseEnvFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return env, nil
}

func prismConfigPath(v *viper.Viper) string {
	if path := firstNonEmpty(os.Getenv("PRISM_CONFIG_FILE"), v.GetString("PRISM_CONFIG_FILE"), v.GetString("config_file")); path != "" {
		return path
	}
	stateDir := firstNonEmpty(os.Getenv("PRISM_STATE_DIR"), v.GetString("PRISM_STATE_DIR"), v.GetString("state_dir"))
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	return filepath.Join(stateDir, "config.env")
}

func parseEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := map[string]string{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("parsing %s:%d: expected KEY=VALUE", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("parsing %s:%d: empty key", path, lineNo)
		}
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		env[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return env, nil
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
