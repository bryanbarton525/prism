package bundles

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	bundlepkg "github.com/bryanbarton525/prism/pkg/bundle"
	"github.com/bryanbarton525/prism/pkg/registry"
)

type BuildOptions struct {
	BundleManifestPath string
	SourceRoot         string
	RegistryID         string
	RegistryVersion    string
	GeneratedAt        time.Time
	OutputPath         string
}

func BuildRegistryManifest(opts BuildOptions) (registry.Manifest, error) {
	if strings.TrimSpace(opts.BundleManifestPath) == "" {
		return registry.Manifest{}, fmt.Errorf("bundle manifest path is required")
	}
	manifest, err := LoadManifest(opts.BundleManifestPath)
	if err != nil {
		return registry.Manifest{}, err
	}
	sourceRoot := opts.SourceRoot
	if strings.TrimSpace(sourceRoot) == "" {
		sourceRoot = filepath.Dir(opts.BundleManifestPath)
	}
	if strings.TrimSpace(opts.RegistryID) == "" {
		opts.RegistryID = manifest.Owner
	}
	if strings.TrimSpace(opts.RegistryID) == "" {
		opts.RegistryID = manifest.ID
	}
	if strings.TrimSpace(opts.RegistryVersion) == "" {
		opts.RegistryVersion = manifest.Version
	}
	generatedAt := opts.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	files := make([]registry.BundleFile, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		if err := validateRegistryPath(file.Path); err != nil {
			return registry.Manifest{}, fmt.Errorf("%s: %w", file.Path, err)
		}
		sum := strings.TrimSpace(file.SHA256)
		if sum == "" {
			path := filepath.Join(sourceRoot, filepath.FromSlash(filepath.Clean(file.Path)))
			sum, err = sha256File(path)
			if err != nil {
				return registry.Manifest{}, fmt.Errorf("%s: %w", file.Path, err)
			}
		}
		files = append(files, registry.BundleFile{
			Kind:   file.Kind,
			Path:   filepath.ToSlash(filepath.Clean(file.Path)),
			SHA256: strings.ToLower(sum),
		})
	}
	out := registry.Manifest{
		RegistryID:  opts.RegistryID,
		Version:     opts.RegistryVersion,
		GeneratedAt: generatedAt,
		Compat: registry.Compat{
			MinPrismVersion: manifest.Compatibility.MinPrismVersion,
			MaxPrismVersion: manifest.Compatibility.MaxPrismVersion,
		},
		Bundles: []registry.Bundle{registryBundle(manifest, files)},
	}
	if strings.TrimSpace(opts.OutputPath) != "" {
		if err := WriteRegistryManifest(opts.OutputPath, out); err != nil {
			return registry.Manifest{}, err
		}
	}
	return out, nil
}

type SignOptions struct {
	ManifestPath string
	PrivateKey   string
	OutputPath   string
}

func SignRegistryManifest(opts SignOptions) (registry.Manifest, error) {
	if strings.TrimSpace(opts.ManifestPath) == "" {
		return registry.Manifest{}, fmt.Errorf("registry manifest path is required")
	}
	manifest, err := registry.LoadManifest(opts.ManifestPath)
	if err != nil {
		return registry.Manifest{}, err
	}
	privateKey, err := parsePrivateKey(opts.PrivateKey)
	if err != nil {
		return registry.Manifest{}, err
	}
	payload, err := registry.SignedPayload(manifest)
	if err != nil {
		return registry.Manifest{}, err
	}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	outputPath := opts.OutputPath
	if strings.TrimSpace(outputPath) == "" {
		outputPath = opts.ManifestPath
	}
	if err := WriteRegistryManifest(outputPath, manifest); err != nil {
		return registry.Manifest{}, err
	}
	return manifest, nil
}

func WriteRegistryManifest(path string, manifest registry.Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func registryBundle(manifest bundlepkg.Manifest, files []registry.BundleFile) registry.Bundle {
	return registry.Bundle{
		ID:                manifest.ID,
		Version:           manifest.Version,
		Channel:           manifest.Channel,
		Owner:             manifest.Owner,
		Description:       manifest.Description,
		RiskLevel:         manifest.RiskLevel,
		Agents:            append([]string{}, manifest.Agents...),
		Skills:            append([]string{}, manifest.Skills...),
		RequiredPlugins:   append([]string{}, manifest.RequiredPlugins...),
		EvaluationSuite:   append([]string{}, manifest.EvaluationSuite...),
		DeprecationStatus: manifest.DeprecationStatus,
		Files:             files,
	}
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func parsePrivateKey(value string) (ed25519.PrivateKey, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("private key is required")
	}
	if data, err := os.ReadFile(value); err == nil {
		value = strings.TrimSpace(string(data))
	}
	var decoded []byte
	if data, err := base64.StdEncoding.DecodeString(value); err == nil {
		decoded = data
	} else if data, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		decoded = data
	} else if data, err := hex.DecodeString(value); err == nil {
		decoded = data
	} else {
		return nil, fmt.Errorf("private key must be base64, hex, or a path containing one of those encodings")
	}
	switch len(decoded) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	default:
		return nil, fmt.Errorf("invalid private key length: got %d", len(decoded))
	}
}
