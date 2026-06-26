package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// Sentinel markers delimit the Prism-managed block so install/uninstall stay
// idempotent and never clobber surrounding user content.
const (
	instructionsBeginMarker = "<!-- BEGIN PRISM INSTRUCTIONS -->"
	instructionsEndMarker   = "<!-- END PRISM INSTRUCTIONS -->"
)

// instructionsTarget describes one place Prism guidance can be installed.
type instructionsTarget struct {
	key      string // CLI value, e.g. "agents"
	path     string // path relative to --dir
	label    string // human-readable agent/tool name
	preamble string // written above the block only when creating a new file
}

// instructionsTargets enumerates supported targets. "agents" (AGENTS.md) is the
// open default; the rest are dedicated files for agents that do not read AGENTS.md.
func instructionsTargets() []instructionsTarget {
	return []instructionsTarget{
		{key: "agents", path: "AGENTS.md", label: "AGENTS.md (open standard: Codex, Cursor, Aider, Gemini, OpenCode, …)"},
		{key: "copilot", path: filepath.Join(".github", "copilot-instructions.md"), label: "GitHub Copilot / VS Code"},
		{key: "claude", path: "CLAUDE.md", label: "Claude Code"},
		{key: "gemini", path: "GEMINI.md", label: "Gemini CLI"},
		{
			key:      "cursor",
			path:     filepath.Join(".cursor", "rules", "prism.mdc"),
			label:    "Cursor (rules file)",
			preamble: "---\ndescription: Prism specialist agent runner guidance\nalwaysApply: true\n---\n\n",
		},
	}
}

func instructionsTargetByKey(key string) (instructionsTarget, bool) {
	for _, t := range instructionsTargets() {
		if t.key == key {
			return t, true
		}
	}
	return instructionsTarget{}, false
}

func instructionsTargetKeys() []string {
	targets := instructionsTargets()
	keys := make([]string, 0, len(targets))
	for _, t := range targets {
		keys = append(keys, t.key)
	}
	return keys
}

// instructionsBlock returns the Prism guidance wrapped in sentinel markers.
func instructionsBlock() string {
	body := `## Prism

Prism is a local-first specialist agent runner. It delegates focused,
evidence-heavy subtasks to bounded local agents (served by Ollama or an
OpenAI-compatible runtime such as SGLang) and returns compact, auditable
summaries.

Use Prism when a task benefits from a dedicated specialist that owns a bulky
tool or context surface (Kubernetes, GitHub, Go scaffolding, Linear, docs
search, downstream MCP servers) instead of loading everything into the parent
model.

### CLI

- ` + "`prism agent list`" + ` — list available specialist agents.
- ` + "`prism run --agent <id> --skill <skill> --input \"<task>\"`" + ` — run one
  specialist with a required skill and a short task brief.
- ` + "`prism doctor`" + ` — check runtime, registry, and skill health.

### MCP

Prism also runs as an MCP server (` + "`prism mcp serve`" + `, stdio). Core tools:

- ` + "`list_agents`, `run_agent`, `get_constitution`, `doctor`" + `
- ` + "`suggest_route`, `run_graph`, `explain_policy`, `list_policies`" + `
- ` + "`list_mcp_servers`, `list_mcp_server_tools`, `call_mcp_tool`" + `

### Workflow

1. Paste a short brief — do not paste every skill and all evidence into chat.
2. Delegate evidence-heavy subtasks to Prism specialists via ` + "`run_agent`" + `.
3. Synthesize their compact summaries; rely on returned evidence artifacts for
   proof of any downstream action.

See README.md and docs/usage.md for setup, configuration, and flags.`

	return instructionsBeginMarker + "\n" + body + "\n" + instructionsEndMarker + "\n"
}

func newInstructionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instructions",
		Short: "Install Prism agent instructions into AGENTS.md and other agent files",
		Long: `Write Prism usage guidance into agent instruction files.

The default target is AGENTS.md (https://agents.md), the open standard read by
Codex, Cursor, Aider, Gemini, OpenCode and others. Dedicated targets cover
agents that read their own file instead of AGENTS.md.

The Prism block is delimited by sentinel comments, so re-running install updates
the block in place and never disturbs surrounding content.`,
	}
	cmd.AddCommand(newInstructionsInstallCmd())
	cmd.AddCommand(newInstructionsUninstallCmd())
	cmd.AddCommand(newInstructionsListCmd())
	return cmd
}

func newInstructionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List supported instruction targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			for _, t := range instructionsTargets() {
				marker := "  "
				if t.key == "agents" {
					marker = "* " // default
				}
				fmt.Fprintf(out, "%s%-9s %-12s %s\n", marker, t.key, t.path, t.label)
			}
			fmt.Fprintln(out, "\n* default target")
			return nil
		},
	}
}

