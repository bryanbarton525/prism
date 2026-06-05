# Prism OSS Product Definition, Architecture Plan, and Constitution

## 1. Project Vision

Prism is a local-first AI delegation control plane for engineering teams.

Its purpose is to keep the primary AI editor, chat model, or MCP host focused by offloading narrow, evidence-heavy, repeatable engineering tasks to governed local specialists.

Prism is not just a CLI, not just an MCP server, and not just a local Ollama wrapper. Prism is the operating layer for safe, repeatable AI offloading across platform, SRE, DevOps, and software engineering workflows.

The long-term goal is to make Prism the best open-source system for:

* running local specialist agents;
* distributing approved skills and agents;
* collecting bounded operational evidence;
* enforcing team-level policy;
* observing adoption and effectiveness;
* benchmarking context and token savings;
* turning internal runbooks and engineering standards into reusable AI workflows.

## 2. The Problem Prism Solves

Modern AI developer workflows are becoming bloated and inconsistent.

Teams often push too much context into premium frontier models:

* repo rules;
* runbooks;
* CI logs;
* Kubernetes dumps;
* Argo CD status;
* Datadog logs;
* internal docs;
* prior chat history;
* skill instructions;
* debugging standards;
* implementation patterns.

This creates several problems:

1. The orchestrator model becomes overloaded.
2. Costs rise as context grows.
3. Output quality degrades because the model has too much irrelevant context.
4. Teams duplicate prompt and skill patterns across repos.
5. There is no consistent way to govern or review AI workflows.
6. Sensitive operational data is pasted into cloud tools unnecessarily.
7. There is no audit trail of what was delegated, what skills were used, or what worked.
8. Platform teams cannot easily distribute approved AI workflows to developers.

Prism solves this by making AI offloading explicit, governed, local-first, observable, and repeatable.

## 3. Product Thesis

Prism should become the open-source standard for local-first AI offloading.

The primary product thesis is:

> Platform and engineering teams should be able to convert their runbooks, tools, debugging patterns, and standards into signed local AI specialists, distribute them safely, observe adoption, measure context savings, and keep sensitive evidence out of cloud model sprawl.

Prism should own the category of:

> AI Workflow Offload Control Plane

or, more specifically:

> Local-first AI delegation infrastructure for engineering operations.

## 4. What Prism Is

Prism is:

* an MCP server;
* a CLI;
* a local specialist agent runtime;
* a skill and agent registry;
* a runtime plugin host;
* an evidence collection system;
* an observability and reporting layer;
* a benchmark and savings engine;
* a governed workflow offload platform;
* a framework for controlled multi-step AI delegation.

## 5. What Prism Is Not

Prism is not:

* a replacement for Cursor, Claude Code, ChatGPT, Gemini, or other AI editors;
* a general-purpose autonomous swarm framework;
* a cloud-first AI SaaS;
* a tool that meters every run;
* a system that encourages pasting unlimited context into frontier models;
* a marketplace of random unreviewed prompts;
* a shell-out automation tool with no safety boundary;
* a system that stores sensitive prompts or raw logs by default.

## 6. Core Design Principles

### 6.1 Keep the Editor in Charge

Prism should not replace the user’s AI editor or orchestrator.

The editor, chat model, or MCP host remains the high-level reasoning interface. Prism provides governed specialist execution underneath it.

### 6.2 Offload Narrow Work

Prism should specialize in bounded, narrow, high-signal tasks.

Good Prism tasks include:

* summarize this CI failure;
* inspect this Kubernetes rollout;
* analyze these pod events;
* search these docs;
* generate a small Go helper;
* review a Terraform plan;
* explain an Argo CD sync issue;
* extract actionable findings from logs.

Bad Prism tasks include:

* “build my entire product autonomously”;
* “read my whole repo and decide everything”;
* “run arbitrary commands until fixed”;
* “act without policy or approval.”

### 6.3 Local-First by Default

Prism should prefer local execution.

The default model path should use local Ollama specialists. Future integrations with other local or remote runtimes may exist, but Prism’s unique advantage is that sensitive evidence can stay close to the developer or team environment.

### 6.4 Metadata Over Raw Prompts

Prism should not store raw prompts, raw logs, or sensitive evidence by default.

The observability system should store metadata:

