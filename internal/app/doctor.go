package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bryanbarton525/prism/internal/llm"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/skill"
)

// Doctor implements AgentRunner. It checks selected-runtime model availability,
// agent registry state, and skill directory structure.
func (r *Runner) Doctor(ctx context.Context) (result.DoctorResult, error) {
	dr := result.DoctorResult{
		OllamaHost: llm.PublicConfig(runtime.Config{BaseURL: r.ollama.Host()}).BaseURL,
		AgentDir:   r.cfg.agentDirLabel(),
		SkillsDir:  r.cfg.skillsDirLabel(),
		AgentCount: len(r.registry.List()),
		Status:     "ok",
	}

	primary := r.cfg.ModelRuntimeConfig
	if primary.Engine == "" {
		primary.Engine = runtime.EngineOllama
	}
	if primary.BaseURL == "" && primary.Engine == runtime.EngineOllama {
		primary.BaseURL = r.ollama.Host()
	}
	dr.RuntimeEngine, dr.RuntimeEndpoint, dr.RuntimeModel = string(primary.Engine), llm.PublicConfig(primary).BaseURL, primary.Model
	models, err := llm.ListModels(ctx, primary)
	checkName := "runtime_models"
	if primary.Engine == runtime.EngineOllama {
		checkName = "ollama_models"
	}
	if err != nil {
		dr.Checks = append(dr.Checks, result.DoctorCheck{Name: checkName, Status: "fail", Message: err.Error()})
		dr.Status = "degraded"
	} else {
		dr.Checks = append(dr.Checks, result.DoctorCheck{Name: checkName, Status: "ok", Message: fmt.Sprintf("%d advertised model(s) at %s: %s (inference not tested)", len(models), primary.BaseURL, strings.Join(models, ", "))})
		r.checkAgentModels(&dr, primary, models)
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

func (r *Runner) checkAgentModels(dr *result.DoctorResult, primary runtime.Config, available []string) {
	var missing []string
	status := "warn"
	for _, spec := range r.registry.List() {
		full, err := r.registry.Get(spec.ID)
		if err != nil {
			continue
		}
		target := primary
		if target.Model == "" {
			target.Model = full.Model
		}
		if !llm.ModelAvailable(target, available) {
			missing = append(missing, fmt.Sprintf("%s needs effective model %q", spec.ID, target.Model))
			status = "fail"
		}
		if recorded := r.managedRuntimeTarget(spec.ID); recorded != nil {
			current := r.cfg.RuntimeTarget
			if current.Model == "" {
				current.Model = recorded.Model
			}
			if recorded.Drifted(current) {
				missing = append(missing, fmt.Sprintf("%s blocked by runtime drift; reconcile with prism agent model set %s --use-configured-model", spec.ID, spec.ID))
			}
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	dr.Checks = append(dr.Checks, result.DoctorCheck{Name: "agent_models", Status: status, Message: strings.Join(missing, "; ")})
	dr.Status = "degraded"
}
