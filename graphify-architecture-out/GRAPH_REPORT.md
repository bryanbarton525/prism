# Graph Report - .  (2026-06-23)

## Corpus Check
- 145 files · ~63,595 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1218 nodes · 2040 edges · 92 communities (80 shown, 12 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 58 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_MCP Control Plane Server|MCP Control Plane Server]]
- [[_COMMUNITY_Kubernetes Runtime Plugin|Kubernetes Runtime Plugin]]
- [[_COMMUNITY_Agent Constitution Specifications|Agent Constitution Specifications]]
- [[_COMMUNITY_GitHub Virtual Filesystem|GitHub Virtual Filesystem]]
- [[_COMMUNITY_Operational Workflow Skills|Operational Workflow Skills]]
- [[_COMMUNITY_Core Agent Runner|Core Agent Runner]]
- [[_COMMUNITY_OpenAI Compatible Runtime|OpenAI Compatible Runtime]]
- [[_COMMUNITY_Agent Tool Execution Loop|Agent Tool Execution Loop]]
- [[_COMMUNITY_DAG Execution Engine|DAG Execution Engine]]
- [[_COMMUNITY_Prism Product Architecture|Prism Product Architecture]]
- [[_COMMUNITY_MCP Prompts and Resources|MCP Prompts and Resources]]
- [[_COMMUNITY_Ollama Model Runtime|Ollama Model Runtime]]
- [[_COMMUNITY_Policy Rule Model|Policy Rule Model]]
- [[_COMMUNITY_Model Runtime Interfaces|Model Runtime Interfaces]]
- [[_COMMUNITY_Signed Bundle Registry|Signed Bundle Registry]]
- [[_COMMUNITY_Remote Bundle Installation|Remote Bundle Installation]]
- [[_COMMUNITY_Observability Dashboard Server|Observability Dashboard Server]]
- [[_COMMUNITY_Ollama API Client|Ollama API Client]]
- [[_COMMUNITY_Agent CLI Commands|Agent CLI Commands]]
- [[_COMMUNITY_Downstream MCP Client|Downstream MCP Client]]
- [[_COMMUNITY_Model Runtime Fallback|Model Runtime Fallback]]
- [[_COMMUNITY_Runtime Evidence Collection|Runtime Evidence Collection]]
- [[_COMMUNITY_Runtime Configuration Loading|Runtime Configuration Loading]]
- [[_COMMUNITY_Policy Evaluation Engine|Policy Evaluation Engine]]
- [[_COMMUNITY_Bundle Lifecycle CLI|Bundle Lifecycle CLI]]
- [[_COMMUNITY_Persistent Event Store|Persistent Event Store]]
- [[_COMMUNITY_Agent Result Parsing|Agent Result Parsing]]
- [[_COMMUNITY_Bundle Build and Signing|Bundle Build and Signing]]
- [[_COMMUNITY_Bundle Installation State|Bundle Installation State]]
- [[_COMMUNITY_Go Utility Skills|Go Utility Skills]]
- [[_COMMUNITY_Evidence Handoff Summaries|Evidence Handoff Summaries]]
- [[_COMMUNITY_Model Runtime Factory|Model Runtime Factory]]
- [[_COMMUNITY_Linear Runtime Plugin|Linear Runtime Plugin]]
- [[_COMMUNITY_MCP Bridge Plugin|MCP Bridge Plugin]]
- [[_COMMUNITY_Agent Routing Engine|Agent Routing Engine]]
- [[_COMMUNITY_Skill Management CLI|Skill Management CLI]]
- [[_COMMUNITY_Filesystem Runtime Plugin|Filesystem Runtime Plugin]]
- [[_COMMUNITY_Local GitHub Plugin|Local GitHub Plugin]]
- [[_COMMUNITY_Go Project Plugin|Go Project Plugin]]
- [[_COMMUNITY_Local Documentation Plugin|Local Documentation Plugin]]
- [[_COMMUNITY_Runtime Plugin Registry|Runtime Plugin Registry]]
- [[_COMMUNITY_Skill Loading and Parsing|Skill Loading and Parsing]]
- [[_COMMUNITY_Reporting CLI Commands|Reporting CLI Commands]]
- [[_COMMUNITY_MCP Management CLI|MCP Management CLI]]
- [[_COMMUNITY_MCP Server Configuration|MCP Server Configuration]]
- [[_COMMUNITY_Graph Data Model|Graph Data Model]]
- [[_COMMUNITY_Runtime Error Classification|Runtime Error Classification]]
- [[_COMMUNITY_Agent Specification Registry|Agent Specification Registry]]
- [[_COMMUNITY_Agent Specification Model|Agent Specification Model]]
- [[_COMMUNITY_Root CLI Bootstrap|Root CLI Bootstrap]]
- [[_COMMUNITY_Agent Run CLI|Agent Run CLI]]
- [[_COMMUNITY_Skill Evaluation Suites|Skill Evaluation Suites]]
- [[_COMMUNITY_Frontend Development Skills|Frontend Development Skills]]
- [[_COMMUNITY_Structured Run Results|Structured Run Results]]
- [[_COMMUNITY_Run Event Observation|Run Event Observation]]
- [[_COMMUNITY_Bundle Manifest Model|Bundle Manifest Model]]
- [[_COMMUNITY_Registry Source CLI|Registry Source CLI]]
- [[_COMMUNITY_Routing CLI Commands|Routing CLI Commands]]
- [[_COMMUNITY_Evidence Pack Model|Evidence Pack Model]]
- [[_COMMUNITY_Go Package Scaffold Skill|Go Package Scaffold Skill]]
- [[_COMMUNITY_Prompt Section Builder|Prompt Section Builder]]
- [[_COMMUNITY_Agent Prompt Assembly|Agent Prompt Assembly]]
- [[_COMMUNITY_Event Management CLI|Event Management CLI]]
- [[_COMMUNITY_Graph Management CLI|Graph Management CLI]]
- [[_COMMUNITY_Policy Management CLI|Policy Management CLI]]
- [[_COMMUNITY_Adoption and Usage Reports|Adoption and Usage Reports]]
- [[_COMMUNITY_Workspace Root Resolution|Workspace Root Resolution]]
- [[_COMMUNITY_Agent Runtime Diagnostics|Agent Runtime Diagnostics]]
- [[_COMMUNITY_Argo Diagnostic Skills|Argo Diagnostic Skills]]
- [[_COMMUNITY_Model Runtime Documentation|Model Runtime Documentation]]
- [[_COMMUNITY_Event Export Formats|Event Export Formats]]
- [[_COMMUNITY_Runtime Engine Configuration|Runtime Engine Configuration]]
- [[_COMMUNITY_Skill Discovery|Skill Discovery]]
- [[_COMMUNITY_Monthly Projection Reports|Monthly Projection Reports]]
- [[_COMMUNITY_Release Notes Analysis|Release Notes Analysis]]
- [[_COMMUNITY_Benchmark CLI Commands|Benchmark CLI Commands]]
- [[_COMMUNITY_Configuration Diagnostics CLI|Configuration Diagnostics CLI]]
- [[_COMMUNITY_Dashboard CLI Commands|Dashboard CLI Commands]]
- [[_COMMUNITY_Downstream MCP CLI Wiring|Downstream MCP CLI Wiring]]
- [[_COMMUNITY_Benchmark Project CLI|Benchmark Project CLI]]
- [[_COMMUNITY_Model Runtime CLI Wiring|Model Runtime CLI Wiring]]
- [[_COMMUNITY_Tool Call JSON Encoding|Tool Call JSON Encoding]]
- [[_COMMUNITY_Doctor Result Model|Doctor Result Model]]
- [[_COMMUNITY_Prism Differentiation Documentation|Prism Differentiation Documentation]]
- [[_COMMUNITY_Main Branch Protection Hook|Main Branch Protection Hook]]
- [[_COMMUNITY_CI Validation Script|CI Validation Script]]
- [[_COMMUNITY_Local Acceptance Script|Local Acceptance Script]]
- [[_COMMUNITY_Prism Root Go Package|Prism Root Go Package]]
- [[_COMMUNITY_Argo Workflow Debug Reference|Argo Workflow Debug Reference]]

