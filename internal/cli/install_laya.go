package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/fileatomic"
	"github.com/bryanbarton525/prism/internal/toolmodel"
)

const managedLayaVersion = "0.3.22"

// installManagedLaya installs the evaluated CPU runtime and a persistent user service.
// The checkpoint revision, device, threads, and token budgets are explicit.
func installManagedLaya(ctx context.Context, stateDir string, port int, uvName string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("managed Laya requires Linux systemd user services; connect an existing Laya endpoint on this platform")
	}
	uv, err := exec.LookPath(firstNonEmptyInstall(uvName, "uv"))
	if err != nil {
		return fmt.Errorf("managed Laya requires uv: %w", err)
	}
	for _, executable := range []string{"systemctl", "loginctl"} {
		if _, err := exec.LookPath(executable); err != nil {
			return err
		}
	}
	if err := runDecisionCommand(ctx, "systemctl", "--user", "show-environment"); err != nil {
		return err
	}
	name := managedLayaUnitName(stateDir)
	active := exec.CommandContext(ctx, "systemctl", "--user", "is-active", "--quiet", name).Run() == nil
	if !active {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			connection.Close()
			return fmt.Errorf("Laya port %d is already in use by a different service", port)
		}
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	if current.Username == "" {
		return fmt.Errorf("cannot identify current user for systemd linger")
	}
	root := filepath.Join(stateDir, "laya")
	venv := filepath.Join(root, "venv")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(venv, "pyvenv.cfg")); os.IsNotExist(err) {
		if err := runDecisionCommand(ctx, uv, "venv", "--python", "3.13", venv); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := runDecisionCommand(ctx, uv, "pip", "install", "--python", filepath.Join(venv, "bin", "python"), "--torch-backend", "cpu", "laya[serve]=="+managedLayaVersion, "torch==2.8.0+cpu", "transformers==5.17.0"); err != nil {
		return fmt.Errorf("install Laya CPU dependencies: %w", err)
	}
	if err := writeLayaUnit(stateDir, venv, name, port); err != nil {
		return err
	}
	if err := runDecisionCommand(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	wasEnabled := exec.CommandContext(ctx, "systemctl", "--user", "is-enabled", "--quiet", name).Run() == nil
	success := false
	defer func() {
		if success {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = runDecisionCommand(cleanup, "systemctl", "--user", "stop", name)
		if !wasEnabled {
			_ = runDecisionCommand(cleanup, "systemctl", "--user", "disable", name)
		}
	}()
	if err := runDecisionCommand(ctx, "systemctl", "--user", "enable", name); err != nil {
		return err
	}
	if err := runDecisionCommand(ctx, "systemctl", "--user", "restart", name); err != nil {
		return err
	}
	if err := waitForManagedLaya(ctx, fmt.Sprintf("http://127.0.0.1:%d", port)); err != nil {
		return fmt.Errorf("managed Laya did not become ready: %w", err)
	}
	if err := runDecisionCommand(ctx, "loginctl", "enable-linger", current.Username); err != nil {
		return err
	}
	success = true
	return nil
}

func managedLayaUnitName(stateDir string) string {
	digest := sha256.Sum256([]byte(filepath.Clean(stateDir)))
	return "prism-laya-" + hex.EncodeToString(digest[:6]) + ".service"
}

func writeLayaUnit(stateDir, venv, name string, port int) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	unitDir := filepath.Join(configDir, "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o700); err != nil {
		return err
	}
	for _, value := range []string{stateDir, venv} {
		if strings.ContainsAny(value, "\n\r") {
			return fmt.Errorf("managed Laya paths cannot contain newlines")
		}
	}
	escape := func(value string) string { return strconv.Quote(strings.ReplaceAll(value, "%", "%%")) }
	unit := "# Managed by Prism: pinned Laya " + managedLayaVersion + "\n" +
		"[Unit]\nDescription=Prism managed Laya choice service\nAfter=network-online.target\nWants=network-online.target\n\n" +
		"[Service]\nType=simple\n" +
		"Environment=LAYA_HOST=127.0.0.1\nEnvironment=LAYA_PORT=" + strconv.Itoa(port) + "\n" +
		"Environment=LAYA_DEVICE=cpu\nEnvironment=LAYA_THREADS=4\nEnvironment=LAYA_MODELS=english\nEnvironment=LAYA_MAX_LOADED=1\nEnvironment=LAYA_PRELOAD=1\n" +
		"Environment=LAYA_REVISION=" + toolmodel.LayaRevision + "\n" +
		"Environment=" + escape("HF_HOME="+filepath.Join(stateDir, "laya", "huggingface")) + "\n" +
		"Environment=USE_TF=0\nEnvironment=CUDA_VISIBLE_DEVICES=-1\n" +
		"ExecStart=" + escape(filepath.Join(venv, "bin", "laya-serve")) + "\n" +
		"Restart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=default.target\n"
	path := filepath.Join(unitDir, name)
	if old, err := os.ReadFile(path); err == nil {
		if string(old) == unit {
			return nil
		}
		if !strings.HasPrefix(string(old), "# Managed by Prism: pinned Laya ") {
			return fmt.Errorf("refusing to replace non-Prism systemd unit %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(unitDir, ".prism-laya-unit-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(unit); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return fileatomic.Replace(tmp.Name(), path)
}

func waitForManagedLaya(ctx context.Context, endpoint string) error {
	readyCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second}
	for {
		req, err := http.NewRequestWithContext(readyCtx, http.MethodGet, endpoint+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			var health struct {
				Device    string            `json:"device"`
				Revisions map[string]string `json:"revisions"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&health)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && decodeErr == nil && health.Device == "cpu" && health.Revisions["english"] == toolmodel.LayaRevision {
				break
			}
		}
		select {
		case <-readyCtx.Done():
			return readyCtx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	decision, err := toolmodel.NewDecisionClient(endpoint, "", "laya-english")
	if err != nil {
		return err
	}
	_, err = decision.Score(readyCtx, "Read service health without changing it", []toolmodel.DecisionTool{
		{Name: "health", Description: "Check service health"},
		{Name: "restart", Description: "Restart the service"},
	})
	return err
}
