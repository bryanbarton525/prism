package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAICompatibleChatSerializesRequest(t *testing.T) {
	var got openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer key" {
			t.Fatalf("Authorization = %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "served",
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer srv.Close()
	rt, err := NewOpenAICompatibleRuntime(Config{Engine: EngineSGLang, BaseURL: srv.URL + "/v1", APIKey: "key", Model: "default"})
	if err != nil {
		t.Fatal(err)
	}
	temp := 0.1
	if _, err := rt.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}, Temperature: &temp, MaxTokens: 10}); err != nil {
		t.Fatal(err)
	}
	if got.Model != "default" || got.Stream {
		t.Fatalf("request = %#v", got)
	}
	if got.Temperature == nil || *got.Temperature != temp || got.MaxTokens != 10 {
		t.Fatalf("request = %#v", got)
	}
}

func TestOpenAICompatibleChatParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "m",
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "hello"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
		})
	}))
	defer srv.Close()
	rt, _ := NewOpenAICompatibleRuntime(Config{Engine: EngineVLLM, BaseURL: srv.URL, Model: "m"})
	res, err := rt.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message.Content != "hello" || res.Usage.TotalTokens != 3 {
		t.Fatalf("res = %#v", res)
	}
}

func TestOpenAICompatibleHealthParsing(t *testing.T) {
	for _, body := range []string{`{"status":"ok"}`, `{"ok":true}`, `{}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		rt, _ := NewOpenAICompatibleRuntime(Config{Engine: EngineSGLang, BaseURL: srv.URL})
		status, err := rt.Health(context.Background())
		srv.Close()
		if err != nil || !status.Healthy {
			t.Fatalf("body %s status=%#v err=%v", body, status, err)
		}
	}
}

func TestStreamParsesSSEFixtures(t *testing.T) {
	sse := ": comment\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"choices\":[{\"message\":{\"content\":\" world\"}}]}\n\ndata: [DONE]\n"
	rt, done := streamRuntime(t, sse, http.StatusOK)
	defer done()
	ch, err := rt.Stream(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	var got []StreamEvent
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != 3 || got[0].Delta != "hello" || got[1].Delta != " world" || got[2].Kind != StreamEventDone {
		t.Fatalf("events = %#v", got)
	}
}

func TestStreamMalformedJSONEmitsError(t *testing.T) {
	rt, done := streamRuntime(t, "data: nope\n", http.StatusOK)
	defer done()
	ch, err := rt.Stream(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	ev := <-ch
	if ev.Kind != StreamEventError || !IsKind(ev.Err, ErrorKindParse) {
		t.Fatalf("event = %#v", ev)
	}
}

func TestStreamProviderErrorResponse(t *testing.T) {
	rt, done := streamRuntime(t, `{"error":"no"}`, http.StatusInternalServerError)
	defer done()
	_, err := rt.Stream(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil || !IsKind(err, ErrorKindProvider) {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	rt, _ := NewOpenAICompatibleRuntime(Config{Engine: EngineSGLang, BaseURL: srv.URL, Model: "m"})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := rt.Stream(ctx, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	ev := <-ch
	if ev.Kind != StreamEventError {
		t.Fatalf("event = %#v", ev)
	}
}

func TestGenerateStructured(t *testing.T) {
	var got openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "m",
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": `{"findings":["ok"]}`}}},
		})
	}))
	defer srv.Close()
	rt, _ := NewOpenAICompatibleRuntime(Config{Engine: EngineSGLang, BaseURL: srv.URL, Model: "m"})
	res, err := rt.GenerateStructured(context.Background(), StructuredRequest{
		ChatRequest: ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}},
		Name:        "evidence_summary",
		Strict:      true,
		Schema:      map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ResponseFormat == nil || got.ResponseFormat.JSONSchema.Name != "evidence_summary" || !got.ResponseFormat.JSONSchema.Strict {
		t.Fatalf("request = %#v", got.ResponseFormat)
	}
	if _, ok := res.Parsed.(map[string]any); !ok {
		t.Fatalf("parsed = %#v", res.Parsed)
	}
}

func TestGenerateStructuredArrayAndInvalidJSON(t *testing.T) {
	rt, done := structuredRuntime(t, `[{"ok":true}]`)
	defer done()
	res, err := rt.GenerateStructured(context.Background(), StructuredRequest{ChatRequest: ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.Parsed.([]any); !ok {
		t.Fatalf("parsed = %#v", res.Parsed)
	}

	bad, badDone := structuredRuntime(t, `not json`)
	defer badDone()
	_, err = bad.GenerateStructured(context.Background(), StructuredRequest{ChatRequest: ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}})
	if err == nil || !IsKind(err, ErrorKindParse) {
		t.Fatalf("err = %v", err)
	}
}

func TestGenerateStructuredMissingContent(t *testing.T) {
	rt, done := structuredRuntime(t, "")
	defer done()
	_, err := rt.GenerateStructured(context.Background(), StructuredRequest{ChatRequest: ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}})
	if err == nil || !IsKind(err, ErrorKindParse) {
		t.Fatalf("err = %v", err)
	}
}

func streamRuntime(t *testing.T, body string, status int) (*OpenAICompatibleRuntime, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	rt, err := NewOpenAICompatibleRuntime(Config{Engine: EngineSGLang, BaseURL: srv.URL, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	return rt, srv.Close
}

func structuredRuntime(t *testing.T, content string) (*OpenAICompatibleRuntime, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "m",
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": content}}},
		})
	}))
	rt, err := NewOpenAICompatibleRuntime(Config{Engine: EngineSGLang, BaseURL: srv.URL, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	return rt, srv.Close
}
