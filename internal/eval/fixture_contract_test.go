package eval

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/result"
)

// An invalid oracle would turn correct live answers into false regressions.
func TestExploratorySuiteOraclesMatchDecisionContracts(t *testing.T) {
	for _, name := range []string{"mcp-response-protocol-v1", "mcp-routing-holdout-v1", "mcp-independent-routing-v1"} {
		t.Run(name, func(t *testing.T) {
			suite, err := LoadSuite(filepath.Join("..", "..", "evals", name, "suite.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range suite.Cases {
				contract, err := result.ResolveDecisionSchema(c.DecisionSchema)
				if err != nil {
					t.Fatalf("%s: %v", c.ID, err)
				}
				decision := map[string]any{}
				for _, check := range c.Checks {
					key, ok := strings.CutPrefix(check.Path, "/decision/")
					if !ok || strings.Contains(key, "/") {
						t.Fatalf("%s: oracle check must name a direct decision field: %s", c.ID, check.Path)
					}
					decision[key] = check.Equals
				}
				if err := contract.Validate(decision); err != nil {
					t.Fatalf("%s: expected answers violate their own schema: %v", c.ID, err)
				}
			}
		})
	}
}
