package toolmodel

import (
	"context"
	"os"
	"testing"
)

func TestPotionRealEmbedding(t *testing.T) {
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if state == "" {
		t.Skip("set PRISM_TEST_MODEL_STATE after models setup potion")
	}
	model, vocab := PotionPaths(state)
	embedder, err := LoadPotion(model, vocab)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := embedder.Embed(context.Background(), "Find open repository issues")
	if err != nil {
		t.Fatal(err)
	}
	close, err := embedder.Embed(context.Background(), "Search open repository issues")
	if err != nil {
		t.Fatal(err)
	}
	far, err := embedder.Embed(context.Background(), "Forecast tomorrow's weather")
	if err != nil {
		t.Fatal(err)
	}
	if len(issue) != 512 || Cosine(issue, close) <= Cosine(issue, far) {
		t.Fatalf("unexpected Potion vectors: dim=%d near=%f far=%f", len(issue), Cosine(issue, close), Cosine(issue, far))
	}
}
