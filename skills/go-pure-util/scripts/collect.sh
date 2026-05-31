#!/usr/bin/env bash
set -euo pipefail

TARGET="${1:-.}"
echo "# go-pure-util collector"
echo "target=${TARGET}"
echo

if ! command -v go >/dev/null 2>&1; then
  echo "go CLI not found" >&2
  exit 2
fi

echo "# module + version"
go env GOMOD GOVERSION
echo

echo "# static checks (if available)"
if command -v gofmt >/dev/null 2>&1; then
  echo "gofmt available"
fi
echo

echo "# suggested verification"
echo "go test ./... -run <UtilityName>"
