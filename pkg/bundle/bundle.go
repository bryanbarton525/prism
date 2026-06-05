// Package bundle defines Prism managed bundle metadata.
package bundle

type Manifest struct {
	ID                string   `json:"id" yaml:"id"`
	Version           string   `json:"version" yaml:"version"`
	Channel           string   `json:"channel,omitempty" yaml:"channel,omitempty"`
	Owner             string   `json:"owner,omitempty" yaml:"owner,omitempty"`
	Description       string   `json:"description,omitempty" yaml:"description,omitempty"`
	RiskLevel         string   `json:"risk_level,omitempty" yaml:"risk_level,omitempty"`
	Agents            []string `json:"agents,omitempty" yaml:"agents,omitempty"`
	Skills            []string `json:"skills,omitempty" yaml:"skills,omitempty"`
	Constitutions     []string `json:"constitutions,omitempty" yaml:"constitutions,omitempty"`
	RequiredPlugins   []string `json:"required_plugins,omitempty" yaml:"required_plugins,omitempty"`
	Compatibility     Compat   `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
	Files             []File   `json:"files,omitempty" yaml:"files,omitempty"`
	EvaluationSuite   []string `json:"evaluation_suite,omitempty" yaml:"evaluation_suite,omitempty"`
	DeprecationStatus string   `json:"deprecation_status,omitempty" yaml:"deprecation_status,omitempty"`
}

type Compat struct {
	MinPrismVersion string `json:"min_prism_version,omitempty" yaml:"min_prism_version,omitempty"`
	MaxPrismVersion string `json:"max_prism_version,omitempty" yaml:"max_prism_version,omitempty"`
}

type File struct {
	Kind   string `json:"kind" yaml:"kind"`
	Path   string `json:"path" yaml:"path"`
	SHA256 string `json:"sha256,omitempty" yaml:"sha256,omitempty"`
}

type Installed struct {
	ID              string   `json:"id" yaml:"id"`
	Version         string   `json:"version" yaml:"version"`
	Channel         string   `json:"channel,omitempty" yaml:"channel,omitempty"`
	Owner           string   `json:"owner,omitempty" yaml:"owner,omitempty"`
	RiskLevel       string   `json:"risk_level,omitempty" yaml:"risk_level,omitempty"`
	RequiredPlugins []string `json:"required_plugins,omitempty" yaml:"required_plugins,omitempty"`
	InstalledAt     string   `json:"installed_at" yaml:"installed_at"`
}
