package extensions

type Provenance struct {
	Installer string `yaml:"installer,omitempty" json:"installer,omitempty"`
	Host      string `yaml:"host,omitempty" json:"host,omitempty"`
	Actor     string `yaml:"actor,omitempty" json:"actor,omitempty"`
}
