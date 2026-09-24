package skill

import (
	"errors"
	"fmt"
	"io"
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
	if err := validateSkillIdentity(skillName); err != nil {
		return nil, err
	}
	root := filepath.ToSlash(path.Join(skillName))
	entries := []ResourceEntry{}
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("resource %q is a symlink", p)
		}
		if p == path.Join(root, "SKILL.md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("resource %q is not a regular file", p)
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
	if err := validateSkillIdentity(skillName); err != nil {
		return ReadResourceResult{}, err
	}
	rel, err := normalizeResourcePath(resourcePath)
	if err != nil {
		return ReadResourceResult{}, err
	}
	mediaType := resourceMediaType(rel)
	if !isTextMediaType(mediaType) {
		return ReadResourceResult{}, fmt.Errorf("reading resource %q: %w", rel, ErrUnsupportedTextResource)
	}
	fullPath := filepath.ToSlash(path.Join(skillName, rel))
	if err := rejectSymlinkComponents(fsys, fullPath); err != nil {
		return ReadResourceResult{}, err
	}
	file, err := fsys.Open(fullPath)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("reading resource %q: %w", rel, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("stat resource %q: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return ReadResourceResult{}, fmt.Errorf("reading resource %q: not a regular file", rel)
	}
	size := info.Size()
	if opts.Offset >= size {
		return ReadResourceResult{
			Path: rel, MediaType: mediaType, Size: size, Offset: opts.Offset,
		}, nil
	}
	max := opts.MaxReadBytes
	if opts.Limit > 0 && opts.Limit < max {
		max = opts.Limit
	}
	start := opts.Offset
	if start > 3 {
		start -= 3
	} else {
		start = 0
	}
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			return ReadResourceResult{}, fmt.Errorf("seek resource %q: %w", rel, err)
		}
	} else if _, err := io.CopyN(io.Discard, file, start); err != nil {
		return ReadResourceResult{}, fmt.Errorf("seek resource %q: %w", rel, err)
	}
	prefix := opts.Offset - start
	remaining := size - start
	want := boundedReadLength(remaining, prefix, max)
	data, err := io.ReadAll(io.LimitReader(file, want))
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("reading resource %q: %w", rel, err)
	}
	// A bounded window can start inside a rune that ends before the requested
	// offset. Drop only that partial prefix; the returned range is then aligned
	// against complete UTF-8 characters.
	for start < opts.Offset && len(data) > 0 && isUTF8Continuation(data[0]) {
		data = data[1:]
		start++
		prefix--
	}
	var valid bool
	data, valid = completeUTF8Prefix(data, start+int64(len(data)) < size)
	if !valid {
		return ReadResourceResult{}, fmt.Errorf("reading resource %q: %w", rel, ErrUnsupportedTextResource)
	}
	begin := int(prefix)
	for begin > 0 && begin < len(data) && isUTF8Continuation(data[begin]) {
		begin--
	}
	if begin == len(data) {
		return ReadResourceResult{Path: rel, MediaType: mediaType, Size: size, Offset: opts.Offset}, nil
	}
	end := begin + int(minInt64(max, int64(len(data)-begin)))
	for end > begin && end < len(data) && isUTF8Continuation(data[end]) {
		end--
	}
	actualOffset := start + int64(begin)
	actualEnd := start + int64(end)
	return ReadResourceResult{
		Path:      rel,
		MediaType: mediaType,
		Size:      size,
		Offset:    actualOffset,
		Truncated: actualEnd < size,
		Content:   string(data[begin:end]),
	}, nil
}

func validateSkillIdentity(name string) error {
	if !standardSkillNamePattern.MatchString(name) {
		return fmt.Errorf("skill name %q must match %s", name, standardSkillNamePattern.String())
	}
	return nil
}

func rejectSymlinkComponents(fsys fs.FS, name string) error {
	current := "."
	for _, component := range strings.Split(name, "/") {
		entries, err := fs.ReadDir(fsys, current)
		if err != nil {
			return fmt.Errorf("reading resource path %q: %w", name, err)
		}
		found := false
		for _, entry := range entries {
			if entry.Name() != component {
				continue
			}
			found = true
			if entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("resource path %q contains symlink %q", name, component)
			}
			break
		}
		if !found {
			return fs.ErrNotExist
		}
		current = path.Join(current, component)
	}
	return nil
}

func boundedReadLength(remaining, prefix, limit int64) int64 {
	if remaining <= 0 {
		return 0
	}
	const runeLookahead = int64(3)
	if limit > remaining-prefix-runeLookahead {
		return remaining
	}
	return prefix + limit + runeLookahead
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func isUTF8Continuation(value byte) bool {
	return value&0xc0 == 0x80
}

func completeUTF8Prefix(data []byte, windowIsTruncated bool) ([]byte, bool) {
	for offset := 0; offset < len(data); {
		r, size := utf8.DecodeRune(data[offset:])
		if r == utf8.RuneError && size == 1 {
			if windowIsTruncated && !utf8.FullRune(data[offset:]) {
				return data[:offset], true
			}
			return nil, false
		}
		offset += size
	}
	return data, true
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
