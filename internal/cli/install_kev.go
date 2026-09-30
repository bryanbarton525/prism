package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

const (
	managedKevRevision   = "eb45fd2381396eb7edc3964b753ebc1b0ab1da2b"
	managedKevRepository = "https://github.com/jaredpalmer/kev.git"
	managedKevCheckpoint = "jaredpalmer/kev-0.8b"
)

// installManagedKev installs a pinned Kev checkout and a persistent user service.
// The checkout, model cache, and unit are owned by the selected Prism state.
func installManagedKev(ctx context.Context, stateDir string, port int, uvName, device string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("managed Kev currently requires Linux systemd user services; connect an existing Kev server on this platform")
	}
	uv, err := exec.LookPath(uvName)
	if err != nil {
		return fmt.Errorf("managed Kev requires uv: %w", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("managed Kev requires git: %w", err)
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("managed Kev requires systemctl: %w", err)
	}
	if _, err := exec.LookPath("loginctl"); err != nil {
		return fmt.Errorf("managed Kev requires loginctl for reboot persistence: %w", err)
	}
	if err := runKevCommand(ctx, "systemctl", "--user", "show-environment"); err != nil {
		return fmt.Errorf("systemd user manager unavailable: %w", err)
	}
	unitName := managedKevUnitName(stateDir)
	active := exec.CommandContext(ctx, "systemctl", "--user", "is-active", "--quiet", unitName).Run() == nil
	if !active {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			connection.Close()
			return fmt.Errorf("Kev port %d is already in use by a different service", port)
		}
	}
	userInfo, err := user.Current()
	if err != nil {
		return err
	}
	if userInfo.Username == "" {
		return fmt.Errorf("cannot identify current user for systemd linger")
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "kev"), 0o700); err != nil {
		return err
	}
	source := filepath.Join(stateDir, "kev", "source")
	if err := installKevCheckout(ctx, source); err != nil {
		return err
	}
	if err := runKevCommandIn(ctx, source, uv, "sync", "--frozen", "--extra", "serve"); err != nil {
		return fmt.Errorf("install Kev Python dependencies: %w", err)
	}
	if err := writeKevUnit(stateDir, source, uv, unitName, port, device); err != nil {
		return err
	}
	if err := runKevCommand(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	wasEnabled := exec.CommandContext(ctx, "systemctl", "--user", "is-enabled", "--quiet", unitName).Run() == nil
	serviceAttempted := false
	serviceStarted := false
	defer func() {
		if serviceStarted || !serviceAttempted {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = runKevCommand(cleanupCtx, "systemctl", "--user", "stop", unitName)
		if !wasEnabled {
			_ = runKevCommand(cleanupCtx, "systemctl", "--user", "disable", unitName)
		}
	}()
	if err := runKevCommand(ctx, "systemctl", "--user", "enable", unitName); err != nil {
		return fmt.Errorf("enable managed Kev service: %w", err)
	}
	serviceAttempted = true
	if err := runKevCommand(ctx, "systemctl", "--user", "restart", unitName); err != nil {
		return fmt.Errorf("start managed Kev service: %w", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForManagedKev(ctx, url); err != nil {
		return fmt.Errorf("managed Kev service did not become ready: %w", err)
	}
	if err := runKevCommand(ctx, "loginctl", "enable-linger", userInfo.Username); err != nil {
		return fmt.Errorf("Kev service is ready, but reboot persistence could not be enabled: %w", err)
	}
	serviceStarted = true
	return nil
}

func installKevCheckout(ctx context.Context, source string) error {
	if _, err := os.Stat(source); err == nil {
		output, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "HEAD").Output()
		if err != nil || strings.TrimSpace(string(output)) != managedKevRevision {
			return fmt.Errorf("existing Kev source at %s is not Prism's pinned revision; move it before retrying", source)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(source), ".kev-source-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := runKevCommand(ctx, "git", "clone", "--quiet", managedKevRepository, tmp); err != nil {
		return err
	}
	if err := runKevCommand(ctx, "git", "-C", tmp, "checkout", "--quiet", "--detach", managedKevRevision); err != nil {
		return err
	}
	return os.Rename(tmp, source)
}

func managedKevUnitName(stateDir string) string {
	digest := sha256.Sum256([]byte(stateDir))
	return "prism-kev-" + hex.EncodeToString(digest[:6]) + ".service"
}

func writeKevUnit(stateDir, source, uv, name string, port int, device string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	unitDir := filepath.Join(configDir, "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(unitDir, name)
	for _, value := range []string{source, uv, stateDir} {
		if strings.ContainsAny(value, "\n\r") {
			return fmt.Errorf("managed Kev paths cannot contain newlines")
		}
	}
	escape := func(value string) string { return strconv.Quote(strings.ReplaceAll(value, "%", "%%")) }
	escapePath := func(value string) string {
		value = strings.ReplaceAll(value, "%", "%%")
		value = strings.ReplaceAll(value, "\\", "\\\\")
		value = strings.ReplaceAll(value, " ", "\\x20")
		return strings.ReplaceAll(value, "\t", "\\x09")
	}
	unit := "# Managed by Prism: pinned Kev " + managedKevRevision + "\n" +
		"[Unit]\nDescription=Prism managed Kev decision service\nAfter=network-online.target\nWants=network-online.target\n\n" +
		"[Service]\nType=simple\nWorkingDirectory=" + escapePath(source) + "\n" +
		"Environment=" + escape("HF_HOME="+filepath.Join(stateDir, "kev", "huggingface")) + "\n" +
		"Environment=KEV_FUSED=0\n" +
		kevDeviceEnvironment(device) +
		"ExecStart=" + escape(uv) + " run --no-sync --extra serve python -m kev.serve --run " + managedKevCheckpoint + " --port " + strconv.Itoa(port) + "\n" +
		"Restart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=default.target\n"
	if old, err := os.ReadFile(path); err == nil {
		if string(old) == unit {
			return nil
		}
		if !strings.HasPrefix(string(old), "# Managed by Prism: pinned Kev ") {
			return fmt.Errorf("refusing to replace non-Prism systemd unit %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(unitDir, ".prism-kev-unit-")
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

func kevDeviceEnvironment(device string) string {
	if device == "cpu" {
		return "Environment=CUDA_VISIBLE_DEVICES=-1\n"
	}
	return ""
}

func waitForManagedKev(ctx context.Context, endpoint string) error {
	readyCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second}
	for {
		req, err := http.NewRequestWithContext(readyCtx, http.MethodGet, endpoint+"/v1/models", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case <-readyCtx.Done():
			return readyCtx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	decision, err := toolmodel.NewDecisionClient(endpoint, "", "kev-latest")
	if err != nil {
		return err
	}
	_, err = decision.Score(readyCtx, "Select a service health tool", []toolmodel.KevTool{{Name: "health", Description: "Check service health"}})
	return err
}

func runKevCommand(ctx context.Context, executable string, args ...string) error {
	return runKevCommandIn(ctx, "", executable, args...)
}

func runKevCommandIn(ctx context.Context, dir, executable string, args ...string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 1200 {
			message = message[len(message)-1200:]
		}
		return fmt.Errorf("%s %s: %w: %s", executable, strings.Join(args, " "), err, message)
	}
	return nil
}
