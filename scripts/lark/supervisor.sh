#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/supervisor.sh start|stop|restart|status|logs|doctor|run-once
  scripts/lark/supervisor.sh run

Behavior:
  - Supervises main/test/loop processes for local autonomous iteration
  - Maintains structured status at .worktrees/test/tmp/lark-supervisor.status.json

Env:
  LARK_SUPERVISOR_TICK_SECONDS   Loop tick interval (default: 5)
  LARK_RESTART_MAX_IN_WINDOW     Max restarts per component in window (default: 5)
  LARK_RESTART_WINDOW_SECONDS    Restart counting window seconds (default: 600)
  LARK_COOLDOWN_SECONDS          Cooldown seconds after restart storm (default: 300)
  MAIN_CONFIG                    Main config path override (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
  TEST_CONFIG                    Test config path override (default: ~/.alex/test.yaml)
  LARK_MAIN_ROOT                 Main root override (tests only)
EOF
}

git_worktree_path_for_branch() {
  local want_branch_ref="$1"
  git -C "${SCRIPT_DIR}" worktree list --porcelain | awk -v want="${want_branch_ref}" '
    $1=="worktree"{p=$2}
    $1=="branch" && $2==want {print p; exit}
  '
}

if [[ -n "${LARK_MAIN_ROOT:-}" ]]; then
  MAIN_ROOT="${LARK_MAIN_ROOT}"
else
  MAIN_ROOT="$(git_worktree_path_for_branch "refs/heads/main" || true)"
  if [[ -z "${MAIN_ROOT}" ]]; then
    MAIN_ROOT="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel 2>/dev/null || true)"
  fi