## God Nodes (most connected - your core abstractions)
1. `Plugin` - 31 edges
2. `Runner` - 24 edges
3. `collectDiagnostics()` - 24 edges
4. `registerTools()` - 20 edges
5. `Policy` - 19 edges
6. `Context` - 17 edges
7. `textResult()` - 17 edges
8. `CallToolResult` - 16 edges
9. `marshalJSON()` - 16 edges
10. `CallToolRequest` - 15 edges

## Surprising Connections (you probably didn't know these)
- `Progressive Skill Disclosure` --semantically_similar_to--> `Runtime Skill Attachment`  [INFERRED] [semantically similar]
  README.md → agents/README.md
- `Read-only Argo Diagnostics` --semantically_similar_to--> `Argo Diagnostic Safety`  [INFERRED] [semantically similar]
  agents/argo.md → constitutions/argo.md
- `Evidence-backed GitHub Diagnostics` --semantically_similar_to--> `Read-only Repository Diagnostics`  [INFERRED] [semantically similar]
  agents/github-cli.md → constitutions/github-cli.md
- `MCP-backed Linear Mutation Evidence` --semantically_similar_to--> `Auditable MCP Mutations`  [INFERRED] [semantically similar]
  agents/linear.md → constitutions/linear.md
- `Local-first Specialist Offload` --rationale_for--> `Agent Specifications`  [INFERRED]
  README.md → agents/README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Specialist Governance Framework** — readme_progressive_disclosure, agents_readme_agent_specifications, constitutions_readme_agent_constitutions, constitutions_readme_shared_agent_rules [INFERRED 0.95]