* run ID;
* actor ID;
* workspace ID;
* source;
* agent ID;
* skills used;
* model;
* status;
* duration;
* token estimates;
* context budget;
* policy decision;
* bundle version.

Raw task capture should be opt-in, local-only, and clearly labeled.

### 6.5 Govern Skills Like Software

Skills and agents are not random prompt files.

Prism should treat skills, agents, and constitutions as versioned operational assets.

They should support:

* linting;
* testing;
* review;
* signing;
* publishing;
* promotion;
* rollback;
* deprecation;
* compatibility checks;
* runtime observation.

### 6.6 Make Safety Structural

Prism should not depend only on model behavior for safety.

Safety should come from:

* skill allowlists;
* runtime plugin boundaries;
* read-only evidence collectors;
* policy checks;
* bounded DAG execution;
* context budgets;
* latency budgets;
* file integrity verification;
* signed bundles;
* explicit approvals for risky actions.

### 6.7 Measure Everything Useful

Prism should make it easy to prove value.

It should report:

* input context reduction;
* estimated token savings;
* agent adoption;
* skill usage;
* validation failures;
* timeouts;
* context budget warnings;
* most effective workflows;
* flaky or low-confidence skills;
* team and workspace trends.

### 6.8 Stay Open Source

All major product capabilities should live in the OSS repo.

Monetization, if pursued later, should come from:

* hosted convenience;
* enterprise support;
* certified skill packs;
* private bundle distribution services;
* consulting;
* training;
* managed upgrades;
* security review support;
* internal platform enablement;
* optional cloud aggregation.

The OSS binary should not be intentionally crippled.

## 7. Bounded DAG Definition

A DAG is a Directed Acyclic Graph.

In Prism, a DAG represents a multi-step offload workflow where each node is a bounded task and edges represent dependencies.

Example:

```text
collect-k8s-evidence
        |
        v
analyze-rollout
        |
        +--> inspect-events
        +--> inspect-container-restarts
        |
        v
summarize-root-cause
```

A DAG is “acyclic” because it cannot loop forever. A task can depend on earlier tasks, but no task can eventually depend on itself.

A Prism DAG must be bounded.

Bounded means the workflow has hard limits, such as:

* maximum number of nodes;
* maximum graph depth;
* maximum parallel nodes;
* maximum runtime;
* maximum retries;
* maximum evidence size;
* maximum prompt size;
* maximum artifacts;
* approved agents only;
* approved skills only;
* approved runtime plugins only.

A bounded DAG gives Prism more power than a single agent call while avoiding uncontrolled autonomous behavior.

Prism should prefer bounded DAGs over open-ended agent loops.

## 8. High-Level Architecture

```text
AI Editor / MCP Host / CLI
        |
        v
Prism Interface Layer
        |
        +--> CLI commands
        +--> MCP tools
        +--> Local API
        |
        v
Offload Router + Policy Engine
        |
        +--> Manual routing
        +--> Suggested routing
        +--> Automatic policy-approved routing
        |
        v
Agent Runner
        |
        +--> Agent specs
        +--> Skill bundles
        +--> Constitutions
        +--> Runtime plugins
        +--> Evidence packs
        |
        v
Local Specialist Model Runtime
        |
        +--> Ollama
        +--> future local runtimes
        +--> future optional remote runtimes
        |
        v
Compact Result Envelope
        |
        v
Observation Sink
        |
        +--> Local event store
        +--> Dashboard
        +--> Reports
        +--> Audit export
```

## 9. Major System Components

### 9.1 Agent Runner

The Agent Runner is responsible for executing one specialist invocation.

It should:

* resolve the requested agent;
* validate requested skills;
* load the constitution;
* load skill content;
* collect bounded runtime evidence;
* assemble the prompt;
* enforce context budget;
* enforce latency budget;
* call the local model runtime;
* normalize output;
* emit a run event.

### 9.2 Offload Router

The Offload Router decides or recommends which specialist should handle a task.

The router should support three modes:

#### Manual Mode

The user or parent model explicitly chooses:

```text
agent_id
skills
task
```

#### Suggest Mode

Prism analyzes the task and recommends an offload plan.

Example result:

```json
{
  "agent_id": "kubectl",
  "skills": ["k8s-rollout-diagnostics"],
  "reason": "The task references a failed rollout and pod health.",
  "risk": "read_only",
  "requires_approval": false
}
```

