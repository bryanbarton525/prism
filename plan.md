# Plan: CLI management of MCP servers, skills, and agents

Status: Sol correctness work published to draft PR #32 on 2026-09-15 and locally
verified. The Spark fixture/contract/CI/documentation queue and remaining PR
review reconciliation remain before the plan can be declared complete.

Phase 2 collision/recovery and phase 5 guided-install checkboxes were reconciled
with implemented Sol behavior on 2026-09-15. The recovery regression verifies
that management does not require a runnable catalog and rename/remove preserve
an old immutable object. `go test ./...`, `go vet ./...`,
`scripts/ci-check.sh`, `git diff --check`, the pinned Graphify
`uv lock --check`, a Windows binary cross-build, and Windows CLI/app/extensions
test-binary compilation pass on the current worktree. Commit `9504f84` was
published to PR #32; its Linux CI and CodeQL checks passed. At least fourteen
focused review threads were replied to with test evidence and resolved. Spark acceptance
coverage and the remaining review threads are still open. A read-only Codex CLI
probe of `gpt-5.3-codex-spark` on 2026-09-15 still returned HTTP 400 with the
current ChatGPT login; no Spark-assigned task was run under a substitute model.

## Implementation status and remaining work

Implemented and locally verified:

- Downstream MCP lifecycle, transports, credential references, bounded calls,
  cross-platform advisory locks, and legacy command compatibility.
- Immutable runtime-extension storage, recovery journals, effective catalog
  composition, portable skills/resources, deterministic agent adapters,
  explicit runtime targets, lifecycle/bindings, zero-skill managed agents,
  per-agent MCP access, and immutable GitHub provenance.
- The pinned Graphify bundle, fixed tool contract, source verification,
  managed `uv` environment, explicit setup/removal/doctor probe, host routing,
  fixtures, evaluation, and optional live smoke path.
- `go test ./...`, `go vet ./...`, `scripts/ci-check.sh`,
  `uv lock --check --project skills/graphify-query/references/managed-environment`,
  a Windows binary cross-build, and Windows test-binary compilation (verified
  2026-09-14).

Sol work completed:

1. Runtime activation now publishes skills, agents, durable binding origins,
   and MCP access through one versioned manifest snapshot. Fault-injection
   coverage proves publication failure restores the whole prior snapshot.
2. Guided and non-interactive runtime setup now support existing managed agents,
   installed/new skill bindings, default/custom/none MCP access, interactive or
   file-based import decisions, local/GitHub package discovery with support
   files, and copy/import display names. Dry runs validate the complete batch.
3. Catalog, top-level list/show, MCP inspection, and doctor now preserve and
   render inactive collision/dependency states with exact recovery diagnostics;
   persisted dependency origin prevents accidental rebinding to bundled items.
   Catalog-backed skill CLI inspection now fails with the underlying recovery
   or object-integrity error instead of silently showing embedded-only content;
   manifest-only managed repair commands remain available.
4. A documented outer runtime-configuration lock serializes access policy,
   downstream server mutation/removal, and Graphify reference/binding changes.
   Repeated concurrent-operation tests verify no updates are lost.
5. Guided Graphify setup discovers executable and self-hosted candidates without
   invoking or contacting them, retains explicit ownership selection, removes a
   newly created managed environment after install failure, and refuses drifted
   owned removal.

Remaining before completion:

1. Run the Spark fixture, output/help, import/resource matrix, Windows/release,
   and documentation tasks listed below.
2. Finish reviewing and resolving addressed comments on draft PR #32 with
   evidence, request fresh review, and confirm final-head Linux/Windows checks.
   Obsolete empty-diff draft PR #34 was closed on 2026-09-15. PR #31 remains
   the foundation PR.

## Remaining-work assignment by model

Use **Sol** for correctness-sensitive state transitions, concurrency, catalog
semantics, and guided workflow design. Use **Spark** for bounded fixtures,
contract coverage, help/output polish, CI, and documentation. Workers must stay
inside their assigned primary files unless a failing test demonstrates a small,
necessary adjacent fix. Only the final integrator updates this plan's completion
checkboxes.

### Sol queue

#### SOL-1 — Atomic runtime activation foundation

**Status: complete (2026-09-14).** MCP access was moved into the versioned
manifest so publication has one reader-visible boundary; batch and injected
publication-failure tests cover the full skill/agent/binding/access snapshot.

**Depends on:** nothing. This blocks SOL-2.

**Primary files:**

- `internal/extensions/transaction.go`
- `internal/extensions/local_agent.go`
- `internal/extensions/local_skill.go`
- `internal/extensions/mcp_access.go`
- new runtime-batch service files and fault-injection tests

**Task brief:** Implement remaining-work item 2. Create one runtime activation
transaction covering selected skills, agents, bindings, and MCP-access changes.
Stage immutable objects first, validate the complete candidate state, and publish
one recoverable transition. A crash or forced exit must not expose a partial
batch. Readers must not observe candidate state while rollback is still
possible. Preserve existing process snapshots and immutable objects. Add tests
for replacement, validation failure, concurrent mutation, publication failure,
and interrupted recovery. Do not redesign the guided CLI beyond the minimum
service API required by SOL-2.

**Exit evidence:** service-level tests prove all-or-nothing activation and crash
recovery for a batch containing at least a skill, agent, binding, and access
rule.

#### SOL-2 — Guided installer parity

**Status: complete (2026-09-14).** Direct flags and the walkthrough share the
same batch planning path, including existing-agent edits, package support files,
import decisions, display names, binding origins, MCP access, and validating
dry-run previews.

**Depends on:** SOL-1.

**Primary files:**

- `internal/cli/install.go`
- `internal/cli/install_runtime_test.go`
- runtime-batch APIs created by SOL-1

**Task brief:** Complete the unchecked guided-install gate. Bring `prism
install` to parity with direct agent, skill, binding, model, import-config, and
MCP-access commands. Support selecting an existing managed agent, selecting
already-installed skills, editing the proposed allowlist, selecting
default/custom/none MCP access, resolving every import finding, importing local
or GitHub agent packages with support files, and choosing a display name for a
bundled copy. Prepare one complete preview before applying the atomic runtime
batch. Keep `--all` and `--yes` from selecting sources, granting capabilities,
accepting translation loss, or installing Graphify. Preserve accurate separate
host/runtime/dependency outcomes.

**Exit evidence:** scripted tests cover success, retry, EOF, cancellation, host
failure, runtime failure, partial host success, and runtime-only retry; no
cancellation path mutates active state.

#### SOL-3 — Inactive and conflicting catalog recovery

**Status: complete (2026-09-14).** Active/inactive state and diagnostics now
flow through CLI, MCP catalog output, and doctor, with origin-sensitive
dependency disabling and explicit rename/removal recovery.

**Depends on:** none, but rebase after SOL-1 before merge.

**Primary files:**

- `internal/extensions/catalog.go`
- `internal/cli/agent.go`
- `internal/cli/skill.go`
- `internal/app/runner.go`
- MCP catalog and doctor handlers

**Task brief:** Complete remaining-work item 3. Unify active and inactive
catalog inspection across top-level CLI, MCP, and doctor without requiring a
runnable catalog. Show origin, active state, collision and dependency-disabled
diagnostics, plus exact rename/removal recovery commands. A future bundle-name
collision must preserve the managed object, leave the bundled item usable,
disable dependent managed agents without rebinding, and keep management
commands operational. Development overrides must remain distinguishable.

**Exit evidence:** future-bundle-collision, dependency-origin, development
override, list/show, doctor, and recovery-command tests pass.

#### SOL-4 — Cross-file MCP configuration concurrency

**Status: complete (2026-09-14).** The outer `runtime-config.lock` establishes
the lock order before extension and downstream stores; concurrent defaults,
custom access, Graphify binding, server replacement, and removal preserve every
update.

**Depends on:** none; coordinate lock ordering with SOL-1.

**Primary files:**

- `internal/downstreammcp/*`
- `internal/extensions/mcp_access.go`
- `internal/cli/mcp.go`
- Graphify-reference coordination in `internal/cli/graphify.go`

**Task brief:** Complete remaining-work item 4. Establish a documented lock
ordering or shared configuration transaction so downstream MCP removal and its
access-policy/Graphify reference checks are atomic relative to concurrent
updates. Prevent lost updates and deadlocks. Preserve unrelated servers and
rules. Dry runs remain non-mutating and endpoint connections remain explicit.

**Exit evidence:** deterministic concurrency tests cover simultaneous default
access, custom access, Graphify binding, server update, and removal operations.

#### SOL-5 — Graphify discovery and lifecycle ownership

**Status: complete (2026-09-14).** Offline discovery exposes user-managed PATH
and configured HTTP candidates for explicit selection. Managed failure cleanup,
drift refusal, dry-run behavior, and external-resource preservation are covered.

**Depends on:** SOL-2. Incorporate SPARK-1 fixtures.

**Primary files:**

- `internal/cli/graphify.go`
- `internal/graphify/*`
- Graphify integration points in `internal/cli/install.go`

**Task brief:** Complete remaining-work item 5. Detect compatible
user-managed Graphify executables and already configured self-hosted endpoints,
then expose them as explicit guided choices without taking ownership. Strengthen
managed registration/environment drift checks, retry diagnostics, and exact
owned removal. Listing, discovery, doctor without `--probe`, dry run, `--all`,
and `--yes` must not download a dependency, build an index, contact an endpoint,
select a cloud backend, or invoke a model.

**Exit evidence:** user-managed, self-hosted, managed, drift, cancellation,
failure, dry-run, and uninstall-ownership fixtures pass.

### Spark queue

#### SPARK-1 — Graphify lifecycle fixtures

**Can run with:** SOL-1. Rebase before SOL-5 integration.

**Primary files:** `internal/cli/graphify_test.go` and isolated Graphify test
helpers.

**Task brief:** Add deterministic fake-`uv` tests for cancellation, command
failure, missing system Python, missing produced executable, dry-run filesystem
invariants, successful exact owned removal, and refusal to remove a drifted
registration. Use temporary directories only. Do not access public Graphify,
install Python packages, or invoke a model. Limit production edits to small
defects directly exposed by a fixture; report architectural issues to SOL-5.

**Exit evidence:** the expanded test set passes without network or credentials.

#### SPARK-2 — CLI JSON and help contracts

**Can run with:** SOL-1 and SOL-4. Rebase after SOL-2 before final merge.

**Primary files:** CLI tests and CLI help/output strings; do not change
transaction or catalog internals.

