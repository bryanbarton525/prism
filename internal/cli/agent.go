package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/config"
)

// NewAgentCmd builds the `prism agent` subcommand tree.
func NewAgentCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage and inspect Prism agents",
	}
	cmd.AddCommand(newAgentListCmd(cfg))
	cmd.AddCommand(newAgentShowCmd(cfg))
	return cmd
}

// newAgentListCmd returns the `prism agent list` command.
func newAgentListCmd(cfg *config.Config) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available agents",
		Long:  "List every agent spec found in the configured agent directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, warnings, err := agent.LoadRegistry(cfg.AgentDir)
			if err != nil {
				return fmt.Errorf("loading agents: %w", err)
			}
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "warning: %v\n", w)
			}

			specs := registry.List()

			if jsonOut {
				return printAgentListJSON(specs)
			}
			return printAgentListTable(specs)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

// newAgentShowCmd returns the `prism agent show <agent-id>` command.
func newAgentShowCmd(cfg *config.Config) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show details for a single agent",
		Long:  "Display the full spec and constitution for a specific agent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			registry, warnings, err := agent.LoadRegistry(cfg.AgentDir)
			if err != nil {
				return fmt.Errorf("loading agents: %w", err)
			}
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "warning: %v\n", w)
			}

			spec, err := registry.Get(id)
			if err != nil {
				return err
			}

			constitution, err := loadConstitution(spec, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}

			if jsonOut {
				return printAgentShowJSON(spec, constitution)
			}
			return printAgentShowText(spec, constitution)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

// loadConstitution returns the constitution text for a spec.
// If constitution_path is set, the file is read from that path (resolved
// relative to the agent directory parent). Otherwise the spec body is used.
func loadConstitution(spec *agent.Spec, cfg *config.Config) (string, error) {
	if spec.ConstitutionPath != "" {
		// Resolve relative to the repo root (parent of agents/).
		repoRoot := parentDir(cfg.AgentDir)
		path := spec.ConstitutionPath
		if !isAbsPath(path) {
			path = joinPath(repoRoot, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading constitution file %q: %w", path, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return spec.Body, nil
}

// ─── table output ────────────────────────────────────────────────────────────

func printAgentListTable(specs []*agent.Spec) error {
	if len(specs) == 0 {
		fmt.Println("No agents found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tMODEL\tSKILLS\tDESCRIPTION")
	fmt.Fprintln(w, strings.Repeat("-", 8)+"\t"+strings.Repeat("-", 20)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 6)+"\t"+strings.Repeat("-", 40))
	for _, s := range specs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			s.ID, s.Name, s.Model, len(s.AllowedSkills), s.Description)
	}
	return w.Flush()
}

func printAgentShowText(spec *agent.Spec, constitution string) error {
	fmt.Printf("Agent: %s\n", spec.ID)
	fmt.Printf("Name:  %s\n", spec.Name)
	fmt.Println()
	fmt.Printf("Description:       %s\n", spec.Description)
	fmt.Printf("Model:             %s\n", spec.Model)
	fmt.Printf("Context budget:    %d tokens\n", spec.ContextBudget)
	fmt.Printf("Latency budget:    %d ms\n", spec.LatencyBudgetMs)
	if spec.Temperature != 0 {
		fmt.Printf("Temperature:       %.2f\n", spec.Temperature)
	}
	if len(spec.Tools) > 0 {
		fmt.Printf("Tools:             %s\n", strings.Join(spec.Tools, ", "))
	}
	if spec.Outputs != "" {
		fmt.Printf("Output sections:   %s\n", spec.Outputs)
	}
	if len(spec.AllowedSkills) > 0 {
		fmt.Printf("Allowed skills:\n")
		for _, sk := range spec.AllowedSkills {
			fmt.Printf("  - %s\n", sk)
		}
	}
	if len(spec.Metadata) > 0 {
		fmt.Printf("Metadata:\n")
		for k, v := range spec.Metadata {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
	fmt.Printf("Source:            %s\n", spec.SourcePath)
	if spec.ConstitutionPath != "" {
		fmt.Printf("Constitution path: %s\n", spec.ConstitutionPath)
	}

	if constitution != "" {
		fmt.Println()
		fmt.Println("── Constitution ──────────────────────────────────────────────")
		fmt.Println(constitution)
	}
	return nil
}

// ─── JSON output ─────────────────────────────────────────────────────────────

type agentSummaryJSON struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Model           string   `json:"model"`
	ContextBudget   int      `json:"context_budget"`
	LatencyBudgetMs int      `json:"latency_budget_ms"`
	AllowedSkills   []string `json:"allowed_skills"`
	Tools           []string `json:"tools"`
}

type agentDetailJSON struct {
	agentSummaryJSON
	Temperature      float64           `json:"temperature,omitempty"`
	Outputs          string            `json:"outputs,omitempty"`
	ConstitutionPath string            `json:"constitution_path,omitempty"`
	Models           []string          `json:"models,omitempty"`
	TokenBudget      int               `json:"token_budget,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Constitution     string            `json:"constitution,omitempty"`
	SourcePath       string            `json:"source_path"`
}

func printAgentListJSON(specs []*agent.Spec) error {
	out := make([]agentSummaryJSON, len(specs))
	for i, s := range specs {
		out[i] = agentSummaryJSON{
			ID:              s.ID,
			Name:            s.Name,
			Description:     s.Description,
			Model:           s.Model,
			ContextBudget:   s.ContextBudget,
			LatencyBudgetMs: s.LatencyBudgetMs,
			AllowedSkills:   s.AllowedSkills,
			Tools:           s.Tools,
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printAgentShowJSON(spec *agent.Spec, constitution string) error {
	out := agentDetailJSON{
		agentSummaryJSON: agentSummaryJSON{
			ID:              spec.ID,
			Name:            spec.Name,
			Description:     spec.Description,
			Model:           spec.Model,
			ContextBudget:   spec.ContextBudget,
			LatencyBudgetMs: spec.LatencyBudgetMs,
			AllowedSkills:   spec.AllowedSkills,
			Tools:           spec.Tools,
		},
		Temperature:      spec.Temperature,
		Outputs:          spec.Outputs,
		ConstitutionPath: spec.ConstitutionPath,
		Models:           spec.Models,
		TokenBudget:      spec.TokenBudget,
		Metadata:         spec.Metadata,
		Constitution:     constitution,
		SourcePath:       spec.SourcePath,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func parentDir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx > 0 {
		return path[:idx]
	}
	return "."
}

func joinPath(a, b string) string {
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

func isAbsPath(p string) bool {
	return strings.HasPrefix(p, "/")
}
