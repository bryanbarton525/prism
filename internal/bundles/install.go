package bundles

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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
	manifest, publicKey, err := loadRegistryInputs(opts)
	if err != nil {
		return registry.Manifest{}, err
	}
	if err := registry.VerifyManifest(sourceRoot(opts), manifest, publicKey, prismVersion(opts)); err != nil {
		return registry.Manifest{}, err
	}
	return manifest, nil
}

func InstallVerified(opts InstallOptions) (registry.Manifest, error) {
	manifest, publicKey, err := loadRegistryInputs(opts)
	if err != nil {
		return registry.Manifest{}, err
	}
	if strings.TrimSpace(opts.DestRoot) == "" {
		return registry.Manifest{}, fmt.Errorf("dest root is required")
	}
	if strings.TrimSpace(opts.StatePath) == "" {
		return registry.Manifest{}, fmt.Errorf("state path is required")
	}
	if err := registry.Install(sourceRoot(opts), opts.DestRoot, manifest, publicKey, prismVersion(opts)); err != nil {
		return registry.Manifest{}, err
	}
	if err := RecordRegistryInstall(opts.StatePath, manifest); err != nil {
		return registry.Manifest{}, err
	}
	return manifest, nil
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

func loadRegistryInputs(opts InstallOptions) (registry.Manifest, ed25519.PublicKey, error) {
	if strings.TrimSpace(opts.ManifestPath) == "" {
		return registry.Manifest{}, nil, fmt.Errorf("manifest path is required")
	}
	manifest, err := registry.LoadManifest(opts.ManifestPath)
	if err != nil {
		return registry.Manifest{}, nil, err
	}
	publicKey, err := parsePublicKey(opts.PublicKey)
	if err != nil {
		return registry.Manifest{}, nil, err
	}
	return manifest, publicKey, nil
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