**Task brief:** Audit runtime-extension commands against this plan. Add
table-driven coverage for stable JSON fields, dry-run actions, `agent`/`agents`
and `skill`/`skills` aliases, selector errors, explicit model selection,
resource flags, import-config diagnostics, and restart-required mutation
messages. Restrict production changes to output/help formatting.

**Exit evidence:** direct and compatibility forms have deterministic output and
nonzero failures in both text and JSON modes.

#### SPARK-3 — Agent import fixture matrix

**Can run with:** SOL-1.

**Primary files:**

- `internal/agent/importer/*_test.go`
- `internal/agent/importconfig/*_test.go`
- import fixtures under `testdata`

**Task brief:** Cover multiline Claude instructions, Codex declared names that
differ from filenames, YAML-sensitive skill values, mixed-format ambiguity,
missing execution targets, source-cloud-model provenance, explicit field
mapping and omission, stale source and adapter digests, native support files,
and exclusion of `AGENTS.md`. Prefer tests and make only isolated adapter fixes
revealed by them.

**Exit evidence:** every Agent translation acceptance-matrix clause maps to a
named deterministic test and translation performs no model call.

#### SPARK-4 — Skill resource acceptance matrix

**Can run with:** SOL-1.

**Primary files:**

- `internal/skill/*_test.go`
- `internal/app/*_test.go`
- `internal/mcp/*_test.go`
- `internal/graph/*_test.go`

**Task brief:** Complete coverage for nested resource paths, media type and
size, UTF-8 boundaries, offset/truncation metadata, binary rejection, traversal,
escaping symlinks, the 32 KiB per-read limit, the 128 KiB per-run limit,
attached-skill enforcement, and explicit CLI/MCP/graph attachments for
non-tool models. Avoid redesigning the resource API; report architectural
issues for Sol work.

**Exit evidence:** every Resource reads acceptance-matrix clause maps to a named
test.

#### SPARK-5 — Windows and release automation

**Can run with:** all code work, but merge after core branches stabilize.

**Primary files:** `.github/workflows/ci.yml`, `scripts/ci-check.sh`, and release
verification documentation.

**Task brief:** Add deterministic Linux and Windows release verification. Run
the normal Go suite on both platforms where supported, validate the embedded
Graphify lock without installing its dependencies, and keep public MCP,
Graphify, and inference smoke checks optional. Never commit generated binaries
or require credentials for required CI.

**Exit evidence:** CI configuration validates, Linux checks pass, Windows tests
run or compile as documented, and `uv lock --check` passes.

#### SPARK-6 — Documentation final audit

**Depends on:** merged SOL-2, SOL-3, SOL-4, and SOL-5 behavior.

**Primary files:** README, usage, model-runtime, agent/skill authoring,
acceptance matrix, and recovery/setup documentation.

**Task brief:** Audit public documentation against the final command help and
behavior. Cover supported sources/formats, scope, activation/restart semantics,
model selection, translation losses, resources, rename/conflict recovery,
atomic and staged outcomes, MCP access, and Graphify ownership. Verify every
documented command against `--help` and every named test against the repository.
Do not mark a plan gate complete without automated evidence.

**Exit evidence:** documentation tests and the complete repository suite pass;
no stale future-tense or unsupported-behavior claims remain.

### Integration and merge order

1. Merge SOL-1.
2. Rebase and merge SOL-4 using the lock order established with SOL-1.
3. Rebase and merge SOL-3.
4. Merge SPARK-1, SPARK-3, and SPARK-4 after conflict-free rebases.
5. Build and merge SOL-2 on SOL-1.
6. Build and merge SOL-5 on SOL-2, incorporating SPARK-1 fixtures.
7. Rebase and merge SPARK-2.
8. Merge SPARK-5 and then SPARK-6.
9. Run the full acceptance audit and update the two remaining phase checkboxes.
10. Commit/push PR #32 and resolve addressed review threads with test evidence.
    Obsolete empty-diff draft PR #34 was closed on 2026-09-15.

Parallel workers must not edit `plan.md`, shared acceptance-matrix rows, or PR
state. They return a summary containing files changed, tests run, unresolved
risks, and the exact commit to integrate. The final integrator owns conflict
resolution, full-suite verification, plan status, and PR reconciliation.

User-reviewed product decisions are captured below. Operational choices for failure handling, upgrade recovery, and release sequencing were finalized during consolidation. See [CONTEXT.md](CONTEXT.md) for terminology and [the boundary decision](docs/adr/0001-runtime-extensions.md) for rationale.

## Confirmed review decisions

- Additions extend Prism's own runtime. Exporting user additions into editor hosts is deferred.
- The release bundle remains immutable. Runtime extensions are tracked separately and must use names unique within their resource kind; replacing bundled agents or skills is deferred.
- `--replace` may update an existing runtime extension, but cannot bypass a collision with bundled content.
- New skill bindings apply only to user-added agents. Bundled agents retain their release-defined allowlists and behavior.
- User-added agents may have an empty skill allowlist and run with zero attached skills. Explicitly attached skills must still be allowed. Bundled agents retain their existing requirement for at least one skill.
- The `prism install` walkthrough must help users set up runtime extensions and bind skills to user-added agents. This assistance does not imply exporting extensions into host directories.
- The walkthrough offers copying a bundled agent into a uniquely named user-added agent alongside importing a native Prism agent. The copy retains its starting configuration and constitution independently of later bundle updates.
- Runtime additions default to user-wide configuration (`~/.prism` unless explicitly configured otherwise). The walkthrough offers explicit project isolation and configures the host's MCP command to use the selected runtime configuration.
- Skills use the Agent Skills open format at agentskills.io, with no required Prism-specific fields or directory layout beyond that standard.
- Initial agent import supports native Prism, Claude Code, and Codex agent definitions through translation into Prism's runtime representation.
- Users may complete an import after explicitly mapping or omitting unsupported features and accepting the resulting behavior changes. Unresolved incompatibilities prevent activation; accepted changes remain visible in provenance.
- Agent import requires an explicitly selected local or self-hosted offload model, including inference on the user's other machines. Self-hosted endpoint support already exists and must be preserved. A source agent's cloud model or inherited host model must not become the execution target by default. Installation and translation make no model calls; only specialist task execution invokes the selected offload model.
- User-added agents support per-agent MCP server selection and a general default option. The general default is a user-selected shared list; agents using it inherit list changes. Registering another server does not automatically add it to that list. The walkthrough assists with both; bundled agents retain existing behavior.
- Configuration changes take effect on the next direct CLI invocation or after restarting a running Prism MCP server. Live reload is deferred; active tasks retain their loaded configuration.
- The initial release supports bounded reading of skill resources. A new general-purpose skill script executor is deferred; compatibility reports describe that limitation.
- Ship local/GitHub and GitHub-backed skills.sh page installation initially; catalog search is a follow-on release.
- The walkthrough uses separate host-install and runtime-extension transactions with explicit outcomes. Upgrade conflicts preserve user content and disable affected extensions until repaired.
- Include Graphify integration in the immutable Prism release bundle: a repository-investigation specialist, its constitution and query skill, and host-facing delegation instructions. Graphify execution dependencies and repository indexes have explicit setup and readiness checks; installing a skill alone does not activate an MCP server.

## Outcome

Let users register downstream MCP servers and install skills and Prism agents with a command, then use them through both the CLI and Prism's MCP server. Keep Prism's embedded defaults available and make installed content persistent, inspectable, and removable.

```sh
prism mcp add openaiDeveloperDocs --url https://developers.openai.com/mcp
prism mcp add local-tools -- npx -y @example/mcp-server
prism mcp list
prism mcp tools openaiDeveloperDocs

prism skills add vercel-labs/agent-skills --skill web-design-guidelines
prism skills add https://skills.sh/vercel-labs/agent-skills/web-design-guidelines
prism skills add ./my-skills --list
prism skills list

prism agents add ./my-prism-agents --agent reviewer --use-configured-model
prism agents add owner/repo --agent reviewer --ref v1.0.0 --model <local-model-id>
prism agent show reviewer
```

The package and agent names in generic examples are placeholders. All commands above describe the planned CLI, not currently implemented behavior. Angle-bracket values are placeholders and must be replaced before running examples. `skill` and `agent` remain canonical commands; `skills` and `agents` become aliases.

## Existing implementation and constraints

This plan follows the current working tree, including the ongoing embedded-bundle and installer changes. Preserve that work; do not restore the deleted bundle marketplace implementation.

| Area | Current behavior | Planned extension |
| --- | --- | --- |
| `internal/cli/mcp.go` | `mcp server add-command`, `add-sse`, `list`, `tools`, and `call` exist | Friendly top-level commands sharing existing handlers/services |
| `internal/downstreammcp/state.go` | Stores servers in YAML; supports `command` and `sse`; save writes directly; add silently upserts | Streamable HTTP, validation, atomic mutation, explicit replacement |
| `internal/downstreammcp/client.go` | Launches command servers or connects through SSE | Use SDK Streamable HTTP transport for URL additions |
| `internal/config/config.go` | Resolves flags/environment/config.env settings; state defaults to `~/.prism` | Reuse the resolved state directory for installed extensions |
| `internal/cli/skill.go` | Lint/test/benchmark; embedded skills unless an explicit directory replaces them | Add/list/show/remove plus one shared installed-content resolver |
| `internal/cli/agent.go`, `internal/app/runner.go` | Embedded agents/skills with replacement development directories | Merge managed additions into the runtime catalog |
| `internal/agent/spec.go` | Prism-specific frontmatter, skill allowlist, constitution resolution | Validate imported agents and preserve their dependencies |
| `internal/skill/discover.go` | Structure validation requires references/scripts/evals | Separate portable skill validity from Prism authoring conventions |
| `internal/rootresolver` | Resolves local/GitHub workspace content | Reuse suitable parsing/auth helpers; add immutable installation resolution |

The pinned MCP Go SDK (`v1.6.1`) already includes `StreamableClientTransport`. Check its behavior against local protocol tests before considering an SDK upgrade. The existing knowledge graph helps locate components, but current source is authoritative because the graph predates working-tree changes.

## Scope and decisions

