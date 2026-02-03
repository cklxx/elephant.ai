#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=../scripts/lib/common/logging.sh
source "${REPO_ROOT}/scripts/lib/common/logging.sh"
# shellcheck source=../scripts/lib/common/cgo.sh
source "${REPO_ROOT}/scripts/lib/common/cgo.sh"

install_macos() {
  if ! command_exists xcode-select || ! xcode-select -p >/dev/null 2>&1; then
    log_warn "Xcode Command Line Tools not found. Launching installer..."
    xcode-select --install || true
    log_warn "Re-run this script after Command Line Tools finish installing."
    return 1
  fi

  if ! command_exists brew; then
    die "Homebrew is required. Install from https://brew.sh and re-run."
  fi

  log_info "Installing sqlite and pkg-config via Homebrew..."
  brew install sqlite pkg-config

  local sqlite_prefix
  sqlite_prefix="$(brew --prefix sqlite 2>/dev/null || true)"
  if [[ -n "$sqlite_prefix" ]]; then
    log_info "If sqlite3 headers are not found during CGO builds, export:"
    echo "  export PKG_CONFIG_PATH=\"${sqlite_prefix}/lib/pkgconfig\""
    echo "  export PATH=\"${sqlite_prefix}/bin:\$PATH\""
  fi
}

install_linux() {
  if command_exists apt-get; then
    log_info "Installing build-essential, pkg-config, libsqlite3-dev via apt..."
    sudo apt-get update
    sudo apt-get install -y build-essential pkg-config libsqlite3-dev
    return 0
  fi
  if command_exists dnf; then
    log_info "Installing gcc, make, pkgconfig, sqlite-devel via dnf..."
    sudo dnf install -y gcc gcc-c++ make pkgconfig sqlite-devel
    return 0
  fi
  if command_exists yum; then
    log_info "Installing gcc, make, pkgconfig, sqlite-devel via yum..."
    sudo yum install -y gcc gcc-c++ make pkgconfig sqlite-devel
    return 0
  fi
  die "Unsupported Linux package manager. Install build tools + sqlite3 dev headers manually."
}

main() {
  local uname_s
  uname_s="$(uname -s)"
  case "$uname_s" in
    Darwin)
      install_macos || return 1
      ;;
    Linux)
      install_linux
      ;;
    *)
      die "Unsupported OS: ${uname_s}"
      ;;
  esac

  if cgo_sqlite_ready; then
    log_success "CGO sqlite dependencies detected."
  else
    log_warn "CGO sqlite dependencies still missing. Review logs above."
    return 1
  fi

  log_info "CGO auto mode: set ALEX_CGO_MODE=auto|on|off (default auto)."
}

main "$@"
