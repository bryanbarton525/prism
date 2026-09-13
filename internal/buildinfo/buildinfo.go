// Package buildinfo centralizes release and VCS identity for the CLI and MCP server.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Version is set to the GitHub release tag with -ldflags. Local builds derive
// a useful development identity from Go's embedded VCS metadata.
var Version string

type Info struct {
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"`
	Dirty    bool   `json:"dirty"`
}

func Current() Info {
	out := Info{Version: strings.TrimSpace(Version)}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				out.Revision = setting.Value
			case "vcs.modified":
				out.Dirty = setting.Value == "true"
			}
		}
	}
	if out.Version == "" {
		out.Version = "devel"
		if out.Revision != "" {
			rev := out.Revision
			if len(rev) > 12 {
				rev = rev[:12]
			}
			out.Version += "+" + rev
		}
		if out.Dirty {
			out.Version += ".dirty"
		}
	}
	return out
}
