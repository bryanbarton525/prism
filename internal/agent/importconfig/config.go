package importconfig

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

const Version = 1

type Config struct {
	Version int               `yaml:"version" json:"version"`
	Map     map[string]string `yaml:"map,omitempty" json:"map,omitempty"`
	Omit    []string          `yaml:"omit,omitempty" json:"omit,omitempty"`
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.Version != Version {
		return Config{}, fmt.Errorf("unsupported import config version %d (want %d)", cfg.Version, Version)
	}
	return cfg, nil
}
