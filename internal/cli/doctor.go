package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/config"
)

// NewConfigCmd builds the `prism config` subcommand tree.
func NewConfigCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration and runtime diagnostics",
	}
	cmd.AddCommand(newConfigDoctorCmd(cfg))
	return cmd
}

// newConfigDoctorCmd returns the `prism config doctor` command.
func newConfigDoctorCmd(cfg *config.Config) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Report configuration state and local runtime readiness",
		Long:  "Check the Prism configuration and verify that local dependencies (Ollama) are reachable.",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := runDoctor(cfg)
			if jsonOut {
				return printDoctorJSON(report)
			}
			return printDoctorText(report)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

// ─── report types ────────────────────────────────────────────────────────────

type checkStatus string

const (
	statusOK   checkStatus = "ok"
	statusWarn checkStatus = "warn"
	statusFail checkStatus = "fail"
)

type check struct {
	Name    string      `json:"name"`
	Status  checkStatus `json:"status"`
	Detail  string      `json:"detail,omitempty"`
}

type doctorReport struct {
	Overall checkStatus `json:"overall"`
	Checks  []check     `json:"checks"`
}

// ─── doctor logic ─────────────────────────────────────────────────────────────

func runDoctor(cfg *config.Config) *doctorReport {
	var checks []check

	// 1. Agent directory
	checks = append(checks, checkDir("Agent directory", cfg.AgentDir))

	// 2. Constitution directory
	checks = append(checks, checkDir("Constitution directory", cfg.ConstitutionDir))

	// 3. Skills directory
	checks = append(checks, checkDir("Skills directory", cfg.SkillDir))

	// 4. Environment variables summary
	checks = append(checks, checkEnvVars())

	// 5. Agent specs
	agentCheck, agentCount := checkAgentSpecs(cfg.AgentDir)
	checks = append(checks, agentCheck)

	// 6. Skills layout
	checks = append(checks, checkSkillsLayout(cfg.SkillDir, agentCount))

	// 7. Ollama connectivity
	checks = append(checks, checkOllama(cfg.OllamaHost))

	// 8. Ollama models (only if Ollama is reachable)
	if checks[len(checks)-1].Status == statusOK {
		checks = append(checks, checkOllamaModels(cfg.OllamaHost, cfg))
	}

	overall := computeOverall(checks)
	return &doctorReport{Overall: overall, Checks: checks}
}

func checkDir(label, path string) check {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return check{Name: label, Status: statusFail,
				Detail: fmt.Sprintf("%q does not exist", path)}
		}
		return check{Name: label, Status: statusFail,
			Detail: fmt.Sprintf("cannot stat %q: %v", path, err)}
	}
	if !info.IsDir() {
		return check{Name: label, Status: statusFail,
			Detail: fmt.Sprintf("%q is not a directory", path)}
	}
	return check{Name: label, Status: statusOK, Detail: path}
}

func checkEnvVars() check {
	type envVar struct {
		key string
		val string
	}
	vars := []envVar{
		{"PRISM_OLLAMA_HOST", os.Getenv("PRISM_OLLAMA_HOST")},
		{"PRISM_DEFAULT_MODEL", os.Getenv("PRISM_DEFAULT_MODEL")},
		{"PRISM_AGENT_DIR", os.Getenv("PRISM_AGENT_DIR")},
		{"PRISM_CONFIG", os.Getenv("PRISM_CONFIG")},
	}
	var set, unset []string
	for _, v := range vars {
		if v.val != "" {
			set = append(set, v.key+"="+v.val)
		} else {
			unset = append(unset, v.key)
		}
	}
	detail := ""
	if len(set) > 0 {
		detail += "set: " + strings.Join(set, ", ")
	}
	if len(unset) > 0 {
		if detail != "" {
			detail += "; "
		}
		detail += "unset (defaults apply): " + strings.Join(unset, ", ")
	}
	return check{Name: "Environment variables", Status: statusOK, Detail: detail}
}

func checkAgentSpecs(dir string) (check, int) {
	registry, warnings, err := agent.LoadRegistry(dir)
	if err != nil {
		return check{Name: "Agent specs", Status: statusFail,
			Detail: err.Error()}, 0
	}
	n := registry.Len()
	status := statusOK
	detail := fmt.Sprintf("%d valid agent spec(s) loaded", n)
	if len(warnings) > 0 {
		status = statusWarn
		msgs := make([]string, len(warnings))
		for i, w := range warnings {
			msgs[i] = w.Error()
		}
		detail += fmt.Sprintf("; %d warning(s): %s", len(warnings), strings.Join(msgs, "; "))
	}
	if n == 0 {
		status = statusWarn
		detail = "no valid agent specs found in " + dir
	}
	return check{Name: "Agent specs", Status: status, Detail: detail}, n
}

