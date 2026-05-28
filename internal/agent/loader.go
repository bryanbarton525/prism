package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoaderOptions controls how the Loader resolves optional fields during load.
type LoaderOptions struct {
	// AgentDir is the directory containing agents/*.md files.
	// Defaults to "agents" relative to RepoRoot when empty.
	AgentDir string
	// SkillDir is the directory containing skills/<name>/SKILL.md files.
	// Defaults to "skills" relative to RepoRoot when empty.
	SkillDir string
	// ConstitutionDir is the legacy directory for behavior contracts.
	// The loader falls back here when ConstitutionPath is empty and the
	// agent body contains no meaningful text.
	// Defaults to "constitutions" relative to RepoRoot when empty.
	ConstitutionDir string
	// RepoRoot is prepended to relative paths. Defaults to the process
	// working directory when empty.
	RepoRoot string
	// ValidateSkillRefs controls whether allowed_skills entries are verified
	// against skill directories on disk. Defaults to true.
	ValidateSkillRefs bool
}

func (o *LoaderOptions) repoRoot() string {
	if o.RepoRoot != "" {
		return o.RepoRoot
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func (o *LoaderOptions) agentDir() string {
	if o.AgentDir != "" {
		return o.AgentDir
	}
	return filepath.Join(o.repoRoot(), "agents")
}

func (o *LoaderOptions) skillDir() string {
	if o.SkillDir != "" {
		return o.SkillDir
	}
	return filepath.Join(o.repoRoot(), "skills")
}

func (o *LoaderOptions) constitutionDir() string {
	if o.ConstitutionDir != "" {
		return o.ConstitutionDir
	}
	return filepath.Join(o.repoRoot(), "constitutions")
}

// Loader loads and validates agent specs from a directory.
type Loader struct {
	opts   LoaderOptions
	skills map[string]*SkillSpec // keyed by skill name
}

// NewLoader creates a Loader with the supplied options.
// Call LoadSkills before LoadAll or LoadFile if skill-reference validation
// is required (ValidateSkillRefs defaults to true).
func NewLoader(opts LoaderOptions) *Loader {
	return &Loader{
		opts:   opts,
		skills: make(map[string]*SkillSpec),
	}
}

// LoadSkills scans opts.SkillDir, loads every SKILL.md found, and stores the
// results for reference validation. It is safe to call multiple times; each
// call replaces the previous skill index.
func (l *Loader) LoadSkills() error {
	sd := l.opts.skillDir()
	entries, err := os.ReadDir(sd)
	if err != nil {
		if os.IsNotExist(err) {
			// No skills directory is not an error; validation will report
			// missing skills if any are referenced.
			return nil
		}
		return fmt.Errorf("reading skill dir %q: %w", sd, err)
	}

	index := make(map[string]*SkillSpec, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillFile := filepath.Join(sd, e.Name(), "SKILL.md")
		sk, err := LoadSkillFile(skillFile)
		if err != nil {
			return fmt.Errorf("loading skill %q: %w", e.Name(), err)
		}
		sk.DirPath = filepath.Join(sd, e.Name())
		index[sk.Name] = sk
	}
	l.skills = index
	return nil
}

// LoadAll loads all agents/*.md files found in opts.AgentDir and returns them
// as a slice ordered by file name. Files named README.md (case-insensitive)
// are skipped.
func (l *Loader) LoadAll() ([]*Spec, error) {
	ad := l.opts.agentDir()
	entries, err := os.ReadDir(ad)
	if err != nil {
		return nil, fmt.Errorf("reading agent dir %q: %w", ad, err)
	}

	var specs []*Spec
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.ToLower(e.Name()) == "readme.md" {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		spec, err := l.LoadFile(filepath.Join(ad, e.Name()))
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// LoadFile loads, parses, and validates a single agent spec Markdown file.
func (l *Loader) LoadFile(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent file %q: %w", path, err)
	}

	spec, err := parseSpecFile(data)
	if err != nil {
		return nil, fmt.Errorf("parsing agent file %q: %w", path, err)
	}
	spec.FilePath = path

	stem := fileStem(path)
	if err := l.validateSpec(spec, stem); err != nil {
		return nil, fmt.Errorf("invalid agent spec %q: %w", path, err)
	}

	if err := l.resolveConstitution(spec); err != nil {
		return nil, fmt.Errorf("resolving constitution for %q: %w", spec.ID, err)
	}

	return spec, nil
}

// validateSpec checks required fields and optional invariants.
func (l *Loader) validateSpec(spec *Spec, fileStem string) error {
	var errs []string

	if spec.ID == "" {
		errs = append(errs, "missing required field: id")
	}
	if spec.Name == "" {
		errs = append(errs, "missing required field: name")
	}
	if spec.Description == "" {
		errs = append(errs, "missing required field: description")
	}
	if spec.Model == "" {
		errs = append(errs, "missing required field: model")
	}
	if spec.ContextBudget <= 0 {
		errs = append(errs, "missing or invalid required field: context_budget (must be > 0)")
	}
	if len(spec.AllowedSkills) == 0 {
		errs = append(errs, "missing required field: allowed_skills (must list at least one skill)")
	}
	if spec.LatencyBudgetMS <= 0 {
		errs = append(errs, "missing or invalid required field: latency_budget_ms (must be > 0)")
	}

	// id must match the file stem so agents can be resolved by name.
	if spec.ID != "" && fileStem != "" && spec.ID != fileStem {
		errs = append(errs, fmt.Sprintf("id %q does not match file stem %q", spec.ID, fileStem))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	// Validate skill references if a skill index has been loaded.
	if l.opts.ValidateSkillRefs && len(l.skills) > 0 {
		if err := l.validateSkillRefs(spec); err != nil {
			return err
		}
	}

	return nil
}

// validateSkillRefs checks every allowed_skills entry against the loaded skill
// index. Returns an error listing every unresolved reference.
func (l *Loader) validateSkillRefs(spec *Spec) error {
	var missing []string
	for _, name := range spec.AllowedSkills {
		if _, ok := l.skills[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("unknown skill references in allowed_skills: %s",
			strings.Join(missing, ", "))
	}
	return nil
}

// resolveConstitution populates spec.Constitution using the following priority:
//  1. spec.ConstitutionPath (explicit pointer to a separate file)
//  2. spec.Body when it contains non-trivial content
//  3. Legacy constitutions/<id>.md (migration support)
func (l *Loader) resolveConstitution(spec *Spec) error {
	if spec.ConstitutionPath != "" {
		return l.loadConstitutionFromPath(spec)
	}

	if strings.TrimSpace(spec.Body) != "" {
		spec.Constitution = spec.Body
		return nil
	}

	// Migration fallback: try constitutions/<id>.md
	fallback := filepath.Join(l.opts.constitutionDir(), spec.ID+".md")
	data, err := os.ReadFile(fallback)
	if err != nil {
		if os.IsNotExist(err) {
			// No constitution found anywhere; leave empty without error.
			// Callers may choose to warn.
			return nil
		}
		return fmt.Errorf("reading fallback constitution %q: %w", fallback, err)
	}
	spec.Constitution = string(data)
	return nil
}

// loadConstitutionFromPath resolves a ConstitutionPath (absolute or relative
// to RepoRoot) and stores the content in spec.Constitution.
func (l *Loader) loadConstitutionFromPath(spec *Spec) error {
	p := spec.ConstitutionPath
	if !filepath.IsAbs(p) {
		p = filepath.Join(l.opts.repoRoot(), p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("reading constitution_path %q: %w", spec.ConstitutionPath, err)
	}
	spec.Constitution = string(data)
	return nil
}

// parseSpecFile splits a Markdown file into YAML frontmatter and body, then
// unmarshals the frontmatter into a Spec.
func parseSpecFile(data []byte) (*Spec, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var spec Spec
	if err := yaml.Unmarshal(fm, &spec); err != nil {
		return nil, fmt.Errorf("unmarshalling frontmatter: %w", err)
	}
	spec.Body = body
	return &spec, nil
}

// splitFrontmatter separates the YAML frontmatter block from the Markdown body.
// The frontmatter must be enclosed by leading and closing "---" delimiters on
// their own lines. Returns an error when no valid frontmatter block is found.
func splitFrontmatter(data []byte) (frontmatter []byte, body string, err error) {
	const delim = "---"

	// Normalize line endings.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")

	if !strings.HasPrefix(text, delim) {
		return nil, "", fmt.Errorf("agent spec must begin with a YAML frontmatter block (---)")
	}

	// Find the closing delimiter.
	rest := text[len(delim):]
	idx := strings.Index(rest, "\n"+delim)
	if idx < 0 {
		return nil, "", fmt.Errorf("agent spec frontmatter is not closed with ---")
	}

	yamlBlock := rest[:idx]
	afterClose := rest[idx+len("\n"+delim):]

	// Skip an optional trailing newline after the closing ---.
	afterClose = strings.TrimPrefix(afterClose, "\n")

	return []byte(yamlBlock), afterClose, nil
}

// fileStem returns the filename without directory path or extension.
func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// Registry holds all loaded agent specs indexed by ID.
type Registry struct {
	specs map[string]*Spec
}

// NewRegistry builds a Registry from a slice of specs.
func NewRegistry(specs []*Spec) *Registry {
	r := &Registry{specs: make(map[string]*Spec, len(specs))}
	for _, s := range specs {
		r.specs[s.ID] = s
	}
	return r
}

// Get returns the spec for the given agent ID, or (nil, false) if not found.
func (r *Registry) Get(id string) (*Spec, bool) {
	s, ok := r.specs[id]
	return s, ok
}

// List returns summaries of all registered agents ordered deterministically
// (sorted by ID).
func (r *Registry) List() []Summary {
	summaries := make([]Summary, 0, len(r.specs))
	for _, s := range r.specs {
		summaries = append(summaries, s.ToSummary())
	}
	// Sort by ID for stable output.
	sortSummaries(summaries)
	return summaries
}

// All returns all Spec pointers in the registry keyed by ID.
func (r *Registry) All() map[string]*Spec {
	out := make(map[string]*Spec, len(r.specs))
	for k, v := range r.specs {
		out[k] = v
	}
	return out
}

// sortSummaries sorts a slice of Summary by ID in place.
func sortSummaries(ss []Summary) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j].ID < ss[j-1].ID; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// LoadRegistry is a convenience wrapper that loads skills, loads all agent
// specs, and returns a ready-to-use Registry.
func LoadRegistry(opts LoaderOptions) (*Registry, error) {
	l := NewLoader(opts)

	// Load skills for reference validation (ignore errors if skill dir absent).
	if err := l.LoadSkills(); err != nil {
		return nil, fmt.Errorf("loading skills: %w", err)
	}

	specs, err := l.LoadAll()
	if err != nil {
		return nil, err
	}
	return NewRegistry(specs), nil
}

// LoadRegistryFrom is like LoadRegistry but accepts explicit directory paths
// for convenience in tests and CLI commands.
func LoadRegistryFrom(repoRoot string, validateSkillRefs bool) (*Registry, error) {
	return LoadRegistry(LoaderOptions{
		RepoRoot:          repoRoot,
		ValidateSkillRefs: validateSkillRefs,
	})
}

