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

ensure_kubectl() {
  local version="${1}" bin_root os arch cache_dir bin
  bin_root="${PRISM_BIN_DIR:-${HOME}/.cache/prism/bin}"
  if [[ -z "${version}" ]]; then
    echo "kubectl version is required (set KUBECTL_VERSION or pass cluster version)" >&2
    exit 2
  fi
  read -r os arch < <(detect_os_arch)
  cache_dir="${bin_root}/kubectl/${version}/${os}-${arch}"
  bin="${cache_dir}/kubectl"
  if [[ -x "${bin}" ]]; then
    echo "${bin}"
    return
  fi
  mkdir -p "${cache_dir}"
  echo "Downloading kubectl ${version} (${os}/${arch}) to ${bin}" >&2
  curl -fsSL "https://dl.k8s.io/release/${version}/bin/${os}/${arch}/kubectl" -o "${bin}"
  chmod +x "${bin}"
  echo "${bin}"
}

CLUSTER_VERSION="${1:-}"
NS="${2:-default}"
POD="${3:-}"

if [[ -z "${CLUSTER_VERSION}" ]]; then
  echo "Usage: $0 <cluster-version> [namespace] [pod]" >&2
  echo "Example: $0 v1.29.3 payments payments-api-abc123" >&2
  exit 1
fi

KUBECTL_VERSION="${KUBECTL_VERSION:-${CLUSTER_VERSION}}"
KUBECTL_BIN="${KUBECTL_BIN:-}"
if [[ -n "${KUBECTL_BIN}" ]] && command -v "${KUBECTL_BIN}" >/dev/null 2>&1; then
  KUBECTL_CMD="${KUBECTL_BIN}"
else
  KUBECTL_CMD="$(ensure_kubectl "${KUBECTL_VERSION}")"
fi

echo "# Context"
echo "requested_cluster_version=${CLUSTER_VERSION}"
echo "kubectl_bin=${KUBECTL_CMD}"
"${KUBECTL_CMD}" version --client || true
"${KUBECTL_CMD}" version || true
"${KUBECTL_CMD}" config current-context
echo

echo "# Pod snapshot (${NS})"
"${KUBECTL_CMD}" get pods -n "${NS}" -o wide
echo

echo "# Workload snapshot (${NS})"
"${KUBECTL_CMD}" get deploy,rs,svc -n "${NS}"
echo

echo "# Recent events (${NS})"
"${KUBECTL_CMD}" get events -n "${NS}" --sort-by=.lastTimestamp | tail -n 100
echo

if [[ -n "${POD}" ]]; then
  echo "# Pod describe (${POD})"
  "${KUBECTL_CMD}" describe pod "${POD}" -n "${NS}" || true
  echo
  echo "# Pod logs previous (${POD})"
  "${KUBECTL_CMD}" logs "${POD}" -n "${NS}" --previous --tail=200 || true
fi
