package importer

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type nativePrismAdapter struct{}

func (nativePrismAdapter) Name() string { return "prism-native" }

func (nativePrismAdapter) Detect(filename string, source []byte) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if (ext != ".md" && ext != ".markdown") || !strings.HasPrefix(strings.TrimSpace(string(source)), "---") {
		return false
	}
	_, err := agent.ParseManaged(source, filename)
	return err == nil
}

func (nativePrismAdapter) Translate(filename string, source []byte, cfg Config) ([]byte, []Finding, error) {
	if strings.TrimSpace(cfg.DefaultModel) == "" {
		return nil, nil, fmt.Errorf("Prism agent imports require an explicit default model")
	}
	spec, err := agent.ParseManaged(source, filename)
	if err != nil {
		return nil, nil, err
	}
	findings := []Finding{}
	if strings.TrimSpace(spec.Model) != strings.TrimSpace(cfg.DefaultModel) {
		findings = append(findings, Finding{Severity: "warning", Field: "model", Message: fmt.Sprintf("source model %q retained as provenance only; using selected Prism model %q", spec.Model, cfg.DefaultModel)})
	}
	spec.Model = cfg.DefaultModel
	if cfg.OverrideSkills {
		spec.AllowedSkills = append([]string(nil), cfg.AllowedSkills...)
	}
	if cfg.ContextBudget > 0 {
		spec.ContextBudget = cfg.ContextBudget
	}
	if cfg.LatencyBudgetMS > 0 {
		spec.LatencyBudgetMS = cfg.LatencyBudgetMS
	}
	output, err := agent.Render(spec)
	if err != nil {
		return nil, nil, err
	}
	return output, findings, nil
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
	model := strings.TrimSpace(cfg.DefaultModel)
	if model == "" {
		return nil, nil, fmt.Errorf("Codex TOML imports require an explicit default model")
	}
	id := defaultID(config.Name)
	fields := map[string]string{
		"id":                id,
		"name":              fmt.Sprintf("%q", config.Name),
		"description":       `"Imported from Codex TOML configuration."`,
		"model":             fmt.Sprintf("%q", model),
		"context_budget":    fmt.Sprintf("%d", cfg.contextBudget()),
		"latency_budget_ms": fmt.Sprintf("%d", cfg.latencyBudget()),
	}
	if config.Description != "" {
		fields["description"] = fmt.Sprintf("%q", config.Description)
	}
	body := strings.TrimSpace(config.DeveloperInstructions)
	if body == "" {
		body = "Imported from Codex TOML."
	}
	findings := []Finding{}
	if strings.TrimSpace(config.Model) != "" && strings.TrimSpace(config.Model) != model {
		findings = append(findings, Finding{Severity: "warning", Field: "model", Message: fmt.Sprintf("source model %q retained as provenance only; using selected Prism model %q", config.Model, model)})
	}
	var raw map[string]any
	if err := toml.Unmarshal(source, &raw); err == nil {
		for key := range raw {
			switch key {
			case "name", "description", "developer_instructions", "model", "allowed_skills":
			default:
				findings = append(findings, Finding{Severity: "unresolved", Field: key, Message: fmt.Sprintf("Codex field %q has no automatic Prism equivalent; explicitly map or omit it", key)})
			}
		}
	}
	sortFindings(findings)
	skills := config.AllowedSkills
	if cfg.OverrideSkills {
		skills = append([]string(nil), cfg.AllowedSkills...)
	}
	return renderPrismSpec(fields, skills, body), findings, nil
}

var modelLine = regexp.MustCompile(`(?m)^model:\s*.*$`)

