#!/bin/bash

# Unified test runner for the Alex project
# Provides shortcuts for unit tests, linting, and CLI smoke tests

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

CLI_BINARY="alex"
CLI_PACKAGE="./cmd/alex"

export GOMODCACHE="${PROJECT_ROOT}/.cache/go/pkg/mod"
export GOCACHE="${PROJECT_ROOT}/.cache/go/build"
mkdir -p "${GOMODCACHE}" "${GOCACHE}"

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

ensure_go() {
    if ! command -v go >/dev/null 2>&1; then
        print_error "Go toolchain not installed"
        exit 1
    fi
}

build_cli() {
    ensure_go
    print_status "Building alex CLI..."
    go build -o "${CLI_BINARY}" "${CLI_PACKAGE}"
    print_success "Built ./alex"
}

run_unit_tests() {
    ensure_go
    print_status "Running unit tests..."
    go test ./cmd/... ./internal/... ./tests/... -count=1
    print_success "Unit tests passed"
}

run_integration_tests() {
    ensure_go
    print_status "Running integration tests..."
    go test ./evaluation/... -count=1
    print_success "Integration tests passed"
}

run_lint() {
    if command -v golangci-lint >/dev/null 2>&1; then
        print_status "Running golangci-lint..."
        golangci-lint run ./...
        print_success "Lint checks passed"
    else
        print_warning "golangci-lint not installed; skipping lint step"
    fi
}

run_cli_smoke() {
    build_cli
    print_status "Running alex CLI smoke checks..."
    ./alex --help >/dev/null
    ./alex version >/dev/null
    if ./alex sessions >/dev/null 2>&1; then
        print_status "Sessions command executed"
    else
        print_warning "Sessions command failed (likely no persistent storage configured)"
    fi
    print_success "CLI smoke test completed"
}

usage() {
    cat <<EOF
Alex Test Script

Usage: $0 [target]

Targets:
  unit           Run Go unit tests (cmd/internal/tests)
  integration    Run evaluation test suites
  lint           Run golangci-lint (if available)
  cli-smoke      Build alex CLI and run non-network smoke tests
  all            Run lint, unit tests, integration tests, and CLI smoke tests
  help           Show this message

Examples:
  $0 unit
  $0 cli-smoke
  $0 all
EOF
}

target="${1:-all}"

case "$target" in
    unit)
        run_unit_tests
        ;;
    integration)
        run_integration_tests
        ;;
    lint)
        run_lint
        ;;
    cli-smoke)
        run_cli_smoke
        ;;
    all)
        run_lint
        run_unit_tests
        run_integration_tests
        run_cli_smoke
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        print_error "Unknown target: $target"
        usage
        exit 1
        ;;
esac
