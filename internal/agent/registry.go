package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Registry loads and caches agent specs from a directory.
type Registry struct {
	dir   string
	specs map[string]*Spec
}

// NewRegistry creates a Registry that reads specs from dir.
func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir, specs: make(map[string]*Spec)}
}

// Load scans the directory for *.md files and parses them as agent specs.
// Files that fail validation are skipped with a warning; the caller may inspect
// errors by iterating LoadAll.
func (r *Registry) Load() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("opening agent directory %s: %w", r.dir, err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "README.md" {
			continue
		}
		path := filepath.Join(r.dir, e.Name())
		spec, err := ParseFile(path)
		if err != nil {
			// Surface the error rather than silently skipping.
			return fmt.Errorf("loading agent spec: %w", err)
		}
		r.specs[spec.ID] = spec
	}
	return nil
}

// Get returns the Spec for agentID or an error if it is not registered.
func (r *Registry) Get(agentID string) (*Spec, error) {
	s, ok := r.specs[agentID]
	if !ok {
		return nil, fmt.Errorf("agent %q not found (available: %s)", agentID, r.listIDs())
	}
	return s, nil
}

// List returns all agent summaries sorted by ID.
func (r *Registry) List() []Summary {
	out := make([]Summary, 0, len(r.specs))
	for _, s := range r.specs {
		out = append(out, s.ToSummary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) listIDs() string {
	ids := make([]string, 0, len(r.specs))
	for id := range r.specs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}
