package importer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
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
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".toml" {
		return false
	}
	s := string(source)
	return strings.Contains(s, "model") || strings.Contains(s, "name")
}

func (codexTOMLAdapter) Translate(filename string, source []byte, cfg Config) ([]byte, []Finding, error) {
	lines := strings.Split(string(source), "\n")
	fields := map[string]string{
		"id":             defaultID(filename),
		"name":           fmt.Sprintf("%q", defaultName(defaultID(filename))),
		"description":    `"Imported from Codex TOML configuration."`,
		"model":          fmt.Sprintf("%q", cfg.DefaultModel),
		"context_budget": "16000",
	}
	if cfg.DefaultModel == "" {
		fields["model"] = `"llama3.1:8b-instruct-q6_K"`
	}
	findings := []Finding{}
	var skills []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "name =") {
			fields["name"] = strings.TrimSpace(strings.TrimPrefix(line, "name ="))
		}
		if strings.HasPrefix(line, "description =") {
			fields["description"] = strings.TrimSpace(strings.TrimPrefix(line, "description ="))
		}
		if strings.HasPrefix(line, "model =") {
			fields["model"] = strings.TrimSpace(strings.TrimPrefix(line, "model ="))
		}
		if strings.HasPrefix(line, "allowed_skills = [") {
			items := strings.TrimSpace(strings.TrimPrefix(line, "allowed_skills = ["))
			items = strings.TrimSuffix(items, "]")
			for _, it := range strings.Split(items, ",") {
				it = strings.TrimSpace(strings.Trim(it, `"`))
				if it != "" {
					skills = append(skills, it)
				}
			}
		}
	}
	if len(skills) == 0 {
		findings = append(findings, Finding{
			Severity: "warning",
			Field:    "allowed_skills",
			Message:  "missing allowed_skills in source; translated with empty list",
		})
	}
	return renderPrismSpec(fields, skills, "Imported from Codex TOML."), findings, nil
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
	if cfg.DefaultModel == "" {
		fields["model"] = `"llama3.1:8b-instruct-q6_K"`
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
		findings = append(findings, Finding{
			Severity: "warning",
			Field:    "allowed_skills",
			Message:  "no explicit allowed skills detected in Markdown source",
		})
	}
	body := strings.TrimSpace(string(source))
	if body == "" {
		body = "Imported from Claude Markdown."
	}
	return renderPrismSpec(fields, skills, body), findings, nil
}
