#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=../lib/common/lark_test_worktree.sh
source "${SCRIPT_DIR}/../lib/common/lark_test_worktree.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/loop.sh watch
  scripts/lark/loop.sh run --base-sha <sha>

Env:
  SLEEP_SECONDS       Watch poll interval (default: 10)
  MAX_CYCLES          Max auto-fix cycles for fast gate (default: 5)
  MAX_CYCLES_SLOW     Max auto-fix cycles for slow gate (default: 2)
  LARK_LOOP_AUTOFIX_ENABLED  Enable codex/claude auto-fix in gates (default: 0)
  FAST_GO_TEST_P      go test -p value for fast gate (default: 2)
  SLOW_GO_TEST_P      go test -p value for slow gate (default: 2)
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
MAIN_SH="${MAIN_ROOT}/scripts/lark/main.sh"
TEST_SH="${MAIN_ROOT}/scripts/lark/test.sh"
MAIN_CONFIG_PATH="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG_PATH}")"
CODEX_LOOP_PID_FILE="${PID_DIR}/lark-codex-loop.pid"

SLEEP_SECONDS="${SLEEP_SECONDS:-10}"
MAX_CYCLES="${MAX_CYCLES:-5}"
MAX_CYCLES_SLOW="${MAX_CYCLES_SLOW:-2}"
LOOP_AUTOFIX_ENABLED="${LARK_LOOP_AUTOFIX_ENABLED:-0}"
FAST_GO_TEST_P="${FAST_GO_TEST_P:-2}"
SLOW_GO_TEST_P="${SLOW_GO_TEST_P:-2}"

# Initialized by init_test_paths (stored in the test worktree to keep logs per worktree).
LOG_DIR=""
TMP_DIR=""
LOCK_DIR=""
LAST_FILE=""
LAST_VALIDATED_FILE=""

LOOP_LOG=""
FAIL_SUMMARY=""
SCENARIO_JSON=""
SCENARIO_MD=""
LOOP_STATE=""

init_test_paths() {
  lark_ensure_test_worktree "${MAIN_ROOT}"

  LOG_DIR="${TEST_ROOT}/logs"
  TMP_DIR="${TEST_ROOT}/tmp"
  LOCK_DIR="${TMP_DIR}/lark-loop.lock"
  LAST_FILE="${TMP_DIR}/lark-loop.last"
  LAST_VALIDATED_FILE="${TMP_DIR}/lark-loop.last-validated"

  LOOP_LOG="${LOG_DIR}/lark-loop.log"
  FAIL_SUMMARY="${LOG_DIR}/lark-loop.fail.txt"
  SCENARIO_JSON="${LOG_DIR}/lark-scenarios.json"
  SCENARIO_MD="${LOG_DIR}/lark-scenarios.md"
  LOOP_STATE="${TMP_DIR}/lark-loop.state.json"

  mkdir -p "${LOG_DIR}" "${TMP_DIR}"
}

acquire_lock() {
  if mkdir "${LOCK_DIR}" 2>/dev/null; then
    printf '%s\n' "pid=$$ started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${LOCK_DIR}/owner"
    return 0
  fi

  local owner_pid=""
  if [[ -f "${LOCK_DIR}/owner" ]]; then
    owner_pid="$(awk -F'[ =]+' '/pid=/{print $2}' "${LOCK_DIR}/owner" 2>/dev/null || true)"
  fi

  # Recover from stale locks left by crashed/terminated loop processes.
  if [[ -z "${owner_pid}" || "${owner_pid}" == "$$" ]] || ! kill -0 "${owner_pid}" 2>/dev/null; then
    rm -f "${LOCK_DIR}/owner" 2>/dev/null || true
    rmdir "${LOCK_DIR}" 2>/dev/null || true
    if mkdir "${LOCK_DIR}" 2>/dev/null; then
      printf '%s\n' "pid=$$ started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${LOCK_DIR}/owner"
      return 0
    fi
  fi

  return 1
}

