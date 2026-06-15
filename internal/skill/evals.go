package skill

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type EvalSuite struct {
	Version int        `yaml:"version"`
	Skill   string     `yaml:"skill"`
	Cases   []EvalCase `yaml:"cases"`
}

type EvalCase struct {
	Name     string       `yaml:"name"`
	Prompt   string       `yaml:"prompt"`
	Expected EvalExpected `yaml:"expected"`
}

type EvalExpected struct {
	Includes []string `yaml:"includes"`
	Excludes []string `yaml:"excludes,omitempty"`
}

func ValidateEvals(fsys fs.FS, name string) (int, error) {
	pattern := filepath.ToSlash(filepath.Join(name, "evals", "*.yaml"))
	paths, err := fs.Glob(fsys, pattern)
	if err != nil {
		return 0, fmt.Errorf("skill %q eval glob: %w", name, err)
	}
	if len(paths) == 0 {
		return 0, fmt.Errorf("skill %q: no eval YAML files found in evals/", name)
	}
	total := 0
	var errs []string
	for _, path := range paths {
		count, err := validateEvalFile(fsys, name, path)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		total += count
	}
	if len(errs) > 0 {
		return total, fmt.Errorf("skill %q evals: %s", name, joinStrings(errs))
	}
	return total, nil
}

func validateEvalFile(fsys fs.FS, name, path string) (int, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	var suite EvalSuite
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&suite); err != nil {
		return 0, fmt.Errorf("%s: YAML parse error: %w", path, err)
	}
	var missing []string
	if suite.Version == 0 {
		missing = append(missing, "version")
	}
	if suite.Skill == "" {
		missing = append(missing, "skill")
	} else if suite.Skill != name {
		missing = append(missing, fmt.Sprintf("skill must be %q", name))
	}
	if len(suite.Cases) == 0 {
		missing = append(missing, "cases")
	}
	for i, c := range suite.Cases {
		prefix := fmt.Sprintf("cases[%d]", i)
		if strings.TrimSpace(c.Name) == "" {
			missing = append(missing, prefix+".name")
		}
		if strings.TrimSpace(c.Prompt) == "" {
			missing = append(missing, prefix+".prompt")
		}
		if len(c.Expected.Includes) == 0 && len(c.Expected.Excludes) == 0 {
			missing = append(missing, prefix+".expected.includes_or_excludes")
		}
	}
	if len(missing) > 0 {
		return 0, fmt.Errorf("%s: missing or invalid fields: %s", path, joinStrings(missing))
	}
	return len(suite.Cases), nil
}
