package result

import (
	"encoding/json"
	"regexp"
	"strings"
)

const DefaultCompactMaxChars = 400

// agentOutput is the JSON envelope local models should emit.
type agentOutput struct {
	Summary    string          `json:"summary"`
	Findings   json.RawMessage `json:"findings"`
	Artifacts  []Artifact      `json:"artifacts"`
	Confidence string          `json:"confidence"`
	Patch      string          `json:"patch"`
	Code       string          `json:"code"`
	Notes      string          `json:"notes"`
}

var jsonFenceRE = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```")

// ParseAgentOutput extracts structured fields from a model response.
// Summary is always set to a compact orchestrator-facing string.
func ParseAgentOutput(raw string, compactMax int) RunResult {
	if compactMax <= 0 {
		compactMax = DefaultCompactMaxChars
	}
	raw = strings.TrimSpace(raw)
	parsed := tryParseJSON(raw)

	findings := parsed.findings
	artifacts := parsed.artifacts
	confidence := normalizeConfidence(parsed.confidence)

	if parsed.patch != "" {
		artifacts = append(artifacts, Artifact{Type: "patch", Label: "patch", Content: parsed.patch})
	}
	if parsed.code != "" {
		artifacts = append(artifacts, Artifact{Type: "snippet", Label: "code", Content: parsed.code})
	}

	summary := strings.TrimSpace(parsed.summary)
	if summary == "" {
		summary = firstParagraph(raw)
	}

	compact := buildCompact(summary, findings, confidence, compactMax)
	if compact == "" {
		compact = truncateText(raw, compactMax)
	}

	return RunResult{
		Summary:    compact,
		RawOutput:  raw,
		Findings:   findings,
		Artifacts:  artifacts,
		Confidence: confidence,
	}
}

type parsedFields struct {
	summary    string
	findings   []Finding
	artifacts  []Artifact
	confidence string
	patch      string
	code       string
}

func tryParseJSON(raw string) parsedFields {
	candidates := []string{raw}
	if m := jsonFenceRE.FindStringSubmatch(raw); len(m) == 2 {
		candidates = append([]string{strings.TrimSpace(m[1])}, candidates...)
	}
	if idx := strings.Index(raw, "{"); idx >= 0 {
		if end := strings.LastIndex(raw, "}"); end > idx {
			candidates = append(candidates, raw[idx:end+1])
		}
	}

	for _, c := range candidates {
		var ao agentOutput
		if err := json.Unmarshal([]byte(c), &ao); err != nil {
			continue
		}
		return parsedFields{
			summary:    ao.Summary,
			findings:   decodeFindings(ao.Findings),
			artifacts:  ao.Artifacts,
			confidence: ao.Confidence,
			patch:      ao.Patch,
			code:       ao.Code,
		}
	}
	return parsedFields{findings: bulletFindings(raw)}
}

func decodeFindings(raw json.RawMessage) []Finding {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err == nil {
		out := make([]Finding, 0, len(asStrings))
		for _, s := range asStrings {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, Finding{Text: t})
			}
		}
		return out
	}
	var asFindings []Finding
	if err := json.Unmarshal(raw, &asFindings); err == nil {
		return asFindings
	}
	return nil
}

func bulletFindings(text string) []Finding {
	var out []Finding
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			out = append(out, Finding{Text: strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")})
		}
	}
	return out
}

func buildCompact(summary string, findings []Finding, confidence string, max int) string {
	var b strings.Builder
	if summary != "" {
		b.WriteString(summary)
	}
	limit := 5
	for i, f := range findings {
		if i >= limit {
			b.WriteString("\n- …")
			break
		}
		text := strings.TrimSpace(f.Text)
		if text == "" {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(text)
	}
	if confidence != "" {
		b.WriteString("\n(confidence: ")
		b.WriteString(confidence)
		b.WriteByte(')')
	}
	return truncateText(strings.TrimSpace(b.String()), max)
}

func firstParagraph(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if idx := strings.Index(text, "\n\n"); idx >= 0 {
		return strings.TrimSpace(text[:idx])
	}
	if idx := strings.Index(text, "\n"); idx >= 0 {
		return strings.TrimSpace(text[:idx])
	}
	return text
}

func truncateText(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func normalizeConfidence(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return c
	default:
		return ConfidenceNone
	}
}