- **Read-only Diagnostic Specialists** — agents_argo_argo_agent, agents_github_cli_github_cli_agent, agents_kubectl_kubernetes_agent, agents_web_docs_search_web_docs_search_agent [INFERRED 0.85]
- **Bounded Implementation Specialists** — agents_frontend_builder_frontend_builder, agents_go_helper_go_helper_agent, agents_go_scaffold_go_scaffold_agent, constitutions_implementer_implementer_constitution, constitutions_planner_planner_constitution [INFERRED 0.85]
- **Governed Local Offload Control Plane** — architecture_control_plane_prism_control_plane, product_prism_oss_product_definition_policy_engine, product_prism_oss_product_definition_evidence_packs, product_prism_oss_product_definition_signed_bundles, product_prism_oss_product_definition_metadata_first_observability [INFERRED 0.95]
- **Read-Only Argo Diagnostics** — argo_sync_health_skill_argo_sync_health, argo_sync_health_skill_read_only_guardrail, argo_workflow_debug_skill_argo_workflow_debug, argo_workflow_debug_skill_read_only_guardrail [INFERRED 0.95]
- **Static Frontend Application Delivery Flow** — frontend_vanilla_spa_skill_accessible_dependency_free_spa, frontend_localstorage_skill_deterministic_todo_persistence, frontend_readme_skill_static_frontend_local_testing [INFERRED 0.85]
- **GitHub Pull Request and CI Diagnostic Flow** — gh_pr_triage_skill_pr_merge_readiness, gh_actions_diagnostics_skill_actions_failure_diagnosis, gh_actions_diagnostics_skill_approval_gated_reruns [INFERRED 0.85]
- **Go Generation and Validation Pattern** — go_package_scaffold_skill_concise_scaffolding, go_pure_util_skill_deterministic_stateless_utilities, go_test_table_skill_table_driven_structure [INFERRED 0.85]
- **Kubernetes Read-Only Evidence Flow** — k8s_rollout_diagnostics_skill_version_aware_rollout_evidence, kubectl_triage_skill_version_aware_incident_triage, k8s_rollout_diagnostics_skill_read_only_rollout_guardrail, kubectl_triage_skill_read_only_triage_guardrail [INFERRED 0.95]
- **Prism MCP Evidence Governance** — linear_issue_management_skill_prism_linear_mcp_bridge, linear_issue_management_skill_write_evidence_guardrail, prism_mcp_orchestrator_skill_evidence_gate, prism_mcp_orchestrator_skill_required_mcp_call_sequence [INFERRED 0.85]