#### Auto Mode

Prism automatically routes the task when policy allows.

Auto mode must always be bounded by policy.

The router should begin as deterministic and rule-based. Later versions may support local model classification, but the initial implementation should be predictable, testable, and explainable.

### 9.3 Policy Engine

The Policy Engine controls what is allowed.

Policies should govern:

* allowed agents;
* allowed skills;
* allowed runtime plugins;
* allowed sources;
* allowed workspaces;
* allowed registry bundles;
* required approvals;
* max graph depth;
* max graph nodes;
* max evidence size;
* max runtime;
* whether raw prompt capture is allowed;
* whether remote model runtimes are allowed;
* whether write actions are allowed.

Initial policy format should be simple YAML.

Example:

```yaml
version: 1

defaults:
  mode: suggest
  raw_prompt_capture: false
  max_graph_nodes: 6
  max_graph_depth: 3
  max_parallel_nodes: 2
  max_runtime_seconds: 120
  max_evidence_bytes: 500000

agents:
  kubectl:
    allowed: true
    plugins:
      kubernetes:
        mode: read_only
    skills:
      - k8s-rollout-diagnostics
      - k8s-pod-crashloop-triage

  go-helper:
    allowed: true
    skills:
      - go-helper-implementation

sources:
  cli:
    allowed: true
  mcp:
    allowed: true

write_actions:
  allowed: false
```

### 9.4 Runtime Plugins

Runtime plugins collect bounded evidence.

They should not blindly expose shell access.

A plugin should:

* declare its capabilities;
* declare read-only vs write behavior;
* enforce limits;
* return structured artifacts;
* redact sensitive data where possible;
* be observable;
* fail closed.

Initial priority plugins:

* Kubernetes;
* GitHub;
* Argo CD;
* local docs;
* filesystem read-only search;
* Go project metadata;
* Terraform plan reader;
* Datadog or log provider adapters later.

### 9.5 Evidence Packs

Evidence packs are structured artifacts collected by runtime plugins.

Example:

```json
{
  "kind": "kubernetes.rollout_evidence",
  "namespace": "payments",
  "workload": "checkout-api",
  "limits": {
    "max_pods": 20,
    "max_events": 50
  },
  "summary": {
    "desired_replicas": 4,
    "available_replicas": 2,
    "unavailable_replicas": 2
  },
  "artifacts": [
    {
      "type": "pod",
      "name": "checkout-api-abc123",
      "status": "CrashLoopBackOff"
    }
  ]
}
```

Specialists should reason over evidence packs instead of collecting unlimited context themselves.

### 9.6 Skill and Agent Registry

Prism should support managed, signed distribution of agents and skills.

The registry system should support:

* signed manifests;
* bundle IDs;
* bundle versions;
* bundle channels;
* compatibility checks;
* file checksums;
* safe install paths;
* promotion;
* rollback;
* deprecation;
* local registry sources.

Example bundle:

```yaml
id: k8s-core-triage
version: 1.4.2
channel: stable
owner: platform-sre
risk_level: read_only

agents:
  - kubectl

skills:
  - k8s-rollout-diagnostics
  - k8s-pod-crashloop-triage
  - k8s-service-connectivity

required_plugins:
  - kubernetes

compat:
  min_prism_version: 0.3.0
  max_prism_version: 1.0.0

evaluation_suite:
  - crashloop-basic
  - failed-rollout
  - imagepullbackoff
```

### 9.7 Skill Lifecycle Manager

Prism should support the full lifecycle of skills and bundles.

Lifecycle stages:

```text
create
lint
test
benchmark
review
sign
publish
install
promote
observe
deprecate
remove
```

Commands should eventually include:

```bash
prism skill lint
prism skill test
prism skill benchmark
prism bundle build
prism bundle sign
prism bundle publish
prism bundle install
prism bundle promote
prism bundle rollback
prism bundle deprecate
```

### 9.8 Run Graph Engine

The Run Graph Engine executes bounded DAG workflows.

It should support:

* graph validation;
* dependency ordering;
* parallel execution where safe;
* max node count;
* max depth;
* max runtime;
* max retries;
* artifact passing between nodes;
* result aggregation;
* policy checks before execution;
* event emission per node and per graph.

