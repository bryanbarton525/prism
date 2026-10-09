# 2. Installation and configuration

[← Quick start](quick-start.md) · [Contents](README.md) · [Examples →](examples.md)

## Keep the scopes distinct

| Selection | Controls | Typical location |
| --- | --- | --- |
| Host project scope | Integration with one editor project | Project `.prism/install.json` and host directories |
| Host global scope | User-wide editor integration | User host configuration and `~/.prism/install.json` |
| Runtime user scope | Shared runtime settings, extensions, and MCP registry | `~/.prism/` |
| Runtime project scope | An isolated project's runtime state | Project `.prism/` |
| Workspace | Repository data for a task | Explicit `workspace.root`, host root, or CLI working directory |

A global host installation cannot select project runtime scope. Generated MCP
registrations include an absolute `--state-dir`, so opening another workspace
does not silently select different runtime state. Explicit process environment
settings override the selected state's saved `config.env`.

Use `--runtime-only` to configure runtime state without editing host files.
Use `--dry-run` to preview selected downloads, writes, and probes without doing
them. If `PRISM_CONFIG_FILE` is set, align it with the selected state's file.

## The three model roles

| Role | Example | Configuration |
| --- | --- | --- |
| Offload model | Your Qwen model through SGLang | `PRISM_MODEL_RUNTIME_*` |
| Candidate retrieval | MiniLM ONNX or Potion embeddings | `PRISM_TOOL_RECOMMEND_MODEL` |
| Candidate reranking | Laya English choices | `PRISM_DECISION_*` |

Laya does not serve chat tasks. MiniLM does not choose tool arguments. The
executing specialist still makes tool calls through Prism's authorization path.

## Managed Laya

```bash
prism install --runtime-only --runtime-scope user \
  --tool-model onnx --decision-service install-laya --laya-port 8010
```

Requires Linux, `uv`, `systemctl`, and `loginctl`. Prism creates `laya/venv`
beneath the selected state, uses Python 3.13, pins Laya 0.3.22, CPU PyTorch
2.8.0 and Transformers 5.17.0, and pins the English checkpoint revision recorded
in the [evaluation](../laya-evaluation.md). It enables a per-state systemd user
service and linger, with loopback binding, four CPU threads, and one resident
checkpoint. Readiness verifies actual CPU execution, the revision, and inference
before saving the endpoint. `--laya-uv` selects another `uv` executable.

The stock Laya server uses PyTorch. MiniLM retrieval uses Prism's pure-Go ONNX
backend. Installing Laya does not export its weights to ONNX.

For an existing endpoint:

```bash
prism install --runtime-only --runtime-scope user \
  --decision-service laya --decision-url http://127.0.0.1:8010
```

HTTP is allowed only on loopback; use HTTPS for remote endpoints. For a token,
use `--decision-key-env LAYA_API_KEY` after exporting `LAYA_API_KEY`.
See the optional settings in [examples/config.env](../../examples/config.env).

## Migrate from Kev

1. Install Laya and run the checks in [chapter 5](testing.md).
2. Confirm saved `PRISM_DECISION_MODEL=laya-english` and the Laya endpoint.
3. Stop and disable your old per-state `prism-kev-<state-hash>.service` after
   successful validation. Find the exact name with `systemctl --user list-unit-files`.
4. Reconnect Prism MCP so it loads the new executable and configuration.

The installer clears saved `PRISM_KEV_*` settings on Laya selection, but does
not stop an independently installed Kev unit. Legacy Kev/Jev remains supported
when no generic decision URL is set. Generic settings take precedence as a group.
To return to managed Kev, select `--decision-service install`, validate it,
then stop the unused Laya unit. Existing model caches can remain for reuse.

Automatic shortlist injection is off by default. Its 500 ms budget is shorter
than evaluated English CPU inference with 20 tools. Use explicit recommendations
before opting agent identities into injection.

## Upgrade and remove

Build or download the chosen Prism executable, then rerun your selected host
installation. The installer updates recorded paths and refuses unmanaged
collisions. Keep the prior binary if you need local rollback. Long-lived MCP
processes retain their startup snapshot until reconnected.

`prism uninstall --global` removes recorded host integration. Managed runtime
services and downloaded models have their own lifecycle; inspect and stop the
specific unit before deleting any selected runtime state.