fi
[[ -n "${MAIN_ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

WORKTREE_SH="${WORKTREE_SH:-${MAIN_ROOT}/scripts/lark/worktree.sh}"
MAIN_SH="${MAIN_SH:-${MAIN_ROOT}/scripts/lark/main.sh}"
TEST_SH="${TEST_SH:-${MAIN_ROOT}/scripts/lark/test.sh}"
LOOP_AGENT_SH="${LOOP_AGENT_SH:-${MAIN_ROOT}/scripts/lark/loop-agent.sh}"

TEST_ROOT="${MAIN_ROOT}/.worktrees/test"
PID_DIR="${TEST_ROOT}/.pids"
LOG_DIR="${TEST_ROOT}/logs"
TMP_DIR="${TEST_ROOT}/tmp"

PID_FILE="${PID_DIR}/lark-supervisor.pid"
LOG_FILE="${LOG_DIR}/lark-supervisor.log"
LOCK_DIR="${TMP_DIR}/lark-supervisor.lock"
STATUS_FILE="${TMP_DIR}/lark-supervisor.status.json"
LOOP_STATE_FILE="${TMP_DIR}/lark-loop.state.json"
LAST_PROCESSED_FILE="${TMP_DIR}/lark-loop.last"

MAIN_PID_FILE="${MAIN_ROOT}/.pids/lark-main.pid"
TEST_PID_FILE="${TEST_ROOT}/.pids/lark-test.pid"
LOOP_PID_FILE="${TEST_ROOT}/.pids/lark-loop.pid"

MAIN_CONFIG_PATH="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
TEST_CONFIG_PATH="${TEST_CONFIG:-$HOME/.alex/test.yaml}"

TICK_SECONDS="${LARK_SUPERVISOR_TICK_SECONDS:-5}"
RESTART_MAX_IN_WINDOW="${LARK_RESTART_MAX_IN_WINDOW:-5}"
RESTART_WINDOW_SECONDS="${LARK_RESTART_WINDOW_SECONDS:-600}"
COOLDOWN_SECONDS="${LARK_COOLDOWN_SECONDS:-300}"
SKIP_HEALTHCHECK="${LARK_SUPERVISOR_SKIP_HEALTHCHECK:-0}"

MODE="degraded"
COOLDOWN_UNTIL=0
LAST_ERROR=""

MAIN_FAIL_COUNT=0
TEST_FAIL_COUNT=0
LOOP_FAIL_COUNT=0

MAIN_RESTART_HISTORY=""
TEST_RESTART_HISTORY=""
LOOP_RESTART_HISTORY=""

OBS_MAIN_PID=""
OBS_TEST_PID=""
OBS_LOOP_PID=""
OBS_MAIN_HEALTH="down"
OBS_TEST_HEALTH="down"
OBS_LOOP_HEALTH="down"
OBS_MAIN_SHA="unknown"
OBS_LAST_PROCESSED_SHA=""
OBS_CYCLE_PHASE="idle"
OBS_CYCLE_RESULT="unknown"
OBS_LOOP_ERROR=""
OBS_RESTART_COUNT_WINDOW=0

json_escape() {
  printf '%s' "${1:-}" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

ensure_dirs() {
  mkdir -p "${PID_DIR}" "${LOG_DIR}" "${TMP_DIR}"
}

append_log() {
  ensure_dirs
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" >> "${LOG_FILE}"
}

ensure_worktree() {
  [[ -x "${WORKTREE_SH}" ]] || die "Missing ${WORKTREE_SH}"
  "${WORKTREE_SH}" ensure >/dev/null
  ensure_dirs
}

sanitize_port() {
  local port="$1"
  if [[ "${port}" =~ ^[0-9]+$ ]]; then
    echo "${port}"
  fi
}

infer_port_from_config() {
  local config_path="$1"
  [[ -f "${config_path}" ]] || return 0

  awk '
    function ltrim(s){sub(/^[ \t]+/, "", s); return s}
    function indent(s){match(s, /^[ \t]*/); return RLENGTH}
    BEGIN{server_indent=-1}
    {
      if ($0 ~ /^[ \t]*server:[ \t]*$/) {
        server_indent = indent($0)
        next
      }
      if (server_indent >= 0) {
        if (indent($0) <= server_indent && $0 ~ /^[ \t]*[A-Za-z0-9_-]+:[ \t]*/) {
          server_indent = -1
          next
        }
        if ($0 ~ /^[ \t]*port:[ \t]*/) {
          line = ltrim($0)
          sub(/^port:[ \t]*/, "", line)
          sub(/[ \t]#.*/, "", line)
          gsub(/^[\"\047]/, "", line)
          gsub(/[\"\047]$/, "", line)
          print line
          exit
        }
      }
    }
  ' "${config_path}"
}

main_health_url() {
  local inferred port
  inferred="$(sanitize_port "$(infer_port_from_config "${MAIN_CONFIG_PATH}" || true)")"
  port="${MAIN_PORT:-${inferred:-8080}}"
  port="$(sanitize_port "${port}")"
  echo "http://127.0.0.1:${port:-8080}/health"
}

test_health_url() {
  local inferred port
  inferred="$(sanitize_port "$(infer_port_from_config "${TEST_CONFIG_PATH}" || true)")"
  port="${TEST_PORT:-${inferred:-8080}}"
  port="$(sanitize_port "${port}")"
  echo "http://127.0.0.1:${port:-8080}/health"
}

read_pid_if_running() {
  local pid_file="$1"
  local pid
  pid="$(read_pid "${pid_file}" || true)"
  if is_process_running "${pid}"; then
    echo "${pid}"
  fi
}

main_health_state() {
  local pid
  pid="$(read_pid_if_running "${MAIN_PID_FILE}")"
  if [[ -z "${pid}" ]]; then
    echo "down"
    return
  fi
  if [[ "${SKIP_HEALTHCHECK}" == "1" ]]; then
    echo "healthy"
    return
  fi
  if curl -sf "$(main_health_url)" >/dev/null 2>&1; then
    echo "healthy"
  else
    echo "unhealthy"
  fi
}

test_health_state() {
  local pid
  pid="$(read_pid_if_running "${TEST_PID_FILE}")"
  if [[ -z "${pid}" ]]; then
    echo "down"
    return
  fi
  if [[ "${SKIP_HEALTHCHECK}" == "1" ]]; then
    echo "healthy"
    return
  fi
  if curl -sf "$(test_health_url)" >/dev/null 2>&1; then
    echo "healthy"
  else
    echo "unhealthy"
  fi
}

loop_health_state() {
  local pid
  pid="$(read_pid_if_running "${LOOP_PID_FILE}")"
  if [[ -z "${pid}" ]]; then
    echo "down"
  else
    echo "alive"
  fi
}

extract_json_string() {
  local file="$1"
  local key="$2"
  local out
  [[ -f "${file}" ]] || return 1
  out="$(
    tr -d '\n' < "${file}" \
      | sed -nE 's/.*"'${key}'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p'
  )"
  [[ -n "${out}" ]] || return 1
  printf '%s' "${out}"
}

history_prune() {
  local history="$1"
  local now_epoch="$2"
  local cutoff_epoch out ts
  cutoff_epoch=$((now_epoch - RESTART_WINDOW_SECONDS))
  out=""
  while IFS= read -r ts; do
    [[ -n "${ts}" ]] || continue
    if (( ts >= cutoff_epoch )); then
      out+="${ts}"$'\n'
    fi
  done <<< "${history}"
  printf '%s' "${out}"
}

history_append() {
  local history="$1"
  local ts="$2"
  if [[ -z "${history}" ]]; then
    printf '%s\n' "${ts}"
    return
  fi
  printf '%s%s\n' "${history}" "${ts}"
}

history_count() {
  local history="$1"
  local count ts
  count=0
  while IFS= read -r ts; do
    [[ -n "${ts}" ]] || continue
    count=$((count + 1))
  done <<< "${history}"
  echo "${count}"
}

restart_history_for_component() {
  local component="$1"
  case "${component}" in
    main) echo "${MAIN_RESTART_HISTORY}" ;;
    test) echo "${TEST_RESTART_HISTORY}" ;;
    loop) echo "${LOOP_RESTART_HISTORY}" ;;
    *) echo "" ;;
  esac
}

set_restart_history_for_component() {
  local component="$1"
  local history="$2"
  case "${component}" in
    main) MAIN_RESTART_HISTORY="${history}" ;;
    test) TEST_RESTART_HISTORY="${history}" ;;
    loop) LOOP_RESTART_HISTORY="${history}" ;;
  esac
}