Example graph definition:

```yaml
id: k8s-rollout-investigation
version: 1

limits:
  max_nodes: 5
  max_depth: 3
  max_parallel: 2
  timeout_seconds: 120

nodes:
  collect:
    agent: kubectl
    skills:
      - k8s-evidence-collector
    task: Collect bounded rollout evidence.

  analyze:
    depends_on:
      - collect
    agent: kubectl
    skills:
      - k8s-rollout-diagnostics
    task: Analyze the collected rollout evidence.

  summarize:
    depends_on:
      - analyze
    agent: kubectl
    skills:
      - incident-summary
    task: Produce a concise root-cause summary and next steps.
```

The graph engine should not support unbounded loops in the initial design.

### 9.9 Local Dashboard

Prism should include an OSS local dashboard.

The dashboard should show:

* total runs;
* runs over time;
* run status breakdown;
* top agents;
* top skills;
* most common validation failures;
* timeouts;
* context budget warnings;
* token estimates;
* estimated context avoided;
* bundle versions;
* workspace adoption;
* policy denials;
* graph executions;
* plugin usage.

The dashboard should run locally.

Possible command:

```bash
prism dashboard serve
```

or:

```bash
prism server
```

The initial dashboard can be simple and embedded in the Go binary.

### 9.10 Event Store

Prism should support local append-only event storage.

Initial storage should use SQLite.

Storage principles:

* local-first;
* metadata-first;
* raw prompts disabled by default;
* append-only event log;
* aggregate views derived from events;
* exportable JSON and CSV;
* safe schema migrations;
* no cloud dependency.

Possible database tables:

```text
runs
run_skills
run_artifacts
run_graphs
graph_nodes
bundles
policies
workspaces
actors
```

### 9.11 Reports

Prism should produce reports for:

* benchmark projections;
* observed run usage;
* adoption;
* estimated savings;
* context avoided;
* bundle health;
* skill quality;
* workspace usage;
* policy denials.

Commands:

```bash
prism report benchmark
prism report usage
prism report adoption
prism report savings
prism report bundles
```

Output formats:

```text
markdown
json
csv
html
```

## 10. Core Contracts

Prism should be built around stable contracts.

### 10.1 Event Contract

A run event represents one completed agent invocation.

Fields should include:

```text
timestamp
run_id
graph_id
graph_node_id
actor_id
workspace_id
source
correlation_id
agent_id
model
status
skills
duration_ms
prompt_tokens_estimate
completion_tokens_estimate
context_budget
prompt_size_estimate
context_budget_exceeded
policy_decision
bundle_id
bundle_version
error
validation_error
```

### 10.2 Policy Contract

A policy describes what Prism is allowed to do.

Policy decisions should be explicit:

```text
allow
deny
require_approval
warn
```

A policy decision should include a reason.

Example:

```json
{
  "decision": "deny",
  "reason": "Agent kubectl requested skill k8s-write-remediation, but write actions are disabled."
}
```

### 10.3 Bundle Contract

A bundle is a signed, versioned package of agents, skills, constitutions, tests, and metadata.

A bundle should include:

```text
id
version
channel
owner
description
risk_level
agents
skills
constitutions
required_plugins
compatibility
files
checksums
signature
evaluation_suite
deprecation_status
```

### 10.4 Evidence Contract

An evidence artifact is bounded structured context collected by a runtime plugin.

It should include:

```text
kind
source
plugin
collection_time
limits
summary
artifacts
redactions
errors
```

### 10.5 Run Graph Contract

A run graph is a bounded DAG of agent invocations.

It should include:

```text
graph_id
version
limits
nodes
edges
policy
status
artifacts
aggregate_result
```

## 11. CLI Design

Prism should continue supporting the current core CLI, then add higher-level commands.

### 11.1 Existing Core Commands

```bash
prism config doctor
prism agent list
prism agent show
prism agent constitution
prism run
prism mcp serve
prism benchmark run
prism benchmark project
```

### 11.2 Proposed New Commands

#### Server and Dashboard

```bash
prism server
prism dashboard serve
```

#### Events

```bash
prism events list
prism events export
prism events summarize
```

#### Registry and Bundles

```bash
prism registry source add
prism registry source list
prism registry sync
prism bundle list
prism bundle install
prism bundle update
prism bundle rollback
prism bundle verify
prism bundle sign
```

