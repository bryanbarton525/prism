// Package prism exposes the immutable agent bundle compiled into the Prism binary.
package prism

import (
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type BundleFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type BundleManifest struct {
	Digest string       `json:"digest"`
	Files  []BundleFile `json:"files"`
}

// embeddedBundle contains every runtime asset needed by Prism. Keeping the
// source files in their normal repository locations makes them easy to review
// while go:embed guarantees that installed binaries are self-contained.
//
//go:embed agents constitutions skills
var embeddedBundle embed.FS

// BundleFS returns the embedded filesystem rooted at the module root.
func BundleFS() fs.FS { return embeddedBundle }

// BundleDigest returns a deterministic SHA-256 over every embedded file. Paths
// and lengths are included so different directory layouts cannot collide.
func BundleDigest() string { return DigestFS(embeddedBundle) }

// Manifest returns the generated inventory of the exact files compiled into
// this binary. It is generated from the immutable embed.FS, so it cannot drift
// from the release artifact it describes.
func Manifest() BundleManifest {
	var paths []string
	_ = fs.WalkDir(embeddedBundle, ".", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			paths = append(paths, path)
		}
		return err
	})
	sort.Strings(paths)
	manifest := BundleManifest{Digest: BundleDigest()}
	for _, path := range paths {
		data, err := fs.ReadFile(embeddedBundle, path)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		manifest.Files = append(manifest.Files, BundleFile{Path: path, SHA256: hex.EncodeToString(sum[:])})
	}
	return manifest
}

// DigestFS calculates the same deterministic identity for a development
// override filesystem.
func DigestFS(fsys fs.FS) string {
	return DigestParts(map[string]fs.FS{"": fsys})
}

// DigestParts deterministically hashes named filesystem trees. Development
// builds use it to identify the exact agent and skill overrides in effect.
func DigestParts(parts map[string]fs.FS) string {
	h := sha256.New()
	type file struct {
		path string
		fsys fs.FS
		rel  string
	}
	var files []file
	for prefix, fsys := range parts {
		_ = fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				files = append(files, file{path: strings.TrimPrefix(filepath.ToSlash(filepath.Join(prefix, path)), "./"), fsys: fsys, rel: path})
			}
			return err
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	for _, item := range files {
		data, err := fs.ReadFile(item.fsys, item.rel)
		if err != nil {
			continue
		}
		writeDigestFrame(h, []byte(item.path))
		writeDigestFrame(h, data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeDigestFrame(h interface{ Write([]byte) (int, error) }, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = h.Write(length[:])
	_, _ = h.Write(value)
}
