package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"unicode/utf8"
)

const (
	maxRetainedResultBytes = 2 << 20
	maxRetainedRunBytes    = 8 << 20
	maxResultReadBytes     = 32 << 10
	maxRunResultReadBytes  = 128 << 10
	resultPreviewBytes     = 2048
)

type retainedResult struct {
	content    string
	incomplete bool
}

type runToolResults struct {
	results   map[string]retainedResult
	bytes     int
	readBytes int
}

type resultRead struct {
	Content    string `json:"content"`
	Offset     int    `json:"offset"`
	NextOffset int    `json:"next_offset"`
	TotalBytes int    `json:"total_bytes"`
	Truncated  bool   `json:"truncated"`
	Incomplete bool   `json:"incomplete,omitempty"`
}

func newRunToolResults() *runToolResults {
	return &runToolResults{results: map[string]retainedResult{}}
}

func (s *runToolResults) Retain(server, tool, content string, isError bool) string {
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return marshalToolResult(map[string]any{"server": server, "tool": tool, "error": "cannot retain large result"})
	}
	id := hex.EncodeToString(idBytes[:])
	available := maxRetainedRunBytes - s.bytes
	if available > maxRetainedResultBytes {
		available = maxRetainedResultBytes
	}
	if available < 0 {
		available = 0
	}
	retained := utf8Prefix(content, available)
	s.results[id] = retainedResult{content: retained, incomplete: len(retained) < len(content)}
	s.bytes += len(retained)
	preview := utf8Prefix(retained, resultPreviewBytes)
	return marshalToolResult(map[string]any{
		"server": server, "tool": tool, "preview": preview, "result_id": id,
		"is_error":       isError,
		"retained_bytes": len(retained), "original_bytes": len(content),
		"incomplete": len(retained) < len(content), "preview_truncated": len(preview) < len(retained),
		"read_with": "read_tool_result",
	})
}

func (s *runToolResults) Read(id string, offset, limit int) (resultRead, error) {
	res, ok := s.results[id]
	if !ok {
		return resultRead{}, fmt.Errorf("result_id is unavailable in this run")
	}
	if offset < 0 || offset > len(res.content) {
		return resultRead{}, fmt.Errorf("offset is outside retained result")
	}
	if offset < len(res.content) && !utf8.RuneStart(res.content[offset]) {
		return resultRead{}, fmt.Errorf("offset splits a UTF-8 character")
	}
	if limit <= 0 || limit > maxResultReadBytes {
		limit = maxResultReadBytes
	}
	remaining := maxRunResultReadBytes - s.readBytes
	if remaining <= 0 {
		return resultRead{}, fmt.Errorf("run result-read budget exhausted")
	}
	if limit > remaining {
		limit = remaining
	}
	end := offset + limit
	if end > len(res.content) {
		end = len(res.content)
	}
	for end > offset && end < len(res.content) && !utf8.RuneStart(res.content[end]) {
		end--
	}
	content := res.content[offset:end]
	s.readBytes += len(content)
	return resultRead{Content: content, Offset: offset, NextOffset: end, TotalBytes: len(res.content), Truncated: end < len(res.content), Incomplete: res.incomplete}, nil
}

func utf8Prefix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	if n <= 0 {
		return ""
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
