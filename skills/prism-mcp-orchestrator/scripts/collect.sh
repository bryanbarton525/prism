#!/usr/bin/env bash
set -euo pipefail

TASK_HINT="${1:-Summarize the failing PR checks and likely blockers.}"

echo "# Prism MCP orchestration collector"
echo
echo "Task hint: ${TASK_HINT}"
echo
echo "## Recommended sequence"
echo "1) list_agents"
echo "2) list_resources + get_resource(run_agent + orchestration-guide)"
echo "3) optional list_prompts/get_prompt"
echo "4) run_agent with bounded task + valid skill_names"
echo "5) synthesize in parent"
echo

if command -v prism >/dev/null 2>&1; then
  echo "## Local prism config doctor"
  prism config doctor || true
else
  echo "prism binary not found on PATH; skip local doctor."
fi