fail_count_for_component() {
  local component="$1"
  case "${component}" in
    main) echo "${MAIN_FAIL_COUNT}" ;;
    test) echo "${TEST_FAIL_COUNT}" ;;
    loop) echo "${LOOP_FAIL_COUNT}" ;;
    *) echo "0" ;;
  esac
}

set_fail_count_for_component() {
  local component="$1"
  local value="$2"
  case "${component}" in
    main) MAIN_FAIL_COUNT="${value}" ;;
    test) TEST_FAIL_COUNT="${value}" ;;
    loop) LOOP_FAIL_COUNT="${value}" ;;
  esac
}

record_restart_attempt() {
  local component="$1"
  local now_epoch="$2"
  local history count
  history="$(restart_history_for_component "${component}")"
  history="$(history_prune "${history}" "${now_epoch}")"
  history="$(history_append "${history}" "${now_epoch}")"
  set_restart_history_for_component "${component}" "${history}"
  count="$(history_count "${history}")"
  echo "${count}"
}

total_restart_count_window() {
  local now_epoch="$1"
  local main_count test_count loop_count
  MAIN_RESTART_HISTORY="$(history_prune "${MAIN_RESTART_HISTORY}" "${now_epoch}")"
  TEST_RESTART_HISTORY="$(history_prune "${TEST_RESTART_HISTORY}" "${now_epoch}")"
  LOOP_RESTART_HISTORY="$(history_prune "${LOOP_RESTART_HISTORY}" "${now_epoch}")"
  main_count="$(history_count "${MAIN_RESTART_HISTORY}")"
  test_count="$(history_count "${TEST_RESTART_HISTORY}")"
  loop_count="$(history_count "${LOOP_RESTART_HISTORY}")"
  echo $((main_count + test_count + loop_count))
}

restart_component() {
  local component="$1"
  case "${component}" in
    main)
      "${MAIN_SH}" restart >> "${LOG_FILE}" 2>&1
      ;;
    test)
      "${TEST_SH}" restart >> "${LOG_FILE}" 2>&1
      ;;
    loop)
      "${LOOP_AGENT_SH}" restart >> "${LOG_FILE}" 2>&1
      ;;
    *)
      return 1
      ;;
  esac
}

component_needs_restart() {
  local component="$1"
  local state="$2"
  case "${component}" in
    main|test)
      [[ "${state}" != "healthy" ]]
      ;;
    loop)
      [[ "${state}" != "alive" ]]
      ;;
    *)
      return 1
      ;;
  esac
}

