package benchmark

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Thresholds holds CI benchmark targets from testdata/benchmarks/thresholds.yaml.
type Thresholds struct {
	TokenReductionPercentMin   float64 `yaml:"token_reduction_percent_min"`
	PassRateDelegatedMin       float64 `yaml:"pass_rate_delegated_min"`
	LatencyBudgetComplianceMin float64 `yaml:"latency_budget_compliance_min"`
}

// SummaryReport is the JSON shape emitted by the benchmark suite.
type SummaryReport struct {
	Baseline                string             `json:"baseline"`
	Delegated               string             `json:"delegated"`
	Scenarios               int                `json:"scenarios"`
	TokenReductionPercent   float64            `json:"token_reduction_percent"`
	PassRate                map[string]float64 `json:"pass_rate"`
	LatencyBudgetCompliance float64            `json:"latency_budget_compliance"`
}

// LoadThresholds reads thresholds.yaml from the repository.
func LoadThresholds(root string) (Thresholds, error) {
	path := filepath.Join(root, "testdata", "benchmarks", "thresholds.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Thresholds{}, err
	}
	var t Thresholds
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Thresholds{}, err
	}
	return t, nil
}
