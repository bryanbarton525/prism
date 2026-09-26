package app

import (
	"context"
	"encoding/json"
	"fmt"

	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
)

const responseHeadroomTokens = 512

// chatWithinBudget keeps result references and correlated tool-call messages
// intact while reclaiming older, re-readable result previews.
func (r *Runner) chatWithinBudget(ctx context.Context, req llmruntime.ChatRequest, results *runToolResults) (*llmruntime.ChatResponse, error) {
	if req.ContextLength <= 0 {
		return r.llm.Chat(ctx, req)
	}
	req.Messages = append([]llmruntime.Message(nil), req.Messages...)
	for estimatedChatTokens(req)+responseHeadroomTokens > req.ContextLength {
		replaced := false
		for i := range req.Messages {
			message := &req.Messages[i]
			if message.Role != "tool" || results == nil {
				continue
			}
			var marker struct {
				ResultID string `json:"result_id"`
			}
			if json.Unmarshal([]byte(message.Content), &marker) != nil || marker.ResultID == "" {
				continue
			}
			if _, ok := results.results[marker.ResultID]; !ok {
				continue
			}
			compact := marshalToolResult(map[string]any{"result_id": marker.ResultID, "omitted_from_context": true, "read_with": "read_tool_result"})
			if len(compact) >= len(message.Content) {
				continue
			}
			message.Content = compact
			replaced = true
			break
		}
		if !replaced {
			return nil, fmt.Errorf("agent context budget exhausted; protected instructions, task, tool definitions, and required history cannot fit")
		}
	}
	return r.llm.Chat(ctx, req)
}

func estimatedChatTokens(req llmruntime.ChatRequest) int {
	encoded, err := json.Marshal(struct {
		Messages []llmruntime.Message `json:"messages"`
		Tools    []llmruntime.Tool    `json:"tools"`
	}{req.Messages, req.Tools})
	if err != nil {
		return 1 << 30
	}
	// Conservative approximation: model-specific tokenization remains the
	// runtime's responsibility, and we retain headroom for the answer.
	return (len(encoded) + 2) / 3
}
