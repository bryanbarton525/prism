package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/fileatomic"
	"github.com/bryanbarton525/prism/internal/filelock"
)

// ReadEnv reads saved values without applying process or workspace overrides.
func ReadEnv(path string) (map[string]string, error) {
	values, err := parseEnvFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	return values, err
}

// WriteEnv updates selected keys while preserving unrelated lines. All runtime
// writers take the same lock and replace the file atomically with private mode.
func WriteEnv(ctx context.Context, stateDir string, settings map[string]string) error {
	if len(settings) == 0 {
		return nil
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	unlock, err := filelock.Acquire(ctx, filepath.Join(stateDir, "runtime-config.lock"), 5*time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(stateDir, "config.env")
	if _, err := ReadEnv(path); err != nil {
		return fmt.Errorf("read saved configuration: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var lines []string
	if len(data) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}
	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(strings.TrimPrefix(trimmed, "export "), "=")
		key = strings.TrimSpace(key)
		if value, found := settings[key]; ok && found {
			lines[i] = key + "=" + strconv.Quote(value)
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !seen[key] {
			lines = append(lines, key+"="+strconv.Quote(settings[key]))
		}
	}
	tmp, err := os.CreateTemp(stateDir, ".config-env-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return fileatomic.Replace(tmp.Name(), path)
}