## Communities (92 total, 12 thin omitted)

### Community 0 - "MCP Control Plane Server"
Cohesion: 0.10
Nodes (63): AgentRunner, CallResult, CallToolRequest, CallToolResult, Client, Constitution, Context, Decision (+55 more)

### Community 1 - "Kubernetes Runtime Plugin"
Cohesion: 0.08
Nodes (44): Builder, Interface, Config, Context, Deployment, EndpointSlice, Event, Namespace (+36 more)

### Community 2 - "Agent Constitution Specifications"
Cohesion: 0.06
Nodes (48): Argo Agent, Read-only Argo Diagnostics, Bounded Vanilla Frontend Scope, Frontend Builder Agent, Evidence-backed GitHub Diagnostics, GitHub CLI Agent, Bounded Go Helpers, Go Helper Agent (+40 more)

### Community 3 - "GitHub Virtual Filesystem"
Cohesion: 0.07
Nodes (17): DirEntry, FileInfo, FileMode, contentEntry, FS, ghDir, ghDirEntry, ghFile (+9 more)

### Community 4 - "Operational Workflow Skills"
Cohesion: 0.06
Nodes (41): Authoritative Source Harvest, Docs Source Harvest Skill, Insufficient Evidence Callback, GitHub Actions Failure Diagnosis, Approval-Gated Workflow Reruns, GitHub Actions Diagnostics Skill, GitHub PR Triage Skill, Pull Request Merge Readiness (+33 more)

### Community 5 - "Core Agent Runner"
Cohesion: 0.10
Nodes (28): AgentRunner, Config, Constitution, DownstreamMCPClient, defaultRuntimePlugins(), eventKind(), isRemoteModelRuntime(), New() (+20 more)

### Community 6 - "OpenAI Compatible Runtime"
Cohesion: 0.09
Nodes (28): ChatRequest, ChatResponse, Client, Config, Context, Engine, HealthStatus, Message (+20 more)

### Community 7 - "Agent Tool Execution Loop"
Cohesion: 0.14
Nodes (25): chatToolResult, agentUsesMCP(), boolArg(), functionTool(), intArg(), mapArg(), marshalToolResult(), ollamaMessagesToRuntime() (+17 more)

### Community 8 - "DAG Execution Engine"
Cohesion: 0.18
Nodes (30): dependencyContext(), executeGraph(), graphArtifacts(), graphDepth(), graphError(), graphPolicyDecision(), Load(), newGraphRunID() (+22 more)

### Community 9 - "Prism Product Architecture"
Cohesion: 0.09
Nodes (27): Governed Specialist Execution, Kubernetes Incident Proof Path, Prism Control Plane, Researcher Agent, Reviewer Agent, Test Designer Agent, Web and Documentation Search Agent, Agent Runner (+19 more)

### Community 10 - "MCP Prompts and Resources"
Cohesion: 0.21
Nodes (23): AgentRunner, CallToolRequest, CallToolResult, Context, GetPromptInput, GetPromptOutput, GetResourceInput, GetResourceOutput (+15 more)

### Community 11 - "Ollama Model Runtime"
Cohesion: 0.14
Nodes (17): ChatRequest, ChatResponse, Client, Config, Context, Engine, HealthStatus, ModelRuntime (+9 more)

