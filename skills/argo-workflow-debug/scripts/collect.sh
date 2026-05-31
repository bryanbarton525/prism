#!/usr/bin/env bash
set -euo pipefail

detect_os_arch() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${arch}" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "unsupported architecture: ${arch}" >&2; exit 2 ;;
  esac
  echo "${os}" "${arch}"
}

ensure_argo() {
  local version="${1}" bin_root os arch cache_dir bin gz
  bin_root="${PRISM_BIN_DIR:-${HOME}/.cache/prism/bin}"
  version="${version:-v3.5.8}"
  read -r os arch < <(detect_os_arch)
  cache_dir="${bin_root}/argo/${version}/${os}-${arch}"
  bin="${cache_dir}/argo"
  if [[ -x "${bin}" ]]; then
    echo "${bin}"
    return
  fi
  mkdir -p "${cache_dir}"
  gz="${cache_dir}/argo.gz"
  echo "Downloading argo ${version} (${os}/${arch}) to ${bin}" >&2
  curl -fsSL "https://github.com/argoproj/argo-workflows/releases/download/${version}/argo-${os}-${arch}.gz" -o "${gz}"
  gunzip -c "${gz}" > "${bin}"
  chmod +x "${bin}"
  echo "${bin}"
}

NS="${1:-default}"
WF="${2:-}"
if [[ -z "${WF}" ]]; then
  echo "Usage: $0 <namespace> <workflow-name>" >&2
  exit 1
fi

ARGO_BIN="${ARGO_BIN:-}"
ARGO_VERSION="${ARGO_VERSION:-}"
if [[ -n "${ARGO_BIN}" ]] && command -v "${ARGO_BIN}" >/dev/null 2>&1; then
  ARGO_CMD="${ARGO_BIN}"
else
  ARGO_CMD="$(ensure_argo "${ARGO_VERSION}")"
fi

echo "# Workflow summary"
"${ARGO_CMD}" get "${WF}" -n "${NS}" || true
echo

echo "# Workflow json"
"${ARGO_CMD}" get "${WF}" -n "${NS}" -o json || true
echo

echo "# Failed-node logs"
"${ARGO_CMD}" logs "${WF}" -n "${NS}" --node-field-selector phase=Failed || true
