package graphify

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/bryanbarton525/prism/internal/textutil"
)

const (
	MaxSourceVerifications = 8
	MaxSourceReadBytes     = 16 << 10
	MaxSourceExcerptBytes  = 2 << 10
)

type SourceCitation struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type SourceVerification struct {
	Citation  SourceCitation `json:"citation"`
	Verified  bool           `json:"verified"`
	Excerpt   string         `json:"excerpt,omitempty"`
	Truncated bool           `json:"truncated,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

var graphSourceCitationRE = regexp.MustCompile(`(?mi)(?:^|\s)(?:source:\s*|at=)([A-Za-z0-9._/-]+\.[A-Za-z0-9_+-]+)(?:\s+L|:)([0-9]+)`)

func ExtractSourceCitations(output string) []SourceCitation {
	matches := graphSourceCitationRE.FindAllStringSubmatch(output, -1)
	out := make([]SourceCitation, 0, len(matches))
	seen := make(map[string]struct{})
	for _, match := range matches {
		line, err := strconv.Atoi(match[2])
		if err != nil || line < 1 {
			continue
		}
		citation := SourceCitation{Path: match[1], Line: line}
		key := citation.Path + ":" + strconv.Itoa(citation.Line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, citation)
		if len(out) == MaxSourceVerifications {
			break
		}
	}
	return out
}

func VerifyGraphSources(workspace fs.FS, graphOutput string) []SourceVerification {
	citations := ExtractSourceCitations(graphOutput)
	out := make([]SourceVerification, 0, len(citations))
	for _, citation := range citations {
		verification := SourceVerification{Citation: citation}
		clean := path.Clean(citation.Path)
		if clean != citation.Path || !fs.ValidPath(clean) {
			verification.Reason = "Graphify cited an unsafe or non-workspace source path"
			out = append(out, verification)
			continue
		}
		if workspace == nil {
			verification.Reason = "workspace sources are unavailable for verification"
			out = append(out, verification)
			continue
		}
		excerpt, truncated, err := readSourceExcerpt(workspace, clean, citation.Line)
		if err != nil {
			verification.Reason = err.Error()
			out = append(out, verification)
			continue
		}
		verification.Verified = true
		verification.Excerpt = excerpt
		verification.Truncated = truncated
		out = append(out, verification)
	}
	return out
}

func readSourceExcerpt(workspace fs.FS, name string, line int) (string, bool, error) {
	file, err := workspace.Open(name)
	if err != nil {
		return "", false, fmt.Errorf("reading cited source %q: %w", name, err)
	}
	defer file.Close()

	reader := bufio.NewReader(io.LimitReader(file, MaxSourceReadBytes+1))
	var lines []string
	var bytesRead int
	for current := 1; ; current++ {
		text, readErr := reader.ReadString('\n')
		if len(text) > 0 {
			bytesRead += len(text)
		}
		if bytesRead > MaxSourceReadBytes {
			return "", true, fmt.Errorf("cited source %q exceeds %d-byte verification limit before line %d", name, MaxSourceReadBytes, line)
		}
		if current >= line-1 && current <= line+1 && len(text) > 0 {
			lines = append(lines, fmt.Sprintf("%d: %s", current, strings.TrimRight(text, "\r\n")))
		}
		if current >= line+1 && len(lines) > 0 {
			excerpt := strings.Join(lines, "\n")
			excerpt = textutil.TruncateWithin(excerpt, MaxSourceExcerptBytes, "\n[excerpt truncated]")
			return excerpt, false, nil
		}
		if readErr == io.EOF {
			if current < line {
				return "", false, fmt.Errorf("cited source %q has no line %d", name, line)
			}
			excerpt := strings.Join(lines, "\n")
			excerpt = textutil.TruncateWithin(excerpt, MaxSourceExcerptBytes, "\n[excerpt truncated]")
			return excerpt, false, nil
		}
		if readErr != nil {
			return "", false, fmt.Errorf("reading cited source %q: %w", name, readErr)
		}
	}
}
