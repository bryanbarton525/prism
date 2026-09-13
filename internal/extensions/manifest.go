package extensions

import "time"

const ManifestVersion = 1

type Manifest struct {
	Version int              `yaml:"version" json:"version"`
	Entries []ManifestEntry  `yaml:"entries" json:"entries"`
}

type ManifestEntry struct {
	Identity    string                 `yaml:"identity" json:"identity"`
	Kind        string                 `yaml:"kind" json:"kind"`
	Source      string                 `yaml:"source" json:"source"`
	Revision    string                 `yaml:"revision,omitempty" json:"revision,omitempty"`
	Subpath     string                 `yaml:"subpath,omitempty" json:"subpath,omitempty"`
	Digest      string                 `yaml:"digest" json:"digest"`
	ObjectPath  string                 `yaml:"object_path" json:"object_path"`
	InstalledAt time.Time              `yaml:"installed_at" json:"installed_at"`
	Provenance  Provenance             `yaml:"provenance,omitempty" json:"provenance,omitempty"`
	Diagnostics []ActivationDiagnostic `yaml:"activation_diagnostics,omitempty" json:"activation_diagnostics,omitempty"`
}

type ActivationDiagnostic struct {
	Code    string `yaml:"code" json:"code"`
	Message string `yaml:"message" json:"message"`
}

func EmptyManifest() Manifest {
	return Manifest{Version: ManifestVersion, Entries: []ManifestEntry{}}
}
