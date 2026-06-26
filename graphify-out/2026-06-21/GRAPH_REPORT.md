# Graph Report - .  (2026-06-21)

## Corpus Check
- 331 files · ~322,692 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1774 nodes · 3536 edges · 112 communities (87 shown, 25 thin omitted)
- Extraction: 84% EXTRACTED · 16% INFERRED · 0% AMBIGUOUS · INFERRED: 550 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_MCP Tool Contracts|MCP Tool Contracts]]
- [[_COMMUNITY_Agent Spec Tests|Agent Spec Tests]]
- [[_COMMUNITY_CLI Runtime Setup|CLI Runtime Setup]]
- [[_COMMUNITY_Graph Execution|Graph Execution]]
- [[_COMMUNITY_Policy Commands|Policy Commands]]
- [[_COMMUNITY_Bundle Installation|Bundle Installation]]
- [[_COMMUNITY_Kubernetes Evidence|Kubernetes Evidence]]
- [[_COMMUNITY_Agent Prompt Assembly|Agent Prompt Assembly]]
- [[_COMMUNITY_Registry State|Registry State]]
- [[_COMMUNITY_Benchmark Comparison|Benchmark Comparison]]
- [[_COMMUNITY_Fallback Runtime|Fallback Runtime]]
- [[_COMMUNITY_Runner Core|Runner Core]]
- [[_COMMUNITY_Config Loading Tests|Config Loading Tests]]
- [[_COMMUNITY_Model Runtime Config|Model Runtime Config]]
- [[_COMMUNITY_Registry Manifests|Registry Manifests]]
- [[_COMMUNITY_Structured Handoff|Structured Handoff]]
- [[_COMMUNITY_MCP Tool Loop|MCP Tool Loop]]
- [[_COMMUNITY_Savings Projection|Savings Projection]]
- [[_COMMUNITY_Fallback Tests|Fallback Tests]]
- [[_COMMUNITY_Ollama Adapter|Ollama Adapter]]
- [[_COMMUNITY_Runtime Types|Runtime Types]]
- [[_COMMUNITY_Result Parsing|Result Parsing]]
- [[_COMMUNITY_Kubernetes Fake Client|Kubernetes Fake Client]]
- [[_COMMUNITY_GitHub File System|GitHub File System]]
- [[_COMMUNITY_Ollama Client|Ollama Client]]
- [[_COMMUNITY_GitHub Contents API|GitHub Contents API]]
- [[_COMMUNITY_Policy Schema|Policy Schema]]
- [[_COMMUNITY_Downstream MCP Client|Downstream MCP Client]]
- [[_COMMUNITY_OpenAI Runtime|OpenAI Runtime]]
- [[_COMMUNITY_Runtime Evidence|Runtime Evidence]]
- [[_COMMUNITY_Assertions Assertions|Assertions Assertions]]
- [[_COMMUNITY_Event Store|Event Store]]
- [[_COMMUNITY_Configuration|Configuration]]
- [[_COMMUNITY_Configuration|Configuration]]
- [[_COMMUNITY_Graph Validation|Graph Validation]]
- [[_COMMUNITY_Bundle Lifecycle|Bundle Lifecycle]]
- [[_COMMUNITY_DB ListOptions|DB ListOptions]]
- [[_COMMUNITY_MCP Server|MCP Server]]
- [[_COMMUNITY_Client Pack|Client Pack]]
- [[_COMMUNITY_formatInt formatProjectionMarkdown|formatInt formatProjectionMarkdown]]
- [[_COMMUNITY_GitHub Integration|GitHub Integration]]
- [[_COMMUNITY_Skill Loading|Skill Loading]]
- [[_COMMUNITY_Plugin Call|Plugin Call]]
- [[_COMMUNITY_Plugin Call|Plugin Call]]
- [[_COMMUNITY_FS ToolCall|FS ToolCall]]
- [[_COMMUNITY_Registry|Registry]]
- [[_COMMUNITY_Skill Loading|Skill Loading]]
- [[_COMMUNITY_GitHub Integration|GitHub Integration]]
- [[_COMMUNITY_Plugin Call|Plugin Call]]
- [[_COMMUNITY_MCP Server|MCP Server]]
- [[_COMMUNITY_Skill Loading|Skill Loading]]
- [[_COMMUNITY_Policy Engine|Policy Engine]]
- [[_COMMUNITY_properties minLength|properties minLength]]
- [[_COMMUNITY_Graph Validation|Graph Validation]]
- [[_COMMUNITY_Agent Registry|Agent Registry]]
- [[_COMMUNITY_EvalCase EvalExpected|EvalCase EvalExpected]]
- [[_COMMUNITY_Duration Artifact|Duration Artifact]]
- [[_COMMUNITY_Route Suggestions|Route Suggestions]]
- [[_COMMUNITY_Skill Loading|Skill Loading]]
- [[_COMMUNITY_Registry|Registry]]
- [[_COMMUNITY_MCP Server|MCP Server]]
- [[_COMMUNITY_Benchmark Data|Benchmark Data]]
- [[_COMMUNITY_Streaming Runtime|Streaming Runtime]]
- [[_COMMUNITY_Reports|Reports]]
- [[_COMMUNITY_Event Store|Event Store]]
- [[_COMMUNITY_Load ReadDir|Load ReadDir]]
- [[_COMMUNITY_Bundle Lifecycle|Bundle Lifecycle]]
- [[_COMMUNITY_Artifact evidence|Artifact evidence]]
- [[_COMMUNITY_Builder Add|Builder Add]]
- [[_COMMUNITY_Policy Engine|Policy Engine]]
- [[_COMMUNITY_additionalProperties properties|additionalProperties properties]]
- [[_COMMUNITY_additionalProperties properties|additionalProperties properties]]
- [[_COMMUNITY_items minItems|items minItems]]
- [[_COMMUNITY_Skill Loading|Skill Loading]]
- [[_COMMUNITY_Dashboard Server|Dashboard Server]]
- [[_COMMUNITY_Agent Registry|Agent Registry]]
- [[_COMMUNITY_Event Store|Event Store]]
- [[_COMMUNITY_Configuration|Configuration]]
- [[_COMMUNITY_MCP Server|MCP Server]]
- [[_COMMUNITY_Reports|Reports]]
- [[_COMMUNITY_rewriteTransport RoundTrip|rewriteTransport RoundTrip]]
- [[_COMMUNITY_ensure collect|ensure collect]]
- [[_COMMUNITY_ensure collect|ensure collect]]
- [[_COMMUNITY_collect detect|collect detect]]
- [[_COMMUNITY_collect detect|collect detect]]
- [[_COMMUNITY_collect detect|collect detect]]
- [[_COMMUNITY_collect detect|collect detect]]
- [[_COMMUNITY_Observer Observe|Observer Observe]]
- [[_COMMUNITY_ToolCallFunction MarshalJSON|ToolCallFunction MarshalJSON]]
- [[_COMMUNITY_doctor DoctorCheck|doctor DoctorCheck]]
- [[_COMMUNITY_additionalProperties type|additionalProperties type]]
- [[_COMMUNITY_Graph Validation|Graph Validation]]
- [[_COMMUNITY_version minimum|version minimum]]
- [[_COMMUNITY_Graph Validation|Graph Validation]]
- [[_COMMUNITY_pre pre|pre pre]]
- [[_COMMUNITY_source type|source type]]
- [[_COMMUNITY_ci ci|ci ci]]
- [[_COMMUNITY_local local|local local]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_collect collect|collect collect]]
- [[_COMMUNITY_GitHub Integration|GitHub Integration]]

