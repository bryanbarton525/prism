#!/usr/bin/env bash
set -euo pipefail

if [[ "${PRISM_GRAPHIFY_SMOKE:-}" != "1" ]]; then
  echo "Refusing live Graphify smoke test. Set PRISM_GRAPHIFY_SMOKE=1 after provisioning the pinned dependency, index, and offload model." >&2
  exit 2
fi

: "${PRISM_GRAPHIFY_WORKSPACE:?set an absolute workspace path}"
: "${PRISM_GRAPHIFY_INDEX:?set the absolute path to a Graphify graph.json}"
: "${PRISM_GRAPHIFY_FINGERPRINT:?set the current deterministic workspace fingerprint}"
: "${PRISM_GRAPHIFY_STATE_DIR:?set an isolated Prism state directory}"
: "${PRISM_MODEL_RUNTIME_ENGINE:?set an explicit local or self-hosted offload runtime engine}"
: "${PRISM_MODEL_RUNTIME_BASE_URL:?set the explicit offload runtime base URL}"
: "${PRISM_MODEL_RUNTIME_MODEL:?set the explicit offload model}"

case "$PRISM_GRAPHIFY_WORKSPACE" in
  /*) ;;
  *) echo "PRISM_GRAPHIFY_WORKSPACE must be absolute" >&2; exit 2 ;;
esac
case "$PRISM_GRAPHIFY_INDEX" in
  /*) ;;
  *) echo "PRISM_GRAPHIFY_INDEX must be absolute" >&2; exit 2 ;;
esac
case "$PRISM_GRAPHIFY_STATE_DIR" in
  /*) ;;
  *) echo "PRISM_GRAPHIFY_STATE_DIR must be absolute" >&2; exit 2 ;;
esac

command -v graphify-mcp >/dev/null
test -f "$PRISM_GRAPHIFY_INDEX"

prism --state-dir "$PRISM_GRAPHIFY_STATE_DIR" mcp add graphify-smoke -- \
  graphify-mcp --graph "$PRISM_GRAPHIFY_INDEX"
prism --state-dir "$PRISM_GRAPHIFY_STATE_DIR" graphify setup --approve \
  --workspace "$PRISM_GRAPHIFY_WORKSPACE" \
  --index "$PRISM_GRAPHIFY_INDEX" \
  --fingerprint "$PRISM_GRAPHIFY_FINGERPRINT" \
  --server graphify-smoke \
  --endpoint-kind local \
  --executable graphify-mcp
prism --state-dir "$PRISM_GRAPHIFY_STATE_DIR" graphify doctor \
  --workspace "$PRISM_GRAPHIFY_WORKSPACE" \
  --fingerprint "$PRISM_GRAPHIFY_FINGERPRINT"
printf '%s\n' "Investigate the repository architecture and cite verified sources." |
  prism --state-dir "$PRISM_GRAPHIFY_STATE_DIR" \
    --root "$PRISM_GRAPHIFY_WORKSPACE" \
    run repo-investigator \
    --skills graphify-query \
    --graphify-fingerprint "$PRISM_GRAPHIFY_FINGERPRINT" \
    --stdin
