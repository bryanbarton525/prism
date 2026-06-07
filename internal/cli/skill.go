package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
			results := lintSkills(args)
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
			results := lintSkills(args)
			for i := range results {
				if !results[i].OK {
					continue
				}
				if !fileExists(filepath.Join(gf.skillsDirOrDefault(), results[i].Name, "references")) {
					results[i].Warnings = append(results[i].Warnings, "no references directory")
				}
				count, err := skill.ValidateEvals(os.DirFS(gf.skillsDirOrDefault()), results[i].Name)
				if err != nil {
					results[i].OK = false
					results[i].Errors = append(results[i].Errors, err.Error())
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
			results := lintSkills(args)
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

func lintSkills(args []string) []skillResult {
	root := gf.skillsDirOrDefault()
	var names []string
	if len(args) == 1 {
		names = []string{args[0]}
	} else {
		entries, err := os.ReadDir(root)
		if err != nil {
			return []skillResult{{Name: root, OK: false, Errors: []string{err.Error()}}}
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
	}
	results := make([]skillResult, 0, len(names))
	fsys := os.DirFS(root)
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
		if err := skill.ValidateStructure(fsys, name); err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
		}
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

func (f globalFlags) skillsDirOrDefault() string {
	if f.skillsDir != "" {
		return f.skillsDir
	}
	return filepath.Join(f.rootDir, "skills")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