#### Skills

```bash
prism skill lint
prism skill test
prism skill benchmark
```

#### Policy

```bash
prism policy validate
prism policy explain
prism policy test
```

#### Router

```bash
prism route suggest
prism route explain
```

#### Graphs

```bash
prism graph run
prism graph validate
prism graph show
```

#### Reports

```bash
prism report usage
prism report savings
prism report adoption
prism report bundles
```

## 12. MCP Design

Prism’s MCP interface should expose tools that support both current single-run behavior and future governed workflows.

Current tools should remain:

```text
list_agents
run_agent
get_constitution
doctor
list_prompts
get_prompt
list_resources
get_resource
```

Future MCP tools:

```text
suggest_route
run_graph
list_bundles
install_bundle
list_policies
explain_policy
get_usage_summary
get_skill_health
```

The MCP layer should never bypass policy.

## 13. Repository Structure

Recommended repo structure:

```text
cmd/
  prism/

internal/
  app/
  agent/
  skill/
  result/
  benchmark/
  ollama/
  mcp/
  cli/

  observe/
  server/
  dashboard/
  events/
  policy/
  router/
  graph/
  bundles/
  registryadmin/
  reports/
  identity/
  workspace/
  plugins/

pkg/
  observe/
  registry/
  report/
  policy/
  graph/
  bundle/
  evidence/

docs/
  architecture/
  product/
  usage.md
  comparison.md
  benchmark-scale.md
  prism-oss-product-definition.md

testdata/
  benchmarks/
  bundles/
  policies/
  graphs/
  skills/
```

Public reusable packages should live under `pkg/`.

Implementation details should remain under `internal/`.

## 14. Implementation Plan

This plan avoids the term “epics” and uses workstreams instead.

## Workstream 1: OSS Product Definition

Goal: Document the product direction and constitution.

Tasks:

* Add this document under `docs/product/prism-oss-product-definition.md`.
* Add a shorter architecture overview under `docs/architecture/control-plane.md`.
* Update README to describe Prism as a local-first AI offload control plane.
* Keep README focused and avoid overloading it with every planned feature.
* Add a roadmap section that distinguishes implemented, in progress, and planned work.
* Make clear that Prism remains useful as a standalone OSS CLI and MCP server.

Acceptance criteria:

* The repo has a clear product definition.
* The repo describes Prism as more than a local agent runner.
* The OSS roadmap is visible without implying incomplete features already exist.

## Workstream 2: Event Store

Goal: Turn run events into local observable history.

Tasks:

* Add `internal/events`.
* Implement SQLite-backed event store.
* Implement append-only run event writes.
* Add event listing.
* Add event export as JSON.
* Add event export as CSV.
* Add retention settings.
* Add migration support.
* Wire the existing observation sink into the event store.
* Add CLI command `prism events list`.
* Add CLI command `prism events export`.

Acceptance criteria:

* Every run can be stored locally.
* Raw prompts are not stored by default.
* Events can be queried and exported.
* Existing OSS behavior remains unchanged unless event storage is enabled.

## Workstream 3: Local Dashboard

Goal: Provide a local UI for Prism adoption and run visibility.

Tasks:

* Add `internal/server`.
* Add `internal/dashboard`.
* Add `prism dashboard serve`.
* Serve a local web UI.
* Show run count.
* Show run status breakdown.
* Show top agents.
* Show top skills.
* Show context budget warnings.
* Show token estimates.
* Show recent runs.
* Add basic filters by actor, workspace, source, agent, skill, and status.

Acceptance criteria:

* A user can run a local dashboard.
* Dashboard reads from local event store.
* Dashboard does not require cloud services.
* Dashboard does not expose sensitive task content by default.

## Workstream 4: Policy Engine

Goal: Add policy-aware execution.

Tasks:

* Add `pkg/policy` for stable policy contract types.
* Add `internal/policy` for implementation.
* Define YAML policy format.
* Support allow, deny, warn, and require approval decisions.
* Enforce allowed agents.
* Enforce allowed skills.
* Enforce allowed plugins.
* Enforce max runtime.
* Enforce max graph nodes.
* Enforce max graph depth.
* Add CLI command `prism policy validate`.
* Add CLI command `prism policy explain`.
* Wire policy checks into `prism run`.

