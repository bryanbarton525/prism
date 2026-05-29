package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// Registry holds the loaded set of agent specs and skills.
type Registry struct {
	Agents map[string]*Spec
	Skills map[string]*SkillSpec
}

// LoadRegistry reads all agent specs from agentDir and all skills from skillsDir.
// Agents are indexed by their ID field. Skills are indexed by their name.
func LoadRegistry(agentDir, skillsDir string) (*Registry, error) {
	agents, err := loadAgents(agentDir)
	if err != nil {
		return nil, err
	}

	skills, err := LoadSkillsFromDir(skillsDir)
	if err != nil {
		return nil, err
	}

	return &Registry{
		Agents: agents,
		Skills: skills,
	}, nil
}

func loadAgents(agentDir string) (map[string]*Spec, error) {
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*Spec{}, nil
		}
		return nil, fmt.Errorf("reading agent directory %s: %w", agentDir, err)
	}

	agents := make(map[string]*Spec)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := entry.Name()
		if name == "README.md" {
			continue
		}
		path := filepath.Join(agentDir, name)
		spec, err := LoadSpec(path)
		if err != nil {
			return nil, err
		}
		if err := spec.Validate(); err != nil {
			return nil, err
		}
		agents[spec.ID] = spec
	}
	return agents, nil
}

// Get returns a spec by agent ID, or an error if not found.
func (r *Registry) Get(id string) (*Spec, error) {
	spec, ok := r.Agents[id]
	if !ok {
		return nil, fmt.Errorf("agent %q not found; run 'prism agent list' to see available agents", id)
	}
	return spec, nil
}

// GetSkills returns the SkillSpec values for the requested skill names, validating
// that each is present in the registry and allowed by the agent.
func (r *Registry) GetSkills(spec *Spec, skillNames []string) ([]*SkillSpec, error) {
	allowed := make(map[string]bool, len(spec.AllowedSkills))
	for _, s := range spec.AllowedSkills {
		allowed[s] = true
	}

	var skills []*SkillSpec
	for _, name := range skillNames {
		if !allowed[name] {
			return nil, fmt.Errorf("skill %q is not in the allowed_skills list for agent %q", name, spec.ID)
		}
		sk, ok := r.Skills[name]
		if !ok {
			return nil, fmt.Errorf("skill %q referenced by agent %q not found in skills directory", name, spec.ID)
		}
		skills = append(skills, sk)
	}
	return skills, nil
}

// ConstitutionText returns the constitution text for a spec. If constitution_path
// is set, the file is read from disk (relative to repoRoot). Otherwise the spec
// body is used.
func (r *Registry) ConstitutionText(spec *Spec, repoRoot string) (string, error) {
	if spec.ConstitutionPath != "" {
		path := filepath.Join(repoRoot, spec.ConstitutionPath)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading constitution at %s: %w", path, err)
		}
		return string(data), nil
	}
	return spec.Body, nil
}
