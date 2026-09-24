package extensions

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type CatalogSnapshot struct {
	CreatedAt time.Time     `json:"created_at"`
	Agents    []CatalogItem `json:"agents"`
	Skills    []CatalogItem `json:"skills"`
}

type CatalogItem struct {
	ID          string                 `json:"id"`
	Kind        string                 `json:"kind"`
	Origin      string                 `json:"origin"` // bundled | managed
	Active      bool                   `json:"active"`
	Digest      string                 `json:"digest,omitempty"`
	Source      string                 `json:"source,omitempty"`
	ObjectPath  string                 `json:"object_path,omitempty"`
	ObjectRoot  string                 `json:"-"`
	Reason      string                 `json:"reason,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	Diagnostics []ActivationDiagnostic `json:"diagnostics,omitempty"`
	Runtime     *RuntimeTarget         `json:"runtime_target,omitempty"`
}

type ComposeInput struct {
	BundleFS         fs.FS
	Manifest         Manifest
	ObjectStoreRoot  string
	AgentOverride    bool
	SkillOverride    bool
	RejectCollisions bool
	Now              func() time.Time
}

func ComposeCatalog(input ComposeInput) (CatalogSnapshot, error) {
	now := input.Now
	if now == nil {
		now = time.Now
	}
	snapshot := CatalogSnapshot{
		CreatedAt: now().UTC(),
		Agents:    []CatalogItem{},
		Skills:    []CatalogItem{},
	}

	bundledAgents, err := bundledAgentIDs(input.BundleFS)
	if err != nil {
		return CatalogSnapshot{}, err
	}
	bundledSkills, err := bundledSkillIDs(input.BundleFS)
	if err != nil {
		return CatalogSnapshot{}, err
	}
	for _, id := range bundledAgents {
		item := CatalogItem{ID: id, Kind: "agent", Origin: "bundled", Active: !input.AgentOverride}
		if input.AgentOverride {
			item.Reason = "agent_override_active"
		}
		snapshot.Agents = append(snapshot.Agents, item)
	}
	for _, id := range bundledSkills {
		item := CatalogItem{ID: id, Kind: "skill", Origin: "bundled", Active: !input.SkillOverride}
		if input.SkillOverride {
			item.Reason = "skill_override_active"
		}
		snapshot.Skills = append(snapshot.Skills, item)
	}
	agentSet := foldedSet(bundledAgents)
	skillSet := foldedSet(bundledSkills)
	managedAgents := map[string]struct{}{}
	managedSkills := map[string]struct{}{}
	var collisionErrs []string
	for _, entry := range input.Manifest.Entries {
		item := CatalogItem{
			ID:          entry.Identity,
			Kind:        strings.ToLower(entry.Kind),
			Origin:      "managed",
			Active:      true,
			Digest:      entry.Digest,
			Source:      entry.Source,
			ObjectPath:  entry.ObjectPath,
			ObjectRoot:  input.ObjectStoreRoot,
			Diagnostics: append([]ActivationDiagnostic{}, entry.Diagnostics...),
			Runtime:     entry.Runtime,
		}
		foldedID := strings.ToLower(entry.Identity)
		switch strings.ToLower(entry.Kind) {
		case "agent":
			if input.AgentOverride {
				item.Active = false
				item.Reason = "agent_override_active"
			}
			if _, exists := agentSet[foldedID]; exists {
				item.Active = false
				item.Reason = "collision_with_bundled_agent"
				collisionErrs = append(collisionErrs, fmt.Sprintf("managed agent %q collides with bundled agent", entry.Identity))
			}
			if _, exists := managedAgents[foldedID]; exists {
				item.Active = false
				item.Reason = "duplicate_managed_agent"
				collisionErrs = append(collisionErrs, fmt.Sprintf("duplicate managed agent %q", entry.Identity))
			}
			managedAgents[foldedID] = struct{}{}
			snapshot.Agents = append(snapshot.Agents, item)
		case "skill":
			if input.SkillOverride {
				item.Active = false
				item.Reason = "skill_override_active"
			}
			if _, exists := skillSet[foldedID]; exists {
				item.Active = false
				item.Reason = "collision_with_bundled_skill"
				collisionErrs = append(collisionErrs, fmt.Sprintf("managed skill %q collides with bundled skill", entry.Identity))
			}
			if _, exists := managedSkills[foldedID]; exists {
				item.Active = false
				item.Reason = "duplicate_managed_skill"
				collisionErrs = append(collisionErrs, fmt.Sprintf("duplicate managed skill %q", entry.Identity))
			}
			managedSkills[foldedID] = struct{}{}
			snapshot.Skills = append(snapshot.Skills, item)
		}
	}
	for i := range snapshot.Agents {
		item := &snapshot.Agents[i]
		if item.Origin != "managed" || !item.Active {
			appendCatalogRecoveryDiagnostic(item)
			continue
		}
		entry, ok := findManifestEntry(input.Manifest, "agent", item.ID)
		if !ok {
			continue
		}
		for _, binding := range entry.SkillBindings {
			if catalogHasActiveSkill(snapshot.Skills, binding) {
				continue
			}
			item.Active = false
			item.Reason = "skill_dependency_unavailable"
			item.Diagnostics = append(item.Diagnostics, ActivationDiagnostic{
				Code:    "skill_dependency_unavailable",
				Message: fmt.Sprintf("skill %q from %s origin is unavailable; restore it or remove the binding with `prism agent skill remove %s %s`", binding.Name, binding.Origin, item.ID, binding.Name),
			})
			break
		}
		appendCatalogRecoveryDiagnostic(item)
	}
	for i := range snapshot.Skills {
		appendCatalogRecoveryDiagnostic(&snapshot.Skills[i])
	}
	sort.Slice(snapshot.Agents, func(i, j int) bool {
		return strings.ToLower(snapshot.Agents[i].ID) < strings.ToLower(snapshot.Agents[j].ID)
	})
	sort.Slice(snapshot.Skills, func(i, j int) bool {
		return strings.ToLower(snapshot.Skills[i].ID) < strings.ToLower(snapshot.Skills[j].ID)
	})
	if input.RejectCollisions && len(collisionErrs) > 0 {
		return snapshot, errors.New(strings.Join(collisionErrs, "; "))
	}
	return snapshot, nil
}

func catalogHasActiveSkill(items []CatalogItem, binding SkillBinding) bool {
	for _, item := range items {
		if item.Active && strings.EqualFold(item.ID, binding.Name) && (binding.Origin == "" || strings.EqualFold(item.Origin, binding.Origin)) {
			return true
		}
	}
	return false
}

func appendCatalogRecoveryDiagnostic(item *CatalogItem) {
	if item.Active || item.Reason == "" {
		return
	}
	for _, diagnostic := range item.Diagnostics {
		if diagnostic.Code == item.Reason {
			return
		}
	}
	message := fmt.Sprintf("%s %q is inactive (%s)", item.Origin, item.ID, item.Reason)
	if item.Origin == "managed" {
		message += fmt.Sprintf("; recover with `prism %s rename %s <new-name>` or `prism %s remove %s`", item.Kind, item.ID, item.Kind, item.ID)
	}
	item.Diagnostics = append(item.Diagnostics, ActivationDiagnostic{Code: item.Reason, Message: message})
}

func foldedSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[strings.ToLower(id)] = struct{}{}
	}
	return out
}

func bundledAgentIDs(root fs.FS) ([]string, error) {
	agents, err := fs.Sub(root, "agents")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("open bundled agents: %w", err)
	}
	entries, err := fs.ReadDir(agents, ".")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list bundled agents: %w", err)
	}
	ids := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" || entry.Name() == "README.md" {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".md"))
	}
	sort.Strings(ids)
	return ids, nil
}

func bundledSkillIDs(root fs.FS) ([]string, error) {
	skills, err := fs.Sub(root, "skills")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("open bundled skills: %w", err)
	}
	entries, err := fs.ReadDir(skills, ".")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list bundled skills: %w", err)
	}
	ids := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}
