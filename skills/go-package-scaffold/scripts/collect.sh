#!/usr/bin/env bash
set -euo pipefail

TARGET="${1:-.}"
echo "# go-package-scaffold collector"
echo "target=${TARGET}"
echo

if ! command -v go >/dev/null 2>&1; then
  echo "go CLI not found" >&2
  exit 2
fi

echo "# module metadata"
go env GOMOD GOVERSION
echo

echo "# directory snapshot"
if [[ -d "${TARGET}" ]]; then
  ls "${TARGET}" | sed -n '1,120p'
fi
echo

echo "# suggested checks"
echo "go list ./..."
echo "go test ./..."
