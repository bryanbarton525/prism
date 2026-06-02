package agent

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Registry loads and caches agent specs from an fs.FS.
type Registry struct {
	fsys  fs.FS
	specs map[string]*Spec
}

// NewRegistry creates a Registry that reads specs from fsys.
// The FS should be the agents directory (or a sub-FS of the project root at "agents").
// For a local directory: agent.NewRegistry(os.DirFS(agentDir))
// For GitHub: agent.NewRegistry(fs.Sub(githubFS, "agents"))
func NewRegistry(fsys fs.FS) *Registry {
	return &Registry{fsys: fsys, specs: make(map[string]*Spec)}
}

// Load scans the FS root for *.md files and parses them as agent specs.
// README.md and directory entries are silently skipped. Any parsing or
// validation error causes Load to return immediately.
func (r *Registry) Load() error {
	entries, err := fs.ReadDir(r.fsys, ".")
	if err != nil {
		return fmt.Errorf("opening agent directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		data, err := fs.ReadFile(r.fsys, e.Name())
		if err != nil {
			return fmt.Errorf("reading agent spec %s: %w", e.Name(), err)
		}
		spec, err := Parse(data, e.Name())
		if err != nil {
			return fmt.Errorf("loading agent spec %s: %w", e.Name(), err)
		}
		r.specs[spec.ID] = spec
	}
	return nil
}

// Get returns the Spec for agentID or a descriptive error.
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
	return strings.Join(ids, ", ")
}