1. Implement native Go installation. Users can follow the familiar `npx skills add` style without Prism invoking that CLI or requiring Node. An explicitly configured MCP command may still use `npx` at runtime.
2. Default runtime additions to user-wide configuration, honoring the effective `--state-dir` / `PRISM_STATE_DIR` / configured state directory. Offer explicit project isolation in the walkthrough, backed by `<project>/.prism`; direct commands use `--state-dir <project>/.prism`. The isolated runtime includes the release bundle and that configuration's additions, without merging user-wide extensions or MCP server entries. Do not introduce automatic project discovery in this release.
3. Retain `config.env` for runtime settings and `mcp-servers.yaml` for MCP servers. Add a versioned extension manifest; do not rewrite `.env` to point at individual installations.
4. Install standard Agent Skills directly. Accept native Prism agents and an explicitly supported initial set of external agent formats through import adapters. Keep Prism-specific execution settings in the normalized agent configuration rather than requiring users to rewrite source files. Do not infer equivalent tool permissions or model behavior from similar names.
5. Installation makes skills available. It does not attach them to every invocation or automatically expand any agent's `allowed_skills`. Explicit binding commands and the install walkthrough may change allowlists only for user-added agents.
6. Runtime processes use a startup snapshot. New direct CLI invocations load current configuration; running `prism mcp serve` processes require restart. Add/remove/replace, skill bindings, MCP access/defaults, and model-target changes all follow this contract. Successful mutations report the affected runtime location and restart requirement without claiming a running server already applied the change. Live reload is deferred.

## CLI contract

### MCP servers

```sh
prism mcp add <name> --url <url> [--transport http|sse]
prism mcp add <name> [options] -- <command> [args...]
prism mcp list
prism mcp show <name>
prism mcp remove <name>
prism mcp tools <name> [--schema]
prism mcp call <name> <tool> --args-json '{}'
```

- URL additions default to `http`, meaning Streamable HTTP, with explicit `sse` for legacy servers. Keep stored `command` and `sse` values compatible. Do not guess SSE from a URL suffix or silently switch transports after authentication failures.
- Preserve argv after `--` exactly and launch without a shell. Reject URL/command combinations, empty names, unsupported schemes, malformed absolute URLs, and explicitly nonpositive limits. Support HTTP for local endpoints as well as HTTPS.
- Share `--timeout-ms`, `--max-bytes`, `--description`, `--dry-run`, and `--replace`. Identical additions are a no-op; conflicting existing entries fail unless replacement is requested.
- Add environment references for authentication/configuration: `--env-from KEY=ENV_VAR` for command processes and `--header-from Header-Name=ENV_VAR` for HTTP. Persist reference names, resolve values at connection time, and redact resolved values from all output/errors. Missing variables fail only when connecting, so offline configuration is possible.
- Registration validates configuration without starting a process or contacting the endpoint. `mcp tools` is the explicit connectivity check.
- Keep existing `mcp server ...` forms functional. Preserve legacy upsert behavior initially, document it, and have old and new commands share validation and persistence code. Do not change `mcp serve` into an installation command.
- Mutate persisted entries only; do not serialize the synthesized Linear entry from `PRISM_LINEAR_MCP_URL`. Show its origin in list/show; removal of an environment-only entry explains how to remove its configuration.

### Skills and agents

```sh
prism skill add <source> [--skill <name> ... | --all | --list] [--ref <ref>]
prism skill list
prism skill show <name>
prism skill remove <name>

prism agent add <source> [--agent <id> ... | --all | --list] [--ref <ref>]
  [--format prism|claude|codex] [--model <id> | --use-configured-model]
  [--import-config <file>] [--path <subpath>] [--as <new-id>]
prism agent remove <id>
prism agent copy <bundled-agent-id> <new-agent-id>
prism agent skill add <agent-id> <skill-name>
prism agent skill remove <agent-id> <skill-name>
prism agent model set <agent-id> (--model <id> | --use-configured-model)
prism agent rename <old-id> <new-id>
prism skill rename <old-name> <new-name>
```

- Agent additions require either `--model`, `--use-configured-model`, or an explicit model selection in `--import-config`; `--list` is exempt. `--use-configured-model` fails if the effective runtime has no model configured. These commands use the existing configured runtime endpoint; model selection is validated against the effective target, including any global model override. Batch additions may share an explicitly supplied model.
- `--as` renames a single selected item; support it for skills too. Update identity fields and filenames in the managed copy while retaining original bytes/provenance. Reject `--as` with multiple selections.
- Single-item sources install directly. Multi-item sources require explicit selectors or `--all`; otherwise print candidates and return an actionable error. `--list` only discovers candidates. This is deterministic in terminals and CI without a mandatory prompt flow.
- Add/remove/binding mutations support `--dry-run`; additions support `--replace`. All new commands honor global `--json`, provide stable fields (kind, ID, source, revision, digest, destination, action), and return nonzero on errors even in JSON mode.
- List/show include embedded, installed, and development origins. Removing or replacing an embedded item through extension commands is rejected, including with `--replace`.
- MCP server removal fails while it is explicitly referenced by a custom selection or the shared default; list those references and require explicit removal first. An environment-derived server must be removed from its originating configuration.
- Skill removal fails while effective agent allowlists reference it. Users explicitly remove bindings or dependent agents first; no silent cascading changes.
- Binding commands accept only user-added agents and reject bundled agents, including with replacement/force flags. Store explicit binding changes separately from imported source content. Validate referenced skills and the resulting effective spec; an empty allowlist is valid for user-added agents, including after removing their last binding. Explicit development-directory mode remains controlled by those files.

### Guided setup in `prism install`

Current `internal/cli/install.go` walks through bundled skill/specialist selection, host targets, host installation scope, copy mode, and a final preview. Extend that walkthrough with an optional runtime-extension setup path using the same services as the direct add/binding commands.

- Explain that making a skill available in Prism and allowing a user-added agent to use it are separate steps.
- Allow users to skip skill selection when creating/importing a user-added agent. An omitted source skill list is not an incompatibility and does not require a placeholder skill. Declared but missing skill dependencies must still be resolved or explicitly omitted through the translation workflow.
- Let users supply a skill source or select an already installed skill, then select a user-added agent and review its proposed allowlist.
- Offer an existing user-added agent, importing an agent in a supported format, or creating a uniquely named copy of a bundled agent. The copy path asks for the new ID and display name, then previews the starting configuration and selected skill bindings. Imported external agents receive a translation preview and assistance resolving missing Prism execution settings.
- Require model selection for imported agents: show the source model as provenance, then select or explicitly confirm a configured local or self-hosted runtime/model. Do not silently accept a preselected value or use the importing host's model. Direct imports use `--model`, `--use-configured-model`, or `--import-config` for the required target selection.
- Show bundled agents as unchanged; never offer to edit their bindings.
- For user-added agents with MCP capability, offer the general default, a custom server selection, or no MCP access. Let users configure the shared default list and show the effective server names in the preview. Do not silently interpret an omitted field as unrestricted access.
- Offer runtime scope separately from host installation scope: user-wide by default, or explicit project isolation. Display the selected location before source selection and mutations. Preserve the existing `prism install --project/--global` meaning for host installation; those flags must not silently choose runtime scope.
- Configure the host's MCP command to use the selected runtime state directory. For project isolation or a custom location, persist an absolute path in the command arguments so host working-directory changes cannot redirect configuration. Handle CLI state-dir flags, environment overrides, and config-file selection consistently between the walkthrough and launched server; test that the effective locations match.
- A project-isolated setup uses project-scoped host registration. If the host installation is global, direct the user to select project host scope or user-wide runtime scope rather than register a project-specific runtime for every repository.
- Prepare and validate both stages, then show one final preview identifying two transactions: host installation and runtime-extension setup. Cancellation before apply changes neither. Apply the host installation first using its existing rollback guarantees; if it fails, do not activate extensions. If extension activation then fails, roll back that entire runtime transaction, retain the successful host installation, and report partial success with a nonzero exit and retry instructions. Do not claim cross-filesystem atomicity or automatically undo a valid host installation.
- Runtime-only setup bypasses host installation; expose `prism install --runtime-only` for repeating the guided import/binding flow. Existing host flags retain their meanings. Use `--runtime-scope user|project` for explicit scope selection; reject conflicting explicit `--state-dir` and scope selections. Interactive runtime-only setup may select no skills and no hosts.
- Reuse one input reader throughout the walkthrough, and test scripted input, EOF, cancellation, invalid selections, and retry paths.
- Keep unattended commands deterministic. Existing `--all`/`--yes` must not choose remote sources, create new agents, or grant additional skills implicitly. `--dry-run` previews without activating changes.
- Host export of runtime extensions remains deferred; users access them through Prism's MCP catalog and execution tools.

### Copying a bundled agent

Expose the same operation through the walkthrough and `prism agent copy <bundled-agent-id> <new-agent-id>`. Copy the agent spec and materialize its resolved constitution into the managed extension object. Rewrite the ID/filename and any constitution path to refer to the copy; never leave a live constitution reference into the release bundle.

Preserve the starting model, budgets, tools, and skill allowlist for the user to review, then apply explicitly selected binding changes. Validate the final spec and unique ID before activation. Store the source agent ID, Prism release identity, and source bundle digest as provenance. Copy creation and initial binding changes activate together through the extension transaction.

Later Prism releases do not overwrite or merge changes into the copied spec or constitution. The copy remains user-owned. Referenced bundled skills and the runtime itself still follow the installed Prism version; copying an agent does not freeze those dependencies or guarantee identical model output. Include that distinction in help and upgrade tests.

### Per-agent MCP access and general default

The current runner enables its MCP bridge when an agent declares `tools: [mcp]`; `internal/app/tool_loop.go` then lists and addresses configured servers through the shared downstream client. Extend this for user-added agents with an explicit access selection: general default, custom server set, or none. Keep this separate from skill bindings.

Persist the mode as well as selected names so an empty custom set cannot accidentally become inherited or unrestricted access. The general default is scoped to the selected runtime configuration; project isolation does not inherit user-wide defaults. Users choose the default server set once, agents selecting default inherit that set, and custom selections replace rather than union with it. The initial shared list is empty until configured. Registering a new server does not expand it. Changes to the shared list affect default-mode agents when the runtime loads the updated configuration; custom and none modes remain unchanged.

Store shared defaults and per-agent access selections in the versioned extension manifest so one atomic snapshot defines effective access. CLI and walkthrough previews identify agents affected by a default-list change. Validate selected server names against the effective configured catalog, including environment-derived entries. If a referenced server later disappears, report the missing reference without falling back to other servers or broader access.

Enforce effective access in server discovery, tool listing, and tool calls using the resolved agent identity. Filter unauthorized servers from agent-facing discovery and reject direct calls to them before connection. Apply the same restriction to any runtime plugin/evidence path acting for that agent, not just the model tool loop. MCP capability and server access are both required; changing a default must not grant MCP capability to an agent that lacks it. Existing policy checks continue to apply.