## God Nodes (most connected - your core abstractions)
1. `contains()` - 55 edges
2. `readFile()` - 48 edges
3. `writeFile()` - 36 edges
4. `Plugin` - 33 edges
5. `Runner` - 26 edges
6. `T` - 26 edges
7. `collectDiagnostics()` - 25 edges
8. `makeTestRoot()` - 24 edges
9. `registerTools()` - 24 edges
10. `RunWithOptions()` - 21 edges

## Surprising Connections (you probably didn't know these)
- `TestLoadManifest()` --calls--> `writeFile()`  [INFERRED]
  pkg/registry/registry_test.go → internal/app/runner_test.go
- `writeRegistryFile()` --calls--> `writeFile()`  [INFERRED]
  pkg/registry/registry_test.go → internal/app/runner_test.go
- `MonthlyProjectionJSON()` --calls--> `ProjectMonthly()`  [INFERRED]
  pkg/report/report.go → internal/benchmark/scale.go
- `MonthlyProjectionMarkdown()` --calls--> `ProjectMonthly()`  [INFERRED]
  pkg/report/report.go → internal/benchmark/scale.go
- `MonthlyProjectionReport()` --calls--> `ProjectMonthly()`  [INFERRED]
  pkg/report/report.go → internal/benchmark/scale.go