observe_states() {
  OBS_MAIN_PID="$(read_pid_if_running "${MAIN_PID_FILE}" || true)"
  OBS_TEST_PID="$(read_pid_if_running "${TEST_PID_FILE}" || true)"
  OBS_LOOP_PID="$(read_pid_if_running "${LOOP_PID_FILE}" || true)"

  OBS_MAIN_HEALTH="$(main_health_state)"
  OBS_TEST_HEALTH="$(test_health_state)"
  OBS_LOOP_HEALTH="$(loop_health_state)"

  OBS_MAIN_SHA="$(git -C "${MAIN_ROOT}" rev-parse main 2>/dev/null || echo "unknown")"
  OBS_LAST_PROCESSED_SHA="$(cat "${LAST_PROCESSED_FILE}" 2>/dev/null || true)"

  OBS_CYCLE_PHASE="$(extract_json_string "${LOOP_STATE_FILE}" "cycle_phase" || echo "idle")"
  OBS_CYCLE_RESULT="$(extract_json_string "${LOOP_STATE_FILE}" "cycle_result" || echo "unknown")"
  OBS_LOOP_ERROR="$(extract_json_string "${LOOP_STATE_FILE}" "last_error" || true)"
}

current_mode() {
  local now_epoch="$1"
  if (( now_epoch < COOLDOWN_UNTIL )); then
    echo "cooldown"
    return
  fi
  if [[ "${OBS_MAIN_HEALTH}" == "healthy" && "${OBS_TEST_HEALTH}" == "healthy" && "${OBS_LOOP_HEALTH}" == "alive" ]]; then
    echo "healthy"
  else
    echo "degraded"
  fi
}

write_status_file() {
  local now_epoch now_utc mode last_error restart_count loop_alive
  now_epoch="$(date +%s)"
  now_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  restart_count="$(total_restart_count_window "${now_epoch}")"
  OBS_RESTART_COUNT_WINDOW="${restart_count}"
  mode="$(current_mode "${now_epoch}")"
  MODE="${mode}"

  if [[ "${OBS_LOOP_HEALTH}" == "alive" ]]; then
    loop_alive="true"
  else
    loop_alive="false"
  fi

  last_error="${LAST_ERROR}"
  if [[ -z "${last_error}" ]]; then
    last_error="${OBS_LOOP_ERROR}"
  fi

  local tmp_file
  tmp_file="${STATUS_FILE}.tmp"
  cat > "${tmp_file}" <<EOF
{
  "ts_utc": "${now_utc}",
  "mode": "${MODE}",
  "main_pid": "${OBS_MAIN_PID}",
  "test_pid": "${OBS_TEST_PID}",
  "loop_pid": "${OBS_LOOP_PID}",
  "main_health": "${OBS_MAIN_HEALTH}",
  "test_health": "${OBS_TEST_HEALTH}",
  "loop_alive": ${loop_alive},
  "main_sha": "${OBS_MAIN_SHA}",
  "last_processed_sha": "${OBS_LAST_PROCESSED_SHA}",
  "cycle_phase": "${OBS_CYCLE_PHASE}",
  "cycle_result": "${OBS_CYCLE_RESULT}",
  "last_error": "$(json_escape "${last_error}")",
  "restart_count_window": ${restart_count}
}
EOF
  mv "${tmp_file}" "${STATUS_FILE}"
}

set_cooldown() {
  local component="$1"
  local count="$2"
  local now_epoch
  now_epoch="$(date +%s)"
  COOLDOWN_UNTIL=$((now_epoch + COOLDOWN_SECONDS))
  LAST_ERROR="restart storm detected for ${component} (${count} in ${RESTART_WINDOW_SECONDS}s), cooldown ${COOLDOWN_SECONDS}s"
  MODE="cooldown"
  append_log "[supervisor] ${LAST_ERROR}"
}

