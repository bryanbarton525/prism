---
name: prism-mcp-orchestrator
description: >-
  Auto-use for context-heavy prompts (logs, CI failures, Kubernetes issues,
  runbooks, docs lookup, PR triage, codegen subtasks) and bounded Laya/Jev
  decisions through downstream MCP. Decide when to delegate
  via Prism MCP and execute the required call order — list_agents, list_resources,
  get_resource, optional list_prompts/get_prompt, run_agent, then parent synthesis.
compatibility: Requires Prism MCP server availability, local repository context, and an MCP-capable host editor/agent.
metadata:
  prism-agents: orchestrator parent-model
---

# Prism MCP orchestrator skill

Use this skill for orchestration and bounded typed decisions. Delegate evidence-heavy specialist work via Prism MCP; use the decision workflow below for configured Laya/Jev tools.

## Activation rule

Activate when prompts mention things like: CI failures, PR blockers, Kubernetes incidents, Argo drift/workflow failures, release notes/docs lookup, large logs/JSON, bounded Laya/Jev decisions, or "delegate/run_agent/MCP".

Do **not** activate for tiny single-step tasks where delegation overhead is not worth it.

## Fail-closed evidence gate

Before delegation, check for minimum identifiers:

- GitHub: `repo` + (`PR`, `run URL`, or `run ID`)
- Kubernetes: `context` + `namespace` + workload (`deployment`/`statefulset`/`pod`)
- Release notes/docs: product/library + version range or target release

If required identifiers are missing, **do not run specialists yet**. Ask the parent LLM to request only missing fields.

## First-party skill precedence

When a subtask targets a specific platform/tool/vendor, prefer that vendor's
first-party skills/docs before community or generic sources.

Examples:

- LaunchDarkly tasks -> prefer LaunchDarkly first-party skills (for example from
  the LaunchDarkly `ai-tooling/skills` source) and official docs.
- Cloud/provider SDK tasks -> prefer official provider skills/docs first.

If a first-party source exists but is not available in the current environment,
callback to parent with `insufficient_evidence` and request permission/inputs to
access the first-party source.

## Required sequence for specialist delegation (must follow)

1. Call `list_agents` first.
2. Call `list_resources`, then `get_resource` for:
   - `prism://resource/tooling/run_agent`
   - `prism://resource/tooling/orchestration-guide`
   - `prism://resource/agent/<agent_id>/constitution` (for selected agents)
3. Optional: `list_prompts` + `get_prompt` for payload templates.
4. Call `run_agent` with bounded task + valid `skill_names`.
5. Synthesize specialist outputs in parent model.

If evidence is insufficient at any step, callback to parent with:

```json
{
  "status": "insufficient_evidence",
  "needs_from_parent": [
    "repo owner/name",
    "PR URL or workflow run URL",
    "namespace/deployment"
  ],
  "reason": "Identifiers required for evidence-backed delegation are missing."
}
```

## Typed decisions through downstream MCP

Use a configured Laya or Jev MCP tool for a bounded classification, a choice
among explicit alternatives, or a rubric score when that judgment helps the
task. The parent model supplies the question and options, interprets the result,
and produces any subsequent general tool-call arguments. Prism executes calls.

1. Call `list_mcp_servers` to discover configured decision servers. If none is
   configured, continue the task without a decision call. Registration and agent
   access setup are described in `docs/decision-services.md` in the Prism repo.
2. Call `list_mcp_server_tools` with `include_schema: true` for the selected
   server. Use its actual tool schema; Laya and Jev wrappers differ.
3. Call `call_mcp_tool` with the discovered server and tool. For Laya's
   `laya_predict`, an example is:

   ```json
   {
     "server": "laya",
     "tool": "laya_predict",
     "arguments": {
       "model": "english",
       "state": {"body": "I was charged twice."},
       "questions": {
         "department": {
           "type": "choice",
           "instructions": "Which team should handle this request?",
           "criteria": {
             "billing": "Invoices and refunds",
             "technical": "Bugs and outages"
           }
         }
       }
     }
   }
   ```