Acceptance criteria:

* Policy can deny invalid or unsafe requests before model execution.
* Policy decisions are recorded in run events.
* Policy failures return clear human-readable reasons.
* Existing users without policy config are not broken.

## Workstream 5: Managed Bundle Commands

Goal: Productize signed skill and agent distribution.

Tasks:

* Build on existing registry verification primitives.
* Add registry source config.
* Add `prism registry source add`.
* Add `prism registry source list`.
* Add `prism registry sync`.
* Add `prism bundle list`.
* Add `prism bundle install`.
* Add `prism bundle verify`.
* Add bundle channel metadata.
* Add bundle owner metadata.
* Add bundle risk level metadata.
* Add bundle deprecation metadata.
* Store installed bundle state locally.
* Record bundle ID and version in run events when applicable.

Acceptance criteria:

* Users can install signed bundles from configured sources.
* Prism verifies signatures and file checksums before install.
* Bundle installs are deterministic and path-safe.
* Installed bundle versions are observable.

## Workstream 6: Skill Lifecycle

Goal: Make skills testable and governable.

Tasks:

* Add `prism skill lint`.
* Validate required skill structure.
* Validate metadata.
* Validate examples.
* Validate expected output contract.
* Add `prism skill test`.
* Add golden task fixtures.
* Add expected output assertions.
* Add context size checks.
* Add benchmark checks.
* Add `prism bundle build`.
* Add `prism bundle sign`.

Acceptance criteria:

* A skill can be validated before use.
* A bundle can be tested before distribution.
* Skill quality can be measured over time.
* Platform teams can review and promote skills confidently.

## Workstream 7: Offload Router

Goal: Recommend the correct agent and skills for a task.

Tasks:

* Add `internal/router`.
* Add route request and route result types.
* Implement deterministic rule-based matching.
* Match on keywords, task category, plugin availability, and policy.
* Add `prism route suggest`.
* Add `prism route explain`.
* Add MCP tool `suggest_route`.
* Record route recommendation in run metadata when used.
* Support manual, suggest, and auto modes.

Acceptance criteria:

* Prism can recommend an agent and skills for common tasks.
* Recommendations include reasons.
* Recommendations respect policy.
* Router behavior is testable and deterministic.

## Workstream 8: Evidence Packs

Goal: Standardize plugin-produced evidence.

Tasks:

* Add `pkg/evidence`.
* Define evidence artifact contract.
* Update Kubernetes plugin to emit typed evidence packs.
* Add evidence limits.
* Add redaction metadata.
* Add evidence summary field.
* Add evidence error field.
* Add tests for bounded evidence size.
* Add docs for evidence pack format.

Acceptance criteria:

* Runtime plugins return structured evidence.
* Evidence collection is bounded.
* Evidence artifacts are usable by specialists and dashboards.
* Evidence format is reusable across plugins.

## Workstream 9: Run Graph Engine

Goal: Support bounded multi-step workflows.

Tasks:

* Add `pkg/graph` for graph contract types.
* Add `internal/graph` for execution engine.
* Define graph YAML format.
* Validate DAG structure.
* Reject cycles.
* Enforce max nodes.
* Enforce max depth.
* Enforce max parallelism.
* Enforce timeout.
* Execute nodes in dependency order.
* Pass artifacts between nodes.
* Emit events per node.
* Emit aggregate graph event.
* Add `prism graph validate`.
* Add `prism graph run`.
* Add MCP tool `run_graph`.

Acceptance criteria:

* Prism can run a bounded DAG workflow.
* Cyclic graphs are rejected.
* Policy is enforced before execution.
* Each node result is observable.
* Graph results are compact and useful.

## Workstream 10: Reports

Goal: Extend benchmark reporting into real usage reporting.

Tasks:

* Keep benchmark projection reports.
* Add observed usage report.
* Add adoption report.
* Add bundle health report.
* Add skill health report.
* Add estimated savings report.
* Add Markdown output.
* Add JSON output.
* Add CSV output.
* Add dashboard integration.

Acceptance criteria:

* Users can report on actual Prism usage.
* Reports use event store data.
* Benchmark and observed usage can be compared.
* Reports are useful for platform teams.

## Workstream 11: Runtime Plugin Expansion

Goal: Make Prism valuable for real platform/SRE workflows.