### Community 12 - "Policy Rule Model"
Cohesion: 0.11
Nodes (22): Agent, Defaults, Bundle, Duration, Plugin, Policy, Source, Agent (+14 more)

### Community 13 - "Model Runtime Interfaces"
Cohesion: 0.10
Nodes (21): Engine, Message, ToolCallFunction, Tool, ToolCall, ToolCallFunction, ToolFunction, ChatRequest (+13 more)

### Community 14 - "Signed Bundle Registry"
Cohesion: 0.18
Nodes (21): Bundle, BundleFile, Compat, PublicKey, Time, Bundle, BundleFile, Compat (+13 more)

### Community 15 - "Remote Bundle Installation"
Cohesion: 0.25
Nodes (20): checkedPublicKey(), downloadFile(), fetchURL(), InstallVerified(), isHTTPURL(), joinRegistryURL(), loadManifestAndSource(), loadRegistryInputs() (+12 more)

### Community 16 - "Observability Dashboard Server"
Cohesion: 0.19
Nodes (17): eventsView, PageData, Server, intParam(), listOptionsFromRequest(), New(), Serve(), viewEvents() (+9 more)

### Community 17 - "Ollama API Client"
Cohesion: 0.12
Nodes (16): Context, Message, Options, Tool, ToolCall, ToolCallFunction, ToolFunction, ChatRequest (+8 more)

### Community 18 - "Agent CLI Commands"
Cohesion: 0.28
Nodes (17): agentConstitution(), agentList(), agentShow(), configuredEventSink(), configuredPolicyEngine(), newAgentCmd(), newAgentConstitutionCmd(), newAgentListCmd() (+9 more)

### Community 19 - "Downstream MCP Client"
Cohesion: 0.20
Nodes (13): ClientSession, Content, CallResult, Client, contentText(), New(), trim(), trimWithFlag() (+5 more)

### Community 20 - "Model Runtime Fallback"
Cohesion: 0.18
Nodes (12): New(), shouldFallback(), Runtime, ChatRequest, ChatResponse, Context, Engine, HealthStatus (+4 more)

### Community 21 - "Runtime Evidence Collection"
Cohesion: 0.20
Nodes (16): collectRuntimeEvidence(), defaultPluginTool(), extractKubernetesDeployment(), extractKubernetesNamespace(), extractKubernetesPod(), extractSearchQuery(), firstRegexCapture(), kubernetesArgs() (+8 more)

### Community 22 - "Runtime Configuration Loading"
Cohesion: 0.30
Nodes (16): configValue(), defaultRoot(), defaultStateDir(), firstNonEmpty(), isConfigNotFound(), isDefaultOnly(), Load(), loadPrismConfigEnv() (+8 more)

### Community 23 - "Policy Evaluation Engine"
Cohesion: 0.24
Nodes (13): Decision, Request, Policy, Engine, IsBlocking(), Load(), LoadTestSuite(), New() (+5 more)

### Community 24 - "Bundle Lifecycle CLI"
Cohesion: 0.34
Nodes (14): isHTTPRegistrySource(), joinRegistrySource(), newBundleBuildCmd(), newBundleCmd(), newBundleDeprecateCmd(), newBundleInstallCmd(), newBundleListCmd(), newBundlePromoteCmd() (+6 more)

### Community 25 - "Persistent Event Store"
Cohesion: 0.30
Nodes (8): DB, ListOptions, Store, boolInt(), Open(), Summary, Context, RunEvent

### Community 26 - "Agent Result Parsing"
Cohesion: 0.27
Nodes (14): Artifact, Finding, RunResult, RawMessage, agentOutput, buildCompact(), bulletFindings(), decodeFindings() (+6 more)

### Community 27 - "Bundle Build and Signing"
Cohesion: 0.25
Nodes (13): BuildOptions, BuildRegistryManifest(), parsePrivateKey(), registryBundle(), sha256File(), SignRegistryManifest(), WriteRegistryManifest(), SignOptions (+5 more)

