package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/bundles"
	"github.com/bryanbarton525/prism/pkg/observe"
)

type runFlags struct {
	skills        []string
	input         string
	stdin         bool
	format        string
	bundleID      string
	bundleVersion string
}

func newRunCmd() *cobra.Command {
	var rf runFlags

	cmd := &cobra.Command{
		Use:   "run <agent-id>",
		Short: "Run a specialist agent with required skills",
		Long: `Invoke a local Ollama specialist agent.

At least one --skills value is required and must be in the agent's allowed_skills.
Provide the task via --input <file> or --stdin; if neither flag is set, the
command reads from stdin automatically when stdin is piped.

Output is JSON (default) or Markdown (--format markdown).

Examples:
  prism run github-cli --skills gh-pr-triage --input task.md
  echo "Check PR #42" | prism run github-cli --skills gh-pr-triage
  prism run kubectl --skills kubectl-triage --format markdown --stdin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent(cmd.Context(), args[0], rf)
		},
	}

	cmd.Flags().StringSliceVar(&rf.skills, "skills", nil,
		"Comma-separated or repeated skill names to attach (required)")
	cmd.Flags().StringVar(&rf.input, "input", "",
		"Path to a file containing the task text")
	cmd.Flags().BoolVar(&rf.stdin, "stdin", false,
		"Read task text from stdin")
	cmd.Flags().StringVar(&rf.format, "format", "json",
		`Output format: "json" or "markdown"`)
	cmd.Flags().StringVar(&rf.bundleID, "bundle-id", "",
		"Installed bundle ID to attribute this run to and check in policy")
	cmd.Flags().StringVar(&rf.bundleVersion, "bundle-version", "",
		"Bundle version for run attribution; defaults to installed version for --bundle-id")
	return cmd
}

func runAgent(ctx context.Context, agentID string, rf runFlags) error {
	if rf.format != "json" && rf.format != "markdown" {
		return fmt.Errorf("--format must be \"json\" or \"markdown\", got %q", rf.format)
	}

	task, err := resolveTask(rf)
	if err != nil {
		return err
	}
	bundleID, bundleVersion, err := resolveBundleProvenance(installedBundlesPath(), rf.bundleID, rf.bundleVersion)
	if err != nil {
		return err
	}

	verboseLog("agent: %s  skills: %v  format: %s", agentID, rf.skills, rf.format)

	runner, cleanup, err := newRunner(ctx)
	if err != nil {
		return fmt.Errorf("initialising runner: %w", err)
	}
	defer cleanup()

	res, err := runner.Run(ctx, app.RunRequest{
		AgentID:       agentID,
		Task:          task,
		SkillNames:    rf.skills,
		Format:        rf.format,
		Metadata:      observe.Metadata{Source: "cli"},
		BundleID:      bundleID,
		BundleVersion: bundleVersion,
	})
	if err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	switch rf.format {
	case "markdown":
		fmt.Print(res.ToMarkdown())
	default:
		data, err := res.ToJSON()
		if err != nil {
			return fmt.Errorf("serialising result: %w", err)
		}
		fmt.Println(string(data))
	}
	return nil
}

func resolveBundleProvenance(statePath, bundleID, bundleVersion string) (string, string, error) {
	if bundleID == "" {
		if bundleVersion != "" {
			return "", "", fmt.Errorf("--bundle-version requires --bundle-id")
		}
		return "", "", nil
	}
	if bundleVersion != "" {
		return bundleID, bundleVersion, nil
	}
	state, err := bundles.Load(statePath)
	if err != nil {
		return "", "", fmt.Errorf("loading installed bundle state: %w", err)
	}
	for _, bundle := range state.Bundles {
		if bundle.ID == bundleID {
			return bundle.ID, bundle.Version, nil
		}
	}
	return "", "", fmt.Errorf("bundle %q is not installed; pass --bundle-version explicitly or install the bundle first", bundleID)
}

func resolveTask(rf runFlags) (string, error) {
	if rf.input != "" && rf.stdin {
		return "", fmt.Errorf("--input and --stdin are mutually exclusive")
	}
	if rf.input != "" {
		data, err := os.ReadFile(rf.input)
		if err != nil {
			return "", fmt.Errorf("reading input file %s: %w", rf.input, err)
		}
		return string(data), nil
	}
	if rf.stdin || stdinIsPiped() {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("provide task text via --input <file> or --stdin (or pipe to stdin)")
}

func stdinIsPiped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}
