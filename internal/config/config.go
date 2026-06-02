// Package config centralizes Prism runtime configuration.
package config

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/viper"
)

const DefaultOllamaHost = "http://127.0.0.1:11434"

// Settings holds process-level configuration read from defaults, .env, and env.
type Settings struct {
	RootDir     string
	AgentDir    string
	SkillsDir   string
	OllamaHost  string
	GitHubToken string
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

	_ = v.BindEnv("github_token", "PRISM_GITHUB_TOKEN", "PRISM_GH_TOKEN", "GITHUB_TOKEN", "GH_TOKEN")
	return v
}

func settingsFrom(v *viper.Viper) Settings {
	rootDir := firstNonEmpty(v.GetString("PRISM_ROOT"), v.GetString("root"))
	if rootDir == "" {
		rootDir = defaultRoot()
	}
	return Settings{
		RootDir:     rootDir,
		AgentDir:    firstNonEmpty(v.GetString("agent_dir"), v.GetString("PRISM_AGENT_DIR")),
		SkillsDir:   firstNonEmpty(v.GetString("skills_dir"), v.GetString("PRISM_SKILLS_DIR")),
		OllamaHost:  firstNonEmpty(v.GetString("PRISM_OLLAMA_HOST"), v.GetString("ollama_host")),
		GitHubToken: firstNonEmpty(v.GetString("github_token"), v.GetString("PRISM_GITHUB_TOKEN"), v.GetString("PRISM_GH_TOKEN"), v.GetString("GITHUB_TOKEN"), v.GetString("GH_TOKEN")),
	}
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

func isConfigNotFound(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	return errors.As(err, &notFound) || errors.Is(err, os.ErrNotExist)
}
