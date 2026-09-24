package skill

import (
	"bytes"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"unicode/utf8"
)

type countingResourceFS struct {
	fs.FS
	bytesRead int64
}

func (c *countingResourceFS) Open(name string) (fs.File, error) {
	file, err := c.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &countingResourceFile{File: file, owner: c}, nil
}

func (c *countingResourceFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(c.FS, name)
}

type countingResourceFile struct {
	fs.File
	owner *countingResourceFS
}

func (c *countingResourceFile) Read(p []byte) (int, error) {
	n, err := c.File.Read(p)
	c.owner.bytesRead += int64(n)
	return n, err
}

func TestReviewResourceBoundsMediaAndSymlinkSafety(t *testing.T) {
	fsys := fstest.MapFS{
		"demo/ref.txt":  {Data: []byte("a€b")},
		"demo/data.bin": {Data: []byte("plain ASCII")},
	}
	res, err := ReadResource(fsys, "demo", "ref.txt", ReadResourceOptions{Offset: 2, Limit: 3, MaxReadBytes: math.MaxInt64})
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(res.Content) || res.Content != "€" || res.Offset != 1 {
		t.Fatalf("rune-aligned result = %#v", res)
	}
	if _, err := ReadResource(fsys, "demo", "data.bin", ReadResourceOptions{}); !errors.Is(err, ErrUnsupportedTextResource) {
		t.Fatalf("binary media error = %v", err)
	}
	if _, err := ReadResource(fsys, ".", "ref.txt", ReadResourceOptions{}); err == nil {
		t.Fatal("unsafe skill identity was accepted")
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "demo", "link.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadResource(os.DirFS(root), "demo", "link.txt", ReadResourceOptions{}); err == nil {
		t.Fatal("resource symlink was followed")
	}
}

func TestReviewNestedSkillMetadataRemainsAResource(t *testing.T) {
	fsys := fstest.MapFS{
		"demo/SKILL.md":            {Data: []byte("root")},
		"demo/references/SKILL.md": {Data: []byte("nested")},
	}
	entries, err := ListResources(fsys, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "references/SKILL.md" {
		t.Fatalf("resources = %#v", entries)
	}
}

func TestReviewResourceReadDoesNotLoadWholeFile(t *testing.T) {
	content := bytes.Repeat([]byte("x"), 1<<20)
	fsys := &countingResourceFS{FS: fstest.MapFS{"demo/large.txt": {Data: content}}}
	result, err := ReadResource(fsys, "demo", "large.txt", ReadResourceOptions{MaxReadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1024 || !result.Truncated || result.Size != int64(len(content)) {
		t.Fatalf("bounded result = %#v", result)
	}
	if fsys.bytesRead > 1027 {
		t.Fatalf("read %d bytes for a 1024-byte request", fsys.bytesRead)
	}
}

func TestReviewResourceUTF8BoundaryAfterMultibyteRune(t *testing.T) {
	fsys := fstest.MapFS{"demo/ref.txt": {Data: []byte("abc€xyz")}}
	result, err := ReadResource(fsys, "demo", "ref.txt", ReadResourceOptions{Offset: 7, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "y" || result.Offset != 7 || !utf8.ValidString(result.Content) {
		t.Fatalf("boundary result = %#v", result)
	}
}

func TestReviewResourceAllUTF8OffsetsRemainValid(t *testing.T) {
	content := "a€b😀c"
	fsys := fstest.MapFS{"demo/ref.txt": {Data: []byte(content)}}
	for offset := int64(0); offset < int64(len(content)); offset++ {
		for _, limit := range []int64{1, 2, 3, 4, 8} {
			result, err := ReadResource(fsys, "demo", "ref.txt", ReadResourceOptions{Offset: offset, Limit: limit})
			if err != nil {
				t.Fatalf("offset %d limit %d: %v", offset, limit, err)
			}
			if !utf8.ValidString(result.Content) || int64(len(result.Content)) > limit {
				t.Fatalf("offset %d limit %d: %#v", offset, limit, result)
			}
		}
	}
}

func TestReviewResourceRejectsMalformedUTF8Text(t *testing.T) {
	fsys := fstest.MapFS{"demo/ref.txt": {Data: []byte{'a', 0xff, 'b'}}}
	if _, err := ReadResource(fsys, "demo", "ref.txt", ReadResourceOptions{}); !errors.Is(err, ErrUnsupportedTextResource) {
		t.Fatalf("malformed UTF-8 error = %v", err)
	}
}

func TestReviewResourceSkillIdentityCannotEscapeOrAliasRoot(t *testing.T) {
	fsys := fstest.MapFS{"demo/ref.txt": {Data: []byte("ok")}, "other/ref.txt": {Data: []byte("secret")}}
	for _, name := range []string{"demo/../other", ".", " demo "} {
		if _, err := ListResources(fsys, name); err == nil {
			t.Fatalf("listed unsafe skill identity %q", name)
		}
		if _, err := ReadResource(fsys, name, "ref.txt", ReadResourceOptions{}); err == nil {
			t.Fatalf("read unsafe skill identity %q", name)
		}
	}
}
