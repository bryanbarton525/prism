package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type Adapter interface {
	Name() string
	Detect(filename string, source []byte) bool
	Translate(filename string, source []byte, cfg Config) ([]byte, []Finding, error)
}

type Config struct {
	DefaultModel    string
	AllowedSkills   []string
	OverrideSkills  bool
	ContextBudget   int
	LatencyBudgetMS int
}

func (c Config) contextBudget() int {
	if c.ContextBudget > 0 {
		return c.ContextBudget
	}
	return 8192
}

func (c Config) latencyBudget() int {
	if c.LatencyBudgetMS > 0 {
		return c.LatencyBudgetMS
	}
	return 30000
}

type Finding struct {
	Severity string `json:"severity"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

type Report struct {
	Adapter       string    `json:"adapter"`
	SourceDigest  string    `json:"source_digest"`
	OutputDigest  string    `json:"output_digest"`
	Findings      []Finding `json:"findings,omitempty"`
	AdapterDigest string    `json:"adapter_digest"`
	SourceModel   string    `json:"source_model,omitempty"`
}

var defaultAdapters = []Adapter{
	nativePrismAdapter{},
	codexTOMLAdapter{},
	claudeMarkdownAdapter{},
}

// Detect returns the first supported adapter name for source.
func Detect(filename string, source []byte) (string, bool) {
	matches := detectAll(filename, source)
	if len(matches) != 1 {
		return "", false
	}
	return matches[0].Name(), true
}

func detectAll(filename string, source []byte) []Adapter {
	matches := []Adapter{}
	for _, adapter := range defaultAdapters {
		if adapter.Detect(filename, source) {
			matches = append(matches, adapter)
		}
	}
	return matches
}

// TranslateWithFormat translates with an explicitly selected adapter. An empty
// format retains deterministic auto-detection.
func TranslateWithFormat(format, filename string, source []byte, cfg Config) ([]byte, Report, error) {
	if strings.TrimSpace(format) == "" || strings.EqualFold(format, "auto") {
		return Translate(filename, source, cfg)
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "prism":
		format = "prism-native"
	case "codex":
		format = "codex-toml"
	case "claude":
		format = "claude-markdown"
	}
	for _, adapter := range defaultAdapters {
		if !strings.EqualFold(adapter.Name(), format) {
			continue
		}
		return translate(adapter, filename, source, cfg)
	}
	return nil, Report{}, fmt.Errorf("unsupported agent format %q (supported: prism, codex, claude)", format)
}

func Translate(filename string, source []byte, cfg Config) ([]byte, Report, error) {
	matches := detectAll(filename, source)
	if len(matches) == 0 {
		return nil, Report{}, fmt.Errorf("no adapter matched input")
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, adapter := range matches {
			names = append(names, adapter.Name())
		}
		return nil, Report{}, fmt.Errorf("ambiguous agent format (%s); pass --format", strings.Join(names, ", "))
	}
	return translate(matches[0], filename, source, cfg)
}

func translate(adapter Adapter, filename string, source []byte, cfg Config) ([]byte, Report, error) {
	output, findings, err := adapter.Translate(filename, source, cfg)
	if err != nil {
		return nil, Report{}, err
	}
	srcDigest := sha256.Sum256(source)
	outDigest := sha256.Sum256(output)
	adapterDigest := sha256.Sum256([]byte(adapter.Name() + ":v1"))
	return output, Report{
		Adapter:       adapter.Name(),
		SourceDigest:  hex.EncodeToString(srcDigest[:]),
		OutputDigest:  hex.EncodeToString(outDigest[:]),
		Findings:      findings,
		AdapterDigest: hex.EncodeToString(adapterDigest[:]),
		SourceModel:   sourceModel(adapter, filename, source),
	}, nil
}

func sourceModel(adapter Adapter, filename string, source []byte) string {
	switch adapter.(type) {
	case nativePrismAdapter:
		if spec, err := agent.ParseManaged(source, filename); err == nil {
			return spec.Model
		}
	case codexTOMLAdapter:
		var cfg codexConfig
		if toml.Unmarshal(source, &cfg) == nil {
			return cfg.Model
		}
	case claudeMarkdownAdapter:
		if frontmatter, _, ok := splitMarkdownFrontmatter(source); ok {
			var raw map[string]any
			if yaml.Unmarshal(frontmatter, &raw) == nil {
				if value, ok := raw["model"].(string); ok {
					return value
				}
			}
		}
	}
	return ""
}

func defaultID(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, "_", "-")
	base = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "imported-agent"
	}
	return base
}

func defaultName(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func renderPrismSpec(fields map[string]string, skills []string, body string) []byte {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("---\n")
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s: %s\n", k, fields[k]))
	}
	b.WriteString("allowed_skills: [")
	for i, sk := range skills {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(sk))
	}
	b.WriteString("]\n")
	b.WriteString("---\n")
	if strings.TrimSpace(body) == "" {
		body = "Imported agent definition."
	}
	b.WriteString(body)
	return []byte(b.String())
}
