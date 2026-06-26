package cli

import (
	"embed"
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

// Instruction content lives in markdown files compiled in via go:embed.
// templates/instructions.md is the single shared body; per-target intro text
// lives in the instructionsTarget struct and is substituted at the
// <!-- END_INTRO --> marker so no content is duplicated across files.
//
//go:embed templates/*.md
var instructionsFS embed.FS

// instructionsTarget describes one place Prism guidance can be installed.
type instructionsTarget struct {
	key      string // CLI value, e.g. "agents"
	path     string // path relative to --dir
	label    string // human-readable agent/tool name
	preamble string // written above the block only when creating a new file
	intro    string // replaces the default intro paragraph (up to <!-- END_INTRO -->); empty = use shared default
}

// instructionsTargets enumerates supported targets. "agents" (AGENTS.md) is the
// open default; the rest are dedicated files for agents that do not read AGENTS.md.
func instructionsTargets() []instructionsTarget {
	return []instructionsTarget{
		{key: "agents", path: "AGENTS.md", label: "AGENTS.md (open standard: Codex, Cursor, Aider, Gemini, OpenCode, …)"},
		{
			key:   "copilot",
			path:  filepath.Join(".github", "copilot-instructions.md"),
			label: "GitHub Copilot / VS Code",
			intro: "Prism is a local-first specialist agent runner. From GitHub Copilot Chat, delegate focused, evidence-heavy subtasks to bounded local agents (served by Ollama or an OpenAI-compatible runtime such as SGLang) instead of loading every tool and skill into this conversation.",
		},
		{
			key:   "claude",
			path:  "CLAUDE.md",
			label: "Claude Code",
			intro: "Prism is a local-first specialist agent runner. When working in Claude Code, delegate focused, evidence-heavy subtasks to bounded local agents (served by Ollama or an OpenAI-compatible runtime such as SGLang) rather than expanding this context window with every tool and skill.",
		},
		{
			key:   "gemini",
			path:  "GEMINI.md",
			label: "Gemini CLI",
			intro: "Prism is a local-first specialist agent runner. From the Gemini CLI, delegate focused, evidence-heavy subtasks to bounded local agents (served by Ollama or an OpenAI-compatible runtime such as SGLang) instead of loading every tool and skill into context.",
		},
		{
			key:      "cursor",
			path:     filepath.Join(".cursor", "rules", "prism.mdc"),
			label:    "Cursor (rules file)",
			preamble: cursorPreamble(),
			intro:    "Prism is a local-first specialist agent runner. In Cursor, delegate focused, evidence-heavy subtasks to bounded local agents (served by Ollama or an OpenAI-compatible runtime such as SGLang) instead of loading every tool and skill into the chat context.",
		},
	}
}

// cursorPreamble returns the Cursor rules frontmatter followed by a blank line,
// normalized so trailing-newline differences in the embedded file don't matter.
func cursorPreamble() string {
	data, _ := instructionsFS.ReadFile("templates/cursor-preamble.md")
	return strings.TrimRight(string(data), "\n") + "\n\n"
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

const introMarker = "<!-- END_INTRO -->"

// instructionsBodyFor returns the markdown body for the given target. The shared
// instructions.md is the single source of truth; if the target carries a custom
// intro, the text before <!-- END_INTRO --> is replaced with it.
func instructionsBodyFor(t instructionsTarget) string {
	data, _ := instructionsFS.ReadFile("templates/instructions.md")
	body := string(data)
	if t.intro == "" {
		// Strip the marker and return the unmodified body.
		return strings.Replace(body, "\n"+introMarker, "", 1)
	}
	parts := strings.SplitN(body, "\n"+introMarker, 2)
	if len(parts) != 2 {
		return body
	}
	// parts[0] is everything up to (not including) the marker line;
	// replace from the first paragraph onwards with the custom intro.
	heading := "## Prism"
	if idx := strings.Index(parts[0], heading); idx >= 0 {
		return heading + "\n\n" + t.intro + parts[1]
	}
	return heading + "\n\n" + t.intro + parts[1]
}

// instructionsBlock wraps a target's body in sentinel markers.
func instructionsBlock(t instructionsTarget) string {
	body := strings.TrimRight(instructionsBodyFor(t), "\n")
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
				action, err := installInstructions(dest, instructionsBlock(t), t.preamble)
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
func installInstructions(dest, block, preamble string) (string, error) {
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

	// Append a fresh block, ensuring exactly one blank line of separation.
	var sep string
	switch {
	case strings.HasSuffix(content, "\n\n"):
		sep = ""
	case strings.HasSuffix(content, "\n"):
		sep = "\n"
	default:
		sep = "\n\n"
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
	// Search for END only within the content that follows BEGIN, so a mention of
	// the END sentinel earlier in the file (e.g., documentation) is ignored.
	relEnd := strings.Index(content[start:], instructionsEndMarker)
	if relEnd == -1 {
		return content, false
	}
	endIdx := start + relEnd
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
	// Search for END only after BEGIN so earlier documentation mentions are ignored.
	relEnd := strings.Index(content[start:], instructionsEndMarker)
	if relEnd == -1 {
		return content, false
	}
	endIdx := start + relEnd
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
