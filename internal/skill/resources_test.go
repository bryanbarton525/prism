package skill

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestListResourcesExcludesSkillMarkdownAndSorts(t *testing.T) {
	fsys := fstest.MapFS{
		"demo/SKILL.md":                    &fstest.MapFile{Data: []byte("---\nname: demo\ndescription: d\n---")},
		"demo/references/REFERENCE.md":     &fstest.MapFile{Data: []byte("# ref")},
		"demo/references/runbook.txt":      &fstest.MapFile{Data: []byte("ok")},
		"demo/references/diagram.bin":      &fstest.MapFile{Data: []byte{0x00, 0x01}},
		"demo/references/nested/info.json": &fstest.MapFile{Data: []byte(`{"x":1}`)},
	}
	got, err := ListResources(fsys, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("resources = %#v", got)
	}
	if got[0].Path != "references/REFERENCE.md" || got[3].Path != "references/runbook.txt" {
		t.Fatalf("unexpected order: %#v", got)
	}
}

func TestReadResourceBoundsAndTraversalProtection(t *testing.T) {
	fsys := fstest.MapFS{
		"demo/references/REFERENCE.md": &fstest.MapFile{Data: []byte("hello world")},
	}
	res, err := ReadResource(fsys, "demo", "references/REFERENCE.md", ReadResourceOptions{
		Offset:       6,
		Limit:        5,
		MaxReadBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "world" || res.Truncated {
		t.Fatalf("unexpected read result: %#v", res)
	}
	if _, err := ReadResource(fsys, "demo", "../outside.txt", ReadResourceOptions{}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestReadResourceRejectsBinaryContent(t *testing.T) {
	fsys := fstest.MapFS{
		"demo/references/blob.bin": &fstest.MapFile{Data: []byte{0xff, 0xfe, 0xfd}},
	}
	_, err := ReadResource(fsys, "demo", "references/blob.bin", ReadResourceOptions{})
	if !errors.Is(err, ErrUnsupportedTextResource) {
		t.Fatalf("err = %v, want ErrUnsupportedTextResource", err)
	}
}

func TestTOMLResourceIsReadableText(t *testing.T) {
	const content = "[project]\nname = \"graphify\"\n"
	fys := fstest.MapFS{
		"demo/references/managed-environment/pyproject.toml": &fstest.MapFile{Data: []byte(content)},
	}
	resources, err := ListResources(fys, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].MediaType != "application/toml" || resources[0].Binary {
		t.Fatalf("unexpected TOML resource entry: %#v", resources)
	}
	result, err := ReadResource(fys, "demo", "references/managed-environment/pyproject.toml", ReadResourceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != content || result.MediaType != "application/toml" {
		t.Fatalf("unexpected TOML content: %#v", result)
	}
}