release_lock() {
  rm -rf "${LOCK_DIR}" 2>/dev/null || true
}

record_codex_pid() {
  local pid="$1"
  mkdir -p "${PID_DIR}"
  printf '%s\n' "${pid}" > "${CODEX_LOOP_PID_FILE}"
}

clear_codex_pid() {
  local expected_pid="${1:-}"
  local current_pid
  current_pid="$(read_pid "${CODEX_LOOP_PID_FILE}" || true)"
  if [[ -z "${current_pid}" ]]; then
    rm -f "${CODEX_LOOP_PID_FILE}" 2>/dev/null || true
    return 0
  fi
  if [[ -n "${expected_pid}" && "${current_pid}" == "${expected_pid}" ]]; then
    rm -f "${CODEX_LOOP_PID_FILE}" 2>/dev/null || true
    return 0
  fi
  if ! is_process_running "${current_pid}"; then
    rm -f "${CODEX_LOOP_PID_FILE}" 2>/dev/null || true
  fi
}

append_log() {
  # shellcheck disable=SC2129
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" >> "${LOOP_LOG}"
}

json_escape() {
  printf '%s' "${1:-}" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

write_loop_state() {
  local base_sha="$1"
  local cycle_phase="$2"
  local cycle_result="$3"
  local last_error="${4:-}"
  local now_utc current_main_sha last_processed_sha

  [[ -n "${LOOP_STATE}" ]] || return 0

  now_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  current_main_sha="$(git -C "${MAIN_ROOT}" rev-parse main 2>/dev/null || true)"
  last_processed_sha="$(cat "${LAST_FILE}" 2>/dev/null || true)"
  local last_validated_sha
  last_validated_sha="$(cat "${LAST_VALIDATED_FILE}" 2>/dev/null || true)"

  local tmp_file
  tmp_file="${LOOP_STATE}.tmp"
  cat > "${tmp_file}" <<EOF
{
  "ts_utc": "${now_utc}",
  "base_sha": "${base_sha}",
  "cycle_phase": "${cycle_phase}",
  "cycle_result": "${cycle_result}",
  "main_sha": "${current_main_sha}",
  "last_processed_sha": "${last_processed_sha}",
  "last_validated_sha": "${last_validated_sha}",
  "validating_sha": "${base_sha}",
  "last_error": "$(json_escape "${last_error}")"
}
EOF
  mv "${tmp_file}" "${LOOP_STATE}"
}

require_tools() {
  command -v git >/dev/null 2>&1 || die "git not found"
  command -v go >/dev/null 2>&1 || die "go not found"
  [[ -x "${MAIN_SH}" ]] || die "Missing ${MAIN_SH}"
  [[ -x "${TEST_SH}" ]] || die "Missing ${TEST_SH}"
}

restart_test_agent() {
  append_log "[test] restart"
  "${TEST_SH}" restart >> "${LOOP_LOG}" 2>&1
}

stop_test_agent() {
  append_log "[test] stop for validation"
  "${TEST_SH}" stop >> "${LOOP_LOG}" 2>&1 || true
}

restore_test_to_validated() {
  local validated_sha
  validated_sha="$(cat "${LAST_VALIDATED_FILE}" 2>/dev/null || true)"
  if [[ -z "${validated_sha}" ]]; then
    append_log "[restore] no last_validated_sha; test bot stays down"
    return 0
  fi
  append_log "[restore] restoring test to last_validated_sha=${validated_sha:0:8}"
  git -C "${TEST_ROOT}" reset --hard "${validated_sha}" >> "${LOOP_LOG}" 2>&1
  restart_test_agent
}

run_scenario_suite() {
  # Use go run so the scenario runner always matches the current worktree code.
  (cd "${TEST_ROOT}" && go run ./cmd/alex lark scenario run --dir tests/scenarios/lark --json-out "${SCENARIO_JSON}" --md-out "${SCENARIO_MD}")
}

run_fast_gate() {
  set +e
  local scenario_rc=0
  local go_rc=0
  {
    echo ""
    echo "===== FAST GATE ====="
    echo "base_sha=${1}"
    echo "worktree=${TEST_ROOT}"
    echo "---------------------"
    echo "[scenario] running"
    run_scenario_suite
    scenario_rc=$?
    echo "[go test] running"
    (cd "${TEST_ROOT}" && CGO_ENABLED=0 go test ./... -count=1 -p "${FAST_GO_TEST_P}")
    go_rc=$?
  } >> "${LOOP_LOG}" 2>&1
  local rc=0
  if [[ ${scenario_rc} -ne 0 ]]; then
    rc=${scenario_rc}
  fi
  if [[ ${go_rc} -ne 0 ]]; then
    rc=${go_rc}
  fi
  set -e
  return "${rc}"
}

run_slow_gate() {
  set +e
  local lint_rc=0
  local test_rc=0
  {
    echo ""
    echo "===== SLOW GATE (CI PARITY) ====="
    echo "base_sha=${1}"
    echo "worktree=${TEST_ROOT}"
    echo "---------------------"
    (cd "${TEST_ROOT}" && ./dev.sh lint)
    lint_rc=$?
    # Limit package-level parallelism to reduce memory pressure (-race is expensive).
    (cd "${TEST_ROOT}" && GOFLAGS="${GOFLAGS:-} -p=${SLOW_GO_TEST_P}" ./dev.sh test)
    test_rc=$?
  } >> "${LOOP_LOG}" 2>&1
  local rc=0
  if [[ ${lint_rc} -ne 0 ]]; then
    rc=${lint_rc}
  fi
  if [[ ${test_rc} -ne 0 ]]; then
    rc=${test_rc}
  fi
  set -e
  return "${rc}"
}

write_fail_summary() {
  tail -n 200 "${LOOP_LOG}" > "${FAIL_SUMMARY}" || true
}

auto_fix() {
  local phase="$1"
  local base_sha="$2"
  if [[ "${LOOP_AUTOFIX_ENABLED}" != "1" ]]; then
    append_log "[auto-fix] phase=${phase} skipped (LARK_LOOP_AUTOFIX_ENABLED=${LOOP_AUTOFIX_ENABLED})"
    return 0
  fi
  write_fail_summary

  local prompt
  prompt="$(cat <<EOF
You are fixing the elephant.ai repo.
Worktree: ${TEST_ROOT}
Base: main@${base_sha} (test worktree HEAD)

Goal:
- Fix the failure shown below so FAST/SLOW gate passes.
- Keep changes minimal and local.

Failure log (tail):
$(cat "${FAIL_SUMMARY}")
EOF
)"

  append_log "[auto-fix] phase=${phase} starting"
  local prompt_file runner_pid rc
  prompt_file="$(mktemp "${TMP_DIR}/lark-loop-autofix.XXXXXX.prompt")"
  printf '%s' "${prompt}" > "${prompt_file}"

  if command -v codex >/dev/null 2>&1; then
    # Use codex exec so the loop is non-interactive and can auto-edit the worktree.
    (
      cd "${TEST_ROOT}" || exit 1
      exec codex exec --dangerously-bypass-approvals-and-sandbox - < "${prompt_file}"
    ) >> "${LOOP_LOG}" 2>&1 &
  elif command -v claude >/dev/null 2>&1; then
    # Claude Code fallback (autonomous mode).
    (
      cd "${TEST_ROOT}" || exit 1
      exec claude -p --dangerously-skip-permissions --allowedTools "Read,Edit,Bash(git *)" -- "$(cat "${prompt_file}")"
    ) >> "${LOOP_LOG}" 2>&1 &
  else
    rm -f "${prompt_file}"
    die "Neither codex nor claude found in PATH; cannot auto-fix"
  fi

  runner_pid="$!"
  record_codex_pid "${runner_pid}"
  rc=0
  wait "${runner_pid}" || rc=$?
  clear_codex_pid "${runner_pid}"
  rm -f "${prompt_file}"
  if (( rc != 0 )); then
    append_log "[auto-fix] codex runner exited with status=${rc}"
  fi

  # Commit if codex/claude changed anything.
  if ! git -C "${TEST_ROOT}" diff --quiet || ! git -C "${TEST_ROOT}" diff --cached --quiet; then
    git -C "${TEST_ROOT}" add -A
    git -C "${TEST_ROOT}" commit -m "fix(test): auto-fix ${phase} (${base_sha:0:8})" >> "${LOOP_LOG}" 2>&1 || true
  fi
}

