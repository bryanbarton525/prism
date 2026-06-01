package rootresolver

import (
	"context"
	"testing"
)

func TestIsRemoteURL(t *testing.T) {
	tests := []struct {
		root string
		want bool
	}{
		{"https://github.com/owner/repo", true},
		{"https://github.com/owner/repo.git", true},
		{"http://github.com/owner/repo", true},
		{"git@github.com:owner/repo.git", true},
		{"ssh://git@github.com/owner/repo.git", true},
		{"/Users/bbarton/go/modules/prism", false},
		{"/absolute/local/path", false},
		{".", false},
		{"..", false},
		{"../prism", false},
		{"relative/path", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.root, func(t *testing.T) {
			got := isRemoteURL(tt.root)
			if got != tt.want {
				t.Errorf("isRemoteURL(%q) = %v, want %v", tt.root, got, tt.want)
			}
		})
	}
}

func TestResolveLocal(t *testing.T) {
	cases := []struct {
		name string
		root string
	}{
		{"absolute path", "/some/local/path"},
		{"dot", "."},
		{"relative", "../prism"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, cleanup, err := Resolve(context.Background(), tc.root)
			if err != nil {
				t.Fatalf("Resolve(%q) unexpected error: %v", tc.root, err)
			}
			defer cleanup()
			if got != tc.root {
				t.Errorf("Resolve(%q) = %q, want %q (local path must be returned unchanged)", tc.root, got, tc.root)
			}
		})
	}
}
