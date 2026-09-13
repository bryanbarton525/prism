package extensions

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestComposeCatalogIncludesBundledAndManaged(t *testing.T) {
	bundle := fstest.MapFS{
		"agents/a.md":          {Data: []byte("x")},
		"agents/README.md":     {Data: []byte("ignore")},
		"skills/s1/SKILL.md":   {Data: []byte("x")},
		"skills/s2/SKILL.md":   {Data: []byte("x")},
		"constitutions/a.md":   {Data: []byte("x")},
	}
	now := time.Unix(100, 0).UTC()
	snapshot, err := ComposeCatalog(ComposeInput{
		BundleFS: bundle,
		Manifest: Manifest{Version: 1, Entries: []ManifestEntry{
			{Identity: "managed-agent", Kind: "agent", Source: "git", Digest: "d1", ObjectPath: "extensions/objects/d1"},
			{Identity: "managed-skill", Kind: "skill", Source: "git", Digest: "d2", ObjectPath: "extensions/objects/d2"},
		}},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.CreatedAt.Equal(now) {
		t.Fatalf("created_at = %v", snapshot.CreatedAt)
	}
	if len(snapshot.Agents) != 2 || len(snapshot.Skills) != 3 {
		t.Fatalf("agents=%d skills=%d", len(snapshot.Agents), len(snapshot.Skills))
	}
}

func TestComposeCatalogPreservesUpgradeCollisionsWithoutStartupError(t *testing.T) {
	bundle := fstest.MapFS{
		"agents/MyAgent.md":     {Data: []byte("x")},
		"skills/MySkill/SKILL.md": {Data: []byte("x")},
	}
	snapshot, err := ComposeCatalog(ComposeInput{
		BundleFS: bundle,
		Manifest: Manifest{Version: 1, Entries: []ManifestEntry{
			{Identity: "myagent", Kind: "agent", Source: "git", Digest: "d1", ObjectPath: "x"},
			{Identity: "myskill", Kind: "skill", Source: "git", Digest: "d2", ObjectPath: "y"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected startup collision error: %v", err)
	}
	inactive := 0
	for _, item := range append(snapshot.Agents, snapshot.Skills...) {
		if item.Origin == "managed" && !item.Active && strings.HasPrefix(item.Reason, "collision_with_bundled_") {
			inactive++
		}
	}
	if inactive != 2 {
		t.Fatalf("expected inactive collision entries, got %d", inactive)
	}
}

func TestComposeCatalogRejectsCollisionsWhenExplicitlyRequested(t *testing.T) {
	bundle := fstest.MapFS{
		"agents/MyAgent.md": {Data: []byte("x")},
	}
	_, err := ComposeCatalog(ComposeInput{
		BundleFS:          bundle,
		RejectCollisions: true,
		Manifest: Manifest{Version: 1, Entries: []ManifestEntry{
			{Identity: "myagent", Kind: "agent", Source: "git", Digest: "d1", ObjectPath: "x"},
		}},
	})
	if err == nil {
		t.Fatal("expected explicit collision error")
	}
}

func TestComposeCatalogMarksManagedInactiveWhenOverridesSet(t *testing.T) {
	bundle := fstest.MapFS{
		"agents/a.md":        {Data: []byte("x")},
		"skills/s/SKILL.md":  {Data: []byte("x")},
	}
	snapshot, err := ComposeCatalog(ComposeInput{
		BundleFS:      bundle,
		AgentOverride: true,
		SkillOverride: true,
		Manifest: Manifest{Version: 1, Entries: []ManifestEntry{
			{Identity: "magent", Kind: "agent", Source: "git", Digest: "d1", ObjectPath: "x"},
			{Identity: "mskill", Kind: "skill", Source: "git", Digest: "d2", ObjectPath: "y"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var inactive int
	for _, item := range snapshot.Agents {
		if item.Origin == "managed" && !item.Active && item.Reason == "agent_override_active" {
			inactive++
		}
	}
	for _, item := range snapshot.Skills {
		if item.Origin == "managed" && !item.Active && item.Reason == "skill_override_active" {
			inactive++
		}
	}
	if inactive != 2 {
		t.Fatalf("inactive managed count = %d", inactive)
	}
}
