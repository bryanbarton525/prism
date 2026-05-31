#!/usr/bin/env bash
set -euo pipefail

QUERY="${1:-}"
if [[ -z "${QUERY}" ]]; then
  echo "Usage: $0 <query-string>" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl not found" >&2
  exit 2
fi

echo "# Query"
echo "${QUERY}"
echo
echo "# Notes"
echo "Use the web-docs-search agent tools (web-search/web-fetch) for source retrieval."
echo "This script records operator-provided hints and can fetch known URLs."
echo

for url in "${@:2}"; do
  echo "## URL: ${url}"
  curl -L --max-time 20 "${url}" | sed -e 's/[[:space:]]\+/ /g' | cut -c1-4000
  echo
done