Agent show, import/copy previews, and run provenance expose the selected mode and effective server set. Imported host MCP declarations map to configured Prism servers through the translation workflow; declaring a server in an import does not automatically register or authorize it. A copied bundled agent becomes user-added and follows the new selection workflow rather than inheriting unrestricted bridge access implicitly.

Keep bundled-agent and existing host-level administrative MCP commands compatible. Do not implement user-agent filtering by mutating the shared global client state, which could leak settings between concurrent runs. Add service operations and CLI access/default configuration commands alongside the walkthrough. CLI syntax: `prism mcp defaults set <server>...` / `show` / `clear`, and `prism agent mcp set <agent-id> --default`, `--none`, or repeated `--server <name>` selections. Reject combinations of selection modes; support dry-run and JSON inspection consistently.

## Bundled Graphify repository investigation

Graphify integration is required in this release, not a registry example or a skill the user must discover separately. The following names are proposed implementation identifiers. Package the assets through the existing embedded bundle and include them in its manifest, digest, catalog, authoring checks, and release tests.

| Bundled asset | Purpose |
| --- | --- |
| `agents/repo-investigator.md` and its constitution | Bounded repository investigation using the configured local or self-hosted offload model |
| `skills/graphify-query/SKILL.md` with references and evals | Specialist instructions for graph queries, relationship traversal, source verification, and evidence reporting; standard Agent Skills format |
| Host-facing repository-investigation instructions and generated specialist wrappers | Tell the parent host when to delegate through Prism and which agent/skill identifiers to pass |
| Versioned Graphify dependency specification and setup documentation | Identify the tested upstream release, MCP extra, supported Python/platform requirements, launch configuration, and license/notice obligations |

The intended path is **parent host → Prism `run_agent` → bundled `repo-investigator` with `graphify-query` → Graphify MCP → bounded evidence → offload-model findings → parent host**. The parent remains the orchestrator. Installing upstream Graphify's general-purpose skill alone does not establish this path: it can encourage the parent to execute Graphify directly. Do not copy that full build/semantic-extraction workflow into Prism's query skill.

### Host routing and runtime capability

- Publish host-facing trigger descriptions for repository architecture, symbol relationships, dependency paths, and change-impact investigations. Direct them to Prism's specialist, with a concrete invocation example and the selected workspace. Ordinary one-file lookups need not incur delegation overhead.
- Keep internal query instructions distinct from host delegation instructions. Extend installer rendering/distribution metadata as needed so selecting this capability installs its host instructions and wrapper without exporting internal instructions that tell the parent to invoke Graphify directly. Any Prism-specific metadata is optional for imported standard skills.
- Give the new bundled specialist a release-defined Graphify capability with a narrow MCP tool allowlist. Start with `query_graph`, `get_node`, `get_neighbors`, and `shortest_path`; verify their schemas and behavior against the pinned upstream release. Do not grant unrestricted downstream MCP access or inherit the user-agent general default. Existing bundled specialists remain unchanged.
- Bind that capability to an explicitly configured Graphify connection and workspace/index identity. Pass the absolute graph path to a local command server; do not rely on the host or server working directory. A self-hosted endpoint must have an explicit matching workspace/index binding. Reject missing or mismatched bindings rather than query another repository's graph.
- Bound result size, traversal depth, tool calls, and total context consumption. Treat graph content as untrusted evidence, not instructions. Verify relevant graph claims against current workspace sources before presenting them as current facts; return file/symbol references and distinguish graph-derived leads from source-verified findings.
- Missing, stale, or incompatible indexes produce explicit diagnostics. A source-only fallback must be labeled and use existing permitted workspace evidence; it must not claim Graphify ran. Never build or refresh an index implicitly during specialist execution.

### Dependency and index setup

The embedded integration assets ship with every Prism binary. The upstream Python executable and generated repository graph are separate runtime prerequisites, not embedded per-repository content. Listing bundled capabilities and starting Prism must still work without Graphify installed. Graphify-backed execution requires a ready connection/index and the user's configured offload runtime, just as other external capabilities require their services.

- Extend `prism install` to offer the bundled repository-investigation capability and check for an existing compatible local Graphify installation or explicitly configured self-hosted Graphify MCP endpoint. Explain separately what is bundled, what must be installed, and what must be indexed.
- Provide a reviewed, version-pinned dependency setup command and explicit launch configuration for the Graphify MCP extra. If the walkthrough offers to execute dependency installation, show the exact operation, destination, and ownership first and require that specific selection; `--all` or `--yes` alone must not download or install Python dependencies. Keep any managed environment separate from system Python and user extensions. Track managed dependency changes and failures separately from host/runtime configuration transactions; do not claim package-manager changes roll back atomically.
- Support an existing user-managed installation without taking ownership of it. Managed uninstall removes only Prism-owned configuration/assets; it preserves user-managed Graphify installations and repository indexes. No installation, dependency repair, graph build, or inference occurs during catalog listing, dry run, registration, or doctor checks.
- Assist users in selecting the repository and existing index, and provide an explicit code-only indexing/refresh command verified against the pinned release. Initial integration uses deterministic local code indexing and graph retrieval. Automatic documentation/media semantic extraction and upstream backend auto-detection are outside this release; setup must not accidentally select a cloud provider from environment credentials.
- Record Graphify version/schema compatibility, workspace identity, and index generation identity or fingerprint in diagnostics and run provenance. Record source revision/dirty-state freshness when available; report freshness as unknown when upstream metadata cannot establish it. Do not equate a successful server connection with a current index.
- Doctor reports missing executable/extra, incompatible versions, missing graph, workspace mismatch, unknown/stale freshness, and unavailable connections with actionable repair steps. Dependency checks should be local; any connection probe must be explicit and must not call a model.

