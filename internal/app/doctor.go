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