### Community 28 - "Bundle Installation State"
Cohesion: 0.24
Nodes (14): RegistrySource, RegistrySources, State, Deprecate(), Load(), LoadManifest(), LoadSources(), Promote() (+6 more)

### Community 29 - "Go Utility Skills"
Cohesion: 0.15
Nodes (14): Complexity and Allocation Tradeoffs, Deterministic Stateless Utilities, Explicit Utility Edge-Case Handling, Go Pure Utility, Deterministic Test Boundaries, Go Table-Driven Test Scaffold, Table-Driven Test Structure, Compact Table-Driven Examples (+6 more)

### Community 30 - "Evidence Handoff Summaries"
Cohesion: 0.19
Nodes (13): EvidenceSummarySchema(), float64Ptr(), GenerateRepoScanSummary(), EvidenceSummary, Finding, RepoScanRequest, Risk, Source (+5 more)

### Community 31 - "Model Runtime Factory"
Cohesion: 0.32
Nodes (11): Config, Engine, ModelRuntime, Builder, NewDefaultRegistry(), newOpenAICompatibleRuntime(), NewRegistry(), NewRuntime() (+3 more)

### Community 32 - "Linear Runtime Plugin"
Cohesion: 0.20
Nodes (10): Context, ToolCall, ToolResult, ToolSpec, Plugin, configuredMCPURL(), containsAny(), extractIssueKeys() (+2 more)

### Community 33 - "MCP Bridge Plugin"
Cohesion: 0.18
Nodes (11): Client, Context, Pack, ToolCall, ToolResult, ToolSpec, Client, Plugin (+3 more)

### Community 34 - "Agent Routing Engine"
Cohesion: 0.26
Nodes (12): Context, Decision, Engine, AgentLister, candidate, Request, Result, Router (+4 more)

### Community 35 - "Skill Management CLI"
Cohesion: 0.41
Nodes (10): fileExists(), lintSkills(), newSkillBenchmarkCmd(), newSkillCmd(), newSkillLintCmd(), newSkillTestCmd(), printSkillResults(), skillResult (+2 more)

### Community 36 - "Filesystem Runtime Plugin"
Cohesion: 0.21
Nodes (9): Plugin, isTextPath(), New(), trim(), Context, FS, ToolCall, ToolResult (+1 more)

### Community 37 - "Local GitHub Plugin"
Cohesion: 0.21
Nodes (9): Plugin, isInteresting(), New(), trim(), Context, FS, ToolCall, ToolResult (+1 more)

### Community 38 - "Go Project Plugin"
Cohesion: 0.21
Nodes (9): Plugin, contains(), New(), trim(), Context, FS, ToolCall, ToolResult (+1 more)

### Community 39 - "Local Documentation Plugin"
Cohesion: 0.21
Nodes (9): Context, FS, ToolCall, ToolResult, ToolSpec, Plugin, isDocPath(), New() (+1 more)

### Community 40 - "Runtime Plugin Registry"
Cohesion: 0.26
Nodes (7): Pack, Plugin, NewRegistry(), Registry, ToolCall, ToolResult, ToolSpec

### Community 41 - "Skill Loading and Parsing"
Cohesion: 0.26
Nodes (7): FS, Skill, dirFromPath(), isNotExist(), LoadDir(), LoadMany(), parse()

### Community 42 - "Reporting CLI Commands"
Cohesion: 0.36
Nodes (10): newReportBundlesCmd(), newReportCmd(), newReportEventsCmd(), newReportSkillsCmd(), reportSkillHealth(), writeSummaryCSV(), reportSkill, Command (+2 more)

### Community 43 - "MCP Management CLI"
Cohesion: 0.51
Nodes (9): newMCPCmd(), newMCPServeCmd(), newMCPServerAddCommandCmd(), newMCPServerAddSSECmd(), newMCPServerCallCmd(), newMCPServerCmd(), newMCPServerListCmd(), newMCPServerToolsCmd() (+1 more)

