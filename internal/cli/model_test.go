package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/config"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

func modelTestState(t *testing.T) string {
	t.Helper()
	oldCfg, oldGF := cfg, gf
	t.Cleanup(func() { cfg, gf = oldCfg, oldGF })
	t.Chdir(t.TempDir())
	for _, key := range []string{"PRISM_CONFIG_FILE", "PRISM_MODEL_RUNTIME_ENGINE", "PRISM_MODEL_RUNTIME_BASE_URL", "PRISM_MODEL_RUNTIME_MODEL", "PRISM_MODEL_RUNTIME_API_KEY", "PRISM_MODEL_RUNTIME_API_KEY_ENV", "PRISM_MODEL_RUNTIME_FALLBACK_ENGINE", "PRISM_MODEL_RUNTIME_FALLBACK_BASE_URL", "PRISM_MODEL_RUNTIME_FALLBACK_MODEL"} {
		t.Setenv(key, "")
	}
	state := t.TempDir()
	gf = globalFlags{stateDir: state, ollamaHost: config.DefaultOllamaHost}
	var err error
	cfg, err = config.LoadForStateDir(state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func executeModel(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newModelCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	returnValue := cmd.ExecuteContext(context.Background())
	return out.String(), returnValue
}

func TestModelSetPersistsValidatedTargetAndPreservesOtherSettings(t *testing.T) {
	state := modelTestState(t)
	os.WriteFile(filepath.Join(state, "config.env"), []byte("# keep\nUNRELATED=keep\nPRISM_MODEL_RUNTIME_FALLBACK_ENGINE=ollama\nPRISM_MODEL_RUNTIME_FALLBACK_MODEL=backup\n"), 0o600)
	t.Setenv("MODEL_TEST_KEY", "private-key")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer private-key" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"new-model"}]}`))
	}))
	defer server.Close()
	out, err := executeModel(t, "set", "new-model", "--engine", "sglang", "--url", server.URL+"/v1", "--api-key-env", "MODEL_TEST_KEY")
	if err != nil {
		t.Fatal(err)
	}
	values, err := config.ReadEnv(filepath.Join(state, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || values["PRISM_MODEL_RUNTIME_MODEL"] != "new-model" || values["UNRELATED"] != "keep" || values["PRISM_MODEL_RUNTIME_FALLBACK_MODEL"] != "backup" {
		t.Fatalf("calls=%d values=%v", calls, values)
	}
	data, _ := os.ReadFile(filepath.Join(state, "config.env"))
	if strings.Contains(string(data), "private-key") || strings.Contains(out, "private-key") {
		t.Fatal("leaked credential")
	}
	if !strings.Contains(out, "Saved shared") || !strings.Contains(out, "inference not tested") {
		t.Fatalf("output=%s", out)
	}
}

func TestModelSetFailureDryRunAndOffline(t *testing.T) {
	for _, mode := range []string{"missing", "unauthorized", "dry-run", "skip-check"} {
		t.Run(mode, func(t *testing.T) {
			state := modelTestState(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if mode == "unauthorized" {
					w.WriteHeader(401)
					return
				}
				w.Write([]byte(`{"models":[{"name":"available"}]}`))
			}))
			defer server.Close()
			args := []string{"set", "missing", "--engine", "ollama", "--url", server.URL}
			if mode == "dry-run" || mode == "skip-check" {
				args = append(args, "--"+mode)
			}
			_, err := executeModel(t, args...)
			_, fileErr := os.Stat(filepath.Join(state, "config.env"))
			if mode == "skip-check" {
				if err != nil || fileErr != nil || calls != 0 {
					t.Fatalf("err=%v file=%v calls=%d", err, fileErr, calls)
				}
				return
			}
			if !os.IsNotExist(fileErr) {
				t.Fatalf("created config on %s", mode)
			}
			if mode == "dry-run" {
				if err != nil || calls != 0 {
					t.Fatalf("err=%v calls=%d", err, calls)
				}
			} else if err == nil || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestModelSetKeepsEngineAndRequiresURLWhenChanging(t *testing.T) {
	modelTestState(t)
	cfg.ModelRuntime.Primary = runtime.Config{Engine: runtime.EngineSGLang, BaseURL: "http://sglang/v1", Model: "old", APIKey: "old-key"}
	cfg.RuntimeKeyEnv = "OLD_KEY"
	cmd := newModelSetCmd()
	target, key, err := selectModelTarget(cmd, "new", modelSetFlags{})
	if err != nil || target.Engine != "sglang" || target.BaseURL != "http://sglang/v1" || key != "OLD_KEY" {
		t.Fatalf("target=%v key=%s err=%v", target, key, err)
	}
	if _, _, err := selectModelTarget(cmd, "new", modelSetFlags{engine: "vllm"}); err == nil {
		t.Fatal("switched remote engines without URL")
	}
	target, key, err = selectModelTarget(cmd, "new", modelSetFlags{engine: "ollama"})
	if err != nil || target.BaseURL != config.DefaultOllamaHost || target.APIKey != "" || key != "" {
		t.Fatalf("credential carried across target change: %+v %s %v", target, key, err)
	}
}

func TestModelSetReportsEnvironmentOverrideAndJSONStaysClean(t *testing.T) {
	state := modelTestState(t)
	gf.jsonOut = true
	t.Setenv("PRISM_MODEL_RUNTIME_MODEL", "host-model")
	cfg, _ = config.LoadForStateDir(state)
	out, err := executeModel(t, "set", "saved-model", "--skip-check")
	if err != nil {
		t.Fatal(err)
	}
	var report modelReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Primary.Model != "host-model" || report.Saved["model"] != "saved-model" || len(report.Warnings) == 0 || report.Sources["model"] != "environment: PRISM_MODEL_RUNTIME_MODEL" {
		t.Fatalf("report=%+v", report)
	}
}

func TestModelStatusOfflineAndCheckExit(t *testing.T) {
	modelTestState(t)
	gf.jsonOut = true
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"data":[{"id":"available"}]}`))
	}))
	defer server.Close()
	cfg.ModelRuntime.Primary = runtime.Config{Engine: runtime.EngineSGLang, BaseURL: server.URL, Model: "missing", APIKey: "secret"}
	cfg.ModelRuntime.Fallback = &runtime.Config{Engine: runtime.EngineVLLM, BaseURL: "http://fallback/v1", APIKey: "fallback-secret", Headers: map[string]string{"X-Key": "private-header"}}
	out, err := executeModel(t, "status")
	if err != nil || calls != 0 {
		t.Fatalf("offline status err=%v calls=%d", err, calls)
	}
	if strings.Contains(out, "secret") || strings.Contains(out, "private-header") {
		t.Fatal("status leaked secrets")
	}
	out, err = executeModel(t, "status", "--check")
	if err == nil || calls != 1 || !json.Valid([]byte(out)) {
		t.Fatalf("check output=%s err=%v calls=%d", out, err, calls)
	}
}

func TestModelSetConfigAlignmentAndNoPromptInScripts(t *testing.T) {
	state := modelTestState(t)
	cfg.ConfigFile = filepath.Join(t.TempDir(), "other.env")
	if _, err := executeModel(t, "set", "new", "--skip-check"); err == nil || !strings.Contains(err.Error(), "align") {
		t.Fatalf("alignment error=%v", err)
	}
	cfg.ConfigFile = filepath.Join(state, "config.env")
	cmd := newModelSetCmd()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(io.Discard)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("script error=%v", err)
	}
	gf.jsonOut = true
	if _, err := executeModel(t, "set"); err == nil || !strings.Contains(err.Error(), "--json") {
		t.Fatalf("json picker error=%v", err)
	}
}

func TestModelPickerNavigationTypedInputAndCancellation(t *testing.T) {
	makePicker := func(input string) modelPicker {
		reader := bufio.NewReader(strings.NewReader(input))
		return modelPicker{read: func() (rune, error) { r, _, err := reader.ReadRune(); return r, err }, out: io.Discard}
	}
	p := makePicker("\x1b[B\x1b[B\x1b[A\n")
	index, err := p.choose("Model", []string{"a", "b", "c"}, 0)
	if err != nil || index != 1 {
		t.Fatalf("index=%d err=%v", index, err)
	}
	p = makePicker("typed\x7f-model\n")
	text, err := p.text("Model", "")
	if err != nil || text != "type-model" {
		t.Fatalf("text=%s err=%v", text, err)
	}
	for _, input := range []string{"\x03", "\x04", ""} {
		p = makePicker(input)
		_, err = p.choose("Model", []string{"a"}, 0)
		if err == nil {
			t.Fatal("cancellation accepted")
		}
	}
	modelTestState(t)
	p = makePicker("\ncustom-model\n\x1b[B\n")
	cmd := newModelSetCmd()
	_, _, err = p.selectTarget(context.Background(), cmd, modelSetFlags{engine: "ollama", url: config.DefaultOllamaHost, skipCheck: true})
	if !errors.Is(err, errModelCancelled) {
		t.Fatalf("final cancellation err=%v", err)
	}
}

func TestAgentModelErrorsAndRuntimeDrift(t *testing.T) {
	modelTestState(t)
	for _, args := range [][]string{{"set"}, {"set", "qwen3.6:27b"}, {"set", "go-helper", "--model", "new"}} {
		cmd := newAgentModelCmd()
		cmd.SetArgs(args)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		if err := cmd.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "prism model set") {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
	target := runtime.Config{Engine: runtime.EngineOllama, BaseURL: config.DefaultOllamaHost, Model: "new"}
	if !runtimeTargetConflicts(&extensions.RuntimeTarget{Engine: "ollama", BaseURL: config.DefaultOllamaHost, Model: "old"}, target) {
		t.Fatal("missed drift")
	}
	effective, source := agentEffectiveModel("declared")
	if effective.Model != "declared" || source != "agent specification" {
		t.Fatalf("effective=%v source=%s", effective, source)
	}
	cfg.ModelRuntime.Primary = target
	effective, source = agentEffectiveModel("declared")
	if effective.Model != "new" || source != "shared runtime" {
		t.Fatalf("effective=%v source=%s", effective, source)
	}
}

func TestModelAgentOutputPreservesDeclaredJSONAndShowsDrift(t *testing.T) {
	state := modelTestState(t)
	service := extensions.NewLocalAgentService(state)
	if _, err := service.CopyBundledAgent(context.Background(), prismbundle.BundleFS(), prismbundle.BundleDigest(), "go-helper", "managed-helper", false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetManagedAgentRuntime(context.Background(), "managed-helper", extensions.RuntimeTarget{Engine: "ollama", BaseURL: config.DefaultOllamaHost, Model: "old"}, false); err != nil {
		t.Fatal(err)
	}
	cfg.ModelRuntime.Primary = runtime.Config{Engine: runtime.EngineOllama, BaseURL: config.DefaultOllamaHost, Model: "shared"}
	cfg.RuntimeSources["model"] = "environment: PRISM_MODEL_RUNTIME_MODEL"
	out, err := captureStdout(t, func() error { return agentList(context.Background(), true) })
	if err != nil {
		t.Fatal(err)
	}
	var agents []struct {
		ID              string `json:"id"`
		Model           string `json:"model"`
		EffectiveModel  string `json:"effective_model"`
		RuntimeConflict bool   `json:"runtime_conflict"`
	}
	if err := json.Unmarshal([]byte(out), &agents); err != nil {
		t.Fatal(err)
	}
	foundBundled, foundManaged := false, false
	for _, agent := range agents {
		if agent.ID == "go-helper" {
			foundBundled = true
			if agent.Model != "qwen3.5:9b" || agent.EffectiveModel != "shared" || agent.RuntimeConflict {
				t.Fatalf("bundled=%+v", agent)
			}
		}
		if agent.ID == "managed-helper" {
			foundManaged = true
			if agent.Model != "old" || agent.EffectiveModel != "shared" || !agent.RuntimeConflict {
				t.Fatalf("managed=%+v", agent)
			}
		}
	}
	if !foundBundled || !foundManaged {
		t.Fatal("missing agent rows")
	}
	out, err = captureStdout(t, func() error { return agentShow(context.Background(), "managed-helper", true) })
	if err != nil {
		t.Fatal(err)
	}
	var show struct {
		Spec struct {
			Model string `json:"model"`
		} `json:"spec"`
		EffectiveRuntime runtime.Config `json:"effective_runtime"`
		RuntimeConflict  bool           `json:"runtime_conflict"`
	}
	if err := json.Unmarshal([]byte(out), &show); err != nil {
		t.Fatal(err)
	}
	if show.Spec.Model != "old" || show.EffectiveRuntime.Model != "shared" || !show.RuntimeConflict {
		t.Fatalf("show=%+v", show)
	}
	out, err = captureStdout(t, func() error { return agentList(context.Background(), false) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "DECLARED MODEL") || !strings.Contains(out, "EFFECTIVE MODEL") || !strings.Contains(out, "runtime-drift") {
		t.Fatalf("human output=%s", out)
	}
	report, err := readModelReport()
	if err != nil {
		t.Fatal(err)
	}
	if err := addManagedModelWarnings(&report, effectivePrimary()); err != nil {
		t.Fatal(err)
	}
	if len(report.Warnings) == 0 || !strings.Contains(strings.Join(report.Warnings, " "), "managed-helper --use-configured-model") {
		t.Fatalf("missing reconciliation guidance: %v", report.Warnings)
	}
}

func TestModelSetClearsSavedCredentialsOnEndpointChange(t *testing.T) {
	state := modelTestState(t)
	if err := config.WriteEnv(context.Background(), state, map[string]string{"PRISM_MODEL_RUNTIME_ENGINE": "sglang", "PRISM_MODEL_RUNTIME_BASE_URL": "http://old/v1", "PRISM_MODEL_RUNTIME_MODEL": "old", "PRISM_MODEL_RUNTIME_API_KEY": "saved-secret", "PRISM_MODEL_RUNTIME_API_KEY_ENV": "OLD_SECRET"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.LoadForStateDir(state)
	if _, err := executeModel(t, "set", "new", "--engine", "ollama", "--skip-check"); err != nil {
		t.Fatal(err)
	}
	values, err := config.ReadEnv(filepath.Join(state, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if values["PRISM_MODEL_RUNTIME_API_KEY"] != "" || values["PRISM_MODEL_RUNTIME_API_KEY_ENV"] != "" {
		t.Fatal("credentials survived an endpoint change")
	}
}

func TestModelSetPreservesCredentialsForImplicitUnchangedOllama(t *testing.T) {
	state := modelTestState(t)
	if err := config.WriteEnv(context.Background(), state, map[string]string{"PRISM_MODEL_RUNTIME_API_KEY": "saved-key"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.LoadForStateDir(state)
	if _, err := executeModel(t, "set", "new-model", "--skip-check"); err != nil {
		t.Fatal(err)
	}
	values, err := config.ReadEnv(filepath.Join(state, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if values["PRISM_MODEL_RUNTIME_API_KEY"] != "saved-key" {
		t.Fatal("cleared credentials although the implicit Ollama target did not change")
	}
}

func TestModelInactiveManagedVariantDoesNotBlockBundledAgent(t *testing.T) {
	state := modelTestState(t)
	cfg.ModelRuntime.Primary = runtime.Config{Engine: runtime.EngineOllama, BaseURL: config.DefaultOllamaHost, Model: "shared"}
	store := extensions.NewStore(state)
	manifest := extensions.EmptyManifest()
	manifest.Entries = append(manifest.Entries, extensions.ManifestEntry{Identity: "go-helper", Kind: "agent", ObjectPath: "missing-object", Digest: "invalid", Runtime: &extensions.RuntimeTarget{Engine: "sglang", BaseURL: "http://other/v1", Model: "other"}})
	if err := store.SaveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error { return agentList(context.Background(), true) })
	if err != nil {
		t.Fatal(err)
	}
	var agents []struct {
		ID       string `json:"id"`
		Origin   string `json:"origin"`
		Active   bool   `json:"active"`
		Conflict bool   `json:"runtime_conflict"`
	}
	if err := json.Unmarshal([]byte(out), &agents); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, agent := range agents {
		if agent.ID == "go-helper" && agent.Origin == "bundled" && agent.Active {
			found = true
			if agent.Conflict {
				t.Fatal("inactive managed variant marked the bundled agent as blocked")
			}
		}
	}
	if !found {
		t.Fatal("bundled fallback missing")
	}
}