Initial plugin priorities:

1. Kubernetes rollout and pod diagnostics.
2. GitHub Actions failure analysis.
3. Argo CD sync diagnostics.
4. Local docs and runbook search.
5. Terraform plan reader.
6. Go project helper.
7. Datadog/log provider adapter.

Tasks:

* Define plugin capability metadata.
* Define read-only vs write capability modes.
* Add plugin policy checks.
* Add plugin evidence size limits.
* Add plugin tests with fixtures.
* Add docs for building new plugins.

Acceptance criteria:

* Plugins are safe by default.
* Plugins declare capabilities.
* Plugins return bounded evidence.
* Plugins can be governed by policy.

## Workstream 12: Documentation and Examples

Goal: Make Prism understandable and adoptable.

Tasks:

* Add architecture docs.
* Add policy examples.
* Add bundle examples.
* Add run graph examples.
* Add skill lifecycle docs.
* Add evidence pack docs.
* Add dashboard docs.
* Add end-to-end Kubernetes incident demo.
* Add end-to-end GitHub Actions triage demo.
* Add “How Prism compares” update.
* Add “Why local-first offloading” document.

Acceptance criteria:

* A new user can understand Prism’s purpose.
* A platform engineer can create a skill bundle.
* A developer can run Prism from an MCP host.
* A team can observe usage locally.

## 15. First Killer Demo

The first complete product demo should be:

> Team-managed Kubernetes incident offload.

Scenario:

A platform team publishes a signed bundle called:

```text
k8s-core-triage
```

The bundle includes:

* `kubectl` agent;
* rollout diagnostics skill;
* crashloop triage skill;
* service connectivity skill;
* incident summary skill;
* Kubernetes read-only plugin requirement;
* golden tests;
* benchmark fixture.

Developer task:

```text
Investigate why checkout-api is failing in staging.
```

Prism flow:

1. MCP host sends task to Prism.
2. Offload router recommends `kubectl` with `k8s-rollout-diagnostics`.
3. Policy confirms the task is read-only and allowed.
4. Kubernetes plugin collects bounded evidence.
5. Local specialist analyzes the evidence.
6. Prism returns compact summary, findings, confidence, and artifacts.
7. Event store records the run.
8. Dashboard shows usage, status, skill, duration, and estimated context avoided.
9. Report shows adoption and savings.

This demo proves:

* local-first execution;
* governed skills;
* signed bundle distribution;
* policy enforcement;
* bounded evidence collection;
* useful offloading;
* observable adoption;
* context savings.

## 16. Success Metrics

Prism should measure success through product and technical metrics.

### Product Metrics

* Number of active workspaces.
* Number of runs per week.
* Number of unique agents used.
* Number of unique skills used.
* Number of installed bundles.
* Number of successful offloads.
* Number of repeated workflows.
* Reduction in validation failures over time.
* Adoption by team or workspace.

### Technical Metrics

* Median run duration.
* Timeout rate.
* Context budget exceeded rate.
* Prompt token estimate.
* Completion token estimate.
* Estimated orchestrator input avoided.
* Plugin evidence size.
* Error rate by agent.
* Error rate by skill.
* Policy denial rate.

### Quality Metrics

* Golden task pass rate.
* Skill regression rate.
* Bundle promotion success rate.
* Low-confidence result rate.
* Human override rate, if tracked later.

## 17. Security and Privacy Requirements

Prism must be safe for sensitive engineering environments.

Requirements:

* Do not store raw prompts by default.
* Do not store raw evidence by default unless explicitly enabled.
* Prefer local event storage.
* Make cloud export optional.
* Redact known sensitive fields in evidence packs.
* Support policy denial for risky plugins.
* Support read-only plugin modes.
* Support signed bundle verification.
* Prevent path traversal in bundle installs.
* Reject untrusted bundles by default when configured.
* Make policy decisions visible.
* Make unsafe actions explicit.

## 18. Open Source Monetization Strategy

Because the full product will live in OSS, monetization should not rely on withholding core features.

Potential monetization paths:

* hosted dashboard convenience;
* hosted private registry;
* certified skill and agent bundles;
* enterprise support;
* platform team onboarding;
* custom plugin development;
* regulated environment deployment assistance;
* security review support;
* training workshops;
* priority maintenance;
* managed upgrade support;
* internal AI workflow consulting;
* sponsorship;
* dual-license only for embedded redistribution if ever needed.

