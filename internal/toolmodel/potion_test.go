package toolmodel

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestPotionEmbedsKnownTokensAndRanksMatchingText(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.bin")
	vocab := filepath.Join(dir, "vocab.txt")
	raw := make([]byte, 20+3*2*4)
	copy(raw, "PULSEM2V")
	binary.LittleEndian.PutUint32(raw[8:], 1)
	binary.LittleEndian.PutUint32(raw[12:], 2)
	binary.LittleEndian.PutUint32(raw[16:], 3)
	values := []float32{0, 0, 1, 0, 0, 1}
	for i, v := range values {
		binary.LittleEndian.PutUint32(raw[20+i*4:], math.Float32bits(v))
	}
	if err := os.WriteFile(model, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vocab, []byte("[UNK]\nissue\nweather\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	embedder, err := LoadPotion(model, vocab)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := embedder.Embed(context.Background(), "issue")
	if err != nil {
		t.Fatal(err)
	}
	weather, err := embedder.Embed(context.Background(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	if Cosine(issue, issue) != 1 || Cosine(issue, weather) != 0 {
		t.Fatalf("issue=%v weather=%v", issue, weather)
	}
}
