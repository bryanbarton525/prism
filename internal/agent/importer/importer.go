package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Adapter interface {
	Name() string
	Detect(filename string, source []byte) bool
	Translate(filename string, source []byte, cfg Config) ([]byte, []Finding, error)
}

type Config struct {
	DefaultModel string
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
}

var defaultAdapters = []Adapter{
	nativePrismAdapter{},
	codexTOMLAdapter{},
	claudeMarkdownAdapter{},
}

func Translate(filename string, source []byte, cfg Config) ([]byte, Report, error) {
	for _, adapter := range defaultAdapters {
		if !adapter.Detect(filename, source) {
			continue
		}
		output, findings, err := adapter.Translate(filename, source, cfg)
		if err != nil {
			return nil, Report{}, err
		}
		srcDigest := sha256.Sum256(source)
		outDigest := sha256.Sum256(output)
		adapterDigest := sha256.Sum256([]byte(adapter.Name() + ":v1"))
		report := Report{
			Adapter:       adapter.Name(),
			SourceDigest:  hex.EncodeToString(srcDigest[:]),
			OutputDigest:  hex.EncodeToString(outDigest[:]),
			Findings:      findings,
			AdapterDigest: hex.EncodeToString(adapterDigest[:]),
		}
		return output, report, nil
	}
	return nil, Report{}, fmt.Errorf("no adapter matched input")
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
		b.WriteString(sk)
	}
	b.WriteString("]\n")
	b.WriteString("---\n")
	if strings.TrimSpace(body) == "" {
		body = "Imported agent definition."
	}
	b.WriteString(body)
	return []byte(b.String())
}
