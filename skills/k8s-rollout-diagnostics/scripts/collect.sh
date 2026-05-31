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
DEPLOY="${3:-}"

if [[ -z "${CLUSTER_VERSION}" || -z "${DEPLOY}" ]]; then
  echo "Usage: $0 <cluster-version> <namespace> <deployment>" >&2
  echo "Example: $0 v1.28.8 payments payments-api" >&2
  exit 1
fi

KUBECTL_VERSION="${KUBECTL_VERSION:-${CLUSTER_VERSION}}"
KUBECTL_BIN="${KUBECTL_BIN:-}"
if [[ -n "${KUBECTL_BIN}" ]] && command -v "${KUBECTL_BIN}" >/dev/null 2>&1; then
  KUBECTL_CMD="${KUBECTL_BIN}"
else
  KUBECTL_CMD="$(ensure_kubectl "${KUBECTL_VERSION}")"
fi

echo "# Rollout status"
echo "requested_cluster_version=${CLUSTER_VERSION}"
echo "kubectl_bin=${KUBECTL_CMD}"
"${KUBECTL_CMD}" version --client || true
"${KUBECTL_CMD}" version || true
"${KUBECTL_CMD}" rollout status "deploy/${DEPLOY}" -n "${NS}" || true
echo

echo "# Deployment describe"
"${KUBECTL_CMD}" describe deploy "${DEPLOY}" -n "${NS}"
echo

echo "# Rollout history"
"${KUBECTL_CMD}" rollout history "deploy/${DEPLOY}" -n "${NS}" || true
echo

echo "# ReplicaSets"
"${KUBECTL_CMD}" get rs -n "${NS}" | grep "${DEPLOY}" || true
echo

echo "# Recent events"
"${KUBECTL_CMD}" get events -n "${NS}" --sort-by=.lastTimestamp | tail -n 120
