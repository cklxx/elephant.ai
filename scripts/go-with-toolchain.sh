#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO_MOD="${REPO_ROOT}/go.mod"
TOOLCHAIN_CACHE="${ALEX_GO_TOOLCHAIN_DIR:-${REPO_ROOT}/.toolchains}"

GO_BIN="${GO_BIN:-$(command -v go || true)}"
if [[ -z "$GO_BIN" ]]; then
  echo "[go-with-toolchain] go executable not found in PATH" >&2
  exit 1
fi

parse_version() {
  local version="${1#go}"
  local major=0
  local minor=0
  if [[ "$version" =~ ^([0-9]+)\.([0-9]+) ]]; then
    major="${BASH_REMATCH[1]}"
    minor="${BASH_REMATCH[2]}"
  fi
  printf "%s %s\n" "$major" "$minor"
}

detect_toolchain_requirement() {
  local requirement=""
  if [[ -f "$GO_MOD" ]]; then
    requirement="$(grep -E '^toolchain ' "$GO_MOD" | awk '{print $2}' | head -n 1 || true)"
  fi
  echo "$requirement"
}

ensure_toolchain_installed() {
  local toolchain="$1"
  local dest="${TOOLCHAIN_CACHE}/${toolchain}"
  local go_bin="${dest}/bin/go"

  if [[ -x "$go_bin" ]]; then
    echo "[go-with-toolchain] Using cached ${toolchain} at ${dest}" >&2
    GO_BIN="$go_bin"
    return
  fi

  local uname_s
  uname_s="$(uname -s)"
  local uname_m
  uname_m="$(uname -m)"
  local platform=""
  case "$uname_s" in
    Linux) platform="linux" ;;
    Darwin) platform="darwin" ;;
    *)
      echo "[go-with-toolchain] Unsupported OS: ${uname_s}" >&2
      exit 1
      ;;
  esac

  local arch=""
  case "$uname_m" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      echo "[go-with-toolchain] Unsupported architecture: ${uname_m}" >&2
      exit 1
      ;;
  esac

  local tarball="${toolchain}.${platform}-${arch}.tar.gz"
  local urls=(
    "https://go.dev/dl/${tarball}"
    "https://dl.google.com/go/${tarball}"
  )
  local tmpdir
  tmpdir="$(mktemp -d)"

  local downloaded="false"
  for url in "${urls[@]}"; do
    echo "[go-with-toolchain] Downloading ${toolchain} for ${platform}/${arch} from ${url}..." >&2
    if env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY \
      curl -fsSL --retry 3 --retry-all-errors -o "${tmpdir}/go.tar.gz" "$url"; then
      downloaded="true"
      break
    fi
    echo "[go-with-toolchain] Failed to download ${url}, trying next mirror..." >&2
  done
  if [[ "$downloaded" != "true" ]]; then
    echo "[go-with-toolchain] Unable to download ${toolchain} (all mirrors failed)" >&2
    rm -rf "$tmpdir"
    exit 1
  fi

  mkdir -p "$TOOLCHAIN_CACHE"
  tar -C "$tmpdir" -xzf "${tmpdir}/go.tar.gz"
  rm -rf "$dest"
  mv "${tmpdir}/go" "$dest"
  rm -rf "$tmpdir"

  echo "[go-with-toolchain] Installed ${toolchain} to ${dest}" >&2
  GO_BIN="$go_bin"
}

required_toolchain="$(detect_toolchain_requirement)"
local_version="$($GO_BIN env GOVERSION 2>/dev/null || $GO_BIN version | awk '{print $3}')"
read -r local_major local_minor < <(parse_version "$local_version")

needs_bundled=false
if [[ -n "$required_toolchain" ]]; then
  if (( local_major < 1 || (local_major == 1 && local_minor < 21) )); then
    needs_bundled=true
  fi
fi

if [[ "${ALEX_FORCE_BUNDLED_GO:-}" == "1" ]]; then
  needs_bundled=true
fi

if [[ "$needs_bundled" == "true" ]]; then
  if [[ -z "$required_toolchain" ]]; then
    required_toolchain="go1.24.9"
  fi
  ensure_toolchain_installed "$required_toolchain"
  local_version="$($GO_BIN version | awk '{print $3}')"
  read -r local_major local_minor < <(parse_version "$local_version")
fi

if (( local_major < 1 || (local_major == 1 && local_minor < 21) )); then
  echo "[go-with-toolchain] Go 1.21 or newer is recommended (found ${local_version}). Attempting to continue..." >&2
fi

# Ensure golang.org/toolchain checksum verification can succeed
original_gosumdb="${GOSUMDB:-}"
if [[ "${GOSUMDB:-}" == "off" ]]; then
  echo "[go-with-toolchain] GOSUMDB=off detected - temporarily enabling sum.golang.org for toolchain verification" >&2
  export GOSUMDB="sum.golang.org"
fi

# Always allow golang.org/toolchain to bypass private module rules
if [[ -n "${GONOSUMDB:-}" ]]; then
  case "${GONOSUMDB}" in
    *golang.org/toolchain*) ;; # already present
    *) export GONOSUMDB="${GONOSUMDB},golang.org/toolchain" ;;
  esac
else
  export GONOSUMDB="golang.org/toolchain"
fi

"$GO_BIN" "$@"
status=$?

if [[ "$original_gosumdb" == "off" ]]; then
  export GOSUMDB="off"
fi

exit $status
