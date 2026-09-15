package extensions

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bryanbarton525/prism/internal/agent"
)

func reviewAgent(id, constitution string) []byte {
	extra := ""
	if constitution != "" {
		extra = "\nconstitution_path: " + constitution
	}
	return []byte("---\nid: " + id + "\nname: Agent\ndescription: d\nmodel: local\ncontext_budget: 100\nallowed_skills: [demo]" + extra + "\n---\nbody")
}

func TestReviewLocalAgentAliasDryRunAndPackageConstitution(t *testing.T) {
	state := t.TempDir()
	source := filepath.Join(t.TempDir(), "agent-package")
	if err := os.MkdirAll(filepath.Join(source, "constitutions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "agent-package.md"), reviewAgent("original", "constitutions/rules.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "constitutions", "rules.md"), []byte("isolated constitution"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewLocalAgentService(state)
	if _, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source, As: "../escape"}); err == nil {
		t.Fatal("unsafe alias was accepted")
	}
	preview, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source, As: "preview", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(preview.ObjectPath); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote object: %v", err)
	}
	entry, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source, As: "aliased"})
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := NewStore(state).RecoverAndLoadManifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := ComposeCatalog(ComposeInput{
		BundleFS:        fstest.MapFS{"agents/README.md": {Data: []byte("base")}},
		Manifest:        manifest,
		ObjectStoreRoot: NewStore(state).ObjectRoot(),
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := MaterializeRuntimeBundle(fstest.MapFS{"agents/README.md": {Data: []byte("base")}}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(runtime, "agents/"+entry.Identity+".md")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := agent.Parse(data, entry.Identity+".md")
	if err != nil {
		t.Fatal(err)
	}
	text, sourceKind, err := spec.ResolveConstitution(runtime)
	if err != nil || sourceKind != "path" || text != "isolated constitution" {
		t.Fatalf("package constitution = %q, %q, %v", text, sourceKind, err)
	}
}

func TestReviewManagedObjectRequiresDigestAndStoreBoundary(t *testing.T) {
	store := NewStore(t.TempDir())
	digest, objectPath, err := store.PutObject(reviewAgent("managed", ""))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := ComposeCatalog(ComposeInput{
		BundleFS:        fstest.MapFS{"agents/README.md": {Data: []byte("base")}},
		ObjectStoreRoot: store.ObjectRoot(),
		Manifest: Manifest{Entries: []ManifestEntry{{
			Identity: "managed", Kind: "agent", Digest: digest, ObjectPath: objectPath,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeRuntimeBundle(fstest.MapFS{"agents/README.md": {Data: []byte("base")}}, snapshot); err != nil {
		t.Fatalf("valid object rejected: %v", err)
	}
	if err := os.WriteFile(objectPath, reviewAgent("managed", "")[:20], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeRuntimeBundle(fstest.MapFS{}, snapshot); err == nil {
		t.Fatal("modified object bypassed its manifest digest")
	}
}

func TestReviewMaterializationRejectsUnsafeManifestIdentities(t *testing.T) {
	base := fstest.MapFS{"agents/README.md": {Data: []byte("base")}}
	for _, kind := range []string{"agent", "skill"} {
		t.Run(kind, func(t *testing.T) {
			snapshot := CatalogSnapshot{}
			item := CatalogItem{ID: "../escape", Origin: "managed", Active: true}
			if kind == "agent" {
				snapshot.Agents = []CatalogItem{item}
			} else {
				snapshot.Skills = []CatalogItem{item}
			}
			if _, err := MaterializeRuntimeBundle(base, snapshot); err == nil {
				t.Fatal("unsafe managed identity escaped its runtime namespace")
			}
		})
	}
}

func TestReviewSkillValidationAndDryRun(t *testing.T) {
	state := t.TempDir()
	source := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: demo\ndescription: d\n---"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewLocalSkillService(state)
	if _, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: source, As: "../bad"}); err == nil {
		t.Fatal("unsafe skill alias was accepted")
	}
	entries, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: source, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(entries[0].ObjectPath); !os.IsNotExist(err) {
		t.Fatalf("skill dry run wrote object: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: source}); err == nil {
		t.Fatal("invalid skill package was accepted")
	}
}

func TestReviewTransactionCommittedJournalAndMCPAccessValidation(t *testing.T) {
	store := NewStore(t.TempDir())
	before := Manifest{Version: ManifestVersion, Entries: []ManifestEntry{{Identity: "before", Kind: "skill"}}}
	after := Manifest{Version: ManifestVersion, Entries: []ManifestEntry{{Identity: "after", Kind: "skill"}}}
	if err := store.SaveManifest(before); err != nil {
		t.Fatal(err)
	}
	if err := store.writeJournal(before); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveManifest(after); err != nil {
		t.Fatal(err)
	}
	if err := store.markJournalCommitted(); err != nil {
		t.Fatal(err)
	}
	if recovered, err := store.Recover(context.Background()); err != nil || recovered {
		t.Fatalf("committed journal recovery = %t, %v", recovered, err)
	}
	got, err := store.LoadManifest()
	if err != nil || got.Entries[0].Identity != "after" {
		t.Fatalf("committed manifest rolled back: %#v, %v", got, err)
	}
	if err := SaveMCPAccess(t.TempDir(), MCPAccessState{Agents: map[string]MCPAccessRule{"agent": {Mode: "typo"}}}); err == nil {
		t.Fatal("unknown MCP access mode was accepted")
	}
}

func TestReviewDirectoryDigestRejectsSymlinksAndSpecialEntries(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := DigestDirectory(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink digest error = %v", err)
	}
}

func TestRuntimeBatchPublishesSkillAgentBindingAndAccessAsOneSnapshot(t *testing.T) {
	state := t.TempDir()
	skill, err := PlanSkillObject(fstest.MapFS{"SKILL.md": {Data: []byte("---\nname: demo\ndescription: d\n---\nbody")}}, ".", "demo", "local", "", "demo", "", false)
	if err != nil {
		t.Fatal(err)
	}
	target := RuntimeTarget{Engine: "ollama", Model: "local/model"}
	agentObject, err := PlanTranslatedAgentObject(TranslatedAgentInstall{
		Name: "worker.md", Normalized: reviewAgent("worker", ""),
		Request: InstallLocalAgentRequest{Runtime: &target, SkillBindings: []SkillBinding{{Name: "demo", Origin: "managed"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	access := MCPAccessState{Agents: map[string]MCPAccessRule{"worker": {Mode: MCPAccessModeNone}}}
	if _, err := ActivateRuntimeBatch(context.Background(), state, RuntimeBatchRequest{Objects: []RuntimeBatchObject{skill, agentObject}, MCPAccess: &access}); err != nil {
		t.Fatal(err)
	}
	manifest, err := NewStore(state).LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 2 || manifest.MCPAccess == nil || manifest.MCPAccess.Agents["worker"].Mode != MCPAccessModeNone {
		t.Fatalf("published snapshot = %#v", manifest)
	}
	if manifest.Entries[1].SkillBindings[0].Origin != "managed" {
		t.Fatalf("binding origin not persisted: %#v", manifest.Entries[1])
	}
}

func TestRuntimeBatchPublicationFailureRestoresWholeSnapshot(t *testing.T) {
	state := t.TempDir()
	store := NewStore(state)
	before := Manifest{Version: ManifestVersion, Entries: []ManifestEntry{{Identity: "before", Kind: "skill"}}}
	if err := store.SaveManifest(before); err != nil {
		t.Fatal(err)
	}
	tx, err := store.BeginTransaction(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tx.UpsertEntry(ManifestEntry{Identity: "after", Kind: "agent"})
	if err := tx.StageMCPAccess(MCPAccessState{Agents: map[string]MCPAccessRule{"after": {Mode: MCPAccessModeNone}}}); err != nil {
		t.Fatal(err)
	}
	writes := 0
	store.writeManifest = func(path string, data []byte) error {
		writes++
		if writes == 1 {
			return errors.New("injected publication failure")
		}
		return writeFileAtomically(path, data)
	}
	if err := tx.Commit(); err == nil {
		t.Fatal("expected publication failure")
	}
	got, err := store.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Identity != "before" || got.MCPAccess != nil {
		t.Fatalf("partial snapshot survived: %#v", got)
	}
}

func TestRuntimeBatchExistingAgentRejectsConcurrentChangeAndIdenticalNoop(t *testing.T) {
	state := t.TempDir()
	object, err := PlanTranslatedAgentObject(TranslatedAgentInstall{Name: "worker.md", Normalized: reviewAgent("worker", ""), Request: InstallLocalAgentRequest{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ActivateRuntimeBatch(context.Background(), state, RuntimeBatchRequest{Objects: []RuntimeBatchObject{object}}); err != nil {
		t.Fatal(err)
	}
	before, err := NewStore(state).LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ActivateRuntimeBatch(context.Background(), state, RuntimeBatchRequest{Objects: []RuntimeBatchObject{object}}); err != nil {
		t.Fatal(err)
	}
	afterNoop, err := NewStore(state).LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !afterNoop.Entries[0].InstalledAt.Equal(before.Entries[0].InstalledAt) {
		t.Fatal("identical activation changed installation metadata")
	}
	first, err := PlanExistingAgentUpdate(state, "worker", "First", nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PlanExistingAgentUpdate(state, "worker", "Second", nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ActivateRuntimeBatch(context.Background(), state, RuntimeBatchRequest{Objects: []RuntimeBatchObject{second}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ActivateRuntimeBatch(context.Background(), state, RuntimeBatchRequest{Objects: []RuntimeBatchObject{first}}); err == nil || !strings.Contains(err.Error(), "changed after planning") {
		t.Fatalf("stale update error = %v", err)
	}
}
