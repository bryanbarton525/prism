#!/usr/bin/env bash
set -euo pipefail

# pre-commit pre-push hook stdin format:
# <local ref> <local sha> <remote ref> <remote sha>

if [[ "${PRISM_ALLOW_MAIN_PUSH:-}" == "1" ]]; then
  exit 0
fi

while read -r local_ref local_sha remote_ref remote_sha; do
  if [[ "${remote_ref}" == "refs/heads/main" ]]; then
    cat <<'EOF' >&2
Direct pushes to 'main' are blocked by pre-push policy.

Use a feature branch and open a PR instead.
If you really need to bypass once:
  PRISM_ALLOW_MAIN_PUSH=1 git push origin <branch-or-ref>
EOF
    exit 1
  fi
done

exit 0