### Community 44 - "MCP Server Configuration"
Cohesion: 0.33
Nodes (3): Server, State, Server

### Community 45 - "Graph Data Model"
Cohesion: 0.20
Nodes (9): Artifact, Definition, Limits, Node, RunResult, ValidationResult, Node, Artifact (+1 more)

### Community 46 - "Runtime Error Classification"
Cohesion: 0.36
Nodes (7): Engine, Error, ErrorKind, IsKind(), Kind(), KindFromStatus(), NewError()

### Community 47 - "Agent Specification Registry"
Cohesion: 0.31
Nodes (5): Registry, NewRegistry(), FS, Spec, Summary

### Community 48 - "Agent Specification Model"
Cohesion: 0.31
Nodes (6): Spec, Parse(), stemFromName(), validate(), Summary, FS

### Community 50 - "Agent Run CLI"
Cohesion: 0.42
Nodes (8): newRunCmd(), resolveBundleProvenance(), resolveTask(), runAgent(), stdinIsPiped(), runFlags, Command, Context

### Community 51 - "Skill Evaluation Suites"
Cohesion: 0.28
Nodes (8): EvalCase, EvalExpected, FS, EvalCase, EvalExpected, validateEvalFile(), ValidateEvals(), EvalSuite

### Community 52 - "Frontend Development Skills"
Cohesion: 0.25
Nodes (9): Deterministic Todo Persistence, Frontend LocalStorage Skill, Frontend README Skill, Static Frontend Local Testing, Accessible Dependency-Free SPA, Frontend Vanilla SPA Skill, Frontend LocalStorage Reference, Frontend README Reference (+1 more)

### Community 53 - "Structured Run Results"
Cohesion: 0.33
Nodes (6): Duration, Artifact, Finding, Error(), RunResult, Usage

### Community 54 - "Run Event Observation"
Cohesion: 0.32
Nodes (6): Metadata, NoopSink, RunEvent, Sink, Context, Time

### Community 55 - "Bundle Manifest Model"
Cohesion: 0.29
Nodes (6): Compat, File, Installed, Manifest, Compat, File

### Community 56 - "Registry Source CLI"
Cohesion: 0.62
Nodes (6): newRegistryCmd(), newRegistrySourceAddCmd(), newRegistrySourceListCmd(), newRegistrySyncCmd(), validateRegistrySource(), Command

### Community 57 - "Routing CLI Commands"
Cohesion: 0.52
Nodes (6): newRouteCmd(), newRouteExplainCmd(), newRouteSuggestCmd(), printRouteHuman(), Command, Result

### Community 58 - "Evidence Pack Model"
Cohesion: 0.29
Nodes (6): Artifact, Limits, Pack, Artifact, Limits, Time

### Community 59 - "Go Package Scaffold Skill"
Cohesion: 0.29
Nodes (7): Concise Go Package Scaffolding, Exported Symbol Documentation, Go Package Scaffold, Go Scaffold Validation Hints, Effective Go Naming and Comment Style, Go Package Scaffold Reference, No Implicit go.mod Changes

### Community 60 - "Prompt Section Builder"
Cohesion: 0.57
Nodes (5): Builder, isStable(), render(), Section, SectionKind

### Community 61 - "Agent Prompt Assembly"
Cohesion: 0.53
Nodes (4): assemblePrompt(), AssemblePromptForTest(), outputFormatInstruction(), Skill

### Community 62 - "Event Management CLI"
Cohesion: 0.73
Nodes (5): newEventsCmd(), newEventsExportCmd(), newEventsListCmd(), newEventsSummarizeCmd(), Command

### Community 63 - "Graph Management CLI"
Cohesion: 0.73
Nodes (5): newGraphCmd(), newGraphRunCmd(), newGraphShowCmd(), newGraphValidateCmd(), Command