func newInstructionsInstallCmd() *cobra.Command {
	var dir string
	var targets []string
	var all bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Prism instructions (default target: AGENTS.md)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selected, err := resolveInstructionsTargets(targets, all)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, t := range selected {
				dest := filepath.Join(dir, t.path)
				action, err := installInstructions(dest, t.preamble)
				if err != nil {
					return fmt.Errorf("%s: %w", t.key, err)
				}
				fmt.Fprintf(out, "%s %s (%s)\n", action, dest, t.label)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to install into")
	cmd.Flags().StringSliceVarP(&targets, "target", "t", []string{"agents"},
		fmt.Sprintf("Target(s) to install: %s", strings.Join(instructionsTargetKeys(), ", ")))
	cmd.Flags().BoolVar(&all, "all", false, "Install into every supported target")
	return cmd
}

func newInstructionsUninstallCmd() *cobra.Command {
	var dir string
	var targets []string
	var all bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the Prism block from instruction files",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selected, err := resolveInstructionsTargets(targets, all)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, t := range selected {
				dest := filepath.Join(dir, t.path)
				action, err := uninstallInstructions(dest)
				if err != nil {
					return fmt.Errorf("%s: %w", t.key, err)
				}
				fmt.Fprintf(out, "%s %s (%s)\n", action, dest, t.label)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to uninstall from")
	cmd.Flags().StringSliceVarP(&targets, "target", "t", []string{"agents"},
		fmt.Sprintf("Target(s) to uninstall: %s", strings.Join(instructionsTargetKeys(), ", ")))
	cmd.Flags().BoolVar(&all, "all", false, "Uninstall from every supported target")
	return cmd
}

func resolveInstructionsTargets(keys []string, all bool) ([]instructionsTarget, error) {
	if all {
		return instructionsTargets(), nil
	}
	seen := make(map[string]bool)
	var out []instructionsTarget
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			continue
		}
		t, ok := instructionsTargetByKey(key)
		if !ok {
			return nil, fmt.Errorf("unknown target %q (supported: %s)", key, strings.Join(instructionsTargetKeys(), ", "))
		}
		seen[key] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no target selected")
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out, nil
}

// installInstructions writes or refreshes the Prism block at dest. It returns a
// short action word ("created", "updated", "appended") describing what happened.
func installInstructions(dest, preamble string) (string, error) {
	block := instructionsBlock()
	existing, err := os.ReadFile(dest)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dest, []byte(preamble+block), 0o644); err != nil {
			return "", err
		}
		return "created", nil
	}

	content := string(existing)
	if updated, ok := replaceInstructionsBlock(content, block); ok {
		if err := os.WriteFile(dest, []byte(updated), 0o644); err != nil {
			return "", err
		}
		return "updated", nil
	}

	// Append a fresh block, ensuring separation from existing content.
	sep := "\n"
	if !strings.HasSuffix(content, "\n") {
		sep = "\n\n"
	} else if !strings.HasSuffix(content, "\n\n") {
		sep = "\n"
	}
	if err := os.WriteFile(dest, []byte(content+sep+block), 0o644); err != nil {
		return "", err
	}
	return "appended", nil
}

// uninstallInstructions removes the Prism block from dest if present.
func uninstallInstructions(dest string) (string, error) {
	existing, err := os.ReadFile(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return "absent", nil
		}
		return "", err
	}
	content := string(existing)
	stripped, ok := removeInstructionsBlock(content)
	if !ok {
		return "absent", nil
	}
	if strings.TrimSpace(stripped) == "" {
		if err := os.Remove(dest); err != nil {
			return "", err
		}
		return "removed (file deleted)", nil
	}
	if err := os.WriteFile(dest, []byte(stripped), 0o644); err != nil {
		return "", err
	}
	return "removed", nil
}

// replaceInstructionsBlock swaps an existing sentinel-delimited block for block.
// It reports false when no block is present.
func replaceInstructionsBlock(content, block string) (string, bool) {
	start := strings.Index(content, instructionsBeginMarker)
	if start == -1 {
		return content, false
	}
	endIdx := strings.Index(content, instructionsEndMarker)
	if endIdx == -1 || endIdx < start {
		return content, false
	}
	end := endIdx + len(instructionsEndMarker)
	// Absorb a single trailing newline so block (which ends in "\n") doesn't
	// accumulate blank lines on repeated installs.
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start] + block + content[end:], true
}

// removeInstructionsBlock deletes the sentinel-delimited block and tidies the
// surrounding whitespace. It reports false when no block is present.
func removeInstructionsBlock(content string) (string, bool) {
	start := strings.Index(content, instructionsBeginMarker)
	if start == -1 {
		return content, false
	}
	endIdx := strings.Index(content, instructionsEndMarker)
	if endIdx == -1 || endIdx < start {
		return content, false
	}
	end := endIdx + len(instructionsEndMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	before := strings.TrimRight(content[:start], "\n")
	after := strings.TrimLeft(content[end:], "\n")
	switch {
	case before == "":
		return after, true
	case after == "":
		return before + "\n", true
	default:
		return before + "\n\n" + after, true
	}
}
