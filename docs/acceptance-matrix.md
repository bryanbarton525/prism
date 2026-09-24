# Release acceptance matrix

This matrix records deterministic coverage for the runtime-extension and
Graphify release work. The listed Go tests use
only embedded assets, temporary workspaces, in-memory MCP transports, and fake
model runtimes. They do not call a live inference endpoint. Managed-environment
tests use a fake `uv`; public-service checks remain optional.

The reviewed Graphify release is `v0.9.61`; its fixed Prism contract is
`prism-graphify-mcp-v0.9.61`.

| Plan requirement | Deterministic automated evidence | Optional live evidence |
| --- | --- | --- |
| Bundle specialist, constitution, query skill, references, script, eval, manifest digest | `TestEmbeddedBundle`, `TestGraphifyReleaseMetadataIsEmbeddedAndPinned` | None |
| Pin release, MCP extra, Python floor, deterministic code-only index command, asset hash, license/notice basis | `TestGraphifyReleaseMetadataIsEmbeddedAndPinned`, `TestPinnedContractFixtureMatchesRuntimeContract` | Re-verify upstream tag, commit, extract command, release asset, and NOTICE before changing the pin |
| Own a pinned environment only after exact opt-in; keep dry-run/unattended defaults inert; never index automatically | `TestGraphifySetupRequiresApprovalAndDryRunDoesNotWrite`, `TestGraphifyManagedEnvironmentRequiresExplicitSelectionAndRegistersOwnedCommand`, `TestInstallGraphifyFlagsRequireSpecificOptIn`, `TestRunInstallRuntimeOnlyUsesSelectedScope` | Run the frozen sync on the operator's supported Python/platform |
| Allow only the reviewed Graphify query tools and reject schema drift | `TestPinnedContractFixtureMatchesRuntimeContract`, `TestValidateToolArgumentsBoundsAndWorkspaceIsolation`, `TestRunner_Run_RepoInvestigatorUsesOnlyBoundGraphifyTools` | `scripts/graphify-smoke.sh` against the exact pinned release |
| Bind exact workspace/index/fingerprint and diagnose stale or missing prerequisites | `TestBindingMatchesExactWorkspaceAndGeneration`, `TestCheckReadinessDoesNotTreatMissingIndexAsReady`, `TestRunner_Run_RepoInvestigatorRejectsUnavailableOrOversizedGraphify` | `prism graphify doctor` with an operator-built index |
| Preserve separate generic MCP access policy | `TestRunner_Run_RepoInvestigatorUsesOnlyBoundGraphifyTools` | None |
| Keep host routing separate from internal query instructions | `TestGraphifyCapabilityCatalogAndHostWrappers` | Inspect generated Claude/Codex wrappers in a real host |
| Trigger architecture, relationship, dependency-path, and impact routes only | `TestSuggestRepositoryInvestigatorForGraphShapedTasks` | Evaluate host language against representative repository tasks |
| Do not over-route a simple one-file lookup | `TestSuggestDoesNotOverRouteOneFileLookupToGraphify`, `TestGraphifyCapabilityCatalogAndHostWrappers` | None |
| Exercise parent → MCP `run_agent` → specialist → skill → fake Graphify → result path | `TestFixtureParentRoutesGraphifyInvestigationThroughRunAgent` | `scripts/graphify-smoke.sh` |
| Record source verification with graph binding, release, and contract provenance | `TestFixtureParentRoutesGraphifyInvestigationThroughRunAgent`, `TestVerifyGraphSourcesBoundsReadsAndRejectsUnsafePaths` | Confirm returned citations against the current checkout |
| Bound hostile or oversized Graphify output | `TestRunner_Run_RepoInvestigatorRejectsUnavailableOrOversizedGraphify` | None |
| Compare fixture retrieval usefulness, source verification, calls, bytes, and context with source-only baseline | `TestGraphifyFixtureEvaluation` | Repeat the evaluation with a selected offload model before recommending broad delegation |
| Keep documentation and public behavior aligned | `TestGraphifyDocumentationMatchesPinnedContract`, `go test ./...` | Review rendered CLI help and generated wrappers |
| Persist and activate managed runtime assets without losing existing state | `TestRunnerLoadsManagedAgentAndSkillFromCatalogSnapshot`, `TestTransactionCommitAndRollbackOnFailure`, `TestRecoverInterruptedRestoresPreviousManifest` | None |
| Import selected agents/skills and retain their selected model target | `TestRuntimeOnlyImportsTranslatedAgentWithSelectedModel`, `TestLocalAgentServiceInstallListCopyRenameRemove`, `TestInstallResolvedSkillsMaterializesSourceFilesystem` | Inspect a chosen external source before importing it |
| Preserve binding, MCP-access, and host/runtime scope boundaries | `TestBindingMatchesExactWorkspaceAndGeneration`, `TestMCPAccessAllowedServers`, `TestInstallMCPIncludesRuntimeStateDirInCommands`, `TestRunInstallRejectsGlobalProjectRuntimeScope` | None |
| Detect conflicts and recover safely from interrupted installation | `TestComposeCatalogRejectsCollisionsWhenExplicitlyRequested`, `TestInstallRejectsUnmanagedCollision`, `TestRollbackKeepsJournalOnRestoreFailure` | Simulate host filesystem interruption with a disposable operator environment |
| Retain source/import, recovery, and authoring documentation safeguards | `TestSentinelStringsInDocumentation`, `TestValidateAuthoringStructure_RequiresBundledPaths`, `TestGraphifyDocumentationMatchesPinnedContract` | Review rendered documentation before release |
| Resource scope and budgets | `TestSkillResourceToolsEnforceAttachmentAndAggregateBudget`, `TestExplicitSkillResourceAttachmentsAreRunScoped`, `TestReviewResourceReadDoesNotLoadWholeFile`, `TestReviewResourceAllUTF8OffsetsRemainValid`, `TestReviewResourceSkillIdentityCannotEscapeOrAliasRoot`, skill resource package tests | None |
| Portable skill resource compatibility | `TestExecutionLimitationsWarnForAnyScriptFile`, `TestReviewNestedSkillMetadataRemainsAResource`, `TestReviewResourceBoundsMediaAndSymlinkSafety`, `TestReviewResourceRejectsMalformedUTF8Text` | None |
| Native import support files and explicit decisions | `TestNativeAgentImportPreservesConstitutionSupportFile`, `TestImportDecisionAppliesExplicitBudgets`, importer adapter tests | Inspect an external package before activation |
| Full release checks | `scripts/ci-check.sh`, `go vet ./...`, Linux tests, Windows cross-build/test compilation, `uv lock --check` | Optional real runtime smoke tests |

`scripts/graphify-smoke.sh` is intentionally guarded by
`PRISM_GRAPHIFY_SMOKE=1`; it assumes an operator has independently provisioned
the Python dependency, MCP endpoint, index, and model. It is never invoked by
CI.
