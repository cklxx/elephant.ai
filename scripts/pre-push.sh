#!/usr/bin/env bash
# pre-push.sh — Local CI gate before pushing to remote.
# Mirrors the critical checks from .github/workflows/ci.yml so they fail
# fast locally instead of waiting for CI.
#
# Skip with: SKIP_PRE_PUSH=1 git push
# Skip specific checks: SKIP_MOD_TIDY=1, SKIP_LINT=1, SKIP_WEB=1, SKIP_BUILD=1
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO="${REPO_ROOT}/scripts/go-with-toolchain.sh"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
RESET='\033[0m'

pass() { printf "${GREEN}✓${RESET} %s\n" "$1"; }
fail() { printf "${RED}✗${RESET} %s\n" "$1"; }
warn() { printf "${YELLOW}⚠${RESET} %s\n" "$1"; }
step() { printf "${BOLD}▸${RESET} %s ... " "$1"; }

errors=0

# Determine which files changed vs remote
changed_files() {
  local remote="$1" url="$2"
  local remote_sha
  remote_sha=$(git ls-remote "$url" "refs/heads/$(git rev-parse --abbrev-ref HEAD)" 2>/dev/null | cut -f1)
  if [[ -z "$remote_sha" ]]; then
    # New branch — diff against main
    git diff --name-only "$(git merge-base HEAD origin/main)" HEAD 2>/dev/null || git diff --name-only HEAD~10 HEAD 2>/dev/null || echo ""
  else
    git diff --name-only "$remote_sha" HEAD 2>/dev/null || echo ""
  fi
}

has_go_changes() {
  local files="$1"
  echo "$files" | grep -qE '\.(go|mod|sum)$'
}

has_web_changes() {
  local files="$1"
  echo "$files" | grep -qE '^web/'
}

# --- Check: go mod tidy ---
check_mod_tidy() {
  if [[ "${SKIP_MOD_TIDY:-}" == "1" ]]; then
    warn "Skipping go mod tidy check (SKIP_MOD_TIDY=1)"
    return
  fi
  step "go mod tidy"
  (cd "$REPO_ROOT" && "$GO" mod tidy 2>/dev/null)
  if ! (cd "$REPO_ROOT" && git diff --quiet go.mod go.sum); then
    echo ""
    fail "go.mod or go.sum changed after 'go mod tidy'"
    echo "  Run: go mod tidy && git add go.mod go.sum && git commit --amend"
    (cd "$REPO_ROOT" && git diff --stat go.mod go.sum)
    # Restore to avoid leaving dirty state
    (cd "$REPO_ROOT" && git checkout -- go.mod go.sum)
    errors=$((errors + 1))
  else
    pass "go mod tidy"
  fi
}

# --- Check: go vet ---
check_go_vet() {
  step "go vet"
  if (cd "$REPO_ROOT" && "$GO" vet ./cmd/... ./internal/... 2>&1); then
    pass "go vet"
  else
    echo ""
    fail "go vet found issues"
    errors=$((errors + 1))
  fi
}

# --- Check: go build ---
check_go_build() {
  if [[ "${SKIP_BUILD:-}" == "1" ]]; then
    warn "Skipping build check (SKIP_BUILD=1)"
    return
  fi
  step "go build"
  local build_ok=true
  for cmd in cmd/alex cmd/alex-server cmd/alex-web; do
    if [[ -d "$REPO_ROOT/$cmd" ]]; then
      if ! (cd "$REPO_ROOT" && "$GO" build -o /dev/null "./$cmd" 2>&1); then
        build_ok=false
      fi
    fi
  done
  if $build_ok; then
    pass "go build (all binaries)"
  else
    echo ""
    fail "go build failed"
    errors=$((errors + 1))
  fi
}

# --- Check: golangci-lint ---
check_lint() {
  if [[ "${SKIP_LINT:-}" == "1" ]]; then
    warn "Skipping lint (SKIP_LINT=1)"
    return
  fi
  if [[ -x "$REPO_ROOT/scripts/run-golangci-lint.sh" ]]; then
    step "golangci-lint"
    if (cd "$REPO_ROOT" && ./scripts/run-golangci-lint.sh run --timeout=3m ./... 2>&1); then
      pass "golangci-lint"
    else
      echo ""
      fail "golangci-lint found issues"
      errors=$((errors + 1))
    fi
  else
    warn "golangci-lint script not found, skipping"
  fi
}

# --- Check: architecture boundaries ---
check_arch() {
  step "architecture boundaries"
  if (cd "$REPO_ROOT" && make check-arch 2>&1 >/dev/null); then
    pass "architecture boundaries"
  else
    echo ""
    fail "architecture boundary violations"
    errors=$((errors + 1))
  fi
}

# --- Check: web lint + build ---
check_web() {
  if [[ "${SKIP_WEB:-}" == "1" ]]; then
    warn "Skipping web checks (SKIP_WEB=1)"
    return
  fi
  if [[ ! -d "$REPO_ROOT/web" ]]; then
    return
  fi
  step "web lint"
  if (cd "$REPO_ROOT" && npm --prefix web run lint 2>&1); then
    pass "web lint"
  else
    echo ""
    fail "web lint failed"
    errors=$((errors + 1))
  fi

  step "web build"
  if (cd "$REPO_ROOT" && npm --prefix web run build 2>&1 >/dev/null); then
    pass "web build"
  else
    echo ""
    fail "web build failed"
    errors=$((errors + 1))
  fi
}

# =============================================================================
main() {
  echo ""
  printf "${BOLD}🔍 Pre-push CI checks${RESET}\n"
  echo "─────────────────────────────────────"

  # Collect changed files (best-effort, fall back to running all checks)
  local files=""
  files=$(changed_files "${1:-origin}" "${2:-}" 2>/dev/null || echo "")
  local go_changed=true
  local web_changed=true

  if [[ -n "$files" ]]; then
    has_go_changes "$files" && go_changed=true || go_changed=false
    has_web_changes "$files" && web_changed=true || web_changed=false
  fi

  # Always run mod tidy — it's fast and catches the most common CI failure
  check_mod_tidy

  if $go_changed; then
    check_go_vet
    check_go_build
    check_lint
    check_arch
  else
    warn "No Go changes detected — skipping Go checks"
  fi

  if $web_changed; then
    check_web
  else
    warn "No web/ changes detected — skipping web checks"
  fi

  echo "─────────────────────────────────────"
  if [[ $errors -gt 0 ]]; then
    fail "${errors} check(s) failed — push aborted"
    echo ""
    echo "  To skip: SKIP_PRE_PUSH=1 git push"
    echo "  To skip one: SKIP_MOD_TIDY=1 / SKIP_LINT=1 / SKIP_WEB=1 / SKIP_BUILD=1"
    exit 1
  else
    pass "All checks passed"
    echo ""
  fi
}

main "$@"
