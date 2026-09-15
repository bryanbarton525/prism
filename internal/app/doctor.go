package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/skill"
)

// Doctor implements AgentRunner. It checks Ollama connectivity, model availability,
// agent registry state, and skill directory structure.
func (r *Runner) Doctor(ctx context.Context) (result.DoctorResult, error) {
	dr := result.DoctorResult{
		OllamaHost: r.ollama.Host(),
		AgentDir:   r.cfg.agentDirLabel(),
		SkillsDir:  r.cfg.skillsDirLabel(),
		AgentCount: len(r.registry.List()),
		Status:     "ok",
	}

	if err := r.ollama.Ping(ctx); err != nil {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name: "ollama_connectivity", Status: "fail", Message: err.Error(),
		})
		dr.Status = "degraded"
	} else {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "ollama_connectivity",
			Status:  "ok",
			Message: fmt.Sprintf("reachable at %s", r.ollama.Host()),
		})
	}

	models, err := r.ollama.ListModels(ctx)
	if err != nil {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name: "ollama_models", Status: "fail", Message: err.Error(),
		})
		dr.Status = "degraded"
	} else {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "ollama_models",
			Status:  "ok",
			Message: fmt.Sprintf("%d model(s): %s", len(models), strings.Join(models, ", ")),
		})
		r.checkAgentModels(&dr, models)
	}

	agents := r.registry.List()
	if len(agents) == 0 {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "agent_registry",
			Status:  "warn",
			Message: fmt.Sprintf("no agents in %s", dr.AgentDir),
		})
	} else {
		ids := make([]string, len(agents))
		for i, a := range agents {
			ids[i] = a.ID
		}
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "agent_registry",
			Status:  "ok",
			Message: fmt.Sprintf("%d agent(s): %s", len(ids), strings.Join(ids, ", ")),
		})
	}

	skills, skillErr := skill.DiscoverAll(r.skillsFS)
	dr.SkillCount = len(skills)
	if skillErr != nil {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name: "skill_registry", Status: "fail", Message: skillErr.Error(),
		})
		dr.Status = "degraded"
	} else if len(skills) == 0 {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "skill_registry",
			Status:  "warn",
			Message: fmt.Sprintf("no skills in %s", dr.SkillsDir),
		})
	} else {
		names := make([]string, len(skills))
		for i, sk := range skills {
			names[i] = sk.Name
		}
		sort.Strings(names)
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "skill_registry",
			Status:  "ok",
			Message: fmt.Sprintf("%d skill(s): %s", len(names), strings.Join(names, ", ")),
		})
	}

	bundledAgents := 0
	managedAgents := 0
	bundledSkills := 0
	managedSkills := 0
	inactive := []string{}
	for _, item := range r.catalog.Agents {
		if item.Origin == "managed" {
			if item.Active {
				managedAgents++
			} else {
				message := item.Reason
				if len(item.Diagnostics) > 0 {
					message = item.Diagnostics[0].Message
				}
				inactive = append(inactive, fmt.Sprintf("agent %q: %s", item.ID, message))
			}
		} else if item.Origin == "bundled" {
			bundledAgents++
		}
	}
	for _, item := range r.catalog.Skills {
		if item.Origin == "managed" {
			if item.Active {
				managedSkills++
			} else {
				message := item.Reason
				if len(item.Diagnostics) > 0 {
					message = item.Diagnostics[0].Message
				}
				inactive = append(inactive, fmt.Sprintf("skill %q: %s", item.ID, message))
			}
		} else if item.Origin == "bundled" {
			bundledSkills++
		}
	}
	catalogCheck := result.DoctorCheck{
		Name:    "extension_catalog",
		Status:  "ok",
		Message: fmt.Sprintf("snapshot: %d bundled agent(s), %d managed agent(s), %d bundled skill(s), %d managed skill(s)", bundledAgents, managedAgents, bundledSkills, managedSkills),
	}
	if len(inactive) > 0 {
		sort.Strings(inactive)
		catalogCheck.Status = "warn"
		catalogCheck.Message += "; inactive managed extensions: " + strings.Join(inactive, ", ")
		dr.Status = "degraded"
	}
	dr.Checks = append(dr.Checks, catalogCheck)

	return dr, nil
}

func (r *Runner) checkAgentModels(dr *result.DoctorResult, available []string) {
	set := make(map[string]struct{}, len(available))
	for _, m := range available {
		set[m] = struct{}{}
	}
	var missing []string
	for _, spec := range r.registry.List() {
		full, err := r.registry.Get(spec.ID)
		if err != nil {
			continue
		}
		if _, ok := set[full.Model]; !ok {
			missing = append(missing, fmt.Sprintf("%s needs %q", spec.ID, full.Model))
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	dr.Checks = append(dr.Checks, result.DoctorCheck{
		Name:    "agent_models",
		Status:  "warn",
		Message: strings.Join(missing, "; "),
	})
}
