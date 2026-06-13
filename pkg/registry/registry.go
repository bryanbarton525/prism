// Package registry verifies and installs signed Prism agent/skill registries.
package registry

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
)

// Manifest is the signed registry document distributed to Prism teams.
type Manifest struct {
	RegistryID  string    `json:"registry_id" yaml:"registry_id"`
	Version     string    `json:"version" yaml:"version"`
	GeneratedAt time.Time `json:"generated_at" yaml:"generated_at"`
	Compat      Compat    `json:"compat" yaml:"compat"`
	Bundles     []Bundle  `json:"bundles" yaml:"bundles"`
	Signature   string    `json:"signature" yaml:"signature"`
}

// Compat describes which Prism versions can consume this registry.
type Compat struct {
	MinPrismVersion string `json:"min_prism_version,omitempty" yaml:"min_prism_version,omitempty"`
	MaxPrismVersion string `json:"max_prism_version,omitempty" yaml:"max_prism_version,omitempty"`
}

// Bundle is one installable group of agent and skill files.
type Bundle struct {
	ID                string       `json:"id" yaml:"id"`
	Version           string       `json:"version" yaml:"version"`
	Channel           string       `json:"channel,omitempty" yaml:"channel,omitempty"`
	Owner             string       `json:"owner,omitempty" yaml:"owner,omitempty"`
	Description       string       `json:"description,omitempty" yaml:"description,omitempty"`
	RiskLevel         string       `json:"risk_level,omitempty" yaml:"risk_level,omitempty"`
	Agents            []string     `json:"agents,omitempty" yaml:"agents,omitempty"`
	Skills            []string     `json:"skills,omitempty" yaml:"skills,omitempty"`
	RequiredPlugins   []string     `json:"required_plugins,omitempty" yaml:"required_plugins,omitempty"`
	EvaluationSuite   []string     `json:"evaluation_suite,omitempty" yaml:"evaluation_suite,omitempty"`
	DeprecationStatus string       `json:"deprecation_status,omitempty" yaml:"deprecation_status,omitempty"`
	Files             []BundleFile `json:"files" yaml:"files"`
}

// BundleFile is one file in a managed registry bundle.
type BundleFile struct {
	Kind   string `json:"kind" yaml:"kind"` // agent | skill | other
	Path   string `json:"path" yaml:"path"`
	SHA256 string `json:"sha256" yaml:"sha256"`
}

// LoadManifest reads a JSON manifest from path.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// SignedPayload returns the canonical bytes covered by the Ed25519 signature.
func SignedPayload(manifest Manifest) ([]byte, error) {
	manifest.Signature = ""
	return json.Marshal(manifest)
}

// VerifySignature verifies manifest.Signature against the canonical payload.
func VerifySignature(manifest Manifest, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length: got %d", len(publicKey))
	}
	sig, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}
	payload, err := SignedPayload(manifest)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, sig) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// VerifyCompat checks whether prismVersion satisfies the manifest compatibility
// bounds. Versions are compared segment-by-segment after trimming an optional
// leading "v".
func VerifyCompat(prismVersion string, compat Compat) error {
	if compat.MinPrismVersion != "" {
		cmp, err := compareVersions(prismVersion, compat.MinPrismVersion)
		if err != nil {
			return fmt.Errorf("comparing min_prism_version: %w", err)
		}
		if cmp < 0 {
			return fmt.Errorf("prism version %q is below min_prism_version %q", prismVersion, compat.MinPrismVersion)
		}
	}
	if compat.MaxPrismVersion != "" {
		cmp, err := compareVersions(prismVersion, compat.MaxPrismVersion)
		if err != nil {
			return fmt.Errorf("comparing max_prism_version: %w", err)
		}
		if cmp > 0 {
			return fmt.Errorf("prism version %q is above max_prism_version %q", prismVersion, compat.MaxPrismVersion)
		}
	}
	return nil
}

// VerifyManifest performs the full verification flow for a registry manifest:
// signature, Prism version compatibility, and file digests.
func VerifyManifest(sourceRoot string, manifest Manifest, publicKey ed25519.PublicKey, prismVersion string) error {
	if err := VerifySignature(manifest, publicKey); err != nil {
		return err
	}
	if err := VerifyCompat(prismVersion, manifest.Compat); err != nil {
		return err
	}
	if err := VerifyFiles(sourceRoot, manifest); err != nil {
		return err
	}
	return nil
}

// VerifyFiles checks every manifest file exists under sourceRoot and matches
// its SHA-256 digest.
func VerifyFiles(sourceRoot string, manifest Manifest) error {
	for _, bundle := range manifest.Bundles {
		for _, file := range bundle.Files {
			src, err := safeJoin(sourceRoot, file.Path)
			if err != nil {
				return fmt.Errorf("%s: %w", file.Path, err)
			}
			sum, err := fileSHA256(src)
			if err != nil {
				return fmt.Errorf("%s: %w", file.Path, err)
			}
			if !strings.EqualFold(sum, file.SHA256) {
				return fmt.Errorf("%s: checksum mismatch: got %s want %s", file.Path, sum, file.SHA256)
			}
		}
	}
	return nil
}

// Install performs the full verification flow and then copies registry files
// from sourceRoot into destRoot.
func Install(sourceRoot, destRoot string, manifest Manifest, publicKey ed25519.PublicKey, prismVersion string) error {
	if err := VerifyManifest(sourceRoot, manifest, publicKey, prismVersion); err != nil {
		return err
	}
	for _, bundle := range manifest.Bundles {
		for _, file := range bundle.Files {
			src, err := safeJoin(sourceRoot, file.Path)
			if err != nil {
				return fmt.Errorf("%s: %w", file.Path, err)
			}
			dst, err := safeJoin(destRoot, file.Path)
			if err != nil {
				return fmt.Errorf("%s: %w", file.Path, err)
			}
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("installing %s: %w", file.Path, err)
			}
		}
	}
	return nil
}

func compareVersions(a, b string) (int, error) {
	aParts, err := parseVersion(a)
	if err != nil {
		return 0, fmt.Errorf("%q: %w", a, err)
	}
	bParts, err := parseVersion(b)
	if err != nil {
		return 0, fmt.Errorf("%q: %w", b, err)
	}
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		switch {
		case av < bv:
			return -1, nil
		case av > bv:
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(v string) ([]int, error) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil, fmt.Errorf("empty version")
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("invalid empty version segment")
		}
		var value int
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, fmt.Errorf("non-numeric version segment %q", part)
			}
			value = value*10 + int(r-'0')
		}
		out = append(out, value)
	}
	return out, nil
}

func fileSHA256(path string) (string, error) {
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

func safeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("path escapes registry root")
	}
	return filepath.Join(root, clean), nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
