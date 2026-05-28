package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		skills  []string
		input   string
		stdin   bool
		format  string
	)

	cmd := &cobra.Command{
		Use:   "run <agent-id>",
		Short: "Run a specialist agent on a task",
		Long: `Invoke a local Ollama specialist agent with the given task input.

At least one skill must be specified via --skills.  Skills must be listed in
the agent's allowed_skills frontmatter field.

Examples:
  prism run github-cli --skills gh-pr-triage --input task.md --format json
  echo "summarise recent PRs" | prism run github-cli --skills gh-pr-triage --stdin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(skills) == 0 {
				return fmt.Errorf("--skills is required: specify at least one skill name")
			}
			if !stdin && input == "" {
				return fmt.Errorf("provide task input via --input <file> or --stdin")
			}
			if stdin && input != "" {
				return fmt.Errorf("--input and --stdin are mutually exclusive")
			}

			// TODO(milestone-2): load agent spec, assemble prompt, call Ollama
			fmt.Fprintf(cmd.OutOrStdout(), "agent: %s\n", args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "skills: %v\n", skills)
			fmt.Fprintf(cmd.OutOrStdout(), "format: %s\n", format)
			fmt.Fprintln(cmd.OutOrStdout(), "(Ollama runner not implemented yet)")
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&skills, "skills", nil, "skill names to attach (required; comma-separated or repeated flag)")
	cmd.Flags().StringVar(&input, "input", "", "path to task input file")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read task input from stdin")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: json or markdown")

	return cmd
}