merge_into_main_ff_only() {
  local base_sha="$1"
  local current_main_sha candidate_sha
  current_main_sha="$(git -C "${MAIN_ROOT}" rev-parse main)"

  if [[ "${current_main_sha}" != "${base_sha}" ]]; then
    append_log "[merge] main moved during cycle (base=${base_sha} now=${current_main_sha}); skipping merge and retrying on latest"
    return 2
  fi

  candidate_sha="$(git -C "${TEST_ROOT}" rev-parse HEAD)"
  append_log "[merge] ff-only ${candidate_sha:0:8} -> main"
  git -C "${MAIN_ROOT}" merge --ff-only "${candidate_sha}" >> "${LOOP_LOG}" 2>&1
}

restart_main_agent() {
  # Only restart the main agent when it's managed via scripts/lark/main.sh.
  if [[ ! -f "${PID_DIR}/lark-main.pid" ]]; then
    append_log "[main] skip restart (missing ${PID_DIR}/lark-main.pid)"
    return 0
  fi

  append_log "[main] restart"
  "${MAIN_SH}" restart >> "${LOOP_LOG}" 2>&1
}

run_cycle_locked() {
  local base_sha="$1"

  append_log "=== CYCLE START base_sha=${base_sha} autofix=${LOOP_AUTOFIX_ENABLED} ==="
  write_loop_state "${base_sha}" "start" "running" ""

  lark_ensure_test_worktree "${MAIN_ROOT}" >> "${LOOP_LOG}" 2>&1 || true

  # Stop the test bot so users never see unvalidated code.
  stop_test_agent
  write_loop_state "${base_sha}" "validating" "running" ""

  # Reset test worktree to the chosen base SHA (main snapshot).
  git -C "${TEST_ROOT}" reset --hard "${base_sha}" >> "${LOOP_LOG}" 2>&1

  # --- FAST GATE ---
  write_loop_state "${base_sha}" "fast_gate" "running" ""
  local i
  local fast_passed=0
  for i in $(seq 1 "${MAX_CYCLES}"); do
    append_log "[fast] attempt ${i}/${MAX_CYCLES}"
    if run_fast_gate "${base_sha}"; then
      append_log "[fast] pass"
      fast_passed=1
      break
    fi
    if [[ "${LOOP_AUTOFIX_ENABLED}" != "1" ]]; then
      append_log "[fast] fail; auto-fix disabled (set LARK_LOOP_AUTOFIX_ENABLED=1 to enable)"
      break
    fi
    append_log "[fast] fail; auto-fixing"
    auto_fix "fast" "${base_sha}"
    if run_fast_gate "${base_sha}"; then
      append_log "[fast] pass after auto-fix"
      fast_passed=1
      break
    fi
  done

  if [[ "${fast_passed}" != "1" ]]; then
    append_log "[fast] exhausted; giving up"
    write_loop_state "${base_sha}" "fast_gate" "failed" "fast gate exhausted"
    restore_test_to_validated
    write_loop_state "${base_sha}" "failed" "failed" "fast gate exhausted"
    return 1
  fi

  # --- SLOW GATE ---
  write_loop_state "${base_sha}" "slow_gate" "running" ""
  local j
  local slow_passed=0
  for j in $(seq 1 "${MAX_CYCLES_SLOW}"); do
    append_log "[slow] attempt ${j}/${MAX_CYCLES_SLOW}"
    if run_slow_gate "${base_sha}"; then
      append_log "[slow] pass"
      slow_passed=1
      break
    fi
    if [[ "${LOOP_AUTOFIX_ENABLED}" != "1" ]]; then
      append_log "[slow] fail; auto-fix disabled (set LARK_LOOP_AUTOFIX_ENABLED=1 to enable)"
      break
    fi
    append_log "[slow] fail; auto-fixing"
    auto_fix "slow" "${base_sha}"
    if run_slow_gate "${base_sha}"; then
      append_log "[slow] pass after auto-fix"
      slow_passed=1
      break
    fi
  done

  if [[ "${slow_passed}" != "1" ]]; then
    append_log "[slow] exhausted; giving up"
    write_loop_state "${base_sha}" "slow_gate" "failed" "slow gate exhausted"
    restore_test_to_validated
    write_loop_state "${base_sha}" "failed" "failed" "slow gate exhausted"
    return 1
  fi

  # --- PROMOTE: merge first, then deploy ---
  write_loop_state "${base_sha}" "promoting" "running" ""

  if merge_into_main_ff_only "${base_sha}"; then
    append_log "[merge] success"
  else
    local rc=$?
    if [[ ${rc} -eq 2 ]]; then
      # main moved; restore test bot, retry on next watch tick.
      append_log "[merge] main moved; restoring test bot"
      restore_test_to_validated
      write_loop_state "${base_sha}" "merge" "skipped" "main moved during cycle"
      return 0
    fi
    append_log "[merge] failed"
    restore_test_to_validated
    write_loop_state "${base_sha}" "merge" "failed" "ff-only merge failed"
    return 1
  fi

  # All gates passed and merge succeeded — deploy validated code.
  write_loop_state "${base_sha}" "deployed" "running" ""
  restart_test_agent
  restart_main_agent

  local new_main_sha
  new_main_sha="$(git -C "${MAIN_ROOT}" rev-parse main)"
  printf '%s\n' "${new_main_sha}" > "${LAST_FILE}"
  printf '%s\n' "${new_main_sha}" > "${LAST_VALIDATED_FILE}"
  append_log "=== CYCLE DONE main_sha=${new_main_sha} ==="
  write_loop_state "${base_sha}" "done" "passed" ""
}

