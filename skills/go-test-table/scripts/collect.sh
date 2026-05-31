#!/usr/bin/env bash
set -euo pipefail

PKG="${1:-./...}"
echo "# go-test-table collector"
echo "pkg=${PKG}"
echo

if ! command -v go >/dev/null 2>&1; then
  echo "go CLI not found" >&2
  exit 2
fi

echo "# package listing"
go list "${PKG}" 2>/dev/null | sed -n '1,40p' || true
echo

echo "# suggested validation"
echo "go test ${PKG} -run <TestName>"
echo "go test ${PKG}"
