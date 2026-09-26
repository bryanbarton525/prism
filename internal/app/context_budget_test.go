package app

import (
	"context"
	"strings"
	"testing"

	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
)

func TestChatWithinBudgetCompactsOnlyReadableToolPreviews(t *testing.T) {
	model := &fakeModelRuntime{}
	runner := &Runner{llm: model}
	results := newRunToolResults()
	preview := results.Retain("issues", "find", strings.Repeat("evidence", 250), false)
	request := llmruntime.ChatRequest{
		Model: "test",
		Messages: []llmruntime.Message{
			{Role: "system", Content: "protected policy"},
			{Role: "user", Content: "protected task"},
			{Role: "tool", Content: preview},
		},
	}
	request.ContextLength = estimatedChatTokens(request) + responseHeadroomTokens - 100
	if _, err := runner.chatWithinBudget(context.Background(), request, results); err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 || len(model.requests) != 1 {
		t.Fatalf("model requests: %+v", model.requests)
	}
	got := model.requests[0].Messages
	if got[0].Content != "protected policy" || got[1].Content != "protected task" || !strings.Contains(got[2].Content, "omitted_from_context") || !strings.Contains(got[2].Content, "result_id") {
		t.Fatalf("unexpected compacted request: %+v", got)
	}
	if request.Messages[2].Content != preview {
		t.Fatal("caller request was mutated")
	}
}

func TestChatWithinBudgetNeverTruncatesProtectedMessages(t *testing.T) {
	model := &fakeModelRuntime{}
	runner := &Runner{llm: model}
	request := llmruntime.ChatRequest{ContextLength: 1, Messages: []llmruntime.Message{{Role: "system", Content: "protected policy"}}}
	if _, err := runner.chatWithinBudget(context.Background(), request, nil); err == nil || !strings.Contains(err.Error(), "protected instructions") {
		t.Fatalf("expected clear budget error, got %v", err)
	}
	if model.calls != 0 {
		t.Fatal("model was called with an over-budget request")
	}
}
