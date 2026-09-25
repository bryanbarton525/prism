package toolmodel

import (
	"context"
	"os"
	"testing"
)

func TestMiniLMRealEmbedding(t *testing.T) {
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if state == "" {
		t.Skip("set PRISM_TEST_MODEL_STATE after models setup onnx")
	}
	model, err := LoadMiniLM(MiniLMPath(state))
	if err != nil {
		t.Fatal(err)
	}
	defer model.Close()
	issue, err := model.Embed(context.Background(), "Find open repository issues")
	if err != nil {
		t.Fatal(err)
	}
	close, err := model.Embed(context.Background(), "Search open repository issues")
	if err != nil {
		t.Fatal(err)
	}
	far, err := model.Embed(context.Background(), "Forecast tomorrow's weather")
	if err != nil {
		t.Fatal(err)
	}
	if len(issue) != 384 || Cosine(issue, close) <= Cosine(issue, far) {
		t.Fatalf("unexpected MiniLM vectors: dim=%d near=%f far=%f", len(issue), Cosine(issue, close), Cosine(issue, far))
	}
}
