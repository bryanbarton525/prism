#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PRISM=(go run ./cmd/prism)
EVENT_STORE="$(mktemp -t prism-events.XXXXXX.db)"

echo "== config doctor =="
"${PRISM[@]}" --root "$ROOT" config doctor || true

echo "== route suggest =="
"${PRISM[@]}" --root "$ROOT" --policy-file testdata/policies/k8s-readonly.yaml route suggest \
  --task "Investigate deployment checkout-api rollout in namespace staging"

echo "== policy validate/explain/test =="
"${PRISM[@]}" --root "$ROOT" policy validate testdata/policies/k8s-readonly.yaml
"${PRISM[@]}" --root "$ROOT" policy explain testdata/policies/k8s-readonly.yaml kubectl \
  --skills k8s-rollout-diagnostics \
  --plugins kubernetes \
  --source cli
"${PRISM[@]}" --root "$ROOT" policy test testdata/policies/k8s-readonly.yaml testdata/policies/k8s-readonly-cases.yaml

echo "== graph validate =="
"${PRISM[@]}" --root "$ROOT" graph validate testdata/graphs/k8s-rollout-investigation.yaml

echo "== bundle verify =="
"${PRISM[@]}" --root "$ROOT" bundle verify testdata/bundles/k8s-core-triage/registry.json \
  --source-root "$ROOT" \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt

echo "== events summarize =="
"${PRISM[@]}" --root "$ROOT" --event-store "$EVENT_STORE" events summarize