## Import Cycles
- None detected.

## Communities (112 total, 25 thin omitted)

### Community 0 - "MCP Tool Contracts"
Cohesion: 0.06
Nodes (91): AgentRunner, CallToolRequest, CallToolResult, Context, AgentRunner, CallResult, CallToolRequest, CallToolResult (+83 more)

### Community 1 - "Agent Spec Tests"
Cohesion: 0.06
Nodes (72): TestAllowsSkill(), TestParse_IDStemMismatch(), TestParse_MissingFrontmatter(), TestParse_MissingRequiredFields(), TestParse_UnclosedFrontmatter(), TestParse_Valid(), TestResolveConstitution_Body(), TestResolveConstitution_Legacy() (+64 more)

### Community 2 - "CLI Runtime Setup"
Cohesion: 0.05
Nodes (75): agentConstitution(), agentList(), agentShow(), configuredEventSink(), configuredPolicyEngine(), newAgentCmd(), newAgentConstitutionCmd(), newAgentListCmd() (+67 more)

### Community 3 - "Graph Execution"
Cohesion: 0.07
Nodes (53): fakeGraphRunner, dependencyContext(), executeGraph(), graphArtifacts(), graphDepth(), graphError(), graphPolicyDecision(), Load() (+45 more)

### Community 4 - "Policy Commands"
Cohesion: 0.05
Nodes (55): Agent, newPolicyCmd(), newPolicyExplainCmd(), newPolicyTestCmd(), newPolicyValidateCmd(), Defaults, Command, Decision (+47 more)

### Community 5 - "Bundle Installation"
Cohesion: 0.08
Nodes (51): BuildOptions, checkedPublicKey(), downloadFile(), fetchURL(), InstallVerified(), isHTTPURL(), joinRegistryURL(), loadManifestAndSource() (+43 more)

### Community 6 - "Kubernetes Evidence"
Cohesion: 0.08
Nodes (44): Builder, Interface, Config, Context, Deployment, EndpointSlice, Event, Namespace (+36 more)

### Community 7 - "Agent Prompt Assembly"
Cohesion: 0.07
Nodes (44): Spec, Parse(), stemFromName(), validate(), Summary, assemblePrompt(), AssemblePromptForTest(), outputFormatInstruction() (+36 more)

### Community 8 - "Registry State"
Cohesion: 0.10
Nodes (44): RegistrySource, RegistrySources, State, Deprecate(), Load(), LoadManifest(), LoadSources(), Promote() (+36 more)

### Community 9 - "Benchmark Comparison"
Cohesion: 0.07
Nodes (39): TestHomelabReleaseIncident(), chatResult, boolRate(), Compare(), formatMarkdown(), truncate(), ComparisonReport, mockOllamaServer() (+31 more)

### Community 10 - "Fallback Runtime"
Cohesion: 0.08
Nodes (34): New(), shouldFallback(), Runtime, ChatRequest, ChatResponse, Context, Engine, HealthStatus (+26 more)

### Community 11 - "Runner Core"
Cohesion: 0.10
Nodes (29): AgentRunner, Config, Constitution, DownstreamMCPClient, defaultRuntimePlugins(), eventKind(), isRemoteModelRuntime(), New() (+21 more)

### Community 12 - "Config Loading Tests"
Cohesion: 0.14
Nodes (32): writeFile(), chdir(), TestLoadDefaultsWithoutDotEnv(), TestLoadPrefersCanonicalGitHubToken(), TestLoadPrefersPrismEnvOverConfigFile(), TestLoadReadsDotEnvAndGitHubTokenAlias(), TestLoadReadsExplicitPrismConfigFile(), TestLoadReadsModelRuntimeConfig() (+24 more)

### Community 13 - "Model Runtime Config"
Cohesion: 0.10
Nodes (27): configuredModelRuntime(), ModelRuntime, Config, Engine, ModelRuntime, ChatRequest, ChatResponse, Context (+19 more)

### Community 14 - "Registry Manifests"
Cohesion: 0.14
Nodes (33): Bundle, BundleFile, Compat, PublicKey, Time, Manifest, PrivateKey, T (+25 more)

