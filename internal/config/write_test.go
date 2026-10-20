package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWriteEnvConcurrentUpdatesPreserveConfiguration(t *testing.T) {
	state := t.TempDir()
	path := filepath.Join(state, "config.env")
	initial := "# keep this note\nUNRELATED=keep\nexport PRISM_MODEL_RUNTIME_MODEL=old\nPRISM_MODEL_RUNTIME_MODEL=duplicate\nPRISM_MODEL_RUNTIME_FALLBACK_MODEL=backup\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for key, value := range map[string]string{"PRISM_MODEL_RUNTIME_MODEL": "new", "PRISM_TOOL_RECOMMEND_MODEL": "onnx"} {
		wg.Add(1)
		go func(key, value string) {
			defer wg.Done()
			if err := WriteEnv(context.Background(), state, map[string]string{key: value}); err != nil {
				t.Error(err)
			}
		}(key, value)
	}
	wg.Wait()
	values, err := ReadEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if values["PRISM_MODEL_RUNTIME_MODEL"] != "new" || values["PRISM_TOOL_RECOMMEND_MODEL"] != "onnx" || values["UNRELATED"] != "keep" || values["PRISM_MODEL_RUNTIME_FALLBACK_MODEL"] != "backup" {
		t.Fatalf("values=%v", values)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "# keep this note\n") || strings.Contains(string(data), "duplicate") {
		t.Fatalf("unexpected rewrite: %s", data)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions=%v", info.Mode())
	}
}

func TestWriteEnvInvalidExistingFileRemainsIntact(t *testing.T) {
	state := t.TempDir()
	path := filepath.Join(state, "config.env")
	original := "not an env assignment\n"
	os.WriteFile(path, []byte(original), 0o600)
	if err := WriteEnv(context.Background(), state, map[string]string{"MODEL": "new"}); err == nil {
		t.Fatal("accepted malformed configuration")
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatal("modified original on failure")
	}
}

func TestRuntimeConfigurationSources(t *testing.T) {
	t.Chdir(t.TempDir())
	state := t.TempDir()
	path := filepath.Join(state, "config.env")
	for _, key := range []string{"PRISM_MODEL_RUNTIME_ENGINE", "PRISM_MODEL_RUNTIME_BASE_URL", "PRISM_MODEL_RUNTIME_MODEL", "PRISM_CONFIG_FILE"} {
		t.Setenv(key, "")
	}
	WriteEnv(context.Background(), state, map[string]string{"PRISM_MODEL_RUNTIME_ENGINE": "ollama", "PRISM_MODEL_RUNTIME_MODEL": "saved"})
	os.WriteFile(".env", []byte("PRISM_MODEL_RUNTIME_MODEL=workspace\n"), 0o600)
	loaded, err := LoadForStateDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RuntimeSources["engine"] != path || loaded.RuntimeSources["model"] != ".env" || loaded.ModelRuntime.Primary.Model != "workspace" {
		t.Fatalf("sources=%v", loaded.RuntimeSources)
	}
	t.Setenv("PRISM_MODEL_RUNTIME_MODEL", "host")
	loaded, err = LoadForStateDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ModelRuntime.Primary.Model != "host" || loaded.RuntimeSources["model"] != "environment: PRISM_MODEL_RUNTIME_MODEL" {
		t.Fatalf("wrong environment provenance: %v", loaded.RuntimeSources)
	}
}
