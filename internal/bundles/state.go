package bundles

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	bundlepkg "github.com/bryanbarton525/prism/pkg/bundle"
	"gopkg.in/yaml.v3"
)

type State struct {
	Bundles []bundlepkg.Installed `json:"bundles" yaml:"bundles"`
}

func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func Save(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func RecordInstall(path string, manifest bundlepkg.Manifest) error {
	state, err := Load(path)
	if err != nil {
		return err
	}
	installed := bundlepkg.Installed{
		ID:              manifest.ID,
		Version:         manifest.Version,
		Channel:         manifest.Channel,
		Owner:           manifest.Owner,
		RiskLevel:       manifest.RiskLevel,
		RequiredPlugins: manifest.RequiredPlugins,
		InstalledAt:     time.Now().UTC().Format(time.RFC3339),
	}
	replaced := false
	for i := range state.Bundles {
		if state.Bundles[i].ID == manifest.ID {
			state.Bundles[i] = installed
			replaced = true
			break
		}
	}
	if !replaced {
		state.Bundles = append(state.Bundles, installed)
	}
	return Save(path, state)
}

func LoadManifest(path string) (bundlepkg.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return bundlepkg.Manifest{}, err
	}
	var manifest bundlepkg.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return bundlepkg.Manifest{}, err
	}
	if manifest.ID == "" || manifest.Version == "" {
		return bundlepkg.Manifest{}, fmt.Errorf("bundle manifest requires id and version")
	}
	return manifest, nil
}

type RegistrySources struct {
	Sources []RegistrySource `json:"sources" yaml:"sources"`
}

type RegistrySource struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url" yaml:"url"`
}

func LoadSources(path string) (RegistrySources, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RegistrySources{}, nil
		}
		return RegistrySources{}, err
	}
	var sources RegistrySources
	return sources, yaml.Unmarshal(data, &sources)
}

func SaveSources(path string, sources RegistrySources) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(sources)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