### Community 15 - "Structured Handoff"
Cohesion: 0.09
Nodes (26): EvidenceSummarySchema(), float64Ptr(), GenerateRepoScanSummary(), TestGenerateRepoScanSummary(), TestGenerateRepoScanSummaryDecodeFailure(), TestGenerateRepoScanSummaryRuntimeFailure(), EvidenceSummary, fakeStructuredRuntime (+18 more)

### Community 16 - "MCP Tool Loop"
Cohesion: 0.14
Nodes (25): chatToolResult, agentUsesMCP(), boolArg(), functionTool(), intArg(), mapArg(), marshalToolResult(), ollamaMessagesToRuntime() (+17 more)

### Community 17 - "Savings Projection"
Cohesion: 0.15
Nodes (26): ModelRateProfile, modelRateProfilesFile, ModelShowcaseRow, MonthlyProjectionReport, ProfileProjection, RunSnapshot, buildModelShowcase(), LoadOrchestratorModelProfiles() (+18 more)

### Community 18 - "Fallback Tests"
Cohesion: 0.14
Nodes (18): fakeRuntime, TestChatFallbackDecisions(), TestHealthBothUnhealthy(), TestHealthFallback(), TestStreamPreStartFallback(), TestStructuredFallbackOnTimeout(), ChatRequest, ChatResponse (+10 more)

### Community 19 - "Ollama Adapter"
Cohesion: 0.14
Nodes (17): ChatRequest, ChatResponse, Client, Config, Context, Engine, HealthStatus, ModelRuntime (+9 more)

### Community 20 - "Runtime Types"
Cohesion: 0.10
Nodes (21): Engine, Message, ToolCallFunction, Tool, ToolCall, ToolCallFunction, ToolFunction, ChatRequest (+13 more)

### Community 21 - "Result Parsing"
Cohesion: 0.19
Nodes (20): Artifact, Finding, RunResult, T, RawMessage, agentOutput, buildCompact(), bulletFindings() (+12 more)

### Community 22 - "Kubernetes Fake Client"
Cohesion: 0.13
Nodes (11): Context, Deployment, EndpointSlice, Event, Namespace, Pod, ReplicaSet, Service (+3 more)

### Community 23 - "GitHub File System"
Cohesion: 0.11
Nodes (6): FileInfo, FileMode, ghDirEntry, ghFile, ghFileInfo, Time

### Community 24 - "Ollama Client"
Cohesion: 0.13
Nodes (16): Context, Message, Options, Tool, ToolCall, ToolCallFunction, ToolFunction, ChatRequest (+8 more)

### Community 25 - "GitHub Contents API"
Cohesion: 0.19
Nodes (12): DirEntry, contentEntry, FS, ghDir, decodeContent(), dirEntries(), New(), ParseURL() (+4 more)

### Community 26 - "Policy Schema"
Cohesion: 0.11
Nodes (19): type, type, minimum, type, minimum, type, agent_id, bundle_id (+11 more)

### Community 27 - "Downstream MCP Client"
Cohesion: 0.20
Nodes (14): ClientSession, Content, CallResult, Client, contentText(), New(), ParseArguments(), trim() (+6 more)

### Community 28 - "OpenAI Runtime"
Cohesion: 0.20
Nodes (12): ChatRequest, ChatResponse, Context, HealthStatus, Request, StreamEvent, openAIChatRequest, Reader (+4 more)

### Community 29 - "Runtime Evidence"
Cohesion: 0.20
Nodes (16): collectRuntimeEvidence(), defaultPluginTool(), extractKubernetesDeployment(), extractKubernetesNamespace(), extractKubernetesPod(), extractSearchQuery(), firstRegexCapture(), kubernetesArgs() (+8 more)

### Community 30 - "Assertions Assertions"
Cohesion: 0.18
Nodes (9): Assertions, Assertions, Delegation, Scenario, LoadScenario(), readFile(), Synthesis, Delegation (+1 more)

### Community 31 - "Event Store"
Cohesion: 0.23
Nodes (15): newReportBundlesCmd(), newReportCmd(), newReportEventsCmd(), newReportSkillsCmd(), reportSkillHealth(), writeSummaryCSV(), reportSkill, Command (+7 more)

