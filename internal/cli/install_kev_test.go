package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWaitForManagedKevRequiresModelAndInference(t *testing.T) {
	models, inference := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			models++
			_, _ = w.Write([]byte(`{"data":[{"id":"kev-latest"}]}`))
		case "/v1/systemone":
			inference++
			_, _ = w.Write([]byte(`{"answers":{"tool_0":{"type":"noul","noul":0.9}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	if err := waitForManagedKev(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	if models != 1 || inference != 1 {
		t.Fatalf("models=%d inference=%d", models, inference)
	}
}

func TestManagedKevUnitIsPerStateAndDoesNotOverwriteForeignUnit(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	state := filepath.Join(t.TempDir(), "state with spaces")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(state, "kev", "source")
	name := managedKevUnitName(state)
	if name == managedKevUnitName(t.TempDir()) {
		t.Fatal("distinct states produced same unit name")
	}
	if err := writeKevUnit(state, source, "/usr/bin/uv", name, 8009, "cpu"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config, "systemd", "user", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{managedKevRevision, "--run jaredpalmer/kev-0.8b --port 8009", "Restart=on-failure", "WantedBy=default.target", "Environment=\"HF_HOME=", "Environment=KEV_FUSED=0", "Environment=CUDA_VISIBLE_DEVICES=-1"} {
		if !strings.Contains(string(data), wanted) {
			t.Fatalf("unit missing %q: %s", wanted, data)
		}
	}
	if err := writeKevUnit(state, source, "/usr/bin/uv", name, 8009, "cpu"); err != nil {
		t.Fatalf("idempotent write: %v", err)
	}
	if _, err := exec.LookPath("systemd-analyze"); err == nil {
		cmd := exec.Command("systemd-analyze", "verify", path)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("systemd unit invalid: %v: %s", err, output)
		}
	}
	if err := os.WriteFile(path, []byte("[Service]\nExecStart=/bin/true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeKevUnit(state, source, "/usr/bin/uv", name, 8009, "cpu"); err == nil {
		t.Fatal("foreign unit overwritten")
	}
}
