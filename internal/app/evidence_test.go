package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRetainedToolResultCanBeReadInBoundedPortions(t *testing.T) {
	results := newRunToolResults()
	preview := results.Retain("issues", "search", strings.Repeat("a", 5000),false)
	if !strings.Contains(preview, "result_id") || strings.Contains(preview, strings.Repeat("a", 3000)) {
		t.Fatalf("preview not bounded: %q", preview)
	}
	var marker struct {
		ResultID string `json:"result_id"`
	}
	if err := json.Unmarshal([]byte(preview), &marker); err != nil {
		t.Fatal(err)
	}
	id := marker.ResultID
	read, err := results.Read(id, 4090, 100)
	if err != nil {
		t.Fatal(err)
	}
	if read.Content != strings.Repeat("a", 100) || !read.Truncated {
		t.Fatalf("read: %+v", read)
	}
	if _, err := newRunToolResults().Read(id, 0, 100); err == nil {
		t.Fatal("a different run could read the result")
	}
}