### Community 32 - "Configuration"
Cohesion: 0.30
Nodes (16): configValue(), defaultRoot(), defaultStateDir(), firstNonEmpty(), isConfigNotFound(), isDefaultOnly(), Load(), loadPrismConfigEnv() (+8 more)

### Community 33 - "Configuration"
Cohesion: 0.15
Nodes (15): Client, Config, Engine, Message, Tool, jsonSchemaFormat, jsonSchemaFormat, firstNonEmpty() (+7 more)

### Community 34 - "Graph Validation"
Cohesion: 0.12
Nodes (17): properties, minimum, type, minimum, type, minimum, type, minimum (+9 more)

### Community 35 - "Bundle Lifecycle"
Cohesion: 0.15
Nodes (16): $ref, additionalProperties, type, additionalProperties, type, properties, agents, bundles (+8 more)

### Community 36 - "DB ListOptions"
Cohesion: 0.31
Nodes (8): DB, ListOptions, Store, boolInt(), Open(), Summary, Context, RunEvent

### Community 37 - "MCP Server"
Cohesion: 0.21
Nodes (10): Context, ToolCall, ToolResult, ToolSpec, Plugin, configuredMCPURL(), containsAny(), extractIssueKeys() (+2 more)

### Community 38 - "Client Pack"
Cohesion: 0.19
Nodes (11): Client, Context, Pack, ToolCall, ToolResult, ToolSpec, Client, Plugin (+3 more)

### Community 39 - "formatInt formatProjectionMarkdown"
Cohesion: 0.27
Nodes (11): formatInt(), formatProjectionMarkdown(), LoadResults(), FormatShowcaseMarkdown(), loadScenarioMeasuredAt(), patchFileBetweenMarkers(), syncTodoScenarioLiveResults(), TestWriteShowcaseDocs() (+3 more)

### Community 40 - "GitHub Integration"
Cohesion: 0.36
Nodes (12): IsURL(), T, Resolve(), initGitRepo(), runGit(), TestCloneFallback_ClonesLocalGitRepo(), TestResolveFSInterface(), TestResolveGitHubFallbackToClone() (+4 more)

### Community 41 - "Skill Loading"
Cohesion: 0.41
Nodes (10): fileExists(), lintSkills(), newSkillBenchmarkCmd(), newSkillCmd(), newSkillLintCmd(), newSkillTestCmd(), printSkillResults(), skillResult (+2 more)

### Community 42 - "Plugin Call"
Cohesion: 0.23
Nodes (9): Plugin, isTextPath(), New(), trim(), Context, FS, ToolCall, ToolResult (+1 more)

### Community 43 - "Plugin Call"
Cohesion: 0.23
Nodes (9): Plugin, isInteresting(), New(), trim(), Context, FS, ToolCall, ToolResult (+1 more)

### Community 44 - "FS ToolCall"
Cohesion: 0.23
Nodes (9): Context, FS, ToolCall, ToolResult, ToolSpec, Plugin, isDocPath(), New() (+1 more)

### Community 45 - "Registry"
Cohesion: 0.27
Nodes (7): Pack, Plugin, NewRegistry(), Registry, ToolCall, ToolResult, ToolSpec

### Community 46 - "Skill Loading"
Cohesion: 0.36
Nodes (9): TestHomelabReleaseIncident_Mock(), repoRoot(), TestAgentSkillAllowlists(), TestAgentSpecsLoad(), TestBenchmarkThresholdsFile(), TestGoldenPromptAssembly_githubCLI(), TestSkillsDiscover(), T (+1 more)

### Community 47 - "GitHub Integration"
Cohesion: 0.42
Nodes (10): mockGitHub(), newTestFS(), TestFS_NotFound(), TestFS_OpenDir(), TestFS_OpenFile(), TestIsURL(), TestParseURL(), FS (+2 more)

### Community 48 - "Plugin Call"
Cohesion: 0.24
Nodes (8): Plugin, New(), trim(), Context, FS, ToolCall, ToolResult, ToolSpec

### Community 49 - "MCP Server"
Cohesion: 0.24
Nodes (7): Constitution, Context, DoctorResult, RunRequest, RunResult, Summary, mcpFakeRunner

### Community 50 - "Skill Loading"
Cohesion: 0.18
Nodes (11): additionalProperties, properties, required, type, agent, type, type, plugins (+3 more)