The OSS version should remain complete and useful.

The business value should come from trust, support, convenience, and expertise.

## 19. Non-Goals

Prism should avoid these traps:

* becoming a generic agent framework;
* building a full IDE;
* competing directly with Cursor or Claude Code UX;
* adding unrestricted autonomous loops;
* storing sensitive raw context by default;
* over-optimizing for cloud SaaS too early;
* making every feature require a server;
* hiding core functionality behind commercial gates;
* allowing plugin execution without policy boundaries;
* treating skills as unversioned prompt snippets.

## 20. Immediate Next Steps

Recommended first implementation sequence:

1. Add this document to the repo.
2. Update README with the new product framing.
3. Add local event store.
4. Add event export commands.
5. Add local dashboard.
6. Add policy contract.
7. Add policy enforcement for agent and skill allowlists.
8. Add registry source commands.
9. Add bundle install/list/verify commands.
10. Add skill linting.
11. Add route suggest command.
12. Add evidence pack contract.
13. Convert Kubernetes plugin evidence to structured evidence packs.
14. Add graph contract.
15. Add bounded graph validation.
16. Add bounded graph execution.
17. Build the Kubernetes incident offload demo.

Current implementation status:

* Signed registry manifests are verified with Ed25519 signatures, Prism version compatibility, SHA-256 file checksums, and path-safety checks before install.
* MCP route suggestions and graph runs use the configured policy engine when a policy file is supplied.
* Graph execution remains serial in v1, but graph validation rejects cycles and unsupported parallel execution, policy is checked before execution, prior node summaries/artifacts are passed forward in bounded form, and graph aggregate events are emitted when event storage is enabled.
* Dashboard and reports summarize local event-store metadata, including graph executions, policy denials, plugin usage, bundle versions, validation failures, and token estimates.
* CLI and MCP runs can carry explicit bundle provenance; CLI runs can resolve the installed bundle version from local state, and both policy and event storage receive the bundle ID/version.
* The `testdata/bundles/k8s-core-triage` fixture includes a signed registry manifest and public key for the Kubernetes incident proof path.

## 21. Constitution

The following principles are binding design rules for Prism development.

### Article 1: Prism Shall Remain Local-First

Prism must work without a required cloud service.

Cloud integrations may be added, but they must not be required for core execution.

### Article 2: Prism Shall Keep the Orchestrator Focused

Prism exists to reduce unnecessary context loaded into the primary AI model.

Features that increase context bloat must justify themselves clearly.

### Article 3: Prism Shall Favor Bounded Delegation Over Autonomy

Prism may support multi-step workflows, but they must be bounded.

Uncontrolled loops, unlimited retries, unlimited tool access, and unbounded context collection are not acceptable defaults.

### Article 4: Prism Shall Treat Skills as Governed Assets

Skills, agents, and constitutions should be versioned, testable, reviewable, signable, and observable.

### Article 5: Prism Shall Fail Closed

When signature verification, policy validation, compatibility checks, or path safety checks fail, Prism must stop rather than continue unsafely.

### Article 6: Prism Shall Not Store Sensitive Content by Default

Prism should store metadata first.

Raw prompts, raw logs, and raw evidence capture must be opt-in.

### Article 7: Prism Shall Make Decisions Explainable

Routing decisions, policy decisions, validation failures, and bundle verification failures should include clear reasons.

### Article 8: Prism Shall Prefer Deterministic Systems Before Model-Based Systems

Routers, policies, validators, and graph execution should begin deterministic and testable.

Model-based classification may be added later, but it should not replace explainable control logic.

### Article 9: Prism Shall Work With Existing Developer Tools

Prism should integrate through CLI and MCP rather than forcing users into a new editor or workflow.

### Article 10: Prism Shall Be Useful as OSS

The open-source version should be a complete and valuable product.

Commercial opportunities should come from support, hosting, certified bundles, and operational expertise rather than intentionally crippling the OSS tool.

## 22. One-Sentence Product Definition

Prism is an open-source, local-first AI offload control plane that lets engineering teams turn repeatable platform, SRE, and development workflows into governed local specialists with signed skills, bounded evidence collection, observable runs, and measurable context savings.
