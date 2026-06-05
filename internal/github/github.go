// Package github provides an fs.FS implementation backed by the GitHub
// Contents API, plus a URL parser for github.com repository URLs.
// It is used by rootresolver to read agent specs and skills directly from
// a remote repository without requiring a local clone.
package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
)

const apiBase = "https://api.github.com"

// ParseURL parses a github.com repository URL and returns owner, repo, and ref.
// If no branch is specified in the URL, ref is "HEAD".
//
// Supported formats:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo.git
//   - https://github.com/owner/repo/tree/branch-or-tag
//   - git@github.com:owner/repo.git
func ParseURL(rawURL string) (owner, repo, ref string, err error) {
	rawURL = strings.TrimSpace(rawURL)

	// SSH: git@github.com:owner/repo.git
	if strings.HasPrefix(rawURL, "git@github.com:") {
		rest := strings.TrimPrefix(rawURL, "git@github.com:")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", "", fmt.Errorf("github: invalid SSH URL %q", rawURL)
		}
		return parts[0], strings.TrimSuffix(parts[1], ".git"), "HEAD", nil
	}

	// HTTPS
	stripped := rawURL
	stripped = strings.TrimPrefix(stripped, "https://github.com/")
	stripped = strings.TrimPrefix(stripped, "http://github.com/")
	if stripped == rawURL {
		return "", "", "", fmt.Errorf("github: not a github.com URL: %q", rawURL)
	}

	parts := strings.Split(stripped, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("github: URL missing owner/repo: %q", rawURL)
	}
	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")

	// https://github.com/owner/repo/tree/branch[/sub...]
	if len(parts) >= 4 && parts[2] == "tree" {
		ref = strings.Join(parts[3:], "/")
	} else {
		ref = "HEAD"
	}
	return owner, repo, ref, nil
}

// IsURL reports whether rawURL is a recognisable github.com repository URL.
func IsURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "https://github.com/") ||
		strings.HasPrefix(rawURL, "http://github.com/") ||
		strings.HasPrefix(rawURL, "git@github.com:")
}

// contentEntry is the JSON shape returned by the GitHub Contents API for a
// single file or one element in a directory listing.
type contentEntry struct {
	Type     string `json:"type"` // "file" or "dir"
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Content  string `json:"content"`  // base64-encoded; present for single-file fetches
	Encoding string `json:"encoding"` // "base64"
}

// FS implements fs.FS using the GitHub Contents API.
// Each FS instance maintains an in-process response cache so that repeated
// reads of the same path (e.g. listing agents/, then opening each file) are
// served without additional API calls.
//
// Concurrency: safe for concurrent use.
type FS struct {
	owner  string
	repo   string
	ref    string
	token  string       // empty = unauthenticated (public repos, rate-limited)
	client *http.Client // injectable for testing
	cache  sync.Map     // key -> []byte (raw JSON response)
}