### Community 51 - "Policy Engine"
Cohesion: 0.18
Nodes (10): additionalProperties, $defs, request, $id, additionalProperties, type, required, $schema (+2 more)

### Community 52 - "properties minLength"
Cohesion: 0.18
Nodes (11): properties, minLength, type, name, request, want_decision, want_reason, $ref (+3 more)

### Community 53 - "Graph Validation"
Cohesion: 0.20
Nodes (9): Artifact, Definition, Limits, Node, RunResult, ValidationResult, Node, Artifact (+1 more)

### Community 54 - "Agent Registry"
Cohesion: 0.50
Nodes (8): makeAgentDir(), TestRegistry_Get_NotFound(), TestRegistry_List_Sorted(), TestRegistry_Load_And_Get(), TestRegistry_Load_InvalidSpec(), TestRegistry_Load_MissingDir(), TestRegistry_Load_RealAgents(), T

### Community 55 - "EvalCase EvalExpected"
Cohesion: 0.28
Nodes (8): EvalCase, EvalExpected, FS, EvalCase, EvalExpected, validateEvalFile(), ValidateEvals(), EvalSuite

### Community 56 - "Duration Artifact"
Cohesion: 0.33
Nodes (6): Duration, Artifact, Finding, Error(), RunResult, Usage

### Community 57 - "Route Suggestions"
Cohesion: 0.28
Nodes (7): Context, Summary, T, fakeLister, TestSuggestKubernetes(), TestSuggestLinear(), TestSuggestReturnsPolicyDenial()

### Community 58 - "Skill Loading"
Cohesion: 0.31
Nodes (7): FS, Skill, T, DiscoverAll(), TestDiscoverAll_realSkillsDir(), ValidateStructure(), joinStrings()

### Community 59 - "Registry"
Cohesion: 0.39
Nodes (5): Registry, NewRegistry(), FS, Spec, Summary

### Community 60 - "MCP Server"
Cohesion: 0.43
Nodes (7): TestContextBudgetAccountingForRemoteMCP(), TestLoadOrchestratorModelProfiles(), TestModelShowcaseDifferentiatedRates(), TestProjectMonthly(), TestScaledSavingsPerRun(), TestSyntheticRemoteMCPToolHandlingSavings(), T

### Community 61 - "Benchmark Data"
Cohesion: 0.36
Nodes (6): FindRepoRoot(), newBenchmarkCmd(), newBenchmarkRunCmd(), newBenchmarkProjectCmd(), Command, Command

### Community 62 - "Streaming Runtime"
Cohesion: 0.54
Nodes (7): OpenAICompatibleRuntime, T, contractRuntime(), TestContractChat(), TestContractHealth(), TestContractStream(), TestContractStructured()

### Community 63 - "Reports"
Cohesion: 0.29
Nodes (6): MonthlyProjection, T, MonthlyProjectionJSON(), MonthlyProjectionMarkdown(), MonthlyProjectionReport(), TestMonthlyProjectionExports()

### Community 64 - "Event Store"
Cohesion: 0.32
Nodes (6): Metadata, NoopSink, RunEvent, Sink, Context, Time

### Community 65 - "Load ReadDir"
Cohesion: 0.38
Nodes (4): Context, FS, cloneFallback(), probe()

### Community 66 - "Bundle Lifecycle"
Cohesion: 0.29
Nodes (6): Compat, File, Installed, Manifest, Compat, File

### Community 67 - "Artifact evidence"
Cohesion: 0.29
Nodes (6): Artifact, Limits, Pack, Artifact, Limits, Time

### Community 68 - "Builder Add"
Cohesion: 0.62
Nodes (5): Builder, isStable(), render(), Section, SectionKind

### Community 69 - "Policy Engine"
Cohesion: 0.29
Nodes (6): additionalProperties, $id, required, $schema, title, type

### Community 70 - "additionalProperties properties"
Cohesion: 0.29
Nodes (7): additionalProperties, properties, type, enum, type, additionalProperties, mode

### Community 71 - "additionalProperties properties"
Cohesion: 0.33
Nodes (7): additionalProperties, properties, required, type, $defs, allowed, allowed

### Community 72 - "items minItems"
Cohesion: 0.29
Nodes (7): items, minItems, type, additionalProperties, required, properties, cases

### Community 73 - "Skill Loading"
Cohesion: 0.29
Nodes (7): type, items, type, plugins, skills, items, type

