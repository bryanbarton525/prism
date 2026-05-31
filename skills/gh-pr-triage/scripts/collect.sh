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

ensure_gh() {
  local version="${1}" version_tag version_file bin_root os arch cache_dir tgz bin
  bin_root="${PRISM_BIN_DIR:-${HOME}/.cache/prism/bin}"
  version="${version:-2.54.0}"
  version_tag="v${version#v}"
  version_file="${version#v}"
  read -r os arch < <(detect_os_arch)
  cache_dir="${bin_root}/gh/${version_tag}/${os}-${arch}"
  bin="${cache_dir}/gh"
  if [[ -x "${bin}" ]]; then
    echo "${bin}"
    return
  fi
  mkdir -p "${cache_dir}"
  tgz="${cache_dir}/gh.tar.gz"
  echo "Downloading gh ${version_tag} (${os}/${arch}) to ${bin}" >&2
  curl -fsSL "https://github.com/cli/cli/releases/download/${version_tag}/gh_${version_file}_${os}_${arch}.tar.gz" -o "${tgz}"
  tar -xzf "${tgz}" -C "${cache_dir}"
  cp "${cache_dir}/gh_${version_file}_${os}_${arch}/bin/gh" "${bin}"
  chmod +x "${bin}"
  echo "${bin}"
}

PR_REF="${1:-}"
if [[ -z "${PR_REF}" ]]; then
  echo "Usage: $0 <pr-number|url|branch>" >&2
  exit 1
fi

GH_BIN="${GH_BIN:-}"
GH_VERSION="${GH_VERSION:-}"
if [[ -n "${GH_BIN}" ]] && command -v "${GH_BIN}" >/dev/null 2>&1; then
  GH_CMD="${GH_BIN}"
else
  GH_CMD="$(ensure_gh "${GH_VERSION}")"
fi

echo "# PR overview"
"${GH_CMD}" pr view "${PR_REF}" --json number,title,state,isDraft,mergeStateStatus,reviewDecision,headRefName,baseRefName,author,url
echo

echo "# PR checks"
"${GH_CMD}" pr checks "${PR_REF}" || true
echo

echo "# Review signals"
"${GH_CMD}" pr view "${PR_REF}" --json latestReviews,reviews
echo

echo "# Changed files"
"${GH_CMD}" pr diff "${PR_REF}" --name-only