// New creates a FS for the given repository.
// token may be empty for public repos (unauthenticated limit: 60 req/hr).
// ref may be empty or "HEAD" to use the default branch.
func New(owner, repo, ref, token string) *FS {
	if ref == "" {
		ref = "HEAD"
	}
	return &FS{
		owner:  owner,
		repo:   repo,
		ref:    ref,
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Open implements fs.FS.
func (f *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		entries, err := f.listDir("")
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return &ghDir{info: &ghFileInfo{name: ".", isDir: true}, entries: entries}, nil
	}

	raw, err := f.apiGet(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	// Try to decode as a single-file entry.
	var single contentEntry
	if json.Unmarshal(raw, &single) == nil && single.Type == "file" {
		data, err := decodeContent(single)
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return &ghFile{
			info: &ghFileInfo{name: pathBase(name), size: int64(len(data))},
			data: data,
		}, nil
	}

	// Try as a directory listing.
	var arr []contentEntry
	if json.Unmarshal(raw, &arr) == nil {
		entries := dirEntries(arr)
		return &ghDir{
			info:    &ghFileInfo{name: pathBase(name), isDir: true},
			entries: entries,
		}, nil
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("unexpected GitHub API response")}
}

// listDir fetches and decodes the directory listing for apiPath (empty = repo root).
func (f *FS) listDir(apiPath string) ([]fs.DirEntry, error) {
	raw, err := f.apiGet(apiPath)
	if err != nil {
		return nil, err
	}
	var arr []contentEntry
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("listing directory: %w", err)
	}
	return dirEntries(arr), nil
}

// dirEntries converts a slice of contentEntry to []fs.DirEntry.
func dirEntries(arr []contentEntry) []fs.DirEntry {
	out := make([]fs.DirEntry, 0, len(arr))
	for _, e := range arr {
		out = append(out, &ghDirEntry{name: e.Name, isDir: e.Type == "dir", size: e.Size})
	}
	return out
}

// decodeContent base64-decodes the Content field of a file entry.
func decodeContent(e contentEntry) ([]byte, error) {
	if e.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported encoding %q", e.Encoding)
	}
	// GitHub includes newlines inside the base64 block.
	cleaned := strings.ReplaceAll(e.Content, "\n", "")
	return base64.StdEncoding.DecodeString(cleaned)
}

// apiGet fetches the raw JSON from the GitHub Contents API for path and caches
// the response. path is relative to the repo root (empty = root listing).
func (f *FS) apiGet(path string) ([]byte, error) {
	cacheKey := "api:" + path
	if v, ok := f.cache.Load(cacheKey); ok {
		return v.([]byte), nil
	}

	url := f.contentsURL(path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	f.cache.Store(cacheKey, body)
	return body, nil
}

// contentsURL builds the GitHub Contents API URL for the given repo-relative path.
func (f *FS) contentsURL(path string) string {
	base := fmt.Sprintf("%s/repos/%s/%s/contents", apiBase, f.owner, f.repo)
	if path != "" && path != "." {
		base += "/" + path
	}
	if f.ref != "" && f.ref != "HEAD" {
		base += "?ref=" + f.ref
	}
	return base
}

// pathBase returns the final path segment (equivalent to filepath.Base but
// only for forward-slash separated fs.FS paths).
func pathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// ── fs.FileInfo ───────────────────────────────────────────────────────────────

type ghFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi *ghFileInfo) Name() string { return fi.name }
func (fi *ghFileInfo) Size() int64  { return fi.size }
func (fi *ghFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (fi *ghFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *ghFileInfo) IsDir() bool        { return fi.isDir }
func (fi *ghFileInfo) Sys() any           { return nil }

// ── fs.File (regular file) ────────────────────────────────────────────────────

type ghFile struct {
	info   *ghFileInfo
	data   []byte
	offset int
}

func (f *ghFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *ghFile) Close() error               { return nil }
func (f *ghFile) Read(b []byte) (int, error) {
	if f.offset >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(b, f.data[f.offset:])
	f.offset += n
	return n, nil
}

// ── fs.ReadDirFile (directory) ─────────────────────────────────────────────────

type ghDir struct {
	info    *ghFileInfo
	entries []fs.DirEntry
	offset  int
}

func (d *ghDir) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *ghDir) Close() error               { return nil }
func (d *ghDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: fmt.Errorf("is a directory")}
}
func (d *ghDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		all := d.entries[d.offset:]
		d.offset = len(d.entries)
		return all, nil
	}
	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}
	end := d.offset + n
	if end > len(d.entries) {
		end = len(d.entries)
	}
	result := d.entries[d.offset:end]
	d.offset = end
	return result, nil
}

// ── fs.DirEntry ───────────────────────────────────────────────────────────────

type ghDirEntry struct {
	name  string
	isDir bool
	size  int64
}

func (e *ghDirEntry) Name() string { return e.name }
func (e *ghDirEntry) IsDir() bool  { return e.isDir }
func (e *ghDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e *ghDirEntry) Info() (fs.FileInfo, error) {
	return &ghFileInfo{name: e.name, size: e.size, isDir: e.isDir}, nil
}
