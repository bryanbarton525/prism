package benchmark

import (
	"math"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ContextBreakdown models how frontier-model prompt budget is consumed.
type ContextBreakdown struct {
	SystemInstructions int `json:"system_instructions"`
	ToolDefinitions    int `json:"tool_definitions"`
	UserMessages       int `json:"user_messages"`
	ToolResults        int `json:"tool_results"`
}

func (b ContextBreakdown) Total() int {
	return b.SystemInstructions + b.ToolDefinitions + b.UserMessages + b.ToolResults
}

func (b ContextBreakdown) PercentOf(section string, total int) float64 {
	if total <= 0 {
		return 0
	}
	var tokens int
	switch section {
	case "system_instructions":
		tokens = b.SystemInstructions
	case "tool_definitions":
		tokens = b.ToolDefinitions
	case "user_messages":
		tokens = b.UserMessages
	case "tool_results":
		tokens = b.ToolResults
	default:
		return 0
	}
	return math.Round((float64(tokens)/float64(total))*1000) / 10
}

func (b ContextBreakdown) EffectiveSavings(totalTokens, remoteMCPTokenSavings int) int {
	if totalTokens <= 0 {
		return 0
	}
	weight := 1.0
	if b.Total() > 0 {
		weight = float64(b.ToolDefinitions+b.ToolResults) / float64(b.Total())
	}
	return totalTokens - int(math.Round(float64(remoteMCPTokenSavings)*weight))
}

// Rates holds per-million-token pricing for cost estimates.
type Rates struct {
	Orchestrator RateModel `yaml:"orchestrator"`
	Local        RateModel `yaml:"local"`
}

// RateModel is input/output USD per million tokens.
type RateModel struct {
	Model            string  `yaml:"model"`
	InputPerMillion  float64 `yaml:"input_per_million"`
	OutputPerMillion float64 `yaml:"output_per_million"`
}

// LoadRates reads testdata/benchmarks/rates.yaml.
func LoadRates(root string) (Rates, error) {
	path := filepath.Join(root, "testdata", "benchmarks", "rates.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Rates{}, err
	}
	var r Rates
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Rates{}, err
	}
	return r, nil
}

// EstimateTokens approximates tokens from text (~4 characters per token).
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

// CostUSD computes estimated cost from token counts and rates.
func CostUSD(inputTokens, outputTokens int, r RateModel) float64 {
	in := float64(inputTokens) / 1_000_000 * r.InputPerMillion
	out := float64(outputTokens) / 1_000_000 * r.OutputPerMillion
	return math.Round((in+out)*1e6) / 1e6
}

// PassesAssertions reports whether text contains all required phrases (case-insensitive).
func PassesAssertions(text string, phrases []string) bool {
	return len(MissingAssertions(text, phrases)) == 0
}

// MissingAssertions returns required phrases not found in text (case-insensitive).
func MissingAssertions(text string, phrases []string) []string {
	lower := strings.ToLower(text)
	var missing []string
	for _, p := range phrases {
		if !strings.Contains(lower, strings.ToLower(p)) {
			missing = append(missing, p)
		}
	}
	return missing
}
