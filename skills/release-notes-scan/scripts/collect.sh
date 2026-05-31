#!/usr/bin/env bash
set -euo pipefail

PRODUCT="${1:-}"
FROM_VER="${2:-}"
TO_VER="${3:-}"
if [[ -z "${PRODUCT}" || -z "${FROM_VER}" || -z "${TO_VER}" ]]; then
  echo "Usage: $0 <product> <from-version> <to-version> [release-note-url...]" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl not found" >&2
  exit 2
fi

echo "# Release notes scan request"
echo "product=${PRODUCT} from=${FROM_VER} to=${TO_VER}"
echo
echo "# Notes"
echo "Prefer official release-note/changelog URLs."
echo "If URLs are provided, this script fetches and truncates each page body."
echo

for url in "${@:4}"; do
  echo "## URL: ${url}"
  curl -L --max-time 20 "${url}" | sed -e 's/[[:space:]]\+/ /g' | cut -c1-5000
  echo
done
