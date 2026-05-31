package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/bryanbarton525/prism/internal/ollama"
)

const defaultBenchmarkModel = "llama3.1:8b"

// OllamaReachable reports whether the Ollama server responds at host.
func OllamaReachable(ctx context.Context, host string) error {
	return ollama.NewClient(host).Ping(ctx)
}

type chatResult struct {
	text       string
	promptTok  int
	outputTok  int
	durationMS int64
}

func ollamaChat(ctx context.Context, host, model, system, user string) (chatResult, error) {
	if model == "" {
		model = defaultBenchmarkModel
	}
	start := time.Now()
	client := ollama.NewClient(host)
	resp, err := client.Chat(ctx, ollama.ChatRequest{
		Model: model,
		Messages: []ollama.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Options: &ollama.Options{Temperature: 0.1},
	})
	if err != nil {
		return chatResult{}, err
	}
	prompt := resp.PromptEvalCount
	output := resp.EvalCount
	if prompt == 0 {
		prompt = EstimateTokens(system + user)
	}
	if output == 0 {
		output = EstimateTokens(resp.Message.Content)
	}
	return chatResult{
		text:       resp.Message.Content,
		promptTok:  prompt,
		outputTok:  output,
		durationMS: time.Since(start).Milliseconds(),
	}, nil
}

func ensureModel(ctx context.Context, host, model string) error {
	client := ollama.NewClient(host)
	models, err := client.ListModels(ctx)
	if err != nil {
		return err
	}
	for _, m := range models {
		if m == model {
			return nil
		}
	}
	return fmt.Errorf("model %q not found in Ollama (pull it first)", model)
}
