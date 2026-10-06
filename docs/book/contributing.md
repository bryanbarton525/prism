# 7. Contributing

[← Troubleshooting](troubleshooting.md) · [Contents](README.md)

## Prepare a development checkout

```bash
git clone https://github.com/bryanbarton525/prism.git
cd prism
git switch -c feat/your-change
go build -o ./prism ./cmd/prism
./prism version --json
```

Read applicable `AGENTS.md` instructions and [CONTEXT.md](../../CONTEXT.md).
Use the project's terms: offload model, release bundle, runtime extension, host
installation, and tool recommender. The [architecture chapter](architecture.md)
points to the main code paths.

## Make the change reviewable

1. Define the behavior, affected users, and expected failure path.
2. Change the relevant module and add deterministic tests at its interface.
3. Keep live-service tests optional and guarded by explicit environment settings.
4. Update the matching book chapter, examples, and detailed reference.
5. Run the release checks and inspect the diff before committing.

For a new skill or agent, follow [skill authoring](../../skills/README.md) and
[agent specifications](../../agents/README.md). A skill may carry references,
scripts, and evals; adding those resources does not grant execution capability.
Importing an external agent requires an explicit offload-model target.

## Validate before opening a PR

```bash
gofmt -w path/to/changed.go
TEST_STATE=$(mktemp -d)
PRISM_STATE_DIR="$TEST_STATE" bash scripts/ci-check.sh
git diff --check
git status --short
```

Run the relevant optional live checks from [chapter 5](testing.md) when changing
model integration. Record the exact configuration, fixtures, result, and limits.
Do not call a fixture smoke test a production benchmark. Keep secrets, local
Python environments, downloaded weights, and machine-specific raw logs out of
the commit. Follow repository instructions for generated knowledge-graph updates.

## Raise a PR

Commit and push a focused branch, then open a PR against `main`. Include:

- A small diagram or diff sketch showing the behavioral change.
- Concrete before/after evidence and commands that reviewers can reproduce.
- Compatibility, rollback behavior, and the affected runtime paths.
- Any live coverage that was skipped and the prerequisites for running it.

An installer change needs coverage for saved configuration, explicit selection,
preview behavior, and failure cleanup. A recommender change needs authorization,
score validation, candidate mapping, fallback, and latency-budget coverage.
Documentation changes need valid local links and command examples supported by
the actual CLI.

CI runs the deterministic release script. Live endpoints remain optional. A PR
is ready for review when its source, documentation, examples, and tests describe
the same behavior.
