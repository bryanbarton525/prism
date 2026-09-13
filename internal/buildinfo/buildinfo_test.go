package buildinfo

import "testing"

func TestLinkerVersionWins(t *testing.T) {
	old := Version
	Version = "v7.8.9"
	t.Cleanup(func() { Version = old })
	if got := Current().Version; got != "v7.8.9" {
		t.Fatalf("version = %q", got)
	}
}
