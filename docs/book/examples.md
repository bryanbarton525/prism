# 3. Worked examples

[← Installation](installation.md) · [Contents](README.md) · [Architecture →](architecture.md)

## A focused Go task through the CLI

```bash
printf '%s\n' 'Write a Go helper that trims surrounding whitespace and rejects an empty name.' | \
  prism run go-helper --skills go-helper-fn --format json
```

Use a small task with concrete inputs and expected behavior. For a bundled
specialist, attach a skill listed in its specification's `allowed_skills`.
The host reviews the result before applying generated code.

## Delegate a repository review through MCP

Call the host's Prism `run_agent` tool with:

```json
{
  "agent_id": "github-cli",
  "skill_names": ["gh-pr-triage"],
  "task": "Report the checks and review blockers for PR 42 in owner/repo.",
  "workspace": {"root": "/absolute/path/to/repository"}
}
```

Replace the repository, PR number, and workspace. GitHub access must already be
configured on the execution machine. A host can discover specialists with
`list_agents` and inspect their constitutions before delegation.

## Discover and recommend authorized tools

Register your own issue MCP endpoint, then explicitly set the agent's access:

```bash
prism mcp add issues --transport streamable-http \
  --url https://YOUR_ISSUE_SERVER/mcp
prism mcp access agent set linear --mode custom --server issues
prism mcp list
prism mcp tools issues
```

Endpoint authentication is server-specific; use `--header-from` for headers
whose values come from environment variables. Registration alone does not mean
an endpoint is reachable or that an agent may execute every tool it exposes.
Review your selected MCP access rule and policy.

After installing Laya, call Prism `recommend_tools` from the host:

```json
{
  "agent_id": "linear",
  "task": "Find issue ENG-731 and return its title without changing it",
  "top_k": 5
}
```

A successful Laya result reports `score_kind: "laya_choice"` and
`model_identity: "laya-english"`. Inspect the returned names, descriptions,
`warnings`, and `incomplete` flag. The list is advisory and no tool was executed.
Choice probabilities compare candidates in this request; do not interpret 0.9
as a calibrated 90% chance of task success.

To execute a bounded lookup, call `run_agent` with the `linear` specialist,
`linear-issue-management` skill, the exact lookup task, and your authorized
endpoint. The model chooses arguments; Prism rechecks authorization at execution.
Use a sandbox endpoint for your first complete test.

## Create an independent managed agent

Copy a bundled specialist and authorize its chosen server:

```bash
prism agent copy linear my-issue-reader
prism agent model set my-issue-reader --use-configured-model
prism mcp access agent set my-issue-reader --mode custom --server issues
prism agent list
```

The copy is a runtime extension; changing it does not edit the release bundle.
For an external agent file, use `prism agent add ./reviewer.md --model MODEL_ID`.
Imports require an explicit model choice and may require decisions for unsupported
source fields. See [agent authoring](../../agents/README.md).

## Validate a bounded graph before running it

```bash
prism graph validate testdata/graphs/k8s-rollout-investigation.yaml
prism graph show testdata/graphs/k8s-rollout-investigation.yaml
```

These commands validate the fixture and show its execution structure. Running
it requires its model, cluster access, and policies. Graph execution and a
repository knowledge graph are separate capabilities; see
[control-plane architecture](../architecture/control-plane.md) and
[Graphify usage](../usage.md#graphify-repository-investigation).
