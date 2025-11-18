#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${REPO_ROOT}/.bin"
LINT_VERSION="${GOLANGCI_LINT_VERSION:-v1.64.8}"
LINT_BIN="${BIN_DIR}/golangci-lint"

ensure_go() {
  if ! command -v go >/dev/null 2>&1; then
    echo "[run-golangci-lint] Go is required to build golangci-lint" >&2
    exit 1
  fi
}

install_linter() {
  ensure_go
  mkdir -p "${BIN_DIR}"
  echo "[run-golangci-lint] Installing golangci-lint ${LINT_VERSION}..." >&2
  GO111MODULE=on GOBIN="${BIN_DIR}" go install "github.com/golangci/golangci-lint/cmd/golangci-lint@${LINT_VERSION}"
}

current_version=""
if [[ -x "${LINT_BIN}" ]]; then
  # Output format: golangci-lint has version vX.Y.Z ...
  if current_version=$("${LINT_BIN}" version 2>/dev/null | awk '{print $4}'); then
    :
  else
    current_version=""
  fi
fi

if [[ "${current_version}" != "${LINT_VERSION}" ]]; then
  install_linter
fi

exec "${LINT_BIN}" "$@"
