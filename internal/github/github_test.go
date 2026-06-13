package github

import (
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── ParseURL ─────────────────────────────────────────────────────────────────

func TestParseURL(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
		wantRef   string
		wantErr   bool
	}{
		{
			url:       "https://github.com/owner/repo",
			wantOwner: "owner", wantRepo: "repo", wantRef: "HEAD",
		},
		{
			url:       "https://github.com/owner/repo.git",
			wantOwner: "owner", wantRepo: "repo", wantRef: "HEAD",
		},
		{
			url:       "https://github.com/owner/repo/tree/main",
			wantOwner: "owner", wantRepo: "repo", wantRef: "main",
		},
		{
			url:       "https://github.com/owner/repo/tree/feat/my-feature",
			wantOwner: "owner", wantRepo: "repo", wantRef: "feat/my-feature",
		},
		{
			url:       "http://github.com/owner/repo",
			wantOwner: "owner", wantRepo: "repo", wantRef: "HEAD",
		},
		{
			url:       "git@github.com:owner/repo.git",
			wantOwner: "owner", wantRepo: "repo", wantRef: "HEAD",
		},
		{
			url:     "https://gitlab.com/owner/repo",
			wantErr: true,
		},
		{
			url:     "/local/path",
			wantErr: true,
		},
		{
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			owner, repo, ref, err := ParseURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseURL(%q): expected error, got owner=%q repo=%q ref=%q", tt.url, owner, repo, ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseURL(%q): unexpected error: %v", tt.url, err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner: want %q, got %q", tt.wantOwner, owner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo: want %q, got %q", tt.wantRepo, repo)
			}
			if ref != tt.wantRef {
				t.Errorf("ref: want %q, got %q", tt.wantRef, ref)
			}
		})
	}
}

// ── IsURL ─────────────────────────────────────────────────────────────────────

func TestIsURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://github.com/owner/repo", true},
		{"http://github.com/owner/repo", true},
		{"git@github.com:owner/repo.git", true},
		{"https://gitlab.com/owner/repo", false},
		{"/local/path", false},
		{".", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := IsURL(tt.url); got != tt.want {
				t.Errorf("IsURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// ── FS (mock server) ──────────────────────────────────────────────────────────

// mockGitHub starts a fake GitHub Contents API server.
// files maps API paths (e.g. "agents/foo.md") to file contents.
// dirs maps API paths (e.g. "agents") to slices of entry names.
func mockGitHub(t *testing.T, files map[string]string, dirs map[string][]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /repos/owner/repo/contents/ prefix.
		const prefix = "/repos/owner/repo/contents"
		path := r.URL.Path
		if len(path) > len(prefix) {
			path = path[len(prefix)+1:] // strip leading slash too
		} else {
			path = ""
		}

		if content, ok := files[path]; ok {
			encoded := base64.StdEncoding.EncodeToString([]byte(content))
			entry := contentEntry{
				Type:     "file",
				Name:     pathBase(path),
				Path:     path,
				Size:     int64(len(content)),
				Content:  encoded,
				Encoding: "base64",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(entry)
			return
		}

		if names, ok := dirs[path]; ok {
			var entries []contentEntry
			for _, name := range names {
				subPath := path
				if subPath != "" {
					subPath += "/"
				}
				subPath += name
				typ := "file"
				if _, isDir := dirs[subPath]; isDir {
					typ = "dir"
				}
				entries = append(entries, contentEntry{
					Type: typ,
					Name: name,
					Path: subPath,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(entries)
			return
		}

		http.NotFound(w, r)
	}))
}

// newTestFS creates a github.FS pointed at the mock server.
func newTestFS(t *testing.T, srv *httptest.Server) *FS {
	t.Helper()
	f := New("owner", "repo", "HEAD", "")
	f.client = &http.Client{
		Transport: &rewriteTransport{base: srv.URL},
	}
	return f
}

// rewriteTransport rewrites every outbound request to hit the mock server
// instead of api.github.com.
type rewriteTransport struct {
	base string // e.g. "http://127.0.0.1:PORT"
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := rt.base + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return http.DefaultTransport.RoundTrip(newReq)
}

func TestFS_OpenFile(t *testing.T) {
	srv := mockGitHub(t,
		map[string]string{
			"agents/github-cli.md": "---\nid: github-cli\n---\nbody text",
		},
		map[string][]string{
			"agents": {"github-cli.md"},
		},
	)
	defer srv.Close()

	f := newTestFS(t, srv)

	file, err := f.Open("agents/github-cli.md")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Name() != "github-cli.md" {
		t.Errorf("Name: want github-cli.md, got %q", fi.Name())
	}
	if fi.IsDir() {
		t.Error("expected file, got dir")
	}

	data, err := fs.ReadFile(f, "agents/github-cli.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "---\nid: github-cli\n---\nbody text" {
		t.Errorf("content mismatch: %q", data)
	}
}

func TestFS_OpenDir(t *testing.T) {
	srv := mockGitHub(t,
		map[string]string{
			"agents/github-cli.md": "content",
			"agents/kubectl.md":    "content2",
		},
		map[string][]string{
			"agents": {"github-cli.md", "kubectl.md"},
		},
	)
	defer srv.Close()

	f := newTestFS(t, srv)

	entries, err := fs.ReadDir(f, "agents")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("want 2 entries, got %d: %v", len(entries), entries)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !names["github-cli.md"] || !names["kubectl.md"] {
		t.Errorf("unexpected entries: %v", names)
	}
}

func TestFS_NotFound(t *testing.T) {
	srv := mockGitHub(t, nil, nil)
	defer srv.Close()

	f := newTestFS(t, srv)
	_, err := f.Open("nonexistent/path.md")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}
