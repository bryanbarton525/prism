# 6. Troubleshooting

[← Testing](testing.md) · [Contents](README.md) · [Contributing →](contributing.md)

## The model endpoint is unreachable

Check DNS, the route from the Prism process, and the exact served model ID.
For SGLang/vLLM, inspect `BASE_URL/models`; the base normally ends in `/v1`.
A failed DNS lookup from one shell does not establish that a Kubernetes workload
is absent. Use your explicit cluster context and namespace to inspect it:

```bash
kubectl --context YOUR_CONTEXT -n YOUR_NAMESPACE get pods
```

If you use a temporary pod IP, a restart may change it. Prefer a stable reachable
service or gateway address. Keep the endpoint choice consistent between the
installer, host process, and live tests.

## Laya is unavailable or returns fallback scores

```bash
curl -fsS http://127.0.0.1:8010/health
systemctl --user list-unit-files 'prism-laya-*.service'
journalctl --user -u UNIT_NAME -n 80 --no-pager
```

Replace `UNIT_NAME` with the exact unit. Confirm `cpu`, English, and the pinned
revision for managed setup. Review warnings in `recommend_tools`; an embedding
score alone does not prove Laya was used. Confirm `PRISM_DECISION_MODEL=laya-english`
and reconnect a long-lived MCP process after changing settings.

The client disables its adapter after an ambiguous five-second HTTP timeout.
First address service load or stalled inference, then restart/reconnect Prism.
Changing a file alone does not reset an already loaded client.

## The catalog is empty

```bash
prism mcp list
prism mcp tools YOUR_SERVER
prism mcp access agent show YOUR_AGENT
```

Check the selected state directory, server transport/authentication, the agent's
MCP capability, and its access rule. Registration, recommendations, and execution
permission are separate checks. Do not broaden access just to suppress an error.
If discovery is truncated or a server fails, inspect `incomplete` and warnings.

## Automatic suggestions do not appear

Automatic injection is off until selected agent IDs are configured. It has a
500 ms budget and can be omitted when the prompt has insufficient room. The
measured 20-candidate English CPU configuration exceeds that time budget.
Use explicit recommendations to check the service before changing routing.

## Setup selects another model or state

Explicit environment values override saved configuration. Inspect the named
settings and the `--state-dir` passed to your host's Prism command. A global
model override can conflict with an imported agent's requested target; choose
the configured model explicitly with `--use-configured-model` when appropriate.
If `PRISM_CONFIG_FILE` points elsewhere, align it before installation.

## Scores look confident but the selected tool is wrong

Choice scores are relative to the supplied candidates and are not calibrated
success probabilities. Check whether retrieval omitted the relevant tool,
whether descriptions or task text exceeded token budgets, and whether the
catalog order changed. Keep execution checks in place and test representative
read, write, long-input, and no-relevant-tool tasks before broad rollout.