4. Check `is_error` and `truncated` before interpreting the result. Laya's
   decision JSON may be encoded in `content` or in `structured_content.result`;
   decode that JSON to read `answers`. Report uncertainty or continue with parent
   reasoning if the call fails. A decision does not grant permission for an action.

Laya custom questions include choices on each call; `laya_preset` supplies only
its built-in question sets. The remote Laya wrapper forwards inference to its
HTTP server and does not support `laya_shortlist` or `laya_tournament`. Keep
finite choices small (at most 20), narrowing them first when needed.

This downstream MCP path is separate from `recommend_tools`, whose built-in
reranker uses `PRISM_DECISION_*` over HTTP. Reconnect a running Prism MCP host
after changing registrations. A CLI probe tests transport; a specialist also
needs access to the server through its configured MCP allowlist.

## Adapted routing checkpoint

When `laya-routing` is registered, use it only for English next-tool choice
questions. Keep general classification, rubric scoring, and built-in presets on
`laya` or the configured Jev service. Inspect the actual MCP schema first.

For a routing call:

1. Supply `model: "english"`, `max_len: 1024`, `head_max_len: 512`. The official
   MCP tool requires an object: use `state: {"text": "TASK:\n<user task>\nCOMPLETED TOOL EVIDENCE:\n<actual completed results>"}`.
   The specialized endpoint unwraps this exact one-field envelope losslessly.
2. Supply a `choice` question with opaque keys `tool_0`, `tool_1`, etc. Values
   describe `server.tool: description`. Use a small relevant catalog (the
   adaptation covers 2–8 choices), preserving descriptions and actual results.
3. Before any call, ask exactly: "Choose the first tool call required by the task.
   Obtain prerequisite identifiers before following linked records."
   After a completed call, ask: "Choose the next action needed for the task using
   the completed results. Finish only after the requested records have been retrieved."
   Include a finish choice only after evidence exists:
   "finish: The retrieved evidence already answers the task, including an explicit
   access denial or empty result. Finish without repeating completed calls."
4. Resolve the returned key against the supplied catalog. Generate arguments from
   the selected tool's discovered input schema; obtain linked identifiers from
   actual preceding results. The classifier does not generate arguments or execute
   MCP. Check completion against the user's requested fields before accepting finish.

The routing model is experimental and confidence is not calibrated. These prompts
were developed against the recorded workflow fixtures; perfect fixture results
would not establish universal routing reliability. Keep automatic recommendation
injection opt-in and apply the existing authorization policy before execution.
See `docs/decision-services.md` and the bound-routing evaluation report for setup,
artifact identity, measured failures, and reproducible commands.

## run_agent shaping

- Keep tasks narrow and evidence-oriented.
- Attach only relevant evidence per specialist.
- For MCP evidence tasks, include an explicit collection protocol in the task:
  "Obtain external facts by actually calling the MCP bridge before producing the
  final envelope. During collection emit a tool call, not a simulated result.
  Use the inventory schemas; obtain linked record IDs from earlier results.
  Once evidence is complete, populate the supplied decision schema, preserving
  field types and nulls. Empty lists and access denials are final evidence."
  Specify required answer fields or pass `decision_schema` when available;
  keep expected values out of the task. Inspect actual call evidence before
  accepting external facts. This protocol helped the recorded SGLang probes;
  a skill-only rewrite did not reproduce that improvement.
- Prefer multiple focused calls over one mixed-domain call.
- Keep final judgment/tradeoffs/recommendation in parent synthesis.
- For tool-specific tasks, include first-party source preference in the task
  text so specialists ground findings in vendor-authored guidance.

## Minimal call template

```json
{
  "agent_id": "github-cli",
  "skill_names": ["gh-pr-triage"],
  "task": "Repository owner/repo. Triage PR #42 for merge blockers with evidence.",
  "format": "json"
}
```

## Parent output contract

- Concise synthesis (no raw dumps)
- Ordered findings with evidence
- Confidence + concrete next action
- If evidence is missing: return `insufficient_evidence` with required follow-up fields, not speculative findings