Upstream reference: [Graphify repository and MCP documentation](https://github.com/Graphify-Labs/graphify). Pin and verify the exact upstream version, tool contract, indexing command, dependency hashes where applicable, and redistribution terms during implementation; do not depend on a floating branch. Graphify's knowledge graph remains distinct from Prism's execution DAG.

## Source support and registry integration

### First release sources

| Input | Resolution |
| --- | --- |
| Local skill directory or repository | Discover root `SKILL.md` and conventional skill directories; copy selected complete directories |
| Local agent file or repository | Discover native Prism agents and supported external formats, then translate and validate selected agents |
| `owner/repo` or HTTPS GitHub repository URL | Resolve a revision, fetch a bounded snapshot, discover candidates |
| GitHub tree URL | Resolve ref/subdirectory without assuming branch names contain no slash; allow explicit `--ref`/`--path` to disambiguate |
| GitHub-backed `skills.sh/<owner>/<repo>/<skill>` page | Normalize to repository plus exact skill selector, then use the GitHub resolver |

Use `--path` to constrain repository discovery, particularly for an agent file or nonstandard layout. Deduplicate candidates and reject ambiguous duplicate names. Reject unsupported source forms with supported alternatives. A skills.sh page is discovery metadata; verify the named skill in the resolved repository rather than downloading HTML as a skill.

The [upstream skills CLI](https://github.com/vercel-labs/skills) documents repository/local sources and selectors. The [web-design-guidelines page](https://skills.sh/vercel-labs/agent-skills/web-design-guidelines) maps to a GitHub repository and skill selector. This supports the initial URL adapter; it does not establish a universal registry protocol.

### Follow-on catalog providers (outside the initial release)

Ship a small source-resolver interface now; add catalog search capabilities in the follow-on release. Keep downloading, validation, and persistence independent of provider-specific response formats.

Follow-on commands:

```sh
prism registry add skills-sh --type skills-sh --url https://skills.sh --token-env VERCEL_OIDC_TOKEN
prism registry list
prism skill search "web design" --registry skills-sh
prism skill add <result-source> --skill <result-name>
```

Persist provider configuration in `<state-dir>/registries.yaml`, with environment references for credentials. Validate configured URLs and supported provider types; accepting a URL does not imply support for an arbitrary registry protocol.

The [skills.sh API documentation](https://skills.sh/docs/api) describes versioned catalog/search endpoints and Vercel OIDC authentication, including token expiry and rate-limit responses. Build its adapter against documented responses, with bounded requests and tests for 401/429/503. Direct GitHub-backed page installation must remain usable without catalog API access. Do not scrape search HTML or add an undocumented API dependency.

Generic Git/SSH hosts, standalone remote Markdown, archive URLs, well-known skill providers, skills.sh packs, and MCP registry package lookup are follow-on adapters. Explicitly report these limits in help. Agent discovery through a skill catalog is not assumed.

## Storage and runtime architecture

### Skill standard and agent import adapters

Skills and agents are distinct concepts. The [Agent Skills overview](https://agentskills.io/home) describes portable capability packages; it does not define an agent's model, execution budgets, or host lifecycle. Use its [specification](https://agentskills.io/specification) as Prism's public skill format. Optional resource directories must remain optional. Preserve source files and metadata without converting them to a Prism-only skill format.

Audit `internal/skill/skill.go` and `discover.go` against the specification: complete standard field handling, correct character-based limits and name validation, optional directories, and preservation of additional resources. Keep any stricter bundled-content authoring checks explicitly separate from format acceptance. Do not treat format validity as proof that the runtime can execute every referenced tool or script; expose capability limitations separately. Replace the current fixed `references/REFERENCE.md` assumption with the bounded resource service described below. Standard format support does not imply arbitrary script execution or multimodal rendering.

Introduce an agent import adapter boundary: detect source format, parse source, translate into a normalized Prism spec plus support files, report compatibility, then validate and activate. Preserve original source bytes, format/adapter version, normalized output, and any explicit user mappings in extension provenance. Ambiguous format detection requires `--format`; source location alone is insufficient.

First-release external adapters:

| Adapter | Accepted definition | Translation contract |
| --- | --- | --- |
| `claude` | Claude Code subagent Markdown with YAML frontmatter | Map name/description and prompt body; report host-specific tools, skill preloading, permission settings, hooks, and memory individually |
| `codex` | Standalone Codex custom-agent TOML, including definitions under `.codex/agents/` | Map name/description and `developer_instructions`; report model/reasoning, sandbox, MCP, and skill configuration individually |

Claude Code's [documented format](https://code.claude.com/docs/en/sub-agents) combines frontmatter with prompt text and host-specific execution settings. Require explicit selection of a local or self-hosted offload model; never assume a Claude model alias denotes an installed local model. Resolve budgets from explicit Prism settings or walkthrough choices. Treat source skill preloading and Prism's allowed-skill list as distinct semantics requiring a visible mapping.

Codex's [custom-agent documentation](https://learn.chatgpt.com/docs/agent-configuration/subagents#custom-agents) defines standalone TOML with `name`, `description`, and `developer_instructions`, plus optional session settings. Use the declared name as identity rather than assuming it equals the filename. Map instructions into the imported agent's constitution. Resolve omitted/inherited settings through explicit Prism configuration or walkthrough choices; do not read the user's live Codex session or silently inherit their personal configuration. Source model names and reasoning settings require supported runtime mappings. Source `skills.config` entries are not equivalent to a Prism skill allowlist. Treat `AGENTS.md` as repository guidance, not a standalone custom-agent definition.

Support explicit `--format prism|claude|codex` with detection when unambiguous, including mixed-format repositories. Discovery recognizes native `agents/*.md`, Claude `.claude/agents/*.md`, and Codex `.codex/agents/*.toml`, as well as explicitly supplied files/directories. Import only selected definitions; never mutate the source host's configuration. Legacy Codex role declarations that reference a separate config layer require a separately documented compatibility adapter and are outside the initial standalone-file contract.

Produce a translation report identifying preserved fields, configured mappings/defaults, unsupported features, and unresolved dependencies. Host tools, hooks, permissions, memory, and MCP declarations require individual compatibility handling; preserving a field in provenance does not make it operational. For each incompatibility, explain the source behavior and effective Prism behavior, then let the user explicitly map it or omit it. Users may activate the translated agent after accepting those changes and passing normal Prism validation. Unresolved incompatibilities prevent activation. Do not execute source hooks or install declared MCP servers during translation.

Persist accepted omissions and mappings with source digest and adapter version, and expose them in agent show and JSON output. Acceptance changes the imported agent's definition; it does not bypass Prism policy or manufacture an unsupported runtime capability. Required execution settings must still resolve. Source or adapter changes require a fresh comparison; prior acceptance does not cover newly introduced incompatibilities.

Unattended import uses `--import-config <file>` for field-specific mappings and omissions. The versioned YAML document identifies each source agent, source digest, adapter version, explicit model selection, and a list of source-field decisions (`map` with a supported target/value, or `omit` with its accepted behavior change). Reject unknown decision fields, stale digests, and mappings the selected adapter cannot enforce. The walkthrough produces the same document; `agent add --dry-run --json` returns the translation report and a configuration skeleton without accepting its unresolved decisions. CLI model flags and a model mapping in the document are mutually exclusive. Dry-run reports may include missing required settings and exit nonzero while returning the configuration skeleton; they never activate incomplete imports. A generic `--yes`, `--all`, or `--replace` cannot silently accept translation losses. The walkthrough gathers those choices before its final activation preview. Cancellation leaves active runtime state unchanged.

Use deterministic adapters and fixtures, not model-generated rewrites. The direct CLI and walkthrough share this translation pipeline. Additional formats implement the same interface; do not advertise arbitrary-format support.

### Required local or self-hosted model mapping

Model selection is a required import step, not an optional override of a source model. Keep the source model value for provenance; store the explicitly selected Prism offload target as execution configuration. Do not infer an offload equivalent from a source cloud-model name or silently fall back to a managed cloud inference provider. A model identifier alone does not establish where inference occurs: show the effective runtime endpoint and configured fallback as well. Localhost and user-operated self-hosted servers on other machines are supported targets. Preserve existing Ollama, SGLang, and vLLM endpoint support; do not impose a loopback-only restriction or equate a non-loopback URL with managed cloud inference. Endpoint ownership cannot be reliably inferred from a hostname alone; use explicit runtime configuration and user selection.

Current `docs/model-runtime.md` documents a global `PRISM_MODEL_RUNTIME_MODEL` override that can supersede an agent model. Preserve that behavior for bundled agents. For user-added agents, preview and record the effective engine, endpoint, model, and configured fallback, excluding secrets. Reject an explicit model that a conflicting global override would replace. At execution, reject a changed effective target with instructions to use `prism agent model set` to explicitly select it again; never silently retarget the agent. Configuration rotation of credentials alone does not change the target. Apply the same explicit selection to configured fallbacks. Reuse existing Ollama/SGLang/vLLM adapters.

For missing import budgets, use documented extension defaults of 8192 context tokens and 30000 ms latency budget, shown in the preview and configurable through the import document. Preserve valid source Prism budgets. Validate positive budgets, and do not claim that a model supports the selected context length until runtime diagnostics establish it. Copies retain their starting budgets and require confirmation of the effective offload target in the walkthrough; direct copy explicitly chooses the bundled source configuration.

Installation, source discovery, translation, and walkthrough suggestions remain ordinary deterministic code. Do not call an LLM to rewrite instructions, choose models, map formats, or resolve compatibility. Imports may be configured while the runtime is offline; model availability checks are explicit diagnostics, and execution fails clearly when the selected model is unavailable. Do not download models or generate text as an installation side effect.

### Skill-optional user agents

Allow a user-added agent, including a copy of a bundled agent, to run with no attached skills whether its allowlist is empty or nonempty. Skills remain explicitly selected per invocation; omitting them does not auto-attach the allowlist. An empty allowlist permits no skill attachments, rather than meaning unrestricted access.

Make validation depend on catalog origin supplied by Prism's resolver. Source frontmatter and MCP request fields cannot label a bundled agent as user-added. Preserve bundled-agent and existing explicit development-mode requirements. Add a validation mode for extension specs instead of globally removing bundled validation in `internal/agent/spec.go`.

Update the shared runner, CLI help, MCP input schema/handler, graph validation/execution, and relevant prompts/resources to honor the same origin-aware rule. Both omitted and empty MCP `skill_names` are valid for user-added agents; bundled agents still receive an actionable error. Remove the MCP handler's unconditional empty-list rejection and let shared origin-aware validation decide. Policy, workspace checks, tool restrictions, and provenance must still execute on zero-skill runs.

### Skill resource reading

Expose a shared read-only resource service from each immutable skill object. List relative resource paths with media type and size; read UTF-8 text from references, assets, and other support files relative to that skill root. Binary assets remain preserved and listable; return an explicit unsupported-content response rather than attempting text decoding or claiming image/document understanding.

Provide `list_skill_resources` and `read_skill_resource` tools to tool-capable user-added agents only for skills attached to that run, independent of MCP-server capability. Keep bundled agents' default tool surfaces unchanged. Pass the run's attached-skill identities into the service; an agent cannot request another skill or workspace file by guessing its path. Add CLI `prism skill resources <name>` and `prism skill read <name> <path>` for user inspection and matching host-facing MCP resource tools for explicit host requests. Host inspection uses the configured catalog; specialist access uses the narrower run context.

Use a 32 KiB per-read cap, a 128 KiB total resource-read budget per run, explicit truncation/offset metadata, and the existing bounded tool-round mechanism. Include returned resource text in context budgeting and source attribution. Reject absolute paths, traversal, and escaping links. Non-tool-capable models may receive resources selected explicitly with repeatable `--skill-resource <skill>:<path>` or the corresponding optional MCP/graph request field. Reject a resource for an unattached skill; do not inject every file automatically. Capability diagnostics explain the difference between on-demand reads and explicit attachments.

Preserve script files as part of the package, and allow text inspection, but do not add a general-purpose executor or execute scripts at installation. Existing bundled runtime plugins keep their behavior. Reports distinguish format-valid skills from skills needing unsupported execution capabilities. Declared or detected script dependencies produce a clear capability limitation; static checks are not a guarantee that all such requirements can be detected. Do not claim successful execution of a script-dependent step when the executor is absent.

### Managed storage

Storage layout:

```text
<state-dir>/
  config.env
  mcp-servers.yaml
  extensions.yaml
  extensions/
    objects/<content-digest>/<package-files>
  # registries.yaml is added only in the follow-on catalog release
```

`extensions.yaml` is versioned and records kind, installed ID, original/canonical source, requested ref, resolved commit (for GitHub), subpath, content digest, object path, installation time, and explicit skill-binding changes for user-added agents. Local installs record a copied snapshot digest rather than pretending to have a Git revision. Store no credentials in provenance URLs.

Use immutable content objects with an atomically replaced manifest as the activation point. Interrupted downloads and failed validation never activate partial installations. Stage on the destination filesystem, verify the final object, then commit the manifest. Unreferenced objects can be cleaned separately; removal first deactivates the manifest entry. Protect read-modify-write operations with a process-safe lock and detect unexpected external edits. MCP YAML mutations likewise use same-directory temporary files and atomic replacement; preserve unrelated entries and YAML comments where practical.

Multi-file runtime transactions stage immutable objects first, lock the affected runtime configuration, and journal before-images for mutable files before replacement. Publish only after validating the complete candidate configuration. A failed operation restores the prior mutable files; after a crash, startup detects the journal and completes rollback before serving requests. If recovery fails, stop with actionable recovery details instead of loading a partial state. Readers cooperate with the same lock. Host installation retains its separate transaction boundary described in the walkthrough.

Rename operates on a managed extension identity and object, leaving the original source/provenance intact. Update skill frontmatter/directory name or agent ID/filename and tracked user-agent dependency/binding references atomically. Do not rewrite arbitrary natural-language instructions or external callers; show those limitations in the preview. Preserve dependency origin in the manifest so upgrade recovery cannot accidentally rebind an extension dependency to a new bundled item. New conflicts are checked before activation. Skills retain standard-compliant names after rename.

### Activation and running processes

Load and validate a consistent runtime snapshot at process initialization, including catalogs, bindings, effective MCP server selections, and model configuration. Coordinate snapshot reads with mutation locks so startup cannot observe a partially applied multi-file change. Existing tasks keep that snapshot; removal or access revocation does not retroactively change an already running task or server. State this clearly in mutation output.

Do not automatically restart host-managed MCP processes or interrupt active tasks. The walkthrough reports how to restart/reconnect Prism through the selected host. Retain immutable objects that existing process snapshots may still reference; replacement/removal cannot delete backing content out from under a running server. Defer object garbage collection until a safe retention or reader-tracking strategy is implemented.

Add `internal/extensions` for the manifest/store, source resolvers, install planning, catalog composition, and provenance. Keep command handlers thin. Reuse narrowly applicable installer utilities, but keep binary installation lifecycle in `internal/installer` distinct from content installation.

Runtime composition rules:

1. Without explicit development directory flags, combine embedded content with uniquely named installed objects by agent ID and skill name. Updating a runtime extension replaces its entire skill directory so support files cannot accidentally come from another version.
2. Reject collisions with embedded content even with `--replace`. Updates to installed extensions require `--replace`. Reject duplicate IDs within a proposed transaction, including case-folded filesystem collisions. If a future release introduces a bundled name already used by an extension, preserve the extension object and mark it inactive with an explicit conflict diagnostic. The bundled item remains available. Disable dependent user agents whose previously resolved extension dependency would otherwise switch to the bundled item; never silently substitute dependency origins. CLI list/show and doctor report the conflict and recovery commands. Management commands must work without constructing a runnable catalog. Users resolve it with rename or removal; no automatic renaming or deletion.
3. `--agent-dir` and `--skills-dir` retain their existing replacement semantics for the corresponding catalog. Report managed items as inactive for a replaced catalog; do not silently write into development directories.
4. Associate every agent with its own source filesystem for constitution resolution. Validate and retain `constitution_path` and legacy constitution dependencies within that source root; do not resolve an imported constitution against the embedded bundle by accident.
5. Apply binding changes only to user-added agents; reject manifest entries targeting bundled agents. Validate effective skill dependencies; a corrupt manifest or missing managed object is an actionable error, not a silent fallback.
6. Route CLI list/show/lint/doctor, agent execution, and MCP prompts/resources/skill health through the same resolver. `mcp serve` must load installed additions even when it has no workspace root.
7. Record a deterministic digest of the effective agents, skills, constitutions, binding overrides, and resolved MCP access selections. Preserve embedded version metadata and distinguish runs using extensions so caches, results, and events do not claim the embedded-only digest.

## Validation and installation behavior

- Validate portable skills with the existing frontmatter parser. Make optional directories optional for portable lint/test; retain a strict Prism authoring profile for repository CI and bundled skills. Preserve skill assets, references, scripts, and license files. Installation never executes these files.
- Validate native and translated agents with `agent.Parse`, filename/ID matching, constitution resolution, and availability of allowed skills in the proposed effective catalog. Missing dependencies produce a list and explicit install commands when the source is known; otherwise request source selection rather than inventing an install command. Do not search for and install arbitrary similarly named skills. A later dependency manifest can make cross-repository dependency resolution explicit.
- Installing a skill alone does not grant tool access. Existing policy checks and skill allowlists remain active; agent additions do not trigger model downloads or execution.
- Reject path traversal, absolute package paths, escaping symlinks, special files, credential-bearing source URLs, and executable Git hooks/submodules during fetch. Bound download size, expanded bytes, file count, and request duration. Keep GitHub authentication scoped to GitHub and never forward it across redirects to unrelated hosts.
- Dry runs may resolve/download into temporary storage to produce an accurate preview, but never change active state. Present selected content, dependencies, replacements, target state directory, and resolved revision.
- Make install batches atomic: if any selected item fails, none becomes active. Identical content is a no-op. Re-adding a changed source requires `--replace`; a dedicated update command is deferred. Verify local modifications/object integrity before replacement.
- Registration/install/remove work without Ollama and do not create runner/event-store side effects unnecessarily.
- Preserve extensions during binary upgrades and default uninstallation; integrate any future explicit purge with the binary installer's ownership rules.

## Implementation sequence

Implement these in order as reviewable changes. No public service or running model is required by the default test suite.

### 1. State mutation and MCP registration

- [x] Extract MCP services from command handlers; preserve legacy commands and synthesized Linear configuration.
- [x] Implement atomic writes, mutation locks, HTTP transport, input validation, environment references, and top-level add/list/show/remove/tools/call commands.
- [x] Exercise command, SSE, and Streamable HTTP clients with protocol fixtures.

Exit: the requested `prism mcp add openaiDeveloperDocs --url ...` syntax persists the correct transport; local fixtures complete initialize/list/call. Registration performs no process launch or inference.

### 2. Extension store and runtime composition

- [x] Implement the versioned manifest, immutable objects, transaction recovery, and provenance.
- [x] Compose catalogs with per-agent constitution roots, origin-preserving dependencies, development overrides, and consistent startup snapshots.
- [x] Add collision diagnostics and management paths that work without a runnable catalog. Retain objects referenced by old snapshots.
- [x] Wire shared catalog inspection into CLI, MCP, doctor, and effective-content digests.

Exit: fixture additions coexist with bundled content and resolve identical identities/constitutions through CLI and MCP. Fault-injected writes restore the prior configuration.

### 3. Standard skills and resources

- [x] Implement local skill discovery, add/list/show/remove/rename, selectors, aliases, replacement, JSON output, and dry run.
- [x] Complete Agent Skills format validation; separate strict repository authoring checks from portable format acceptance.
- [x] Implement bounded resource listing/reading, run-scoped tools, explicit attachments, and script capability reporting.

Exit: a minimal standard skill and one with nested text/binary resources install without Prism-only metadata. Text resources are usable within bounds; scripts are preserved without adding execution support.

### 4. Agent imports, model selection, and bindings

- [x] Implement native Prism, Claude Code Markdown, and Codex TOML imports using deterministic adapters and versioned translation documents.
- [x] Require explicit effective offload target selection; implement model remapping and configuration-drift checks using existing runtime adapters.
- [x] Implement copy/rename/remove, independent constitutions, skill bindings, and origin-aware zero-skill validation across CLI/MCP/graphs.
- [x] Implement per-agent MCP access/default selection and enforcement across tool and evidence paths.

Exit: translated and copied user agents run with a fake model, with or without attached skills, while respecting their selected MCP servers. Bundled behavior stays compatible.

### 5. Remote sources and guided setup

- [x] Implement GitHub shorthand/repository/tree resolution, immutable revision fetching, credentials, bounded downloads, and GitHub-backed skills.sh page normalization.
- [x] Extend `prism install` with runtime scope, source selection, copy/import, model mapping, skills, MCP access, translation preview, and runtime-only retry.
- [x] Pass selected runtime configuration into generated host registrations and implement separate host/runtime transaction outcomes.

Exit: equivalent GitHub and skills.sh page inputs install the same selected revision without Node or catalog credentials. Scripted walkthrough tests cover successful setup, cancellation, partial success, and retries.

### 6. Bundled Graphify capability

- [x] Add the bundled repository specialist, constitution, standard query skill, references/evals, and host delegation instructions; cover embedding, digest, catalog, and installer selection.
- [x] Implement workspace-bound Graphify access with the fixed query-tool allowlist, bounded results, source verification, and provenance.
- [x] Add pinned dependency guidance, explicit setup choices, endpoint/index binding, compatibility/freshness diagnostics, and safe lifecycle ownership.
- [x] Test host rendering for Claude and Codex so instructions route repository investigations through Prism; keep internal query instructions inside the specialist workflow.
- [x] Add fixture-based end-to-end runs and a documented optional real Graphify/offload-model smoke test. Measure retrieval usefulness, evidence correctness, and tool/context cost against source-only investigation before recommending broad use.

Exit: a released bundle contains the complete integration; a configured fixture host invocation reaches the bundled specialist and only its allowed Graphify tools. Missing prerequisites remain diagnosable without breaking Prism startup or unrelated specialists. Setup performs zero inference calls.

### 7. Release verification and documentation

- [x] Update README, usage, model-runtime, and agent/skill authoring docs, including stale claims about tool calling and directory requirements.
- [x] Document supported sources/formats, scope, restart activation, model selection, translation losses, resource limits, rename recovery, and staged setup outcomes.
- [x] Run focused package tests, then `go test ./...` and applicable repository CI checks. Keep live public-server checks optional.

Exit: all acceptance cases below pass with fixtures and documentation matches shipped behavior. All seven phases are required for the initial release. Catalog search is follow-on work.

## Acceptance matrix

| Area | Required verification |
| --- | --- |
| MCP registration | Correct HTTP/SSE/command transport; argv preserved without shell expansion; malformed inputs rejected; legacy state and Linear origin preserved; registration remains offline |
| Authentication | Reference names persist, secret values do not appear in files/output/errors; missing credentials fail clearly on connection; redirects do not leak credentials |
| Portable skills | Minimal standard skill accepted; standard fields/resources preserved; strict bundled authoring checks remain separate |
| Resource reads | Attached-skill restriction, nested paths, traversal rejection, UTF-8/binary distinction, pagination/truncation, total budget/context accounting, and explicit resource attachments for non-tool models |
| Agent translation | Claude and Codex fixtures, name/filename differences, multiline instructions, mixed-format detection, required settings, explicit omissions, stale mapping documents, and no automatic AGENTS.md import |
| Offload models | Required local/self-hosted target selection, network-hosted endpoint support, effective override/fallback validation, drift detection, no source-cloud-model inheritance, zero setup inference calls |
| Bindings and access | User-added zero-skill runs across CLI/MCP/graphs; allowlist enforcement; MCP default/custom/none; newly registered servers do not expand defaults; direct calls/evidence paths cannot bypass access; policy remains enforced |
| Bundled behavior | No bundled replacement or user binding edits; existing skill requirements, development overrides, and default tool surfaces preserved; copied agents use user-extension rules |
| Graphify packaging and routing | Release embeds specialist/constitution/query skill/host instructions; manifests and digests include them; host exports delegate to Prism; internal instructions are not exported as a competing direct workflow; fixture invocation follows the intended route |
| Graphify access and evidence | Only approved query tools; workspace/index mismatch rejected; bounded traversal/results/context; current-source verification; missing/stale/unknown index diagnostics; truthful fallback and version/index provenance |
| Graphify setup and lifecycle | Missing Python/Graphify does not break startup/catalog; compatible existing installation and self-hosted endpoint supported; pinned setup is explicit; dry run and unattended defaults install nothing; no cloud auto-selection or setup inference; uninstall preserves user installations/indexes |
| Persistence | Idempotent adds; explicit replacement; concurrency; all-or-nothing runtime batches; crash recovery; unrelated content preservation; management without runner side effects |
| Activation | Existing processes retain snapshots after mutations; new processes see committed state; backing objects remain readable; CLI reports restart requirements and delayed revocation |
| Scope and setup | User-wide versus isolated project state; host scope remains distinct; absolute runtime path survives changed host CWD; dry run/cancellation/EOF do not activate extensions; per-stage failures return accurate status and retry instructions |
| Upgrade recovery | Conflicting extensions preserved/inactive with diagnostics; dependent user agents do not rebind to bundled names; transactional rename repairs tracked references; original source remains intact |
| Provenance | Digests include effective content/access; source revision, adapter decisions, copies, and model targets inspectable; credentials excluded; binary upgrades and default uninstall preserve extensions |

## Follow-on catalog milestone

After the initial release, add provider configuration and skills.sh catalog search through the resolver boundary. Test documented authentication, pagination, rate limits, expired tokens, missing entries, and service failures. Search results provide canonical source/selector data to the existing installer. Catalog failures never block direct-source installation. Reverify the provider's documented API contract before implementing this milestone.

## Deferred work

A general-purpose skill script executor, binary/multimodal asset interpretation, catalog search/provider configuration, replacing bundled agents or skills, exporting runtime extensions to editor hosts, automatic global/project overlays, interactive catalog browsing beyond the install walkthrough, live MCP reload, OAuth login flows, agent formats outside the agreed initial adapter set, automatic model installation, automatic remote dependency installation (the explicit, pinned Graphify setup above is in scope), automatic Graphify indexing or semantic extraction, signed package publishing, dedicated update/sync commands, and additional registry/source protocols. These can extend the resolver/store interfaces after the initial workflow is stable.

---

## Detailed implementation execution plan

This section converts the design above into reviewable PR-sized slices. Stabilize
the current embedded-bundle and host-installer refactor first, then build one
shared runtime-extension foundation and layer CLI, runtime, import,
remote-source, guided setup, and Graphify behavior on top of it.

### Current implementation baseline

- The working tree contains an unfinished embedded-bundle/installer refactor;
  establish a compiling, tested baseline before feature work.
- Embedded content is exposed through root `bundle.go`. Do not restore the
  deleted marketplace-style `internal/bundles`, `pkg/bundle`, or
  `pkg/registry` implementations.
- MCP state supports command and legacy SSE transports, but persistence is a
  direct YAML overwrite with silent upsert and no locking.
- The runner loads one agent registry and one skill filesystem rather than a
  composed bundled-plus-managed catalog.
- Native agent parsing exists, but bundled validation requires a nonempty skill
  allowlist. Skill discovery still assumes Prism-specific directories and
  resource reading is fixed to one reference file.
- `internal/installer` owns host integration; `internal/cli/install.go` does
  not yet manage runtime extensions.
- No bundled `repo-investigator`, `graphify-query` skill, Graphify
  connection/index binding, dependency readiness checks, or host-routing
  assets currently exist.

### Implementation boundaries

1. Keep the release bundle immutable. Runtime extensions cannot shadow bundled
   identities.
2. Add `internal/extensions` for manifests, immutable objects, locks,
   transactions, provenance, catalog composition, bindings, MCP access, and
   source-resolution interfaces.
3. Keep `internal/installer` responsible for host integration. Host and runtime
   transactions remain separate.
4. Keep Cobra handlers thin; direct commands and the walkthrough use the same
   services.
5. Build immutable runtime snapshots at process startup. Running MCP servers do
   not live-reload.
6. Preserve source bytes and deterministic translation reports. Setup and
   translation make no model calls.
7. Resolve secrets only at connection time and never persist or print values.
8. Treat Graphify as a release-bundle capability, not a runtime extension.
   Bundle Prism-owned instructions and policy while keeping the executable and
   repository indexes explicit external prerequisites.

### Slice 0 — Stabilize the current refactor

**Work**

- Finish the root bundle/build-info and `internal/installer` changes.
- Remove stale references to deleted marketplace packages without restoring
  them.
- Align CLI, runner, MCP server, root resolver, and installer callers with the
  new embedded bundle APIs.

**Primary files**

- `bundle.go`, `bundle_test.go`
- `internal/buildinfo/*`
- `internal/installer/*`
- `internal/cli/install.go`, `root.go`, and `version.go`

**Exit gate**

- `go test ./...` passes on the refactor baseline.
- Host installer tests cover config preservation, rollback, and idempotent
  reinstall.
- No references remain to the deleted marketplace packages.

### Slice 1 — Downstream MCP service and safe persistence

**Work**

- Split server validation, persisted state, and mutation operations inside
  `internal/downstreammcp`.
- Return explicit created, unchanged, replaced, conflict, and removed outcomes.
- Add same-directory atomic writes, process-safe locking, external-edit
  detection, and private permissions.
- Preserve legacy YAML and synthesized Linear behavior without persisting the
  environment-derived entry.

**Primary files**

- `internal/downstreammcp/state.go`
- new `internal/downstreammcp/store.go` and `service.go`
- `internal/cli/downstream_mcp.go`

**Exit gate**

- Concurrent fixture mutations do not lose entries.
- Identical additions are no-ops and conflicts require replacement.
- Linear remains runtime-visible but absent from persisted YAML.

### Slice 2 — Streamable HTTP, auth references, and MCP CLI

**Work**

- Add Streamable HTTP through the pinned MCP Go SDK while retaining command and
  explicit legacy SSE.
- Persist command environment and HTTP header reference names; resolve values
  only while connecting.
- Add top-level `mcp add/list/show/remove/tools/call` with description, timeout,
  size, dry-run, replacement, and JSON support.
- Keep `mcp server add-command` and `add-sse` as compatibility wrappers.

**Primary files**

- `internal/downstreammcp/client.go`
- `internal/cli/mcp.go`
- shared administrative output in `internal/mcp/server.go`

**Exit gate**

- Local fixtures initialize, list, and call over all three transports.
- Registration is offline and secrets do not appear in state, output, or
  errors.
- `prism mcp add openaiDeveloperDocs --url
  https://developers.openai.com/mcp` works.

### Slice 3 — Extension manifest and immutable object store

**Work**

- Add versioned `extensions.yaml` and
  `extensions/objects/<content-digest>`.
- Record identity, kind, canonical source, revision, subpath, digest, object
  path, installation time, provenance, and activation diagnostics.
- Add staging, locking, mutable-file journaling, atomic publication, rollback,
  and startup recovery.
- Retain unreferenced objects until a safe garbage-collection design exists.

**Primary files**

- new `internal/extensions/manifest.go`
- new `internal/extensions/store.go`
- new `internal/extensions/transaction.go`
- new `internal/extensions/lock.go`
- new `internal/extensions/provenance.go`

**Exit gate**

- Fault-injected writes restore the prior manifest.
- Startup recovers interrupted mutations or fails with actionable recovery
  details.
- Dry runs never activate content.

### Slice 4 — Catalog composition and startup snapshots

**Work**

- Compose bundled and managed content with origin, source filesystem,
  constitution root, dependency origin, activation state, and effective digest.
- Reject bundled and case-folded collisions.
- Preserve future conflicting extensions as inactive and disable dependent
  agents without rebinding them.
- Keep development-directory replacement semantics and report managed items
  inactive under overrides.
- Route runner, CLI, MCP, doctor, and digests through one startup snapshot.

**Primary files**

- new `internal/extensions/catalog.go`
- `internal/agent/registry.go`
- `internal/app/runner.go`
- agent/skill CLI and MCP catalog surfaces

**Exit gate**

- Embedded-only behavior remains compatible.
- Managed content resolves identically through CLI and MCP.
- Existing processes keep their snapshot while new processes see committed
  state.

### Slice 5 — Portable Agent Skills validation

**Work**

- Separate portable format validation from strict bundled-authoring checks.
- Implement standard name and character-count validation.
- Make resource directories optional and preserve complete skill packages.
- Report execution limitations separately from format validity.

**Primary files**

- `internal/skill/skill.go`
- `internal/skill/discover.go`
- validation fixtures under `internal/skill` and `testdata`

**Exit gate**

- A minimal standard skill installs successfully.
- Bundled CI retains stricter authoring checks.
- Script-bearing skills are preserved but never executed during installation.

### Slice 6 — Bounded skill resources

**Work**

- Add resource listing and UTF-8 reads with media type, size, offsets, and
  truncation.
- Enforce per-read/per-run budgets, attached-skill scope, traversal prevention,
  and escaping-link rejection.
- Preserve and list binary resources with explicit unsupported-text behavior.
- Add CLI inspection, host MCP inspection, run-scoped tools, and explicit
  attachments for non-tool models.

**Primary files**

- new `internal/skill/resources.go`
- `internal/app` prompt/tool integration
- skill CLI and MCP resources

**Exit gate**

- Nested text reads work within bounds.
- Binary, traversal, unattached-skill, and aggregate-budget cases fail safely.
- Resource text participates in context accounting and provenance.

### Slice 7 — Local skill lifecycle

**Work**

- Implement local discovery, selectors, `--list`, `--all`, `--as`, replacement,
  remove, rename, dry-run, JSON, and the `skills` alias.
- Copy complete selected directories into immutable objects and activate them
  transactionally.
- Reject bundled mutation and removal of referenced skills.

**Primary files**

- `internal/extensions/source.go` and `plan.go`
- `internal/cli/skill.go`

**Exit gate**

- Single and multi-skill sources behave deterministically.
- Batch activation is all-or-nothing.
- List/show distinguish bundled, managed, development, inactive, and
  conflicting origins.

### Slice 8 — Deterministic agent import adapters

**Work**

- Add detect, parse, translate, compatibility-report, and adapter-version
  interfaces.
- Implement native Prism, Claude Code Markdown, and Codex TOML adapters.
- Preserve source bytes, digest, adapter version, normalized output, support
  files, and field-level findings.
- Add a strict versioned import-config document for map/omit decisions and
  stale-digest checks.

**Primary files**

- new `internal/agent/importer/*`
- new `internal/agent/importconfig/*`
- native, Claude, Codex, and mixed-format fixtures

**Exit gate**

- Translation is deterministic and performs no model calls.
- Unsupported features remain unresolved until explicitly mapped or omitted.
- Generic acceptance flags cannot approve translation loss.

### Slice 9 — Explicit offload targets and drift checks

**Work**

- Store source-model provenance separately from the selected execution target.
- Reuse Ollama, SGLang, vLLM, and fallback runtime configuration.
- Implement model selection, configured-model selection, import mappings,
  preview, and `agent model set`.
- Reject conflicting global overrides and execution after target drift.

**Primary files**

- `internal/llm/runtime/*`
- runtime-target types in `internal/extensions`
- `internal/config/config.go`
- agent CLI and runner validation

**Exit gate**

- Imports complete offline without inference.
- Fake-runtime tests cover self-hosted endpoints, fallback, override conflicts,
  and drift.

### Slice 10 — Agent lifecycle, bindings, and zero-skill execution

**Work**

- Add origin-aware validation allowing user agents to run with no skills.
- Implement add/list/show/remove/copy/rename, bindings, and the `agents` alias.
- Materialize copied constitutions and store binding overrides separately from
  source content.
- Share invocation validation across CLI, MCP, graphs, prompts, and policy.

**Primary files**

- `internal/agent/spec.go`
- extension binding/catalog services
- agent/run/graph CLI
- runner and MCP handlers

**Exit gate**

- User agents run with zero or selected allowed skills.
- Bundled agents retain current skill requirements and reject binding changes.
- Copies remain independent from later bundle updates.

### Slice 11 — Per-agent MCP access

**Work**

- Persist the shared default set and per-agent default/custom/none mode.
- Add default and agent-access commands with dry-run and JSON.
- Resolve access in the startup snapshot.
- Filter discovery and authorize listing/calls independently for each agent.
- Apply the same restrictions to MCP bridge evidence and graph/MCP execution.
- Block removal of referenced servers.

**Primary files**

- extension access services
- `internal/app/tool_loop.go`, `runner.go`
- `internal/plugins/mcpbridge/plugin.go`
- MCP and agent CLI/handlers

**Exit gate**

- Default, custom, and none work across CLI, MCP, graphs, and evidence.
- New servers do not expand defaults.
- Unauthorized calls fail before connection.

### Slice 12 — Bounded remote source resolvers

**Work**

- Add a resolver interface returning bounded content, canonical source,
  requested ref, resolved revision, subpath, digest, and cleanup.
- Support local paths, GitHub shorthand/HTTPS/tree URLs, explicit ref/path, and
  GitHub-backed skills.sh page normalization.
- Enforce request/file/expanded-size bounds, redirect safety, credential
  scoping, and unsafe-file rejection.
- Keep catalog search and arbitrary registries deferred.

**Primary files**

- new `internal/extensions/resolver/*`
- `internal/rootresolver/*`
- `internal/github/*`

**Exit gate**

- Equivalent GitHub and skills.sh inputs resolve the same selected commit.
- Credentials are absent from provenance and unrelated redirects.
- Direct installation requires neither Node nor catalog credentials.

### Slice 13 — Guided runtime-extension setup

**Work**

- Refactor the walkthrough around one reusable input reader.
- Add runtime scope, source discovery, skill installation, agent import/copy,
  translation decisions, model selection, bindings, and MCP access.
- Add `--runtime-only` and `--runtime-scope user|project`.
- Prepare both plans before one preview; apply host installation first and
  report partial success accurately if runtime activation fails.
- Keep unattended flags from selecting sources or granting capabilities.

**Primary files**

- `internal/cli/install.go`
- walkthrough helpers/tests
- host installer integration
- extension transaction/service APIs

**Exit gate**

- Scripted tests cover success, retries, EOF, cancellation, host failure,
  runtime failure, partial success, and runtime-only retry.
- Cancellation before apply changes neither transaction.

### Slice 14 — Runtime scope propagation

**Work**

- Keep host scope and runtime scope separate.
- Put absolute state/config paths into generated MCP commands for project or
  custom runtimes.
- Reject incompatible global-host/project-runtime combinations.
- Verify setup and launched-server configuration precedence match.

**Primary files**

- `internal/installer/installer.go`
- host config generators/tests
- install/root CLI and config loading

**Exit gate**

- Hosts launched from another working directory resolve the selected runtime.
- Unmanaged host configuration remains intact.

### Slice 15 — Pin and bundle the Graphify capability

**Work**

- Pin and verify the Graphify release, MCP extra, Python/platform support,
  query-tool schemas, deterministic code-index command, dependency hashes where
  applicable, and redistribution obligations.
- Add `repo-investigator`, its constitution, the standard `graphify-query`
  skill, bounded references/evals, and host delegation metadata.
- Keep host routing separate from internal query instructions.
- Add the assets to manifests, digests, catalog checks, installer selection,
  and release tests.
- Fix the allowed contract to `query_graph`, `get_node`, `get_neighbors`, and
  `shortest_path`; reject drift.

**Primary files**

- new `agents/repo-investigator.md`
- new `constitutions/repo-investigator.md`
- new `skills/graphify-query/*`
- bundle/catalog tests
- installer wrapper metadata
- dependency specification and notices

**Exit gate**

- Every Graphify integration asset is embedded and digested.
- Claude and Codex wrappers route through Prism.
- Contract fixtures match the pinned upstream schemas.

### Slice 16 — Workspace-bound Graphify runtime

**Work**

- Add a Graphify service/config model for local command and explicitly
  configured self-hosted MCP connections.
- Bind absolute index path, workspace identity, upstream/schema version, and
  generation fingerprint.
- Give the bundled specialist fixed capability independent of user-agent MCP
  defaults.
- Restrict calls to the approved tools and bound traversal, results, rounds,
  and context.
- Reject missing/mismatched bindings before querying.
- Treat graph output as untrusted leads and verify relevant claims against
  current workspace sources.
- Record truthful fallback, freshness, graph leads, verification, and version
  provenance.

**Primary files**

- new `internal/graphify/*`
- runner/tool/evidence integration
- dedicated fixed capability adapter
- workspace and provenance paths

**Exit gate**

- Fixture runs can access only approved tools and the matching index.
- Wrong workspace, missing/stale/unknown index, oversized output, and
  instruction-like graph content remain bounded and explicit.
- Existing specialists and user-agent MCP modes remain unchanged.

### Slice 17 — Graphify setup, readiness, and lifecycle

**Work**

- Offer the bundled capability in guided and runtime-only setup.
- Detect compatible user-managed installations and self-hosted endpoints
  without taking ownership.
- Offer pinned managed-environment setup only after explicit review and
  selection.
- Report dependency installation separately from host/runtime transactions.
- Assist with explicit repository/index selection and deterministic code-only
  indexing; never build implicitly.
- Add local readiness checks and explicit probes for executable/extra,
  version/schema, graph, workspace, and freshness.
- Ensure dry-run, listing, doctor, `--all`, and `--yes` install nothing, build
  nothing, select no cloud backend, and invoke no model.
- Preserve user-managed installations and indexes on uninstall.

**Primary files**

- `internal/cli/install.go`
- installer ownership metadata
- Graphify setup/readiness/doctor helpers
- setup and lifecycle documentation

**Exit gate**

- Missing Graphify does not break startup, catalogs, or other specialists.
- Tests cover user-managed, self-hosted, managed, cancellation, failure, and
  uninstall ownership.
- No unattended path downloads, indexes, selects cloud, or invokes inference.

### Slice 18 — Graphify routing and evidence evaluation

**Work**

- Add fixture hosts and a fake Graphify MCP/index for the full parent →
  `run_agent` → `repo-investigator`/`graphify-query` → evidence → result path.
- Test Claude and Codex trigger descriptions for architecture, relationship,
  path, and impact investigations.
- Ensure ordinary one-file lookups are not over-routed.
- Compare retrieval usefulness, source-verification correctness, tool calls,
  bytes, and context cost with a source-only baseline.
- Document a separate optional real Graphify/offload-model smoke test.

**Primary files**

- Graphify fixtures under `testdata`
- wrapper-rendering and runner/MCP integration tests
- benchmark/eval fixtures and optional smoke script

**Exit gate**

- Fixture routing produces source-cited findings through the intended path.
- Internal instructions are not exported as a direct host workflow.
- Cost and quality measurements exist before broad delegation is recommended.

### Slice 19 — Release hardening and documentation

**Work**

- Complete end-to-end fixtures for persistence, activation, imports, model
  targets, bindings, MCP access, scope, conflicts, recovery, provenance, and
  Graphify.
- Update README, usage, model-runtime, authoring, import/source, recovery,
  guided-install, and Graphify documentation.
- Remove stale directory/tool-call claims and document deferred features.
- Run focused suites before the full repository and CI checks.

**Exit gate**

- Every acceptance-matrix row maps to an automated test or explicitly optional
  live check.
- `go test ./...` and applicable CI checks pass.
- Documentation matches shipped behavior.

### Dependency order

- Slice 0 blocks all feature work.
- Slices 1–2 establish MCP registration.
- Slice 3 blocks managed persistence; Slice 4 blocks runtime-visible
  extensions.
- Slices 5–6 define portable skills; Slice 7 depends on Slices 3–6.
- Slice 8 depends on extension provenance; Slice 9 depends on Slice 8.
- Slice 10 depends on Slices 4, 7, 8, and 9.
- Slice 11 depends on MCP services, catalog snapshots, and managed agents.
- Slice 12 depends on local install/activation services.
- Slices 13–14 depend on stable direct services.
- Slice 15 depends on the stable bundle and portable-skill validation.
- Slice 16 depends on MCP services, snapshots, bounded evidence, per-agent
  access, and Slice 15.
- Slice 17 depends on guided setup, scope propagation, and the Graphify runtime.
- Slice 18 depends on the complete Graphify setup path.
- Slice 19 follows all feature slices.

After Slice 4, skill validation/resources, agent adapter fixtures, remote
resolver fixtures, Graphify contract research/assets, and documentation test
inventory can proceed in parallel. Merge them only after shared manifest and
catalog contracts stabilize.

### Verification strategy

1. Use deterministic protocol servers, fake model runtimes, in-memory
   filesystems, fake GitHub servers, a fake Graphify MCP/index, and fault
   injection by default.
2. Test services before CLI parsing and stable JSON contracts.
3. Reverify bundled compatibility whenever validation, catalogs, prompts, or
   tools change.
4. Run focused package suites and then `go test ./...` at every slice exit.
5. Keep public MCP/GitHub/Graphify checks optional and outside required CI.
6. Preserve unrelated working-tree changes during implementation.

### Main implementation risks

- Building on an unstable bundle/installer baseline.
- Allowing managed content to shadow or mutate bundled content.
- Partial activation under crashes or concurrent mutation.
- Silent dependency rebinding after future bundle-name collisions.
- Treating skill validity as executable capability.
- Importing model/tool semantics without explicit compatible mappings.
- Leaking environment/header credentials.
- Leaving MCP authorization bypasses across direct, plugin, graph, or host
  paths.
- Confusing host and runtime scope.
- Coupling catalog providers to core source resolution.
- Drifting from Graphify's pinned schemas or exposing unrestricted tools.
- Querying the wrong graph, overstating stale evidence, or treating graph
  content as instructions.
- Taking ownership of user-managed Python environments/indexes or triggering
  unattended dependency, index, cloud, or inference work.

### Completion criteria

The implementation is complete when all 20 slices meet their exit gates, every
acceptance-matrix row is covered, bundled behavior remains compatible, runtime
extensions survive binary upgrades and default uninstall, the Graphify route is
workspace-bound and diagnosable, and the documented commands work through both
the direct CLI and Prism MCP server.
