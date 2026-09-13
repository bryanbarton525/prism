package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/skill"
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Lint, test, and benchmark Prism skills",
	}
	cmd.AddCommand(newSkillLintCmd())
	cmd.AddCommand(newSkillTestCmd())
	cmd.AddCommand(newSkillBenchmarkCmd())
	return cmd
}

func newSkillLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [skill-name]",
		Short: "Validate skill structure and metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			results := lintSkills(args, strictSkillAuthoringMode())
			return printSkillResults(results)
		},
	}
}

func newSkillTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [skill-name]",
		Short: "Run structural skill tests",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			strict := strictSkillAuthoringMode()
			results := lintSkills(args, strict)
			for i := range results {
				if !results[i].OK {
					continue
				}
				fsys := configuredSkillsFS()
				results[i].Warnings = append(results[i].Warnings, skill.ExecutionLimitations(fsys, results[i].Name)...)
				count, err := skill.ValidateEvals(fsys, results[i].Name)
				if err != nil {
					if strict {
						results[i].OK = false
						results[i].Errors = append(results[i].Errors, err.Error())
					} else {
						results[i].Warnings = append(results[i].Warnings, "eval validation skipped for portable mode: "+err.Error())
					}
					continue
				}
				results[i].Evals = count
			}
			return printSkillResults(results)
		},
	}
}

func newSkillBenchmarkCmd() *cobra.Command {
	var maxChars int
	cmd := &cobra.Command{
		Use:   "benchmark [skill-name]",
		Short: "Check skill context size budgets",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			results := lintSkills(args, strictSkillAuthoringMode())
			for i := range results {
				if results[i].Chars > maxChars {
					results[i].OK = false
					results[i].Errors = append(results[i].Errors, fmt.Sprintf("skill size %d exceeds max chars %d", results[i].Chars, maxChars))
				}
			}
			return printSkillResults(results)
		},
	}
	cmd.Flags().IntVar(&maxChars, "max-chars", 24000, "Maximum SKILL.md character count")
	return cmd
}

type skillResult struct {
	Name     string   `json:"name"`
	OK       bool     `json:"ok"`
	Chars    int      `json:"chars"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Evals    int      `json:"evals,omitempty"`
}

func lintSkills(args []string, strict bool) []skillResult {
	fsys := configuredSkillsFS()
	var names []string
	if len(args) == 1 {
		names = []string{args[0]}
	} else {
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			return []skillResult{{Name: "skills", OK: false, Errors: []string{err.Error()}}}
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
	}
	results := make([]skillResult, 0, len(names))
	for _, name := range names {
		res := skillResult{Name: name, OK: true}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(name, "SKILL.md")))
		if err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
			results = append(results, res)
			continue
		}
		res.Chars = len(data)
		if _, err := skill.LoadDir(fsys, name); err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
		}
		if strict {
			if err := skill.ValidateAuthoringStructure(fsys, name); err != nil {
				res.OK = false
				res.Errors = append(res.Errors, err.Error())
			}
		} else if err := skill.ValidatePortableStructure(fsys, name); err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
		}
		res.Warnings = append(res.Warnings, skill.ExecutionLimitations(fsys, name)...)
		if !strings.Contains(string(data), "##") {
			res.Warnings = append(res.Warnings, "no markdown section headings")
		}
		results = append(results, res)
	}
	return results
}

func printSkillResults(results []skillResult) error {
	if gf.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	ok := true
	for _, res := range results {
		status := "ok"
		if !res.OK {
			status = "fail"
			ok = false
		}
		if res.Evals > 0 {
			fmt.Printf("%s\t%s\t%d chars\t%d eval(s)\n", status, res.Name, res.Chars, res.Evals)
		} else {
			fmt.Printf("%s\t%s\t%d chars\n", status, res.Name, res.Chars)
		}
		for _, err := range res.Errors {
			fmt.Printf("  error: %s\n", err)
		}
		for _, warn := range res.Warnings {
			fmt.Printf("  warn: %s\n", warn)
		}
	}
	if !ok {
		return fmt.Errorf("one or more skills failed")
	}
	return nil
}

func configuredSkillsFS() fs.FS {
	if gf.skillsDir != "" {
		return os.DirFS(gf.skillsDir)
	}
	skillsFS, _ := fs.Sub(prismbundle.BundleFS(), "skills")
	return skillsFS
}

func strictSkillAuthoringMode() bool {
	// Embedded bundle validation retains strict authoring checks for CI.
	return gf.skillsDir == ""
}
