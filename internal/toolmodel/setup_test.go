package toolmodel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupMiniLMRejectsCorruptExistingModel(t *testing.T) {
	state := t.TempDir()
	target := MiniLMPath(state)
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "model.onnx"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "tokenizer.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetupMiniLM(context.Background(), state); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum error, got %v", err)
	}
}