run_cycle() {
  local base_sha="$1"
  local rc=0

  require_tools
  init_test_paths
  clear_codex_pid

  if ! acquire_lock; then
    log_warn "Loop already running (lock: ${LOCK_DIR})"
    return 0
  fi

  # Keep EXIT trap for abnormal termination, but always release lock on normal return.
  trap release_lock EXIT
  run_cycle_locked "${base_sha}" || rc=$?
  trap - EXIT
  release_lock
  clear_codex_pid
  return "${rc}"
}

watch() {
  require_tools
  init_test_paths
  write_loop_state "" "watch" "idle" ""

  log_info "Watching main commits (poll=${SLEEP_SECONDS}s, autofix=${LOOP_AUTOFIX_ENABLED})..."

  while true; do
    local main_sha last_sha
    main_sha="$(git -C "${MAIN_ROOT}" rev-parse main)"
    last_sha="$(cat "${LAST_FILE}" 2>/dev/null || true)"

    if [[ "${main_sha}" != "${last_sha}" ]]; then
      log_info "Detected new main SHA: ${main_sha:0:8} (last: ${last_sha:0:8})"
      if run_cycle "${main_sha}"; then
        :
      else
        log_warn "Cycle failed for ${main_sha:0:8} (see ${LOOP_LOG})"
        # Prevent tight re-runs on the same broken SHA; wait for main to advance.
        printf '%s\n' "${main_sha}" > "${LAST_FILE}"
        write_loop_state "${main_sha}" "watch" "failed" "cycle failed"
      fi
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
