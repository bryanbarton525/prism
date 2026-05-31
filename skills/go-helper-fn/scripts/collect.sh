#!/usr/bin/env bash
set -euo pipefail

TARGET="${1:-.}"
echo "# go-helper-fn collector"
echo "target=${TARGET}"
echo

if ! command -v go >/dev/null 2>&1; then
  echo "go CLI not found" >&2
  exit 2
fi

echo "# go env"
go env GOMOD GOVERSION
echo

echo "# files"
if [[ -d "${TARGET}" ]]; then
  ls "${TARGET}" | sed -n '1,80p'
else
  echo "target is not a directory; continuing"
fi
echo

echo "# suggested validation"
echo "gofmt -w <file.go>"
echo "go test ./... -run <HelperName>"
