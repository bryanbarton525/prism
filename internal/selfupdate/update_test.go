package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func archiveFixture(t *testing.T, name string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUpdate(t *testing.T) {
	for _, tc := range []struct {
		name, current, version, entry, wantError string
		check, corrupt, updated                  bool
		requests                                 int
		fault                                    string
	}{
		{name: "symlink update", current: "v0.1.27", updated: true, requests: 2},
		{name: "check only", current: "v0.1.27", check: true, requests: 1},
		{name: "current", current: "v0.1.28", requests: 1},
		{name: "no downgrade", current: "v0.1.100", requests: 1},
		{name: "rollback", current: "v0.1.100", version: "v0.1.28", updated: true, requests: 2},
		{name: "development", current: "devel", wantError: "development build", requests: 1},
		{name: "explicit development", current: "devel", version: "v0.1.28", updated: true, requests: 2},
		{name: "checksum mismatch", current: "v0.1.27", corrupt: true, wantError: "checksum mismatch", requests: 2},
		{name: "traversal", current: "v0.1.27", entry: "../prism", wantError: "unexpected archive entry", requests: 2},
		{name: "invalid tag", version: "../bad", wantError: "stable release tag"},
		{name: "missing digest", current: "v0.1.27", fault: "digest", wantError: "no valid SHA-256", requests: 1},
		{name: "untrusted URL", current: "v0.1.27", fault: "url", wantError: "unexpected release download URL", requests: 1},
		{name: "missing platform", current: "v0.1.27", fault: "asset", wantError: "no asset", requests: 1},
		{name: "failed download", current: "v0.1.27", fault: "download", wantError: "HTTP 503", requests: 2},
		{name: "failed release lookup", current: "v0.1.27", fault: "lookup", wantError: "HTTP 403", requests: 1},
		{name: "prerelease", current: "v0.1.27", fault: "prerelease", wantError: "stable release", requests: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "binary")
			link := filepath.Join(dir, "prism")
			if err := os.WriteFile(dest, []byte("old"), 0751); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(dest, link); err != nil {
				t.Fatal(err)
			}
			entry := tc.entry
			if entry == "" {
				entry = "prism"
			}
			data := archiveFixture(t, entry)
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
			if tc.corrupt {
				digest = "sha256:" + strings.Repeat("0", 64)
			}
			name := "prism_v0.1.28_darwin_arm64.tar.gz"
			rel := release{Tag: "v0.1.28", Assets: []asset{{Name: name, URL: "https://github.com/" + repository + "/releases/download/v0.1.28/" + name, Digest: digest}}}
			switch tc.fault {
			case "digest":
				rel.Assets[0].Digest = ""
			case "url":
				rel.Assets[0].URL = "https://example.com/prism.tar.gz"
			case "asset":
				rel.Assets = nil
			case "prerelease":
				rel.Prerelease = true
			}
			metadata, err := json.Marshal(rel)
			if err != nil {
				t.Fatal(err)
			}
			u := New()
			u.goos = "darwin"
			u.goarch = "arm64"
			requests := 0
			u.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				body := metadata
				status := http.StatusOK
				if tc.fault == "lookup" {
					status = http.StatusForbidden
				}
				if r.URL.Host == "github.com" {
					body = data
					if tc.fault == "download" {
						status = http.StatusServiceUnavailable
					}
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
			})}
			out, err := u.Run(context.Background(), link, tc.current, tc.version, tc.check)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error %v; want %s", err, tc.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if out.Updated != tc.updated {
				t.Fatalf("updated=%v", out.Updated)
			}
			if requests != tc.requests {
				t.Fatalf("requests=%d; want %d", requests, tc.requests)
			}
			content, err := os.ReadFile(dest)
			if err != nil {
				t.Fatal(err)
			}
			want := "old"
			if tc.updated {
				want = "new"
			}
			if string(content) != want {
				t.Fatalf("content=%q", content)
			}
			info, err := os.Stat(dest)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0751 {
				t.Fatal("permissions changed")
			}
			if _, err := os.Readlink(link); err != nil {
				t.Fatal("symlink changed", err)
			}
			staging, err := filepath.Glob(filepath.Join(dir, ".prism-update-*"))
			if err != nil || len(staging) != 0 {
				t.Fatal("staging left", staging, err)
			}
		})
	}
}

func TestRejectArchives(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []tar.Header
	}{
		{name: "empty"},
		{name: "duplicate", entries: []tar.Header{{Name: "prism", Typeflag: tar.TypeReg, Size: 1}, {Name: "prism", Typeflag: tar.TypeReg, Size: 1}}},
		{name: "symlink", entries: []tar.Header{{Name: "prism", Typeflag: tar.TypeSymlink, Linkname: "elsewhere"}}},
		{name: "oversize", entries: []tar.Header{{Name: "prism", Typeflag: tar.TypeReg, Size: maxBinary + 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			for _, h := range tc.entries {
				if err := tw.WriteHeader(&h); err != nil {
					t.Fatal(err)
				}
				if h.Size == 1 {
					if _, err := tw.Write([]byte("x")); err != nil {
						t.Fatal(err)
					}
				}
			}
			tw.Close()
			gz.Close()
			if err := extractBinary(&buf, io.Discard); err == nil {
				t.Fatal("malformed archive accepted")
			}
		})
	}
	if err := extractBinary(strings.NewReader("bad gzip"), io.Discard); err == nil {
		t.Fatal("invalid gzip accepted")
	}
}