func replaceModel(source []byte, model string) ([]byte, bool) {
	content := strings.TrimSpace(string(source))
	if !strings.HasPrefix(content, "---") {
		return source, false
	}
	end := strings.Index(content[3:], "\n---")
	if end < 0 {
		return source, false
	}
	end += 3
	frontmatter := content[:end]
	if !modelLine.MatchString(frontmatter) {
		return source, false
	}
	return []byte(modelLine.ReplaceAllString(frontmatter, "model: "+fmt.Sprintf("%q", model)) + content[end:]), true
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
	if strings.EqualFold(filepath.Base(filename), "SKILL.md") {
		return false
	}
	frontmatter, _, ok := splitMarkdownFrontmatter(source)
	if ok {
		var raw map[string]any
		if yaml.Unmarshal(frontmatter, &raw) == nil {
			if _, nativeID := raw["id"]; nativeID {
				return false
			}
			if strings.Contains(filepath.ToSlash(strings.ToLower(filename)), ".claude/agents/") {
				return true
			}
			for _, expected := range []string{"tools", "permissionmode", "skills", "hooks", "memory"} {
				for actual := range raw {
					if strings.EqualFold(actual, expected) {
						return true
					}
				}
			}
		}
	}
	s := strings.ToLower(string(source))
	return strings.Contains(s, "# claude") || strings.Contains(s, "system prompt") || strings.Contains(s, "role:")
}

func (claudeMarkdownAdapter) Translate(filename string, source []byte, cfg Config) ([]byte, []Finding, error) {
	id := defaultID(filename)
	fields := map[string]string{
		"id":                id,
		"name":              fmt.Sprintf("%q", defaultName(id)),
		"description":       `"Imported from Claude Markdown prompt."`,
		"model":             fmt.Sprintf("%q", cfg.DefaultModel),
		"context_budget":    fmt.Sprintf("%d", cfg.contextBudget()),
		"latency_budget_ms": fmt.Sprintf("%d", cfg.latencyBudget()),
	}
	if strings.TrimSpace(cfg.DefaultModel) == "" {
		return nil, nil, fmt.Errorf("Claude Markdown imports require an explicit default model")
	}
	skills := []string{}
	body := strings.TrimSpace(string(source))
	findings := []Finding{}
	if frontmatter, markdownBody, ok := splitMarkdownFrontmatter(source); ok {
		var raw map[string]any
		if err := yaml.Unmarshal(frontmatter, &raw); err != nil {
			return nil, nil, fmt.Errorf("parse Claude frontmatter: %w", err)
		}
		if value, ok := raw["name"].(string); ok && strings.TrimSpace(value) != "" {
			fields["name"] = fmt.Sprintf("%q", value)
			id = defaultID(value)
			fields["id"] = id
		}
		if value, ok := raw["description"].(string); ok && strings.TrimSpace(value) != "" {
			fields["description"] = fmt.Sprintf("%q", value)
		}
		body = strings.TrimSpace(string(markdownBody))
		for key := range raw {
			switch strings.ToLower(key) {
			case "name", "description", "model":
			default:
				findings = append(findings, Finding{Severity: "unresolved", Field: key, Message: fmt.Sprintf("Claude field %q has no automatic Prism equivalent; explicitly map or omit it", key)})
			}
		}
	}
	re := regexp.MustCompile(`(?mi)allowed[_ -]?skills?\s*:\s*(.+)$`)
	if match := re.FindSubmatch(source); len(match) == 2 {
		for _, token := range strings.Split(string(match[1]), ",") {
			token = strings.TrimSpace(strings.Trim(token, `"'[]`))
			if token != "" {
				skills = append(skills, token)
			}
		}
	}
	if cfg.OverrideSkills {
		skills = append([]string(nil), cfg.AllowedSkills...)
	}
	if body == "" {
		body = "Imported from Claude Markdown."
	}
	sortFindings(findings)
	return renderPrismSpec(fields, skills, body), findings, nil
}

func splitMarkdownFrontmatter(source []byte) ([]byte, []byte, bool) {
	trimmed := bytes.TrimSpace(source)
	if !bytes.HasPrefix(trimmed, []byte("---\n")) {
		return nil, nil, false
	}
	end := bytes.Index(trimmed[4:], []byte("\n---"))
	if end < 0 {
		return nil, nil, false
	}
	end += 4
	return bytes.TrimSpace(trimmed[4:end]), bytes.TrimSpace(trimmed[end+4:]), true
}

func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Field == findings[j].Field {
			return findings[i].Message < findings[j].Message
		}
		return findings[i].Field < findings[j].Field
	})
}
