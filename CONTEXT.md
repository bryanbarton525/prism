# Prism

Prism executes bounded specialist work for an AI editor or MCP host that remains the orchestrator.

## Language

**Offload model**:
A model served locally or on a user-operated self-hosted machine that executes Prism specialist tasks. It is distinct from the parent host's orchestrating model.
_Avoid_: Translation model, same-machine-only model

**Skill**:
A portable package of task instructions and optional resources in the Agent Skills open format. A skill describes a capability an agent can use, rather than defining the agent itself.
_Avoid_: Agent specification

**Skill resource**:
A supporting file packaged with a skill, such as reference material, a template, an asset, or a script. Packaging a resource does not grant permission or capability to execute it.
_Avoid_: Workspace file

**Agent specification**:
The definition of a Prism specialist's instructions and execution configuration, including its model and permitted skills. Imported specifications may originate in another host's agent format.
_Avoid_: Skill

**Release bundle**:
The immutable collection of built-in agents, skills, and supporting content distributed with a Prism release.
_Avoid_: Extension, workspace

**Runtime extension**:
A separately tracked user-added agent or skill that extends what Prism can execute alongside its release bundle.
_Avoid_: Release bundle, host installation

**Host installation**:
The integration of Prism with an AI editor or MCP host through skills, specialist wrappers, and an MCP server registration.
_Avoid_: Runtime extension

**Skill binding**:
An explicit allowance for a Prism agent to use a named skill on an invocation. User-managed bindings belong to user-added agents; bundled agents retain their release-defined allowances.
_Avoid_: Skill installation, automatic skill attachment

**Runtime scope**:
The reach of a Prism runtime configuration and its user-added agents, skills, and downstream MCP servers: user-wide or explicitly isolated to a project.
_Avoid_: Host installation scope, workspace

**Host installation scope**:
The reach of Prism's integration with an editor or MCP host: a single project or the user's host configuration across projects.
_Avoid_: Runtime scope

**Default MCP server set**:
A user-selected shared list of downstream MCP servers available to user-added agents that choose the default. Those agents inherit changes to the list; registering a server does not add it automatically.
_Avoid_: All registered servers

**Repository knowledge graph**:
An index of repository entities and their relationships used to locate and connect evidence. Its contents may lag current source files.
_Avoid_: Execution DAG, authoritative source snapshot

**Graphify integration**:
Prism's bundled repository-investigation capability, connecting a specialist to Graphify's repository knowledge graph. Its instructions, external service, and repository index are distinct parts of the capability.
_Avoid_: Automatically active MCP server, general-purpose graph-building agent

**Tool recommender**:
Prism's advisory capability for finding useful tools within an agent's authorized tool catalog. It neither executes tools nor grants access to them.
_Avoid_: Offload model, tool executor

**Tool shortlist**:
A bounded set of candidate tools supplied by the tool recommender for a task. A shortlist does not assert that any candidate is suitable.
_Avoid_: Approved tool set, execution plan

**Retained tool result**:
The portion of a downstream tool result preserved for the owning agent run and accessible through bounded reads. A result preview is an explicitly incomplete view when the retained portion is larger.
_Avoid_: Permanent artifact, complete source snapshot
