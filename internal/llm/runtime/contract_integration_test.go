//go:build integration

package runtime

import (
	"context"
	"os"
	"testing"
	"time"
)

func contractRuntime(t *testing.T) *OpenAICompatibleRuntime {
	t.Helper()
	base := os.Getenv("PRISM_LLM_CONTRACT_BASE_URL")
	model := os.Getenv("PRISM_LLM_CONTRACT_MODEL")
	engine := Engine(os.Getenv("PRISM_LLM_CONTRACT_ENGINE"))
	if base == "" || model == "" || engine == "" {
		t.Skip("set PRISM_LLM_CONTRACT_BASE_URL, PRISM_LLM_CONTRACT_MODEL, and PRISM_LLM_CONTRACT_ENGINE")
	}
	t.Logf("contract target engine=%s base_url=%s", engine, base)
	rt, err := NewOpenAICompatibleRuntime(Config{
		Engine:  engine,
		BaseURL: base,
		APIKey:  os.Getenv("PRISM_LLM_CONTRACT_API_KEY"),
		Model:   model,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

func TestContractHealth(t *testing.T) {
	rt := contractRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := rt.Health(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestContractChat(t *testing.T) {
	rt := contractRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := rt.Chat(ctx, ChatRequest{Messages: []Message{{Role: "user", Content: "Reply with the word prism."}}, MaxTokens: 16})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message.Content == "" {
		t.Fatalf("empty content: %#v", res)
	}
}

func TestContractStream(t *testing.T) {
	rt := contractRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ch, err := rt.Stream(ctx, ChatRequest{Messages: []Message{{Role: "user", Content: "Reply with the word prism."}}, MaxTokens: 16})
	if err != nil {
		t.Fatal(err)
	}
	var sawDelta bool
	for ev := range ch {
		if ev.Kind == StreamEventError {
			t.Fatal(ev.Err)
		}
		if ev.Kind == StreamEventDelta && ev.Delta != "" {
			sawDelta = true
		}
	}
	if !sawDelta {
		t.Fatal("no stream delta received")
	}
}

func TestContractStructured(t *testing.T) {
	rt := contractRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := rt.GenerateStructured(ctx, StructuredRequest{
		ChatRequest: ChatRequest{Messages: []Message{{Role: "user", Content: "Return {\"ok\": true}."}}, MaxTokens: 64},
		Name:        "contract_result",
		Strict:      true,
		Schema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"ok"},
			"properties":           map[string]any{"ok": map[string]any{"type": "boolean"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Parsed == nil {
		t.Fatalf("missing parsed structured response: %#v", res)
	}
}
