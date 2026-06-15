package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
)

func configuredDownstreamMCPState() (downstreammcp.State, error) {
	state, err := downstreammcp.Load(mcpServersPath())
	if err != nil {
		return downstreammcp.State{}, err
	}
	return withConfiguredLinearMCP(state, cfg.LinearMCPURL)
}

func withConfiguredLinearMCP(state downstreammcp.State, rawURL string) (downstreammcp.State, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return state, nil
	}
	if _, ok := state.Get("linear"); ok {
		return state, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return downstreammcp.State{}, fmt.Errorf("PRISM_LINEAR_MCP_URL must be an absolute URL")
	}
	state.Upsert(downstreammcp.Server{
		Name:        "linear",
		Transport:   downstreammcp.TransportCommand,
		Command:     "npx",
		Args:        []string{"-y", "mcp-remote", rawURL},
		Description: "Linear MCP server from PRISM_LINEAR_MCP_URL",
	})
	return state, nil
}
