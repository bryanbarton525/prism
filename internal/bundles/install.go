package bundles

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	bundlepkg "github.com/bryanbarton525/prism/pkg/bundle"
	"github.com/bryanbarton525/prism/pkg/registry"
)

type InstallOptions struct {
	ManifestPath string
	SourceRoot   string
	DestRoot     string
	StatePath    string
	PublicKey    string
	PrismVersion string
}

func VerifyRegistryManifest(opts InstallOptions) (registry.Manifest, error) {
	inputs, err := loadRegistryInputs(opts)
	if err != nil {
		return registry.Manifest{}, err
	}
	defer inputs.cleanup()
	if err := registry.VerifyManifest(inputs.sourceRoot, inputs.manifest, inputs.publicKey, prismVersion(opts)); err != nil {
		return registry.Manifest{}, err
	}
	return inputs.manifest, nil
}

func InstallVerified(opts InstallOptions) (registry.Manifest, error) {
	inputs, err := loadRegistryInputs(opts)
	if err != nil {
		return registry.Manifest{}, err
	}
	defer inputs.cleanup()
	if strings.TrimSpace(opts.DestRoot) == "" {
		return registry.Manifest{}, fmt.Errorf("dest root is required")
	}
	if strings.TrimSpace(opts.StatePath) == "" {
		return registry.Manifest{}, fmt.Errorf("state path is required")
	}
	if err := registry.Install(inputs.sourceRoot, opts.DestRoot, inputs.manifest, inputs.publicKey, prismVersion(opts)); err != nil {
		return registry.Manifest{}, err
	}
	if err := RecordRegistryInstall(opts.StatePath, inputs.manifest); err != nil {
		return registry.Manifest{}, err
	}
	return inputs.manifest, nil
}

func RecordRegistryInstall(path string, manifest registry.Manifest) error {
	state, err := Load(path)
	if err != nil {
		return err
	}
	for _, bundle := range manifest.Bundles {
		installed := bundlepkg.Installed{
			ID:              bundle.ID,
			Version:         bundle.Version,
			Channel:         bundle.Channel,
			Owner:           bundle.Owner,
			RiskLevel:       bundle.RiskLevel,
			RequiredPlugins: append([]string{}, bundle.RequiredPlugins...),
			InstalledAt:     time.Now().UTC().Format(time.RFC3339),
		}
		replaced := false
		for i := range state.Bundles {
			if state.Bundles[i].ID == bundle.ID {
				state.Bundles[i] = installed
				replaced = true
				break
			}
		}
		if !replaced {
			state.Bundles = append(state.Bundles, installed)
		}
	}
	return Save(path, state)
}

type registryInputs struct {
	manifest   registry.Manifest
	publicKey  ed25519.PublicKey
	sourceRoot string
	cleanup    func()
}

func loadRegistryInputs(opts InstallOptions) (registryInputs, error) {
	if strings.TrimSpace(opts.ManifestPath) == "" {
		return registryInputs{}, fmt.Errorf("manifest path is required")
	}
	manifest, sourceRoot, cleanup, err := loadManifestAndSource(opts)
	if err != nil {
		return registryInputs{}, err
	}
	publicKey, err := parsePublicKey(opts.PublicKey)
	if err != nil {
		cleanup()
		return registryInputs{}, err
	}
	return registryInputs{
		manifest:   manifest,
		publicKey:  publicKey,
		sourceRoot: sourceRoot,
		cleanup:    cleanup,
	}, nil
}

func loadManifestAndSource(opts InstallOptions) (registry.Manifest, string, func(), error) {
	cleanup := func() {}
	if !isHTTPURL(opts.ManifestPath) {
		manifest, err := registry.LoadManifest(opts.ManifestPath)
		return manifest, sourceRoot(opts), cleanup, err
	}

	manifest, err := loadRemoteManifest(opts.ManifestPath)
	if err != nil {
		return registry.Manifest{}, "", cleanup, err
	}
	tmp, err := os.MkdirTemp("", "prism-registry-*")
	if err != nil {
		return registry.Manifest{}, "", cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	base := remoteSourceBase(opts)
	for _, bundle := range manifest.Bundles {
		for _, file := range bundle.Files {
			if err := validateRegistryPath(file.Path); err != nil {
				cleanup()
				return registry.Manifest{}, "", cleanup, fmt.Errorf("%s: %w", file.Path, err)
			}
			fileURL, err := joinRegistryURL(base, file.Path)
			if err != nil {
				cleanup()
				return registry.Manifest{}, "", cleanup, err
			}
			dst := filepath.Join(tmp, filepath.FromSlash(filepath.Clean(file.Path)))
			if err := downloadFile(fileURL, dst); err != nil {
				cleanup()
				return registry.Manifest{}, "", cleanup, fmt.Errorf("%s: %w", file.Path, err)
			}
		}
	}
	return manifest, tmp, cleanup, nil
}

func loadRemoteManifest(rawURL string) (registry.Manifest, error) {
	body, err := fetchURL(rawURL)
	if err != nil {
		return registry.Manifest{}, err
	}
	var manifest registry.Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return registry.Manifest{}, fmt.Errorf("parsing remote registry manifest: %w", err)
	}
	return manifest, nil
}

func remoteSourceBase(opts InstallOptions) string {
	if isHTTPURL(opts.SourceRoot) {
		return opts.SourceRoot
	}
	u, err := url.Parse(opts.ManifestPath)
	if err != nil {
		return opts.ManifestPath
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	if idx := strings.LastIndex(u.Path, "/"); idx >= 0 {
		u.Path = u.Path[:idx]
	}
	return u.String()
}

func joinRegistryURL(base, rel string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/") {
		u.Path += "/" + url.PathEscape(part)
	}
	return u.String(), nil
}

func downloadFile(rawURL, dst string) error {
	body, err := fetchURL(rawURL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, body, 0o644)
}

func fetchURL(rawURL string) ([]byte, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("fetching %s: HTTP %d: %s", rawURL, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func isHTTPURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func validateRegistryPath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path escapes registry root")
	}
	return nil
}

func parsePublicKey(value string) (ed25519.PublicKey, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("public key is required")
	}
	if data, err := os.ReadFile(value); err == nil {
		value = strings.TrimSpace(string(data))
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return checkedPublicKey(decoded)
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return checkedPublicKey(decoded)
	}
	if decoded, err := hex.DecodeString(value); err == nil {
		return checkedPublicKey(decoded)
	}
	return nil, fmt.Errorf("public key must be base64, hex, or a path containing one of those encodings")
}

func checkedPublicKey(data []byte) (ed25519.PublicKey, error) {
	if len(data) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length: got %d", len(data))
	}
	return ed25519.PublicKey(data), nil
}

func sourceRoot(opts InstallOptions) string {
	if strings.TrimSpace(opts.SourceRoot) != "" {
		return opts.SourceRoot
	}
	return filepath.Dir(opts.ManifestPath)
}

func prismVersion(opts InstallOptions) string {
	if strings.TrimSpace(opts.PrismVersion) != "" {
		return opts.PrismVersion
	}
	return "0.1.0"
}
