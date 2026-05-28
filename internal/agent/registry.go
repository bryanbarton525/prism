package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Registry holds all loaded agent specs indexed by ID.
type Registry struct {
	specs  map[string]*Spec
	dir    string
}

// LoadRegistry reads all *.md files from dir, parses them as agent specs,
// and returns a Registry. Files that fail to parse or validate are collected
// as non-fatal warnings; only load-blocking errors are returned.
func LoadRegistry(dir string) (*Registry, []error, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("reading agent directory %q: %w", dir, err)
	}

	r := &Registry{
		specs: make(map[string]*Spec),
		dir:   dir,
	}
	var warnings []error

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if strings.EqualFold(e.Name(), "README.md") {
			continue
		}

		path := filepath.Join(dir, e.Name())
		spec, err := ParseFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("skipping %q: %w", e.Name(), err))
			continue
		}
		if err := spec.Validate(); err != nil {
			warnings = append(warnings, fmt.Errorf("skipping %q: %w", e.Name(), err))
			continue
		}

		if existing, dup := r.specs[spec.ID]; dup {
			warnings = append(warnings, fmt.Errorf(
				"duplicate agent ID %q in %q (already loaded from %q); skipping",
				spec.ID, path, existing.SourcePath,
			))
			continue
		}

		r.specs[spec.ID] = spec
	}

	return r, warnings, nil
}

// List returns all agent specs sorted by ID.
func (r *Registry) List() []*Spec {
	specs := make([]*Spec, 0, len(r.specs))
	for _, s := range r.specs {
		specs = append(specs, s)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].ID < specs[j].ID
	})
	return specs
}

// Get returns the spec for the given ID, or an error if not found.
func (r *Registry) Get(id string) (*Spec, error) {
	s, ok := r.specs[id]
	if !ok {
		return nil, fmt.Errorf("agent %q not found (checked %q)", id, r.dir)
	}
	return s, nil
}

// Len returns the number of loaded agents.
func (r *Registry) Len() int {
	return len(r.specs)
}