### Community 64 - "Policy Management CLI"
Cohesion: 0.73
Nodes (5): newPolicyCmd(), newPolicyExplainCmd(), newPolicyTestCmd(), newPolicyValidateCmd(), Command

### Community 65 - "Adoption and Usage Reports"
Cohesion: 0.47
Nodes (4): Summary, AdoptionMarkdown(), SavingsMarkdown(), UsageMarkdown()

### Community 66 - "Workspace Root Resolution"
Cohesion: 0.60
Nodes (5): Context, FS, cloneFallback(), probe(), Resolve()

### Community 67 - "Agent Runtime Diagnostics"
Cohesion: 0.60
Nodes (3): Runner, Context, DoctorResult

### Community 68 - "Argo Diagnostic Skills"
Cohesion: 0.50
Nodes (5): Argo Sync Health Skill, Argo Sync Read-Only Guardrail, Argo Workflow Debug Skill, Argo Workflow Mutation Guardrail, Argo Skill Reference Material

### Community 69 - "Model Runtime Documentation"
Cohesion: 0.40
Nodes (5): Model Runtime Fallback, ModelRuntime Interface, OpenAI-Compatible Runtime Adapter, Stable Prompt Prefix Layout, Prism-Owned Runtime Configuration

### Community 70 - "Event Export Formats"
Cohesion: 0.60
Nodes (4): WriteCSV(), WriteJSON(), RunEvent, Writer

### Community 71 - "Runtime Engine Configuration"
Cohesion: 0.40
Nodes (4): Duration, Config, Engine, RuntimeConfig

### Community 72 - "Skill Discovery"
Cohesion: 0.60
Nodes (4): FS, Skill, DiscoverAll(), ValidateStructure()

### Community 74 - "Release Notes Analysis"
Cohesion: 0.40
Nodes (5): Release Notes Skill Reference, Official Release Note Coverage, Release Notes Scan, Upgrade Delta Analysis, Web Docs Search Agent

### Community 75 - "Benchmark CLI Commands"
Cohesion: 0.83
Nodes (3): newBenchmarkCmd(), newBenchmarkRunCmd(), Command

### Community 76 - "Configuration Diagnostics CLI"
Cohesion: 0.83
Nodes (3): newConfigCmd(), newDoctorCmd(), Command

### Community 77 - "Dashboard CLI Commands"
Cohesion: 0.83
Nodes (3): newDashboardCmd(), newDashboardServeCmd(), Command

### Community 78 - "Downstream MCP CLI Wiring"
Cohesion: 0.83
Nodes (3): configuredDownstreamMCPState(), withConfiguredLinearMCP(), State

## Knowledge Gaps
- **239 isolated node(s):** `github.com/bryanbarton525/prism`, `Summary`, `FS`, `Context`, `AgentRunner` (+234 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What connects `github.com/bryanbarton525/prism`, `Summary`, `FS` to the rest of the system?**
  _250 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `MCP Control Plane Server` be split into smaller, more focused modules?**
  _Cohesion score 0.1001984126984127 - nodes in this community are weakly interconnected._
- **Should `Kubernetes Runtime Plugin` be split into smaller, more focused modules?**
  _Cohesion score 0.07832080200501253 - nodes in this community are weakly interconnected._
- **Should `Agent Constitution Specifications` be split into smaller, more focused modules?**
  _Cohesion score 0.05585106382978723 - nodes in this community are weakly interconnected._
- **Should `GitHub Virtual Filesystem` be split into smaller, more focused modules?**
  _Cohesion score 0.07317073170731707 - nodes in this community are weakly interconnected._
- **Should `Operational Workflow Skills` be split into smaller, more focused modules?**
  _Cohesion score 0.06341463414634146 - nodes in this community are weakly interconnected._
- **Should `Core Agent Runner` be split into smaller, more focused modules?**
  _Cohesion score 0.09957325746799431 - nodes in this community are weakly interconnected._