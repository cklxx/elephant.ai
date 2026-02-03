#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/loop.sh watch
  scripts/lark/loop.sh run --base-sha <sha>

Env:
  SLEEP_SECONDS       Watch poll interval (default: 2)
  MAX_CYCLES          Max auto-fix cycles for fast gate (default: 5)
  MAX_CYCLES_SLOW     Max auto-fix cycles for slow gate (default: 2)
  MAIN_PORT           Main agent health port for restart verification (default: 8080)
EOF
}

git_worktree_path_for_branch() {
  local want_branch_ref="$1" # e.g. refs/heads/main
  git worktree list --porcelain | awk -v want="${want_branch_ref}" '
    $1=="worktree"{p=$2}
    $1=="branch" && $2==want {print p; exit}
  '
}

MAIN_ROOT="$(git_worktree_path_for_branch "refs/heads/main" || true)"
if [[ -z "${MAIN_ROOT}" ]]; then
  MAIN_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
fi
[[ -n "${MAIN_ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

TEST_ROOT="${MAIN_ROOT}/.worktrees/test"
WORKTREE_SH="${MAIN_ROOT}/scripts/lark/worktree.sh"
MAIN_SH="${MAIN_ROOT}/scripts/lark/main.sh"

SLEEP_SECONDS="${SLEEP_SECONDS:-2}"
MAX_CYCLES="${MAX_CYCLES:-5}"
MAX_CYCLES_SLOW="${MAX_CYCLES_SLOW:-2}"
MAIN_PORT="${MAIN_PORT:-8080}"

LOG_DIR="${MAIN_ROOT}/logs"
TMP_DIR="${MAIN_ROOT}/tmp"
LOCK_DIR="${TMP_DIR}/lark-loop.lock"
LAST_FILE="${TMP_DIR}/lark-loop.last"

mkdir -p "${LOG_DIR}" "${TMP_DIR}"

LOOP_LOG="${LOG_DIR}/lark-loop.log"
FAIL_SUMMARY="${LOG_DIR}/lark-loop.fail.txt"
SCENARIO_JSON="${LOG_DIR}/lark-scenarios.json"
SCENARIO_MD="${LOG_DIR}/lark-scenarios.md"

acquire_lock() {
  if mkdir "${LOCK_DIR}" 2>/dev/null; then
    printf '%s\n' "pid=$$ started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${LOCK_DIR}/owner"
    return 0
  fi
  return 1
}

release_lock() {
  rm -rf "${LOCK_DIR}" 2>/dev/null || true
}

append_log() {
  # shellcheck disable=SC2129
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" >> "${LOOP_LOG}"
}

require_tools() {
  command -v git >/dev/null 2>&1 || die "git not found"
  command -v go >/dev/null 2>&1 || die "go not found"
  command -v curl >/dev/null 2>&1 || die "curl not found"
  [[ -x "${WORKTREE_SH}" ]] || die "Missing ${WORKTREE_SH}"
  [[ -x "${MAIN_SH}" ]] || die "Missing ${MAIN_SH}"
}

run_scenario_suite() {
  # Use go run so the scenario runner always matches the current worktree code.
  (cd "${TEST_ROOT}" && go run ./cmd/alex lark scenario run --dir tests/scenarios/lark --json-out "${SCENARIO_JSON}" --md-out "${SCENARIO_MD}")
}

run_fast_gate() {
  set +e
  {
    echo ""
    echo "===== FAST GATE ====="
    echo "base_sha=${1}"
    echo "worktree=${TEST_ROOT}"
    echo "---------------------"
    echo "[scenario] running"
    run_scenario_suite
    echo "[go test] running"
    (cd "${TEST_ROOT}" && CGO_ENABLED=0 go test ./... -count=1)
  } >> "${LOOP_LOG}" 2>&1
  local rc=$?
  set -e
  return "${rc}"
}

run_slow_gate() {
  set +e
  {
    echo ""
    echo "===== SLOW GATE (CI PARITY) ====="
    echo "base_sha=${1}"
    echo "worktree=${TEST_ROOT}"
    echo "---------------------"
    (cd "${TEST_ROOT}" && ./dev.sh lint)
    (cd "${TEST_ROOT}" && ./dev.sh test)
  } >> "${LOOP_LOG}" 2>&1
  local rc=$?
  set -e
  return "${rc}"
}

write_fail_summary() {
  tail -n 200 "${LOOP_LOG}" > "${FAIL_SUMMARY}" || true
}

auto_fix() {
  local phase="$1"
  local base_sha="$2"
  write_fail_summary

  local prompt
  prompt="$(cat <<EOF
You are fixing the elephant.ai repo.
Worktree: ${TEST_ROOT}
Branch: test (based on main@${base_sha})

Goal:
- Fix the failure shown below so FAST/SLOW gate passes.
- Keep changes minimal and local.

Failure log (tail):
$(cat "${FAIL_SUMMARY}")
EOF
)"

  append_log "[auto-fix] phase=${phase} starting"

  if command -v codex >/dev/null 2>&1; then
    (cd "${TEST_ROOT}" && codex --approval-policy auto-edit --quiet "${prompt}") >> "${LOOP_LOG}" 2>&1 || true
  elif command -v claude >/dev/null 2>&1; then
    # Claude Code fallback (autonomous mode).
    (cd "${TEST_ROOT}" && claude -p --dangerously-skip-permissions --allowedTools "Read,Edit,Bash(git *)" -- "${prompt}") >> "${LOOP_LOG}" 2>&1 || true
  else
    die "Neither codex nor claude found in PATH; cannot auto-fix"
  fi

  # Commit if codex/claude changed anything.
  if ! git -C "${TEST_ROOT}" diff --quiet || ! git -C "${TEST_ROOT}" diff --cached --quiet; then
    git -C "${TEST_ROOT}" add -A
    git -C "${TEST_ROOT}" commit -m "fix(test): auto-fix ${phase} (${base_sha:0:8})" >> "${LOOP_LOG}" 2>&1 || true
  fi
}

merge_into_main_ff_only() {
  local base_sha="$1"
  local current_main_sha
  current_main_sha="$(git -C "${MAIN_ROOT}" rev-parse main)"

  if [[ "${current_main_sha}" != "${base_sha}" ]]; then
    append_log "[merge] main moved during cycle (base=${base_sha} now=${current_main_sha}); skipping merge and retrying on latest"
    return 2
  fi

  append_log "[merge] ff-only test -> main"
  git -C "${MAIN_ROOT}" merge --ff-only test >> "${LOOP_LOG}" 2>&1
}

restart_main_agent() {
  append_log "[main] restart"
  "${MAIN_SH}" restart >> "${LOOP_LOG}" 2>&1

  local health_url="http://127.0.0.1:${MAIN_PORT}/health"
  local i
  for i in $(seq 1 30); do
    if curl -sf "${health_url}" >/dev/null 2>&1; then
      append_log "[main] healthy ${health_url}"
      return 0
    fi
    sleep 1
  done

  append_log "[main] restart timed out (health not ready)"
  return 1
}

run_cycle() {
  local base_sha="$1"

  require_tools

  if ! acquire_lock; then
    log_warn "Loop already running (lock: ${LOCK_DIR})"
    return 0
  fi
  trap release_lock EXIT

  append_log "=== CYCLE START base_sha=${base_sha} ==="

  "${WORKTREE_SH}" ensure >> "${LOOP_LOG}" 2>&1

  # Reset test branch to the chosen base SHA (main snapshot).
  git -C "${TEST_ROOT}" switch test >> "${LOOP_LOG}" 2>&1
  git -C "${TEST_ROOT}" reset --hard "${base_sha}" >> "${LOOP_LOG}" 2>&1

  local i
  for i in $(seq 1 "${MAX_CYCLES}"); do
    append_log "[fast] attempt ${i}/${MAX_CYCLES}"
    if run_fast_gate "${base_sha}"; then
      append_log "[fast] pass"
      break
    fi
    append_log "[fast] fail; auto-fixing"
    auto_fix "fast" "${base_sha}"
  done

  if ! run_fast_gate "${base_sha}"; then
    append_log "[fast] exhausted; giving up"
    return 1
  fi

  local j
  for j in $(seq 1 "${MAX_CYCLES_SLOW}"); do
    append_log "[slow] attempt ${j}/${MAX_CYCLES_SLOW}"
    if run_slow_gate "${base_sha}"; then
      append_log "[slow] pass"
      break
    fi
    append_log "[slow] fail; auto-fixing"
    auto_fix "slow" "${base_sha}"
  done

  if ! run_slow_gate "${base_sha}"; then
    append_log "[slow] exhausted; giving up"
    return 1
  fi

  if ! merge_into_main_ff_only "${base_sha}"; then
    append_log "[merge] success"
  else
    local rc=$?
    if [[ ${rc} -eq 2 ]]; then
      # main moved; do not update last file so watch will retry on latest.
      return 0
    fi
    append_log "[merge] failed"
    return 1
  fi

  restart_main_agent

  local new_main_sha
  new_main_sha="$(git -C "${MAIN_ROOT}" rev-parse main)"
  printf '%s\n' "${new_main_sha}" > "${LAST_FILE}"
  append_log "=== CYCLE DONE main_sha=${new_main_sha} ==="
}

watch() {
  require_tools

  log_info "Watching main commits (poll=${SLEEP_SECONDS}s)..."

  while true; do
    local main_sha last_sha
    main_sha="$(git -C "${MAIN_ROOT}" rev-parse main)"
    last_sha="$(cat "${LAST_FILE}" 2>/dev/null || true)"

    if [[ "${main_sha}" != "${last_sha}" ]]; then
      log_info "Detected new main SHA: ${main_sha:0:8} (last: ${last_sha:0:8})"
      run_cycle "${main_sha}" || log_warn "Cycle failed for ${main_sha:0:8} (see ${LOOP_LOG})"
    fi

    sleep "${SLEEP_SECONDS}"
  done
}

cmd="${1:-watch}"
shift || true

case "${cmd}" in
  watch) watch ;;
  run)
    base_sha=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --base-sha)
          base_sha="${2:-}"
          shift 2
          ;;
        *)
          usage; die "Unknown arg: $1"
          ;;
      esac
    done
    [[ -n "${base_sha}" ]] || { usage; die "--base-sha is required"; }
    run_cycle "${base_sha}"
    ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac

