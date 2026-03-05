#!/usr/bin/env bash
# pre-push.sh — Local CI gate before pushing to remote.
# Mirrors the critical checks from .github/workflows/ci.yml so they fail
# fast locally instead of waiting for CI.
#
# Skip with: SKIP_PRE_PUSH=1 git push
# Skip specific checks: SKIP_MOD_TIDY=1, SKIP_LINT=1, SKIP_WEB=1, SKIP_BUILD=1, SKIP_TEST=1
# Modes:
#   PRE_PUSH_MODE=fast (default) -> go test + changed-pkgs race
#   PRE_PUSH_MODE=full           -> full go test -race ./...
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

guard_origin_remote() {
  local origin_url
  origin_url="$(git remote get-url origin 2>/dev/null || true)"
  if [[ -z "$origin_url" ]]; then
    return 0
  fi

  case "$origin_url" in
    /*|file://*)
      echo "origin remote uses a local filesystem path: $origin_url"
      echo "Refusing push from a local-path clone; use the primary repository/worktree with hosted origin."
      echo "Override intentionally with: ALLOW_LOCAL_ORIGIN_PUSH=1 git push"
      return 1
      ;;
  esac
}

# Heavy checks can consume large memory when run together.
# Default: run them sequentially to avoid OOM kills on developer machines.
HEAVY_MODE="${PRE_PUSH_HEAVY_MODE:-sequential}" # sequential | parallel
PRE_PUSH_MODE="${PRE_PUSH_MODE:-fast}"          # fast | full
CHANGED_GO_PKGS=()

# Determine which files changed vs remote
changed_files() {
  local remote="$1" url="$2"
  local remote_sha
  remote_sha=$(git ls-remote "$url" "refs/heads/$(git rev-parse --abbrev-ref HEAD)" 2>/dev/null | cut -f1)
  if [[ -z "$remote_sha" ]]; then
    git diff --name-only "$(git merge-base HEAD origin/main)" HEAD 2>/dev/null || git diff --name-only HEAD~10 HEAD 2>/dev/null || echo ""
  else
    git diff --name-only "$remote_sha" HEAD 2>/dev/null || echo ""
  fi
}

has_go_changes() { echo "$1" | grep -qE '\.(go|mod|sum)$'; }
has_web_changes() { echo "$1" | grep -qE '^web/'; }

collect_changed_go_packages() {
  local files="$1"
  CHANGED_GO_PKGS=()
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    local dir pkg
    dir="$(dirname "$file")"
    if [[ "$dir" == "." ]]; then
      continue
    fi
    pkg="./$dir"
    if [[ -d "$REPO_ROOT/$dir" ]] && "$GO" list "$pkg" >/dev/null 2>&1; then
      CHANGED_GO_PKGS+=("$pkg")
    fi
  done < <(echo "$files" | grep -E '\.go$' || true)

  if ((${#CHANGED_GO_PKGS[@]} > 0)); then
    local deduped
    deduped="$(printf '%s\n' "${CHANGED_GO_PKGS[@]}" | sort -u)"
    CHANGED_GO_PKGS=()
    while IFS= read -r pkg; do
      [[ -n "$pkg" ]] && CHANGED_GO_PKGS+=("$pkg")
    done <<< "$deduped"
  fi
}

# lint_base_rev returns the merge-base commit to use for incremental lint.
# golangci-lint --new-from-rev only checks issues introduced after this commit,
# which drops memory from ~30 GB (full ./...) to <2 GB.
lint_base_rev() {
  local remote_sha
  remote_sha=$(git ls-remote origin "refs/heads/$(git rev-parse --abbrev-ref HEAD)" 2>/dev/null | cut -f1)
  if [[ -n "$remote_sha" ]]; then
    echo "$remote_sha"
    return
  fi
  git merge-base HEAD origin/main 2>/dev/null || true
}

# --- Parallel job runner ---
# Each check writes its result to a temp file: "PASS <name>" or "FAIL <name>\n<output>"
TMPDIR_JOBS="$(mktemp -d "${TMPDIR:-/tmp}/prepush.XXXXXX")"
trap 'rm -rf "$TMPDIR_JOBS"' EXIT
PIDS=()
JOB_NAMES=()

run_job() {
  local name="$1"; shift
  local outfile="$TMPDIR_JOBS/$name"
  (
    if "$@" >"$outfile.log" 2>&1; then
      echo "PASS" > "$outfile.status"
    else
      echo "FAIL" > "$outfile.status"
    fi
  ) &
  PIDS+=($!)
  JOB_NAMES+=("$name")
}

run_job_sync() {
  local name="$1"; shift
  local outfile="$TMPDIR_JOBS/$name.sync"
  if "$@" >"$outfile.log" 2>&1; then
    pass "$name"
    return 0
  fi
  fail "$name"
  if [[ -f "$outfile.log" ]] && [[ -s "$outfile.log" ]]; then
    sed 's/^/  /' "$outfile.log" | tail -30
  fi
  return 1
}

collect_results() {
  local errors=0
  for i in "${!PIDS[@]}"; do
    local pid="${PIDS[$i]}"
    local name="${JOB_NAMES[$i]}"
    local outfile="$TMPDIR_JOBS/$name"
    wait "$pid" 2>/dev/null || true
    local status="FAIL"
    [[ -f "$outfile.status" ]] && status="$(cat "$outfile.status")"
    if [[ "$status" == "PASS" ]]; then
      pass "$name"
    else
      fail "$name"
      if [[ -f "$outfile.log" ]] && [[ -s "$outfile.log" ]]; then
        sed 's/^/  /' "$outfile.log" | tail -30
      fi
      errors=$((errors + 1))
    fi
  done
  PIDS=()
  JOB_NAMES=()
  return $errors
}

# --- Individual checks (designed to run in subshells) ---

job_mod_tidy() {
  cd "$REPO_ROOT"
  "$GO" mod tidy 2>/dev/null
  git diff --quiet go.mod go.sum || {
    echo "go.mod or go.sum changed after 'go mod tidy'"
    echo "Run: go mod tidy && git add go.mod go.sum && git commit --amend"
    git diff --stat go.mod go.sum
    git checkout -- go.mod go.sum
    return 1
  }
}

job_go_vet() {
  cd "$REPO_ROOT" && "$GO" vet ./cmd/... ./internal/...
}

job_go_build() {
  cd "$REPO_ROOT"
  local ok=true
  for cmd in cmd/alex cmd/alex-server cmd/alex-web; do
    [[ -d "$REPO_ROOT/$cmd" ]] && "$GO" build -o /dev/null "./$cmd" || ok=false
  done
  $ok
}

job_go_test_full_race() {
  cd "$REPO_ROOT" && "$GO" test -race -count=1 ./...
}

job_go_test_quick() {
  cd "$REPO_ROOT" && "$GO" test -count=1 ./...
}

job_go_test_changed_race() {
  cd "$REPO_ROOT"
  if ((${#CHANGED_GO_PKGS[@]} == 0)); then
    return 0
  fi
  "$GO" test -race -count=1 "${CHANGED_GO_PKGS[@]}"
}

job_lint() {
  cd "$REPO_ROOT"
  local base_rev
  base_rev=$(lint_base_rev)
  if [[ -n "$base_rev" ]]; then
    ./scripts/run-golangci-lint.sh run --timeout=3m --new-from-rev="$base_rev" ./...
  else
    ./scripts/run-golangci-lint.sh run --timeout=3m ./...
  fi
}

job_arch() {
  cd "$REPO_ROOT" && make check-arch 2>&1
}

job_arch_policy() {
  cd "$REPO_ROOT" && make check-arch-policy 2>&1
}

job_web_lint() {
  cd "$REPO_ROOT" && npm --prefix web run lint
}

job_web_build() {
  cd "$REPO_ROOT" && npm --prefix web run build >/dev/null
}

# =============================================================================
main() {
  local start_time=$SECONDS
  echo ""
  printf "${BOLD}🔍 Pre-push CI checks (parallel)${RESET}\n"
  echo "─────────────────────────────────────"
  echo "mode: PRE_PUSH_MODE=${PRE_PUSH_MODE}, PRE_PUSH_HEAVY_MODE=${HEAVY_MODE}"

  if [[ "${ALLOW_LOCAL_ORIGIN_PUSH:-}" == "1" ]]; then
    warn "Skipping origin remote guard (ALLOW_LOCAL_ORIGIN_PUSH=1)"
  else
    if ! guard_origin_remote; then
      echo "─────────────────────────────────────"
      fail "origin remote guard failed — push aborted"
      exit 1
    fi
  fi

  # Collect changed files
  local files=""
  files=$(changed_files "${1:-origin}" "${2:-}" 2>/dev/null || echo "")
  local go_changed=true web_changed=true

  if [[ -n "$files" ]]; then
    has_go_changes "$files" && go_changed=true || go_changed=false
    has_web_changes "$files" && web_changed=true || web_changed=false
    collect_changed_go_packages "$files"
    if [[ "$PRE_PUSH_MODE" != "full" ]] && $go_changed; then
      echo "go changed packages for race: ${#CHANGED_GO_PKGS[@]}"
    fi
  fi

  # Phase 1: mod tidy must run first (mutates go.mod/go.sum)
  local phase1_errors=0
  if [[ "${SKIP_MOD_TIDY:-}" == "1" ]]; then
    warn "Skipping go mod tidy (SKIP_MOD_TIDY=1)"
  else
    run_job "go mod tidy" job_mod_tidy
    collect_results || phase1_errors=$?
  fi

  if [[ $phase1_errors -gt 0 ]]; then
    echo "─────────────────────────────────────"
    fail "mod tidy failed — push aborted"
    echo "  To skip: SKIP_PRE_PUSH=1 git push"
    exit 1
  fi

  # Phase 2: lightweight checks in parallel
  if $go_changed; then
    run_job "go vet" job_go_vet
    [[ "${SKIP_BUILD:-}" != "1" ]] && run_job "go build" job_go_build || warn "Skipping build (SKIP_BUILD=1)"
    run_job "arch boundaries" job_arch
    run_job "arch policy" job_arch_policy
  else
    warn "No Go changes — skipping Go checks"
  fi

  if $web_changed && [[ "${SKIP_WEB:-}" != "1" ]] && [[ -d "$REPO_ROOT/web" ]]; then
    run_job "web lint" job_web_lint
  elif [[ "${SKIP_WEB:-}" == "1" ]]; then
    warn "Skipping web (SKIP_WEB=1)"
  elif ! $web_changed; then
    warn "No web/ changes — skipping web checks"
  fi

  local total_errors=0
  collect_results || total_errors=$?

  if [[ $total_errors -gt 0 ]]; then
    echo "─────────────────────────────────────"
    fail "${total_errors} check(s) failed — push aborted (${SECONDS}s)"
    echo ""
    echo "  To skip: SKIP_PRE_PUSH=1 git push"
    echo "  To skip one: SKIP_MOD_TIDY=1 / SKIP_LINT=1 / SKIP_WEB=1 / SKIP_BUILD=1 / SKIP_TEST=1"
    exit 1
  fi

  # Phase 3: heavy checks (default sequential for stability)
  local heavy_errors=0
  if [[ "$HEAVY_MODE" == "parallel" ]]; then
    if $go_changed; then
      if [[ "${SKIP_TEST:-}" != "1" ]]; then
        if [[ "$PRE_PUSH_MODE" == "full" ]]; then
          run_job "go test -race" job_go_test_full_race
        else
          run_job "go test" job_go_test_quick
          if ((${#CHANGED_GO_PKGS[@]} > 0)); then
            run_job "go test -race (changed)" job_go_test_changed_race
          fi
        fi
      else
        warn "Skipping test (SKIP_TEST=1)"
      fi
      if [[ "${SKIP_LINT:-}" != "1" ]] && [[ -x "$REPO_ROOT/scripts/run-golangci-lint.sh" ]]; then
        run_job "golangci-lint" job_lint
      elif [[ "${SKIP_LINT:-}" == "1" ]]; then
        warn "Skipping lint (SKIP_LINT=1)"
      fi
    fi
    if $web_changed && [[ "${SKIP_WEB:-}" != "1" ]] && [[ -d "$REPO_ROOT/web" ]]; then
      run_job "web build" job_web_build
    fi
    collect_results || heavy_errors=$?
  else
    if $go_changed; then
      if [[ "${SKIP_TEST:-}" != "1" ]]; then
        if [[ "$PRE_PUSH_MODE" == "full" ]]; then
          run_job_sync "go test -race" job_go_test_full_race || heavy_errors=$((heavy_errors + 1))
        else
          run_job_sync "go test" job_go_test_quick || heavy_errors=$((heavy_errors + 1))
          if ((${#CHANGED_GO_PKGS[@]} > 0)); then
            run_job_sync "go test -race (changed)" job_go_test_changed_race || heavy_errors=$((heavy_errors + 1))
          else
            warn "No changed Go packages — skipping changed-pkgs race"
          fi
        fi
      else
        warn "Skipping test (SKIP_TEST=1)"
      fi

      if [[ "${SKIP_LINT:-}" != "1" ]] && [[ -x "$REPO_ROOT/scripts/run-golangci-lint.sh" ]]; then
        run_job_sync "golangci-lint" job_lint || heavy_errors=$((heavy_errors + 1))
      elif [[ "${SKIP_LINT:-}" == "1" ]]; then
        warn "Skipping lint (SKIP_LINT=1)"
      fi
    fi

    if $web_changed && [[ "${SKIP_WEB:-}" != "1" ]] && [[ -d "$REPO_ROOT/web" ]]; then
      run_job_sync "web build" job_web_build || heavy_errors=$((heavy_errors + 1))
    fi
  fi

  local elapsed=$(( SECONDS - start_time ))
  echo "─────────────────────────────────────"
  if [[ $heavy_errors -gt 0 ]]; then
    fail "${heavy_errors} check(s) failed — push aborted (${elapsed}s)"
    echo ""
    echo "  To skip: SKIP_PRE_PUSH=1 git push"
    echo "  To skip one: SKIP_MOD_TIDY=1 / SKIP_LINT=1 / SKIP_WEB=1 / SKIP_BUILD=1 / SKIP_TEST=1"
    exit 1
  else
    pass "All checks passed (${elapsed}s)"
    echo ""
  fi
}

main "$@"
