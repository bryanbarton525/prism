package extensions

import (
	"fmt"
	"strings"
)

type RuntimeTarget struct {
	Engine         string `yaml:"engine" json:"engine"`
	BaseURL        string `yaml:"base_url" json:"base_url"`
	Model          string `yaml:"model" json:"model"`
	ResolvedDigest string `yaml:"resolved_digest,omitempty" json:"resolved_digest,omitempty"`
}

func (t RuntimeTarget) Validate() error {
	if strings.TrimSpace(t.Engine) == "" {
		return fmt.Errorf("runtime target engine is required")
	}
	if strings.TrimSpace(t.BaseURL) == "" {
		return fmt.Errorf("runtime target base_url is required")
	}
	if strings.TrimSpace(t.Model) == "" {
		return fmt.Errorf("runtime target model is required")
	}
	return nil
}

func (t RuntimeTarget) Drifted(current RuntimeTarget) bool {
	return !strings.EqualFold(strings.TrimSpace(t.Engine), strings.TrimSpace(current.Engine)) ||
		strings.TrimSpace(t.BaseURL) != strings.TrimSpace(current.BaseURL) ||
		strings.TrimSpace(t.Model) != strings.TrimSpace(current.Model)
}
