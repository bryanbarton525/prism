package extensions

import "time"

const ManifestVersion = 1

type Manifest struct {
	Version   int             `yaml:"version" json:"version"`
	Entries   []ManifestEntry `yaml:"entries" json:"entries"`
	MCPAccess *MCPAccessState `yaml:"mcp_access,omitempty" json:"mcp_access,omitempty"`
}

type ManifestEntry struct {
	Identity      string                 `yaml:"identity" json:"identity"`
	Kind          string                 `yaml:"kind" json:"kind"`
	Source        string                 `yaml:"source" json:"source"`
	Revision      string                 `yaml:"revision,omitempty" json:"revision,omitempty"`
	Subpath       string                 `yaml:"subpath,omitempty" json:"subpath,omitempty"`
	Digest        string                 `yaml:"digest" json:"digest"`
	ObjectPath    string                 `yaml:"object_path" json:"object_path"`
	InstalledAt   time.Time              `yaml:"installed_at" json:"installed_at"`
	Provenance    Provenance             `yaml:"provenance,omitempty" json:"provenance,omitempty"`
	Runtime       *RuntimeTarget         `yaml:"runtime_target,omitempty" json:"runtime_target,omitempty"`
	SkillBindings []SkillBinding         `yaml:"skill_bindings,omitempty" json:"skill_bindings,omitempty"`
	Diagnostics   []ActivationDiagnostic `yaml:"activation_diagnostics,omitempty" json:"activation_diagnostics,omitempty"`
}

type SkillBinding struct {
	Name   string `yaml:"name" json:"name"`
	Origin string `yaml:"origin" json:"origin"` // bundled | managed | development
}

type ActivationDiagnostic struct {
	Code    string `yaml:"code" json:"code"`
	Message string `yaml:"message" json:"message"`
}

func EmptyManifest() Manifest {
	return Manifest{Version: ManifestVersion, Entries: []ManifestEntry{}}
}

func (m Manifest) Clone() Manifest {
	out := Manifest{
		Version: m.Version,
		Entries: make([]ManifestEntry, len(m.Entries)),
	}
	if m.MCPAccess != nil {
		state := MCPAccessState{DefaultServers: append([]string{}, m.MCPAccess.DefaultServers...), Agents: map[string]MCPAccessRule{}}
		for id, rule := range m.MCPAccess.Agents {
			state.Agents[id] = MCPAccessRule{Mode: rule.Mode, Servers: append([]string{}, rule.Servers...)}
		}
		out.MCPAccess = &state
	}
	for i, entry := range m.Entries {
		out.Entries[i] = entry.Clone()
	}
	return out
}

func (e ManifestEntry) Clone() ManifestEntry {
	out := e
	if e.Diagnostics != nil {
		out.Diagnostics = append([]ActivationDiagnostic{}, e.Diagnostics...)
	}
	if e.Provenance != (Provenance{}) {
		out.Provenance = e.Provenance
	}
	if e.Runtime != nil {
		runtime := *e.Runtime
		out.Runtime = &runtime
	}
	if e.SkillBindings != nil {
		out.SkillBindings = append([]SkillBinding{}, e.SkillBindings...)
	}
	return out
}
