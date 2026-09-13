package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	internalgithub "github.com/bryanbarton525/prism/internal/github"
	"github.com/bryanbarton525/prism/internal/rootresolver"
)

type Bounds struct {
	MaxFiles int
	MaxBytes int64
}

type Result struct {
	FS               fs.FS
	Cleanup          func()
	CanonicalSource  string
	RequestedRef     string
	ResolvedRevision string
	Digest           string
	FileCount        int
	TotalBytes       int64
}

func Resolve(ctx context.Context, source, token string, bounds Bounds) (Result, error) {
	fsys, cleanup, err := rootresolver.Resolve(ctx, source, token)
	if err != nil {
		return Result{}, err
	}
	if cleanup == nil {
		cleanup = func() {}
	}
	canonical, requestedRef, resolvedRevision := describeSource(source)
	digest, files, total, err := digestFS(fsys, bounds)
	if err != nil {
		cleanup()
		return Result{}, err
	}
	return Result{
		FS:               fsys,
		Cleanup:          cleanup,
		CanonicalSource:  canonical,
		RequestedRef:     requestedRef,
		ResolvedRevision: resolvedRevision,
		Digest:           digest,
		FileCount:        files,
		TotalBytes:       total,
	}, nil
}

func describeSource(source string) (canonical string, requestedRef string, resolvedRevision string) {
	if internalgithub.IsURL(source) {
		owner, repo, ref, err := internalgithub.ParseURL(source)
		if err == nil {
			if ref == "" {
				ref = "HEAD"
			}
			return fmt.Sprintf("github://%s/%s", owner, repo), ref, ref
		}
	}
	abs, err := filepath.Abs(source)
	if err == nil {
		source = abs
	}
	return source, "", ""
}

func digestFS(fsys fs.FS, bounds Bounds) (string, int, int64, error) {
	if bounds.MaxFiles <= 0 {
		bounds.MaxFiles = 2000
	}
	if bounds.MaxBytes <= 0 {
		bounds.MaxBytes = 20 * 1024 * 1024
	}
	files := []string{}
	total := int64(0)
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(path))
		if len(files) > bounds.MaxFiles {
			return fmt.Errorf("resolver bounds exceeded: file count %d > %d", len(files), bounds.MaxFiles)
		}
		total += info.Size()
		if total > bounds.MaxBytes {
			return fmt.Errorf("resolver bounds exceeded: bytes %d > %d", total, bounds.MaxBytes)
		}
		return nil
	}); err != nil {
		return "", 0, 0, err
	}
	sort.Strings(files)
	hash := sha256.New()
	for _, path := range files {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return "", 0, 0, err
		}
		_, _ = hash.Write([]byte(strings.TrimSpace(path)))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(data)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), len(files), total, nil
}
