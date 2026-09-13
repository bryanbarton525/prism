package importer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type nativePrismAdapter struct{}

func (nativePrismAdapter) Name() string { return "prism-native" }

func (nativePrismAdapter) Detect(filename string, source []byte) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return (ext == ".md" || ext == ".markdown") && strings.HasPrefix(strings.TrimSpace(string(source)), "---")
}

func (nativePrismAdapter) Translate(_ string, source []byte, _ Config) ([]byte, []Finding, error) {
	return append([]byte{}, source...), nil, nil
}

type codexTOMLAdapter struct{}

func (codexTOMLAdapter) Name() string { return "codex-toml" }

func (codexTOMLAdapter) Detect(filename string, source []byte) bool {
	if strings.ToLower(filepath.Ext(filename)) != ".toml" {
		return false
	}
	var config codexConfig
	return toml.Unmarshal(source, &config) == nil && strings.TrimSpace(config.Name) != ""
}

func (codexTOMLAdapter) Translate(filename string, source []byte, cfg Config) ([]byte, []Finding, error) {
	var config codexConfig
	if err := toml.Unmarshal(source, &config); err != nil {
		return nil, nil, fmt.Errorf("parse Codex TOML: %w", err)
	}
	if strings.TrimSpace(config.Name) == "" {
		return nil, nil, fmt.Errorf("Codex TOML requires name")
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = strings.TrimSpace(cfg.DefaultModel)
	}
	if model == "" {
		return nil, nil, fmt.Errorf("Codex TOML requires model or an explicit default model")
	}
	id := defaultID(config.Name)
	fields := map[string]string{
		"id":             id,
		"name":           fmt.Sprintf("%q", config.Name),
		"description":    `"Imported from Codex TOML configuration."`,
		"model":          fmt.Sprintf("%q", model),
		"context_budget": "16000",
	}
	if config.Description != "" {
		fields["description"] = fmt.Sprintf("%q", config.Description)
	}
	if len(config.AllowedSkills) == 0 {
		return nil, nil, fmt.Errorf("Codex TOML requires allowed_skills to produce a runnable Prism agent")
	}
	body := strings.TrimSpace(config.DeveloperInstructions)
	if body == "" {
		body = "Imported from Codex TOML."
	}
	return renderPrismSpec(fields, config.AllowedSkills, body), nil, nil
}

type codexConfig struct {
	Name                  string   `toml:"name"`
	Description           string   `toml:"description"`
	Model                 string   `toml:"model"`
	AllowedSkills         []string `toml:"allowed_skills"`
	DeveloperInstructions string   `toml:"developer_instructions"`
}

type claudeMarkdownAdapter struct{}

func (claudeMarkdownAdapter) Name() string { return "claude-markdown" }

func (claudeMarkdownAdapter) Detect(filename string, source []byte) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".md" && ext != ".markdown" {
		return false
	}
	s := strings.ToLower(string(source))
	return strings.Contains(s, "claude") || strings.Contains(s, "system prompt") || strings.Contains(s, "role:")
}

func (claudeMarkdownAdapter) Translate(filename string, source []byte, cfg Config) ([]byte, []Finding, error) {
	id := defaultID(filename)
	fields := map[string]string{
		"id":             id,
		"name":           fmt.Sprintf("%q", defaultName(id)),
		"description":    `"Imported from Claude Markdown prompt."`,
		"model":          fmt.Sprintf("%q", cfg.DefaultModel),
		"context_budget": "16000",
	}
	if strings.TrimSpace(cfg.DefaultModel) == "" {
		return nil, nil, fmt.Errorf("Claude Markdown imports require an explicit default model")
	}
	skills := []string{}
	re := regexp.MustCompile(`(?mi)allowed[_ -]?skills?\s*:\s*(.+)$`)
	if match := re.FindSubmatch(source); len(match) == 2 {
		for _, token := range strings.Split(string(match[1]), ",") {
			token = strings.TrimSpace(strings.Trim(token, `"'[]`))
			if token != "" {
				skills = append(skills, token)
			}
		}
	}
	findings := []Finding{}
	if len(skills) == 0 {
		return nil, nil, fmt.Errorf("Claude Markdown requires explicit allowed skills to produce a runnable Prism agent")
	}
	body := strings.TrimSpace(string(source))
	if body == "" {
		body = "Imported from Claude Markdown."
	}
	return renderPrismSpec(fields, skills, body), findings, nil
}
