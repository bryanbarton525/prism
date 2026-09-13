package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrUnsupportedTextResource = errors.New("resource is not valid UTF-8 text")

type ResourceEntry struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	Binary    bool   `json:"binary"`
}

type ReadResourceOptions struct {
	Offset       int64
	Limit        int64
	MaxReadBytes int64
}

type ReadResourceResult struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	Offset    int64  `json:"offset"`
	Truncated bool   `json:"truncated"`
	Content   string `json:"content"`
}

// ListResources lists files under a skill directory, excluding SKILL.md.
func ListResources(fsys fs.FS, skillName string) ([]ResourceEntry, error) {
	root := filepath.ToSlash(path.Join(skillName))
	entries := []ResourceEntry{}
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(path.Base(p), "SKILL.md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return fmt.Errorf("resource %q escapes skill root", p)
		}
		mediaType := resourceMediaType(rel)
		entries = append(entries, ResourceEntry{
			Path:      rel,
			MediaType: mediaType,
			Size:      info.Size(),
			Binary:    !isTextMediaType(mediaType),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing resources for skill %q: %w", skillName, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// ReadResource reads a bounded UTF-8 text slice of a skill resource.
func ReadResource(fsys fs.FS, skillName, resourcePath string, opts ReadResourceOptions) (ReadResourceResult, error) {
	if opts.MaxReadBytes <= 0 {
		opts.MaxReadBytes = 32 * 1024
	}
	if opts.Offset < 0 {
		return ReadResourceResult{}, fmt.Errorf("offset must be >= 0")
	}
	if opts.Limit < 0 {
		return ReadResourceResult{}, fmt.Errorf("limit must be >= 0")
	}
	rel, err := normalizeResourcePath(resourcePath)
	if err != nil {
		return ReadResourceResult{}, err
	}
	fullPath := filepath.ToSlash(path.Join(skillName, rel))
	data, err := fs.ReadFile(fsys, fullPath)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("reading resource %q: %w", rel, err)
	}
	if !utf8.Valid(data) {
		return ReadResourceResult{}, fmt.Errorf("reading resource %q: %w", rel, ErrUnsupportedTextResource)
	}
	size := int64(len(data))
	if opts.Offset >= size {
		return ReadResourceResult{
			Path:      rel,
			MediaType: resourceMediaType(rel),
			Size:      size,
			Offset:    opts.Offset,
			Truncated: false,
			Content:   "",
		}, nil
	}
	max := opts.MaxReadBytes
	if opts.Limit > 0 && opts.Limit < max {
		max = opts.Limit
	}
	end := opts.Offset + max
	if end > size {
		end = size
	}
	return ReadResourceResult{
		Path:      rel,
		MediaType: resourceMediaType(rel),
		Size:      size,
		Offset:    opts.Offset,
		Truncated: end < size,
		Content:   string(data[opts.Offset:end]),
	}, nil
}

func normalizeResourcePath(p string) (string, error) {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" {
		return "", fmt.Errorf("resource path is required")
	}
	clean := path.Clean(p)
	if path.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("resource path %q escapes skill root", p)
	}
	return clean, nil
}

func resourceMediaType(resourcePath string) string {
	ext := strings.ToLower(path.Ext(resourcePath))
	if ext == "" {
		return "application/octet-stream"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	switch ext {
	case ".md":
		return "text/markdown"
	case ".txt", ".log":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	default:
		return "application/octet-stream"
	}
}

func isTextMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "text/") ||
		strings.Contains(mediaType, "json") ||
		strings.Contains(mediaType, "yaml") ||
		strings.Contains(mediaType, "xml")
}
