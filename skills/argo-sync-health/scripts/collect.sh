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

ensure_argocd() {
  local version="${1}" bin_root os arch cache_dir bin
  bin_root="${PRISM_BIN_DIR:-${HOME}/.cache/prism/bin}"
  version="${version:-v2.11.7}"
  read -r os arch < <(detect_os_arch)
  cache_dir="${bin_root}/argocd/${version}/${os}-${arch}"
  bin="${cache_dir}/argocd"
  if [[ -x "${bin}" ]]; then
    echo "${bin}"
    return
  fi
  mkdir -p "${cache_dir}"
  echo "Downloading argocd ${version} (${os}/${arch}) to ${bin}" >&2
  curl -fsSL "https://github.com/argoproj/argo-cd/releases/download/${version}/argocd-${os}-${arch}" -o "${bin}"
  chmod +x "${bin}"
  echo "${bin}"
}

APP="${1:-}"
if [[ -z "${APP}" ]]; then
  echo "Usage: $0 <argocd-app-name>" >&2
  exit 1
fi

ARGOCD_BIN="${ARGOCD_BIN:-}"
ARGOCD_VERSION="${ARGOCD_VERSION:-}"
if [[ -n "${ARGOCD_BIN}" ]] && command -v "${ARGOCD_BIN}" >/dev/null 2>&1; then
  ARGOCD_CMD="${ARGOCD_BIN}"
else
  ARGOCD_CMD="$(ensure_argocd "${ARGOCD_VERSION}")"
fi

echo "# App summary"
"${ARGOCD_CMD}" app get "${APP}"
echo

echo "# App resources"
"${ARGOCD_CMD}" app resources "${APP}" || true
echo

echo "# App history"
"${ARGOCD_CMD}" app history "${APP}" || true
echo

echo "# App diff"
"${ARGOCD_CMD}" app diff "${APP}" || true
