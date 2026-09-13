#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PRISM=(go run ./cmd/prism)
EVENT_STORE="$(mktemp -t prism-events.XXXXXX.db)"

echo "== config doctor =="
"${PRISM[@]}" config doctor || true

echo "== route suggest =="
"${PRISM[@]}" --policy-file testdata/policies/k8s-readonly.yaml route suggest \
  --task "Investigate deployment checkout-api rollout in namespace staging"

echo "== policy validate/explain/test =="
"${PRISM[@]}" policy validate testdata/policies/k8s-readonly.yaml
"${PRISM[@]}" policy explain testdata/policies/k8s-readonly.yaml kubectl \
  --skills k8s-rollout-diagnostics \
  --plugins kubernetes \
  --source cli
"${PRISM[@]}" policy test testdata/policies/k8s-readonly.yaml testdata/policies/k8s-readonly-cases.yaml

echo "== graph validate =="
"${PRISM[@]}" graph validate testdata/graphs/k8s-rollout-investigation.yaml

echo "== embedded release identity =="
"${PRISM[@]}" version --json

echo "== events summarize =="
"${PRISM[@]}" --event-store "$EVENT_STORE" events summarize