### Community 74 - "Dashboard Server"
Cohesion: 0.53
Nodes (5): TestOpenMigratesPriorSchema(), TestStoreDoesNotPersistRawPromptFields(), TestStoreIsAppendOnlyAndFiltersMetadata(), TestSummaryIncludesDashboardAndReportFields(), T

### Community 75 - "Agent Registry"
Cohesion: 0.60
Nodes (3): Runner, Context, DoctorResult

### Community 76 - "Event Store"
Cohesion: 0.60
Nodes (4): WriteCSV(), WriteJSON(), RunEvent, Writer

### Community 77 - "Configuration"
Cohesion: 0.50
Nodes (4): Duration, Config, Engine, RuntimeConfig

### Community 78 - "MCP Server"
Cohesion: 0.60
Nodes (4): T, TestMCPContextExtractsIssueKeysAndAction(), TestMCPContextFallsBackToDefaultURL(), TestMCPContextInfersDraftIssueAsCreate()

### Community 79 - "Reports"
Cohesion: 0.67
Nodes (3): LoadThresholds(), SummaryReport, Thresholds

### Community 80 - "rewriteTransport RoundTrip"
Cohesion: 0.50
Nodes (3): rewriteTransport, Request, Response

### Community 90 - "additionalProperties type"
Cohesion: 0.67
Nodes (3): additionalProperties, type, defaults

### Community 91 - "Graph Validation"
Cohesion: 0.67
Nodes (3): minimum, type, max_graph_depth

### Community 92 - "version minimum"
Cohesion: 0.67
Nodes (3): version, minimum, type

### Community 93 - "Graph Validation"
Cohesion: 0.67
Nodes (3): minimum, type, graph_depth

## Knowledge Gaps
- **372 isolated node(s):** `$schema`, `$id`, `title`, `type`, `required` (+367 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **25 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `readFile()` connect `Assertions Assertions` to `MCP Tool Contracts`, `CLI Runtime Setup`, `Graph Execution`, `Policy Commands`, `Bundle Installation`, `Agent Prompt Assembly`, `Registry State`, `Config Loading Tests`, `Registry Manifests`, `Savings Projection`, `Event Store`, `formatInt formatProjectionMarkdown`, `GitHub Integration`, `Skill Loading`, `Plugin Call`, `Plugin Call`, `FS ToolCall`, `GitHub Integration`, `Plugin Call`, `EvalCase EvalExpected`, `Load ReadDir`, `Reports`?**
  _High betweenness centrality (0.125) - this node is a cross-community bridge._
- **Why does `contains()` connect `Agent Spec Tests` to `MCP Tool Contracts`, `Graph Execution`, `Policy Commands`, `MCP Server`, `formatInt formatProjectionMarkdown`, `Agent Prompt Assembly`, `Benchmark Comparison`, `Skill Loading`, `Plugin Call`, `FS ToolCall`, `Config Loading Tests`, `Skill Loading`, `Structured Handoff`, `Plugin Call`, `Result Parsing`, `Kubernetes Fake Client`, `Reports`, `Event Store`?**
  _High betweenness centrality (0.118) - this node is a cross-community bridge._
- **Why does `NewError()` connect `Fallback Tests` to `Configuration`, `Fallback Runtime`, `Model Runtime Config`, `Ollama Adapter`, `OpenAI Runtime`?**
  _High betweenness centrality (0.073) - this node is a cross-community bridge._
- **Are the 53 inferred relationships involving `contains()` (e.g. with `TestParse_IDStemMismatch()` and `TestParse_MissingRequiredFields()`) actually correct?**
  _`contains()` has 53 INFERRED edges - model-reasoned connections that need verification._
- **Are the 40 inferred relationships involving `readFile()` (e.g. with `.Load()` and `.ResolveConstitution()`) actually correct?**
  _`readFile()` has 40 INFERRED edges - model-reasoned connections that need verification._
- **Are the 32 inferred relationships involving `writeFile()` (e.g. with `makeAgentDir()` and `TestResolveConstitution_Legacy()`) actually correct?**
  _`writeFile()` has 32 INFERRED edges - model-reasoned connections that need verification._
- **What connects `$schema`, `$id`, `title` to the rest of the system?**
  _372 weakly-connected nodes found - possible documentation gaps or missing edges._