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
	ID         string                 `json:"id"`
	Origin     string                 `json:"origin"` // bundled | managed
	Active     bool                   `json:"active"`
	Digest     string                 `json:"digest,omitempty"`
	Source     string                 `json:"source,omitempty"`
	ObjectPath string                 `json:"object_path,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Metadata   map[string]string      `json:"metadata,omitempty"`
	Diagnostics []ActivationDiagnostic `json:"diagnostics,omitempty"`
}

type ComposeInput struct {
	BundleFS      fs.FS
	Manifest      Manifest
	AgentOverride bool
	SkillOverride bool
	Now           func() time.Time
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
		snapshot.Agents = append(snapshot.Agents, CatalogItem{ID: id, Origin: "bundled", Active: true})
	}
	for _, id := range bundledSkills {
		snapshot.Skills = append(snapshot.Skills, CatalogItem{ID: id, Origin: "bundled", Active: true})
	}

	agentSet := foldedSet(bundledAgents)
	skillSet := foldedSet(bundledSkills)
	var collisionErrs []string
	for _, entry := range input.Manifest.Entries {
		item := CatalogItem{
			ID:          entry.Identity,
			Origin:      "managed",
			Active:      true,
			Digest:      entry.Digest,
			Source:      entry.Source,
			ObjectPath:  entry.ObjectPath,
			Diagnostics: append([]ActivationDiagnostic{}, entry.Diagnostics...),
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
			snapshot.Skills = append(snapshot.Skills, item)
		}
	}
	sort.Slice(snapshot.Agents, func(i, j int) bool { return strings.ToLower(snapshot.Agents[i].ID) < strings.ToLower(snapshot.Agents[j].ID) })
	sort.Slice(snapshot.Skills, func(i, j int) bool { return strings.ToLower(snapshot.Skills[i].ID) < strings.ToLower(snapshot.Skills[j].ID) })
	if len(collisionErrs) > 0 {
		return snapshot, errors.New(strings.Join(collisionErrs, "; "))
	}
	return snapshot, nil
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
