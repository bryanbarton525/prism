package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/config"
)

func TestInstallToolRoutingPersistsSelectedStateAndPreservesOtherSettings(t *testing.T) {
	state := t.TempDir()
	path := filepath.Join(state, "config.env")
	if err := os.WriteFile(path, []byte("# operator note\nUNRELATED=keep\nPRISM_TOOL_RECOMMEND_MODEL=old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := installFlags{toolModel: "potion", toolRecommendAgents: []string{"researcher", "planner"}, primaryEngine: "sglang", primaryURL: "http://127.0.0.1:30000", primaryModel: "local-model", primaryAPIKeyEnv: "TEST_LLM_KEY", decisionService: "jev", decisionURL: "https://jev.example", decisionKeyEnv: "TEST_JEV_KEY"}
	if err := saveInstallToolRouting(flags, state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# operator note\nUNRELATED=keep\n") || strings.Count(string(data), "PRISM_TOOL_RECOMMEND_MODEL=") != 1 {
		t.Fatalf("config changed unexpectedly: %s", data)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions: %v %v", info, err)
	}
	t.Setenv("TEST_LLM_KEY", "secret-key")
	loaded, err := config.LoadForStateDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ToolRecommendModel != "potion" || strings.Join(loaded.ToolRecommendAgents, ",") != "researcher,planner" || loaded.ModelRuntime.Primary.Engine != "sglang" || loaded.ModelRuntime.Primary.APIKey != "secret-key" || loaded.KevModel != "jev-latest" {
		t.Fatalf("loaded wrong configuration: %+v", loaded)
	}
	if strings.Contains(string(data), "secret-key") {
		t.Fatal("secret written to config")
	}
}

func TestInstallToolRoutingDryRunDoesNotProbeOrPersist(t *testing.T) {
	state := t.TempDir()
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	if err := cmd.Flags().Set("state-dir", state); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	flags := installFlags{runtimeOnly: true, runtimeScope: "user", dryRun: true, primaryEngine: "sglang", primaryURL: server.URL, primaryModel: "local-model"}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("dry run contacted the endpoint")
	}
	if _, err := os.Stat(filepath.Join(state, "config.env")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote config: %v", err)
	}
	if !strings.Contains(out.String(), "Tool routing configuration") {
		t.Fatalf("missing preview: %s", out.String())
	}
	out.Reset()
	flags = installFlags{runtimeOnly: true, runtimeScope: "user", dryRun: true, decisionService: "install", kevPort: 8009}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "kev")); !os.IsNotExist(err) {
		t.Fatalf("managed Kev dry run created files: %v", err)
	}
	if !strings.Contains(out.String(), "persistent systemd user service") {
		t.Fatalf("managed Kev preview missing: %s", out.String())
	}
}

func TestInstallToolRoutingProbesPrimaryAndDecisionService(t *testing.T) {
	primaryCalls, decisionCalls := 0, 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls++
		if r.URL.Path != "/health" {
			t.Errorf("primary path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer primary.Close()
	decision := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decisionCalls++
		if r.URL.Path != "/v1/systemone" {
			t.Errorf("decision path = %s", r.URL.Path)
		}
		var request struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != "kev-latest" {
			t.Errorf("model = %s", request.Model)
		}
		_, _ = w.Write([]byte(`{"answers":{"tool_0":{"type":"noul","noul":0.9}}}`))
	}))
	defer decision.Close()
	flags := installFlags{primaryEngine: "sglang", primaryURL: primary.URL, primaryModel: "local", decisionService: "local", decisionURL: decision.URL}
	if err := validateInstallToolRouting(flags); err != nil {
		t.Fatal(err)
	}
	if err := prepareInstallToolRouting(&cobra.Command{}, flags, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if primaryCalls != 1 || decisionCalls != 1 {
		t.Fatalf("probes: primary=%d decision=%d", primaryCalls, decisionCalls)
	}
}

func TestInstallToolRoutingRejectsUnsafeDecisionEndpoint(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/path"} {
		flags := installFlags{decisionService: "jev", decisionURL: raw}
		if err := validateInstallToolRouting(flags); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if err := validateInstallToolRouting(installFlags{decisionService: "local", decisionURL: "https://example.com"}); err == nil {
		t.Fatal("accepted remote endpoint as local Kev")
	}
	if err := validateInstallToolRouting(installFlags{decisionService: "local", decisionURL: "https://127.0.0.1:8009"}); err != nil {
		t.Fatalf("rejected local HTTPS: %v", err)
	}
}

func TestInstallManagedKevConfigurationIsExplicitAndLoopback(t *testing.T) {
	flags := installFlags{decisionService: "install", kevPort: 8009}
	if err := validateInstallToolRouting(flags); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	if err := saveInstallToolRouting(flags, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadForStateDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.KevURL != "http://127.0.0.1:8009" || loaded.KevModel != "kev-latest" {
		t.Fatalf("managed Kev config: %+v", loaded)
	}
	flags.decisionModel = "jev-latest"
	if err := validateInstallToolRouting(flags); err == nil {
		t.Fatal("accepted Jev model for managed Kev")
	}
	flags.decisionModel = "kev-latest"
	flags.kevPort = 80
	if err := validateInstallToolRouting(flags); err == nil {
		t.Fatal("accepted privileged port")
	}
}
