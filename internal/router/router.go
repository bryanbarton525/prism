package router

import (
	"context"
	"sort"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

type AgentLister interface {
	ListAgents(context.Context) ([]agent.Summary, error)
}

type Request struct {
	Task   string `json:"task"`
	Source string `json:"source,omitempty"`
}

type Result struct {
	AgentID          string             `json:"agent_id"`
	SkillNames       []string           `json:"skill_names"`
	Reason           string             `json:"reason"`
	Risk             string             `json:"risk"`
	RequiresApproval bool               `json:"requires_approval"`
	PolicyDecision   policypkg.Decision `json:"policy_decision"`
}

type Router struct {
	agents AgentLister
	policy *internalpolicy.Engine
}

func New(agents AgentLister, policy *internalpolicy.Engine) *Router {
	return &Router{agents: agents, policy: policy}
}

func (r *Router) Suggest(ctx context.Context, req Request) (Result, error) {
	agents, err := r.agents.ListAgents(ctx)
	if err != nil {
		return Result{}, err
	}
	byID := map[string]agent.Summary{}
	for _, a := range agents {
		byID[a.ID] = a
	}
	task := strings.ToLower(req.Task)
	candidates := candidateRules(task)
	for _, candidate := range candidates {
		summary, ok := byID[candidate.AgentID]
		if !ok {
			continue
		}
		skills := intersect(candidate.SkillNames, summary.AllowedSkills)
		if len(skills) == 0 {
			continue
		}
		decision := policypkg.Allow("no policy configured")
		if r.policy != nil {
			decision = r.policy.Explain(policypkg.Request{
				AgentID: candidate.AgentID,
				Skills:  skills,
				Source:  req.Source,
			})
		}
		return Result{
			AgentID:          candidate.AgentID,
			SkillNames:       skills,
			Reason:           candidate.Reason,
			Risk:             candidate.Risk,
			RequiresApproval: decision.Decision == policypkg.DecisionRequireApproval,
			PolicyDecision:   decision,
		}, nil
	}
	return Result{
		Reason:         "no deterministic route matched the task",
		Risk:           "unknown",
		PolicyDecision: policypkg.Warn("no route matched"),
	}, nil
}

type candidate struct {
	AgentID    string
	SkillNames []string
	Reason     string
	Risk       string
}

func candidateRules(task string) []candidate {
	var out []candidate
	if containsAny(task, "linear", "linear issue", "linear ticket", "linear project", "linear cycle", "linear roadmap") {
		out = append(out, candidate{
			AgentID:    "linear",
			SkillNames: []string{"linear-issue-management"},
			Reason:     "The task references Linear issue, project, cycle, or roadmap workflows.",
			Risk:       "requires_write_approval",
		})
	}
	if containsAny(task, "kubernetes", "kubectl", "pod", "deployment", "rollout", "namespace", "crashloop", "imagepullbackoff") {
		out = append(out, candidate{
			AgentID:    "kubectl",
			SkillNames: []string{"k8s-rollout-diagnostics", "kubectl-triage"},
			Reason:     "The task references Kubernetes rollout, pod, or namespace signals.",
			Risk:       "read_only",
		})
	}
	if containsAny(task, "github action", "workflow", "ci", "pull request", " pr ", "gh ") {
		out = append(out, candidate{
			AgentID:    "github-cli",
			SkillNames: []string{"gh-actions-diagnostics", "gh-pr-triage"},
			Reason:     "The task references GitHub Actions, CI, or pull request triage.",
			Risk:       "read_only",
		})
	}
	if containsAny(task, "argo", "sync", "workflow") {
		out = append(out, candidate{
			AgentID:    "argo",
			SkillNames: []string{"argo-sync-health", "argo-workflow-debug"},
			Reason:     "The task references Argo CD sync or workflow diagnostics.",
			Risk:       "read_only",
		})
	}
	if containsAny(task, "go ", ".go", "golang", "test", "helper") {
		out = append(out, candidate{
			AgentID:    "go-helper",
			SkillNames: []string{"go-helper-fn", "go-test-table", "go-pure-util"},
			Reason:     "The task references Go implementation or tests.",
			Risk:       "code_generation",
		})
	}
	return out
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func intersect(preferred, allowed []string) []string {
	allowedSet := map[string]bool{}
	for _, name := range allowed {
		allowedSet[name] = true
	}
	var out []string
	for _, name := range preferred {
		if allowedSet[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
