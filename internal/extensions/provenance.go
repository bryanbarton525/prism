package extensions

type Provenance struct {
	Installer     string `yaml:"installer,omitempty" json:"installer,omitempty"`
	Host          string `yaml:"host,omitempty" json:"host,omitempty"`
	Actor         string `yaml:"actor,omitempty" json:"actor,omitempty"`
	SourceDigest  string `yaml:"source_digest,omitempty" json:"source_digest,omitempty"`
	SourceModel   string `yaml:"source_model,omitempty" json:"source_model,omitempty"`
	Adapter       string `yaml:"adapter,omitempty" json:"adapter,omitempty"`
	AdapterDigest string `yaml:"adapter_digest,omitempty" json:"adapter_digest,omitempty"`
	ImportConfig  string `yaml:"import_config_digest,omitempty" json:"import_config_digest,omitempty"`
	CopiedFrom    string `yaml:"copied_from,omitempty" json:"copied_from,omitempty"`
	BundleDigest  string `yaml:"bundle_digest,omitempty" json:"bundle_digest,omitempty"`
}
