package reports

import (
	"fmt"
	"strings"

	"github.com/bryanbarton525/prism/internal/events"
)

func UsageMarkdown(sum events.Summary) string {
	var b strings.Builder
	b.WriteString("# Prism Usage Report\n\n")
	b.WriteString(fmt.Sprintf("- Total runs: %d\n", sum.Total))
	b.WriteString(fmt.Sprintf("- Context budget warnings: %d\n", sum.ContextBudgetWarnings))
	b.WriteString(fmt.Sprintf("- Policy denials: %d\n", sum.PolicyDenials))
	b.WriteString(fmt.Sprintf("- Validation failures: %d\n", sum.ValidationFailures))
	b.WriteString(fmt.Sprintf("- Timeouts: %d\n", sum.Timeouts))
	b.WriteString(fmt.Sprintf("- Graph executions: %d\n", sum.GraphExecutions))
	b.WriteString(fmt.Sprintf("- Prompt tokens estimate: %d\n", sum.PromptTokensEstimate))
	b.WriteString(fmt.Sprintf("- Completion tokens estimate: %d\n", sum.CompletionTokensEstimate))
	b.WriteString(fmt.Sprintf("- Plugin usage: %v\n", sum.TopPlugins))
	return b.String()
}

func SavingsMarkdown(sum events.Summary) string {
	avoided := sum.PromptTokensEstimate
	return fmt.Sprintf("# Prism Savings Report\n\nEstimated orchestrator input avoided: %d tokens\n", avoided)
}

func AdoptionMarkdown(sum events.Summary) string {
	return fmt.Sprintf("# Prism Adoption Report\n\nActive agents: %d\nActive skills: %d\nInstalled bundle versions observed: %d\nGraph executions: %d\nTotal runs: %d\n", len(sum.TopAgents), len(sum.TopSkills), len(sum.BundleVersions), sum.GraphExecutions, sum.Total)
}

func BundlesMarkdown(bundleCount int) string {
	return fmt.Sprintf("# Prism Bundle Report\n\nInstalled bundles: %d\n", bundleCount)
}
