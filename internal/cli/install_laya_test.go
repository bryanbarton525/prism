package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/config"
	"github.com/bryanbarton525/prism/internal/toolmodel"
	"github.com/spf13/cobra"
)

func TestInstallLayaPersistsGenericSettingsAndClearsKev(t *testing.T) {
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "config.env"), []byte("PRISM_KEV_URL=http://127.0.0.1:8009\nPRISM_KEV_MODEL=kev-latest\nPRISM_TOOL_RECOMMEND_MODEL=onnx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := installFlags{decisionService: "install-laya", layaPort: 8010}
	if err := validateInstallToolRouting(flags); err != nil {
		t.Fatal(err)
	}
	if err := saveInstallToolRouting(flags, state); err != nil {
		t.Fatal(err)
	}
	settings, err := config.LoadForStateDir(state)
	if err != nil {
		t.Fatal(err)
	}
	if settings.DecisionURL != "http://127.0.0.1:8010" || settings.DecisionModel != "laya-english" || settings.KevURL != "" || settings.KevModel != "" || settings.ToolRecommendModel != "onnx" {
		t.Fatalf("settings=%+v", settings)
	}
	flags.decisionModel = "kev-latest"
	if err := validateInstallToolRouting(flags); err == nil {
		t.Fatal("managed Laya accepted Kev identity")
	}
	flags.decisionModel = ""
	flags.layaPort = 80
	if err := validateInstallToolRouting(flags); err == nil {
		t.Fatal("privileged port accepted")
	}
}

func TestManagedLayaUnitPinsCPUAndWeights(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	state := t.TempDir()
	name := managedLayaUnitName(state)
	if err := writeLayaUnit(state, filepath.Join(state, "laya", "venv"), name, 8010); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "systemd", "user", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{"LAYA_DEVICE=cpu", "LAYA_HOST=127.0.0.1", "LAYA_PORT=8010", "LAYA_THREADS=4", "LAYA_MODELS=english", toolmodel.LayaRevision, "Restart=on-failure", "WantedBy=default.target"} {
		if !strings.Contains(string(data), wanted) {
			t.Errorf("unit missing %q", wanted)
		}
	}
	if err := os.WriteFile(path, []byte("# foreign unit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeLayaUnit(state, filepath.Join(state, "laya", "venv"), name, 8010); err == nil {
		t.Fatal("foreign unit overwritten")
	}
}

func TestWaitForManagedLayaChecksRevisionAndChoiceInference(t *testing.T) {
	inferred := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			fmt.Fprintf(w, `{"device":"cpu","revisions":{"english":%q}}`, toolmodel.LayaRevision)
			return
		}
		inferred = true
		fmt.Fprint(w, `{"answers":{"tool":{"type":"choice","probabilities":{"tool_0":0.9,"tool_1":0.1}}}}`)
	}))
	defer server.Close()
	if err := waitForManagedLaya(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	if !inferred {
		t.Fatal("readiness did not exercise inference")
	}
}

func TestManagedLayaDryRunDoesNotInstallOrPersist(t *testing.T) {
	state := t.TempDir()
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", state, "")
	if err := cmd.Flags().Set("state-dir", state); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runInstall(cmd, installFlags{runtimeOnly: true, runtimeScope: "user", dryRun: true, decisionService: "install-laya", layaPort: 8010}); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"config.env", "laya"} {
		if _, err := os.Stat(filepath.Join(state, file)); !os.IsNotExist(err) {
			t.Fatalf("dry run touched %s: %v", file, err)
		}
	}
	if !strings.Contains(out.String(), "Laya "+managedLayaVersion) || !strings.Contains(out.String(), "http://127.0.0.1:8010") {
		t.Fatalf("preview=%s", out.String())
	}
}

func TestManagedLayaReadinessRejectsWrongDeviceOrRevision(t *testing.T) {
	for _, health := range []string{
		fmt.Sprintf(`{"device":"cuda:0","revisions":{"english":%q}}`, toolmodel.LayaRevision),
		`{"device":"cpu","revisions":{"english":"wrong-revision"}}`,
	} {
		t.Run(health, func(t *testing.T) {
			var inference atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					inference.Add(1)
				}
				fmt.Fprint(w, health)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if err := waitForManagedLaya(ctx, server.URL); err == nil {
				t.Fatal("invalid runtime accepted")
			}
			if inference.Load() != 0 {
				t.Fatal("inference ran before valid pinned CPU readiness")
			}
		})
	}
}

func TestLayaDocumentationAndExampleConfiguration(t *testing.T) {
	for _, file := range []string{"docs/usage.md", "docs/tool-recommendation.md", "examples/README.md"} {
		data, err := os.ReadFile(filepath.Join("..", "..", file))
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range []string{"--decision-service install-laya", "--decision-service laya", "laya_choice", "500 ms"} {
			if !strings.Contains(string(data), marker) {
				t.Errorf("%s missing %q", file, marker)
			}
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	for _, setting := range []string{"PRISM_TOOL_RECOMMEND_MODEL=onnx", "PRISM_DECISION_URL=http://127.0.0.1:8010", "PRISM_DECISION_MODEL=laya-english", "PRISM_DECISION_API_KEY_ENV=LAYA_API_KEY"} {
		if !strings.Contains(string(data), "# "+setting) {
			t.Errorf("missing optional example %q", setting)
		}
	}
}