restart_with_backoff() {
  local component="$1"
  local now_epoch state fail_count delay count
  now_epoch="$(date +%s)"
  state="$2"

  if (( now_epoch < COOLDOWN_UNTIL )); then
    MODE="cooldown"
    return 0
  fi

  if ! component_needs_restart "${component}" "${state}"; then
    set_fail_count_for_component "${component}" 0
    return 0
  fi

  MODE="degraded"
  fail_count="$(fail_count_for_component "${component}")"
  fail_count=$((fail_count + 1))
  set_fail_count_for_component "${component}" "${fail_count}"
  delay=$((1 << (fail_count - 1)))
  if (( delay > 60 )); then
    delay=60
  fi

  count="$(record_restart_attempt "${component}" "${now_epoch}")"
  if (( count > RESTART_MAX_IN_WINDOW )); then
    set_cooldown "${component}" "${count}"
    return 1
  fi

  append_log "[supervisor] ${component}=${state}; restart in ${delay}s (attempt=${fail_count} window=${count})"
  sleep "${delay}"
  if restart_component "${component}"; then
    append_log "[supervisor] ${component} restart success"
    set_fail_count_for_component "${component}" 0
    LAST_ERROR=""
    return 0
  fi

  LAST_ERROR="${component} restart failed"
  append_log "[supervisor] ${LAST_ERROR}"
  return 1
}

run_tick() {
  observe_states

  restart_with_backoff "main" "${OBS_MAIN_HEALTH}" || true
  observe_states
  restart_with_backoff "test" "${OBS_TEST_HEALTH}" || true
  observe_states
  restart_with_backoff "loop" "${OBS_LOOP_HEALTH}" || true
  observe_states

  write_status_file
}

acquire_lock() {
  if mkdir "${LOCK_DIR}" 2>/dev/null; then
    printf 'pid=%s started_at=%s\n' "$$" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${LOCK_DIR}/owner"
    return 0
  fi
  return 1
}

release_lock() {
  rm -rf "${LOCK_DIR}" 2>/dev/null || true
}

cleanup() {
  release_lock
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if [[ "${pid}" == "$$" ]]; then
    rm -f "${PID_FILE}"
  fi
}

run_supervisor() {
  ensure_worktree
  ensure_dirs

  if ! acquire_lock; then
    die "Supervisor already running (lock: ${LOCK_DIR})"
  fi
  trap cleanup EXIT INT TERM
  echo "$$" > "${PID_FILE}"
  append_log "[supervisor] start tick=${TICK_SECONDS}s window=${RESTART_WINDOW_SECONDS}s max=${RESTART_MAX_IN_WINDOW} cooldown=${COOLDOWN_SECONDS}s"

  while true; do
    run_tick
    sleep "${TICK_SECONDS}"
  done
}

start() {
  ensure_worktree
  ensure_dirs

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Supervisor already running (PID: ${pid})"
    return 0
  fi

  nohup "${SCRIPT_DIR}/supervisor.sh" run >> "${LOG_FILE}" 2>&1 &
  echo "$!" > "${PID_FILE}"
  sleep 1
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Supervisor started (PID: ${pid})"
    return 0
  fi
  die "Supervisor failed to start (see ${LOG_FILE})"
}

stop_components() {
  "${LOOP_AGENT_SH}" stop >> "${LOG_FILE}" 2>&1 || true
  "${TEST_SH}" stop >> "${LOG_FILE}" 2>&1 || true
  "${MAIN_SH}" stop >> "${LOG_FILE}" 2>&1 || true
}

stop() {
  ensure_worktree
  ensure_dirs
  stop_service "Lark supervisor" "${PID_FILE}" >> "${LOG_FILE}" 2>&1 || true
  stop_components
  MODE="degraded"
  observe_states
  write_status_file
  log_success "Supervisor and child processes stopped"
}

restart() {
  stop
  start
}

status() {
  ensure_worktree
  ensure_dirs
  observe_states
  write_status_file

  local pid state mode
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    state="running"
  else
    state="stopped"
  fi
  mode="$(extract_json_string "${STATUS_FILE}" "mode" || echo "${MODE}")"

  echo "supervisor: ${state} pid=${pid:-}"
  echo "mode: ${mode}"
  echo "main: ${OBS_MAIN_HEALTH} pid=${OBS_MAIN_PID}"
  echo "test: ${OBS_TEST_HEALTH} pid=${OBS_TEST_PID}"
  echo "loop: ${OBS_LOOP_HEALTH} pid=${OBS_LOOP_PID}"
  echo "main_sha: ${OBS_MAIN_SHA}"
  echo "last_processed_sha: ${OBS_LAST_PROCESSED_SHA}"
  echo "cycle_phase: ${OBS_CYCLE_PHASE}"
  echo "cycle_result: ${OBS_CYCLE_RESULT}"
  echo "restart_count_window: ${OBS_RESTART_COUNT_WINDOW}"
  echo "status_file: ${STATUS_FILE}"
}