func checkSkillsLayout(dir string, _ int) check {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return check{Name: "Skills layout", Status: statusFail,
			Detail: fmt.Sprintf("cannot read skills directory %q: %v", dir, err)}
	}

	var ok, bad []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		skillDir := filepath.Join(dir, name)
		issues := validateSkillDir(skillDir, name)
		if len(issues) == 0 {
			ok = append(ok, name)
		} else {
			bad = append(bad, name+"("+strings.Join(issues, ", ")+")")
		}
	}

	if len(ok)+len(bad) == 0 {
		return check{Name: "Skills layout", Status: statusWarn, Detail: "no skill directories found in " + dir}
	}

	status := statusOK
	detail := fmt.Sprintf("%d valid skill(s)", len(ok))
	if len(bad) > 0 {
		status = statusWarn
		detail += fmt.Sprintf("; %d skill(s) with issues: %s", len(bad), strings.Join(bad, ", "))
	}
	return check{Name: "Skills layout", Status: status, Detail: detail}
}

func validateSkillDir(dir, name string) []string {
	var issues []string

	skillMD := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillMD); err != nil {
		issues = append(issues, "missing SKILL.md")
	}

	refsDir := filepath.Join(dir, "references")
	if info, err := os.Stat(refsDir); err != nil || !info.IsDir() {
		issues = append(issues, "missing references/")
	}

	scriptsDir := filepath.Join(dir, "scripts")
	if info, err := os.Stat(scriptsDir); err != nil || !info.IsDir() {
		issues = append(issues, "missing scripts/")
	}

	_ = name
	return issues
}

func checkOllama(host string) check {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/tags", nil)
	if err != nil {
		return check{Name: "Ollama connectivity", Status: statusFail,
			Detail: fmt.Sprintf("building request: %v", err)}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return check{Name: "Ollama connectivity", Status: statusFail,
			Detail: fmt.Sprintf("%s is not reachable: %v", host, err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return check{Name: "Ollama connectivity", Status: statusFail,
			Detail: fmt.Sprintf("%s returned HTTP %d", host, resp.StatusCode)}
	}

	return check{Name: "Ollama connectivity", Status: statusOK,
		Detail: host + " is reachable"}
}

// ollamaTagsResponse mirrors the minimal shape of GET /api/tags.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func checkOllamaModels(host string, cfg *config.Config) check {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/tags", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return check{Name: "Ollama models", Status: statusFail, Detail: err.Error()}
	}
	defer resp.Body.Close()

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return check{Name: "Ollama models", Status: statusWarn,
			Detail: "could not parse model list: " + err.Error()}
	}

	available := make(map[string]bool, len(tags.Models))
	names := make([]string, len(tags.Models))
	for i, m := range tags.Models {
		available[m.Name] = true
		names[i] = m.Name
	}

	if len(names) == 0 {
		return check{Name: "Ollama models", Status: statusWarn,
			Detail: "no models found on the local Ollama instance"}
	}

	// Cross-reference with agent specs to flag missing models.
	registry, _, _ := agent.LoadRegistry(cfg.AgentDir)
	var missing []string
	if registry != nil {
		for _, spec := range registry.List() {
			if !available[spec.Model] {
				missing = append(missing, fmt.Sprintf("%s (agent: %s)", spec.Model, spec.ID))
			}
		}
	}

	status := statusOK
	detail := fmt.Sprintf("%d model(s) available: %s", len(names), strings.Join(names, ", "))
	if len(missing) > 0 {
		status = statusWarn
		detail += fmt.Sprintf("; missing model(s) required by agents: %s", strings.Join(missing, ", "))
	}
	return check{Name: "Ollama models", Status: status, Detail: detail}
}

func computeOverall(checks []check) checkStatus {
	overall := statusOK
	for _, c := range checks {
		switch c.Status {
		case statusFail:
			return statusFail
		case statusWarn:
			overall = statusWarn
		}
	}
	return overall
}

// ─── output formatters ────────────────────────────────────────────────────────

func printDoctorText(r *doctorReport) error {
	icon := map[checkStatus]string{
		statusOK:   "✓",
		statusWarn: "⚠",
		statusFail: "✗",
	}

	maxName := 0
	for _, c := range r.Checks {
		if len(c.Name) > maxName {
			maxName = len(c.Name)
		}
	}

	fmt.Println("Prism config doctor")
	fmt.Println(strings.Repeat("─", 60))
	for _, c := range r.Checks {
		padding := strings.Repeat(" ", maxName-len(c.Name))
		if c.Detail != "" {
			fmt.Printf("  %s %s%s  %s\n", icon[c.Status], c.Name, padding, c.Detail)
		} else {
			fmt.Printf("  %s %s\n", icon[c.Status], c.Name)
		}
	}
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Overall: %s\n", r.Overall)
	return nil
}

func printDoctorJSON(r *doctorReport) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