logs() {
  ensure_worktree
  ensure_dirs
  touch \
    "${LOG_FILE}" \
    "${MAIN_ROOT}/logs/lark-main.log" \
    "${TEST_ROOT}/logs/lark-test.log" \
    "${TEST_ROOT}/logs/lark-loop.log" \
    "${TEST_ROOT}/logs/lark-loop-agent.log"
  tail -n 200 -f \
    "${LOG_FILE}" \
    "${MAIN_ROOT}/logs/lark-main.log" \
    "${TEST_ROOT}/logs/lark-test.log" \
    "${TEST_ROOT}/logs/lark-loop.log" \
    "${TEST_ROOT}/logs/lark-loop-agent.log"
}

doctor() {
  local failures warnings pid port
  failures=0
  warnings=0

  echo "== lark doctor =="
  echo "main_root: ${MAIN_ROOT}"
  echo "test_root: ${TEST_ROOT}"

  for cmd in git go curl lsof; do
    if command_exists "${cmd}"; then
      echo "[ok] command: ${cmd}"
    else
      echo "[fail] missing command: ${cmd}"
      failures=$((failures + 1))
    fi
  done

  for script in "${WORKTREE_SH}" "${MAIN_SH}" "${TEST_SH}" "${LOOP_AGENT_SH}"; do
    if [[ -x "${script}" ]]; then
      echo "[ok] script: ${script}"
    else
      echo "[fail] missing/non-executable script: ${script}"
      failures=$((failures + 1))
    fi
  done

  if git -C "${MAIN_ROOT}" worktree list --porcelain | awk -v p="${TEST_ROOT}" '$1=="worktree" && $2==p {found=1} END{exit found?0:1}'; then
    echo "[ok] test worktree registered"
  else
    echo "[warn] test worktree is not registered in git worktree list"
    warnings=$((warnings + 1))
  fi

  if [[ -f "${MAIN_CONFIG_PATH}" ]]; then
    echo "[ok] main config: ${MAIN_CONFIG_PATH}"
  else
    echo "[warn] missing main config: ${MAIN_CONFIG_PATH}"
    warnings=$((warnings + 1))
  fi

  if [[ -f "${TEST_CONFIG_PATH}" ]]; then
    echo "[ok] test config: ${TEST_CONFIG_PATH}"
  else
    echo "[warn] missing test config: ${TEST_CONFIG_PATH}"
    warnings=$((warnings + 1))
  fi

  echo "[info] main health URL: $(main_health_url)"
  echo "[info] test health URL: $(test_health_url)"

  for pid_file in "${PID_FILE}" "${MAIN_PID_FILE}" "${TEST_PID_FILE}" "${LOOP_PID_FILE}"; do
    pid="$(read_pid "${pid_file}" || true)"
    if [[ -z "${pid}" ]]; then
      echo "[warn] missing pid file value: ${pid_file}"
      warnings=$((warnings + 1))
      continue
    fi
    if is_process_running "${pid}"; then
      echo "[ok] pid alive: ${pid_file} -> ${pid}"
    else
      echo "[warn] stale pid: ${pid_file} -> ${pid}"
      warnings=$((warnings + 1))
    fi
  done

  port="$(main_health_url)"
  port="${port##*:}"
  port="${port%/health}"
  if lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "[ok] main port listening: ${port}"
  else
    echo "[warn] main port not listening: ${port}"
    warnings=$((warnings + 1))
  fi

  port="$(test_health_url)"
  port="${port##*:}"
  port="${port%/health}"
  if lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "[ok] test port listening: ${port}"
  else
    echo "[warn] test port not listening: ${port}"
    warnings=$((warnings + 1))
  fi

  echo "summary: failures=${failures} warnings=${warnings}"
  if (( failures > 0 )); then
    return 1
  fi
}

run_once() {
  ensure_worktree
  ensure_dirs
  run_tick
  status
}

cmd="${1:-start}"
shift || true

case "${cmd}" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  status) status ;;
  logs) logs ;;
  doctor) doctor ;;
  run) run_supervisor ;;
  run-once) run_once ;;
  help|-h|--help) usage ;;
  *)
    usage
    die "Unknown command: ${cmd}"
    ;;
esac
