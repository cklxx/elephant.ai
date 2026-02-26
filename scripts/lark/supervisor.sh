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
  scripts/lark/supervisor.sh start|stop|restart|status|logs|doctor|run-once
  scripts/lark/supervisor.sh run

Behavior:
  - Supervises main/kernel/loop processes for local autonomous iteration
  - Maintains structured status at .worktrees/test/tmp/lark-supervisor.status.json

Env:
  LARK_SUPERVISOR_TICK_SECONDS   Loop tick interval (default: 5)
  LARK_RESTART_MAX_IN_WINDOW     Max restarts per component in window (default: 5)
  LARK_RESTART_WINDOW_SECONDS    Restart counting window seconds (default: 600)
  LARK_COOLDOWN_SECONDS          Cooldown seconds after restart storm (default: 300)
  LARK_SUPERVISOR_AUTOFIX_ENABLED            Enable autofix runner (default: 0)
  LARK_SUPERVISOR_AUTOFIX_TRIGGER            Trigger policy (default: cooldown)
  LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS    Codex timeout seconds (default: 1800)
  LARK_SUPERVISOR_AUTOFIX_MAX_IN_WINDOW      Max autofix runs in window (default: 3)
  LARK_SUPERVISOR_AUTOFIX_WINDOW_SECONDS     Autofix counting window (default: 3600)
  LARK_SUPERVISOR_AUTOFIX_COOLDOWN_SECONDS   Autofix cooldown seconds (default: 900)
  LARK_SUPERVISOR_AUTOFIX_SCOPE              Prompt scope hint (default: repo)
  LARK_LOOP_AUTOFIX_ENABLED                  Enable loop gate auto-fix edits (default: 0)
  LARK_SUPERVISOR_NOTIFY_SH                  Notification sender script (default: scripts/lark/notify.sh)
  LARK_NOTICE_STATE_FILE                     Notice binding state file path (default: .worktrees/test/tmp/lark-notice.state.json)
  LARK_STALE_LOOP_STATE_TIMEOUT_SECONDS      Max seconds to keep validation phase while loop is down before stale-state recovery (default: 30)
  LARK_PID_DIR                    Shared pid dir override (default: <dirname(MAIN_CONFIG)>/pids)
  MAIN_CONFIG                    Main config path override (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
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

MAIN_SH="${MAIN_SH:-${MAIN_ROOT}/scripts/lark/main.sh}"
LOOP_AGENT_SH="${LOOP_AGENT_SH:-${MAIN_ROOT}/scripts/lark/loop-agent.sh}"
KERNEL_SH="${KERNEL_SH:-${MAIN_ROOT}/scripts/lark/kernel.sh}"
AUTOFIX_SH="${AUTOFIX_SH:-${MAIN_ROOT}/scripts/lark/autofix.sh}"
NOTIFY_SH="${LARK_SUPERVISOR_NOTIFY_SH:-${MAIN_ROOT}/scripts/lark/notify.sh}"

TEST_ROOT="${MAIN_ROOT}/.worktrees/test"
MAIN_CONFIG_PATH="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
PID_DIR="${LARK_PID_DIR:-$(lark_shared_pid_dir "${MAIN_CONFIG_PATH}")}"
LOG_DIR="${TEST_ROOT}/logs"
TMP_DIR="${TEST_ROOT}/tmp"

PID_FILE="${PID_DIR}/lark-supervisor.pid"
LOG_FILE="${LOG_DIR}/lark-supervisor.log"
LOCK_DIR="${TMP_DIR}/lark-supervisor.lock"
STATUS_FILE="${TMP_DIR}/lark-supervisor.status.json"
LOOP_STATE_FILE="${TMP_DIR}/lark-loop.state.json"
LAST_PROCESSED_FILE="${TMP_DIR}/lark-loop.last"
LAST_VALIDATED_FILE="${TMP_DIR}/lark-loop.last-validated"
NOTICE_STATE_FILE="${LARK_NOTICE_STATE_FILE:-${TMP_DIR}/lark-notice.state.json}"
AUTOFIX_STATE_FILE="${TMP_DIR}/lark-autofix.state.json"
AUTOFIX_LOCK_DIR="${TMP_DIR}/lark-autofix.lock"
AUTOFIX_HISTORY_FILE="${TMP_DIR}/lark-autofix.history"
AUTOFIX_LAST_SIGNATURE_FILE="${TMP_DIR}/lark-autofix.last-signature"
AUTOFIX_APPLIED_INCIDENT_FILE="${TMP_DIR}/lark-autofix.applied"
CLEANUP_ORPHANS_SH="${MAIN_ROOT}/scripts/lark/cleanup_orphan_agents.sh"

MAIN_PID_FILE="${PID_DIR}/lark-main.pid"
LOOP_PID_FILE="${PID_DIR}/lark-loop.pid"
KERNEL_PID_FILE="${PID_DIR}/lark-kernel.pid"
CODEX_LOOP_PID_FILE="${PID_DIR}/lark-codex-loop.pid"
CODEX_AUTOFIX_PID_FILE="${PID_DIR}/lark-codex-autofix.pid"
MAIN_SHA_FILE="${PID_DIR}/lark-main.sha"
KERNEL_SHA_FILE="${PID_DIR}/lark-kernel.sha"

TICK_SECONDS="${LARK_SUPERVISOR_TICK_SECONDS:-5}"
RESTART_MAX_IN_WINDOW="${LARK_RESTART_MAX_IN_WINDOW:-5}"
RESTART_WINDOW_SECONDS="${LARK_RESTART_WINDOW_SECONDS:-600}"
COOLDOWN_SECONDS="${LARK_COOLDOWN_SECONDS:-300}"
AUTOFIX_ENABLED="${LARK_SUPERVISOR_AUTOFIX_ENABLED:-0}"
AUTOFIX_TRIGGER="${LARK_SUPERVISOR_AUTOFIX_TRIGGER:-cooldown}"
AUTOFIX_TIMEOUT_SECONDS="${LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS:-1800}"
AUTOFIX_MAX_IN_WINDOW="${LARK_SUPERVISOR_AUTOFIX_MAX_IN_WINDOW:-3}"
AUTOFIX_WINDOW_SECONDS="${LARK_SUPERVISOR_AUTOFIX_WINDOW_SECONDS:-3600}"
AUTOFIX_COOLDOWN_SECONDS="${LARK_SUPERVISOR_AUTOFIX_COOLDOWN_SECONDS:-900}"
AUTOFIX_SCOPE="${LARK_SUPERVISOR_AUTOFIX_SCOPE:-repo}"
LOOP_AUTOFIX_ENABLED="${LARK_LOOP_AUTOFIX_ENABLED:-0}"
STALE_LOOP_STATE_TIMEOUT_SECONDS="${LARK_STALE_LOOP_STATE_TIMEOUT_SECONDS:-30}"

MODE="degraded"
COOLDOWN_UNTIL=0
LAST_ERROR=""

MAIN_FAIL_COUNT=0
LOOP_FAIL_COUNT=0
KERNEL_FAIL_COUNT=0

MAIN_RESTART_HISTORY=""
LOOP_RESTART_HISTORY=""
KERNEL_RESTART_HISTORY=""

OBS_MAIN_PID=""
OBS_LOOP_PID=""
OBS_KERNEL_PID=""
OBS_CODEX_LOOP_PID=""
OBS_CODEX_AUTOFIX_PID=""
OBS_MAIN_HEALTH="down"
OBS_LOOP_HEALTH="down"
OBS_KERNEL_HEALTH="down"
OBS_MAIN_SHA="unknown"
OBS_MAIN_DEPLOYED_SHA="unknown"
OBS_KERNEL_DEPLOYED_SHA="unknown"
OBS_LAST_PROCESSED_SHA=""
OBS_CYCLE_PHASE="idle"
OBS_CYCLE_RESULT="unknown"
OBS_LOOP_ERROR=""
OBS_LAST_VALIDATED_SHA=""
OBS_RESTART_COUNT_WINDOW=0
OBS_AUTOFIX_STATE="idle"
OBS_AUTOFIX_INCIDENT_ID=""
OBS_AUTOFIX_LAST_REASON=""
OBS_AUTOFIX_LAST_STARTED_AT=""
OBS_AUTOFIX_LAST_FINISHED_AT=""
OBS_AUTOFIX_LAST_COMMIT=""
OBS_AUTOFIX_RESTART_REQUIRED="false"
OBS_AUTOFIX_RUNS_WINDOW=0
OBS_STALE_STATE_RECOVERED="false"
OBS_STALE_STATE_RECOVERED_AT=""
AUTOFIX_COOLDOWN_UNTIL=0

STALE_LOOP_STATE_SINCE_EPOCH=0

json_escape() {
  printf '%s' "${1:-}" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

VALIDATION_PHASES="validating fast_gate slow_gate promoting restoring"

is_validation_active() {
  local phase="${OBS_CYCLE_PHASE}" p
  for p in ${VALIDATION_PHASES}; do
    [[ "${phase}" == "${p}" ]] && return 0
  done
  return 1
}

ensure_dirs() {
  mkdir -p "${PID_DIR}" "${LOG_DIR}" "${TMP_DIR}"
}

append_log() {
  ensure_dirs
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" >> "${LOG_FILE}"
}

ensure_worktree() {
  lark_ensure_test_worktree "${MAIN_ROOT}" >/dev/null
  ensure_dirs
}

cleanup_orphan_lark_agents() {
  if [[ -x "${CLEANUP_ORPHANS_SH}" ]]; then
    "${CLEANUP_ORPHANS_SH}" cleanup --scope main --quiet || true
  fi
}

assert_main_config() {
  [[ -f "${MAIN_CONFIG_PATH}" ]] || die "Missing MAIN_CONFIG: ${MAIN_CONFIG_PATH}"
}

read_pid_if_running() {
  local pid_file="$1"
  local pid
  pid="$(read_pid "${pid_file}" || true)"
  if is_process_running "${pid}"; then
    echo "${pid}"
  fi
}

read_pid_value() {
  local pid_file="$1"
  read_pid "${pid_file}" 2>/dev/null || true
}

clear_pid_file_if_stale() {
  local pid_file="$1"
  local pid
  pid="$(read_pid_value "${pid_file}")"
  if [[ -n "${pid}" ]] && ! is_process_running "${pid}"; then
    rm -f "${pid_file}"
  fi
}

stop_tracked_process() {
  local label="$1"
  local pid_file="$2"
  local pid

  pid="$(read_pid_value "${pid_file}")"
  if [[ -z "${pid}" ]]; then
    rm -f "${pid_file}" 2>/dev/null || true
    return 0
  fi

  stop_pid "${pid}" "${label}" "${SIGTERM_GRACE_ATTEMPTS}" "${SIGTERM_GRACE_SLEEP}" >/dev/null 2>&1 || true
  rm -f "${pid_file}" 2>/dev/null || true
}

main_health_state() {
  local pid
  pid="$(read_pid_if_running "${MAIN_PID_FILE}")"
  if [[ -n "${pid}" ]]; then
    echo "healthy"
  else
    echo "down"
  fi
}

kernel_health_state() {
  local pid
  pid="$(read_pid_if_running "${KERNEL_PID_FILE}")"
  if [[ -n "${pid}" ]]; then
    echo "healthy"
  else
    echo "down"
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
      | sed -nE "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"([^\"]*)\".*/\\1/p"
  )"
  [[ -n "${out}" ]] || return 1
  printf '%s' "${out}"
}

extract_json_number() {
  local file="$1"
  local key="$2"
  local out
  [[ -f "${file}" ]] || return 1
  out="$(
    tr -d '\n' < "${file}" \
      | sed -nE "s/.*\"${key}\"[[:space:]]*:[[:space:]]*([0-9]+).*/\\1/p"
  )"
  [[ -n "${out}" ]] || return 1
  printf '%s' "${out}"
}

write_loop_state_recovery() {
  local reason="$1"
  local now_utc current_main_sha
  now_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  current_main_sha="$(git -C "${MAIN_ROOT}" rev-parse main 2>/dev/null || true)"

  local tmp_file
  tmp_file="${LOOP_STATE_FILE}.tmp.$$.$RANDOM"
  cat > "${tmp_file}" <<EOF
{
  "ts_utc": "${now_utc}",
  "base_sha": "",
  "cycle_phase": "idle",
  "cycle_result": "stale_recovered",
  "main_sha": "${current_main_sha}",
  "last_processed_sha": "${OBS_LAST_PROCESSED_SHA}",
  "last_validated_sha": "${OBS_LAST_VALIDATED_SHA}",
  "validating_sha": "",
  "last_error": "$(json_escape "${reason}")"
}
EOF
  mv "${tmp_file}" "${LOOP_STATE_FILE}"
}

reconcile_stale_loop_state() {
  local now_epoch now_utc reason elapsed
  now_epoch="$(date +%s)"

  if is_validation_active && [[ "${OBS_LOOP_HEALTH}" != "alive" ]]; then
    if (( STALE_LOOP_STATE_SINCE_EPOCH == 0 )); then
      STALE_LOOP_STATE_SINCE_EPOCH="${now_epoch}"
    fi
    elapsed=$((now_epoch - STALE_LOOP_STATE_SINCE_EPOCH))
    if (( elapsed >= STALE_LOOP_STATE_TIMEOUT_SECONDS )); then
      reason="loop process down while cycle_phase=${OBS_CYCLE_PHASE}; stale phase recovered"
      append_log "[supervisor] stale loop state detected; resetting cycle phase to idle (elapsed=${elapsed}s)"
      write_loop_state_recovery "${reason}"
      now_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      OBS_STALE_STATE_RECOVERED="true"
      OBS_STALE_STATE_RECOVERED_AT="${now_utc}"
      STALE_LOOP_STATE_SINCE_EPOCH=0
      OBS_CYCLE_PHASE="idle"
      OBS_CYCLE_RESULT="stale_recovered"
      OBS_LOOP_ERROR="${reason}"
    fi
    return 0
  fi

  STALE_LOOP_STATE_SINCE_EPOCH=0
  OBS_STALE_STATE_RECOVERED="false"
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

autofix_runs_window_count() {
  local now_epoch="$1"
  local cutoff_epoch count ts out
  cutoff_epoch=$((now_epoch - AUTOFIX_WINDOW_SECONDS))
  count=0
  out=""
  if [[ -f "${AUTOFIX_HISTORY_FILE}" ]]; then
    while IFS= read -r ts; do
      [[ -n "${ts}" ]] || continue
      if [[ "${ts}" =~ ^[0-9]+$ ]] && (( ts >= cutoff_epoch )); then
        out+="${ts}"$'\n'
        count=$((count + 1))
      fi
    done < "${AUTOFIX_HISTORY_FILE}"
  fi
  printf '%s' "${out}" > "${AUTOFIX_HISTORY_FILE}"
  echo "${count}"
}

record_autofix_run() {
  local now_epoch="$1"
  mkdir -p "${TMP_DIR}"
  printf '%s\n' "${now_epoch}" >> "${AUTOFIX_HISTORY_FILE}"
  autofix_runs_window_count "${now_epoch}"
}

observe_autofix_state() {
  OBS_AUTOFIX_STATE="$(extract_json_string "${AUTOFIX_STATE_FILE}" "autofix_state" || echo "idle")"
  OBS_AUTOFIX_INCIDENT_ID="$(extract_json_string "${AUTOFIX_STATE_FILE}" "autofix_incident_id" || true)"
  OBS_AUTOFIX_LAST_REASON="$(extract_json_string "${AUTOFIX_STATE_FILE}" "autofix_last_reason" || true)"
  OBS_AUTOFIX_LAST_STARTED_AT="$(extract_json_string "${AUTOFIX_STATE_FILE}" "autofix_last_started_at" || true)"
  OBS_AUTOFIX_LAST_FINISHED_AT="$(extract_json_string "${AUTOFIX_STATE_FILE}" "autofix_last_finished_at" || true)"
  OBS_AUTOFIX_LAST_COMMIT="$(extract_json_string "${AUTOFIX_STATE_FILE}" "autofix_last_commit" || true)"
  OBS_AUTOFIX_RESTART_REQUIRED="$(extract_json_string "${AUTOFIX_STATE_FILE}" "autofix_restart_required" || echo "false")"
  OBS_AUTOFIX_RUNS_WINDOW="$(autofix_runs_window_count "$(date +%s)")"
}

load_notice_chat_id() {
  extract_json_string "${NOTICE_STATE_FILE}" "chat_id" || true
}

send_lark_notice() {
  local text="$1"
  local chat_id
  chat_id="$(load_notice_chat_id)"
  if [[ -z "${chat_id}" ]]; then
    append_log "[notice] no bound chat_id; skip"
    return 0
  fi
  if [[ ! -x "${NOTIFY_SH}" ]]; then
    append_log "[notice] notify sender unavailable: ${NOTIFY_SH}"
    return 0
  fi

  if "${NOTIFY_SH}" send --chat-id "${chat_id}" --text "${text}" --config "${MAIN_CONFIG_PATH}" >> "${LOG_FILE}" 2>&1; then
    append_log "[notice] sent to chat_id=${chat_id}"
    return 0
  fi

  append_log "[notice] failed to send to chat_id=${chat_id}"
  return 0
}

build_transition_notice_text() {
  local previous_mode="$1"
  local current_mode="$2"
  local current_error
  current_error="${LAST_ERROR}"
  if [[ -z "${current_error}" ]]; then
    current_error="${OBS_LOOP_ERROR}"
  fi
  if [[ -z "${current_error}" ]]; then
    current_error="n/a"
  fi

  local status_line
  status_line="main=${OBS_MAIN_HEALTH} kernel=${OBS_KERNEL_HEALTH} loop=${OBS_LOOP_HEALTH}"
  local autofix_line
  autofix_line="state=${OBS_AUTOFIX_STATE} incident=${OBS_AUTOFIX_INCIDENT_ID:-none}"

  if [[ "${current_mode}" == "healthy" ]]; then
    printf '[lark-supervisor] recovered\nmode: %s -> %s\n%s\nautofix: %s\nlast_error: %s' \
      "${previous_mode}" "${current_mode}" "${status_line}" "${autofix_line}" "${current_error}"
    return 0
  fi

  printf '[lark-supervisor] degraded\nmode: %s -> %s\n%s\nautofix: %s\nlast_error: %s' \
    "${previous_mode}" "${current_mode}" "${status_line}" "${autofix_line}" "${current_error}"
}

maybe_notify_mode_transition() {
  local previous_mode="$1"
  local current_mode="$2"
  local text

  if [[ -z "${previous_mode}" || "${previous_mode}" == "${current_mode}" ]]; then
    return 0
  fi

  if [[ "${previous_mode}" == "healthy" && ( "${current_mode}" == "degraded" || "${current_mode}" == "cooldown" ) ]]; then
    text="$(build_transition_notice_text "${previous_mode}" "${current_mode}")"
    send_lark_notice "${text}" || true
    return 0
  fi
  if [[ ( "${previous_mode}" == "degraded" || "${previous_mode}" == "cooldown" ) && "${current_mode}" == "healthy" ]]; then
    text="$(build_transition_notice_text "${previous_mode}" "${current_mode}")"
    send_lark_notice "${text}" || true
  fi
}

trigger_autofix() {
  local component="$1"
  local reason="$2"
  local now_epoch signature previous_signature incident_id runs_in_window started_at

  if [[ "${AUTOFIX_ENABLED}" != "1" ]]; then
    return 0
  fi
  if [[ "${AUTOFIX_TRIGGER}" != "cooldown" ]]; then
    return 0
  fi
  if [[ ! -x "${AUTOFIX_SH}" ]]; then
    append_log "[autofix] disabled: missing executable ${AUTOFIX_SH}"
    return 0
  fi

  now_epoch="$(date +%s)"
  if (( now_epoch < AUTOFIX_COOLDOWN_UNTIL )); then
    OBS_AUTOFIX_STATE="cooldown"
    return 0
  fi

  runs_in_window="$(autofix_runs_window_count "${now_epoch}")"
  if (( runs_in_window >= AUTOFIX_MAX_IN_WINDOW )); then
    AUTOFIX_COOLDOWN_UNTIL=$((now_epoch + AUTOFIX_COOLDOWN_SECONDS))
    OBS_AUTOFIX_STATE="cooldown"
    OBS_AUTOFIX_RUNS_WINDOW="${runs_in_window}"
    append_log "[autofix] run limit reached (${runs_in_window}/${AUTOFIX_MAX_IN_WINDOW}), cooldown ${AUTOFIX_COOLDOWN_SECONDS}s"
    return 0
  fi

  signature="${component}|${reason}|${OBS_MAIN_SHA}"
  previous_signature="$(cat "${AUTOFIX_LAST_SIGNATURE_FILE}" 2>/dev/null || true)"
  if [[ "${signature}" == "${previous_signature}" ]]; then
    append_log "[autofix] duplicate signature, skip trigger"
    return 0
  fi

  if [[ -d "${AUTOFIX_LOCK_DIR}" ]]; then
    OBS_AUTOFIX_STATE="running"
    append_log "[autofix] existing autofix lock, skip trigger"
    return 0
  fi

  incident_id="afx-$(date -u +%Y%m%dT%H%M%SZ)-${component}-${OBS_MAIN_SHA:0:8}"
  started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '%s\n' "${signature}" > "${AUTOFIX_LAST_SIGNATURE_FILE}"
  runs_in_window="$(record_autofix_run "${now_epoch}")"

  OBS_AUTOFIX_STATE="running"
  OBS_AUTOFIX_INCIDENT_ID="${incident_id}"
  OBS_AUTOFIX_LAST_REASON="${reason}"
  OBS_AUTOFIX_LAST_STARTED_AT="${started_at}"
  OBS_AUTOFIX_LAST_FINISHED_AT=""
  OBS_AUTOFIX_LAST_COMMIT=""
  OBS_AUTOFIX_RESTART_REQUIRED="false"
  OBS_AUTOFIX_RUNS_WINDOW="${runs_in_window}"

  append_log "[autofix] trigger incident=${incident_id} reason=${reason}"
  nohup env \
    LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS="${AUTOFIX_TIMEOUT_SECONDS}" \
    LARK_SUPERVISOR_AUTOFIX_SCOPE="${AUTOFIX_SCOPE}" \
    "${AUTOFIX_SH}" trigger \
      --incident-id "${incident_id}" \
      --reason "${reason}" \
      --signature "${signature}" \
      --main-sha "${OBS_MAIN_SHA}" >> "${LOG_FILE}" 2>&1 &
}

handle_autofix_success_restart() {
  local applied_incident
  if [[ "${OBS_AUTOFIX_STATE}" != "succeeded" ]]; then
    return 0
  fi
  if [[ "${OBS_AUTOFIX_RESTART_REQUIRED}" != "true" ]]; then
    return 0
  fi
  if [[ -z "${OBS_AUTOFIX_INCIDENT_ID}" ]]; then
    return 0
  fi

  applied_incident="$(cat "${AUTOFIX_APPLIED_INCIDENT_FILE}" 2>/dev/null || true)"
  if [[ "${applied_incident}" == "${OBS_AUTOFIX_INCIDENT_ID}" ]]; then
    return 0
  fi

  append_log "[autofix] applying post-success restart for incident=${OBS_AUTOFIX_INCIDENT_ID}"
  restart_component "kernel" || true
  restart_component "main" || true
  restart_component "loop" || true
  printf '%s\n' "${OBS_AUTOFIX_INCIDENT_ID}" > "${AUTOFIX_APPLIED_INCIDENT_FILE}"
}

restart_history_for_component() {
  local component="$1"
  case "${component}" in
    main) echo "${MAIN_RESTART_HISTORY}" ;;
    loop) echo "${LOOP_RESTART_HISTORY}" ;;
    kernel) echo "${KERNEL_RESTART_HISTORY}" ;;
    *) echo "" ;;
  esac
}

set_restart_history_for_component() {
  local component="$1"
  local history="$2"
  case "${component}" in
    main) MAIN_RESTART_HISTORY="${history}" ;;
    loop) LOOP_RESTART_HISTORY="${history}" ;;
    kernel) KERNEL_RESTART_HISTORY="${history}" ;;
  esac
}

fail_count_for_component() {
  local component="$1"
  case "${component}" in
    main) echo "${MAIN_FAIL_COUNT}" ;;
    loop) echo "${LOOP_FAIL_COUNT}" ;;
    kernel) echo "${KERNEL_FAIL_COUNT}" ;;
    *) echo "0" ;;
  esac
}

set_fail_count_for_component() {
  local component="$1"
  local value="$2"
  case "${component}" in
    main) MAIN_FAIL_COUNT="${value}" ;;
    loop) LOOP_FAIL_COUNT="${value}" ;;
    kernel) KERNEL_FAIL_COUNT="${value}" ;;
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
  local main_count loop_count kernel_count
  main_count="$(restart_count_window_for_component "main" "${now_epoch}")"
  loop_count="$(restart_count_window_for_component "loop" "${now_epoch}")"
  kernel_count="$(restart_count_window_for_component "kernel" "${now_epoch}")"
  echo $((main_count + loop_count + kernel_count))
}

restart_count_window_for_component() {
  local component="$1"
  local now_epoch="$2"
  local history
  history="$(restart_history_for_component "${component}")"
  history="$(history_prune "${history}" "${now_epoch}")"
  set_restart_history_for_component "${component}" "${history}"
  history_count "${history}"
}

restart_component() {
  local component="$1"
  case "${component}" in
    main)
      "${MAIN_SH}" restart >> "${LOG_FILE}" 2>&1
      ;;
    loop)
      "${LOOP_AGENT_SH}" restart >> "${LOG_FILE}" 2>&1
      ;;
    kernel)
      "${KERNEL_SH}" restart >> "${LOG_FILE}" 2>&1
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
    main|kernel)
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

component_pid_file() {
  local component="$1"
  case "${component}" in
    main) echo "${MAIN_PID_FILE}" ;;
    kernel) echo "${KERNEL_PID_FILE}" ;;
    loop) echo "${LOOP_PID_FILE}" ;;
    *) return 1 ;;
  esac
}

append_restart_diagnostics() {
  local component="$1"
  local state="$2"
  local pid_file pid cmd

  pid_file="$(component_pid_file "${component}" 2>/dev/null || true)"
  [[ -n "${pid_file}" ]] || return 0

  pid="$(read_pid_value "${pid_file}")"
  if [[ -z "${pid}" ]]; then
    append_log "[health] ${component}=${state} reason=pid_missing pid_file=${pid_file}"
    return 0
  fi

  if is_process_running "${pid}"; then
    cmd="$(ps -p "${pid}" -o args= 2>/dev/null || true)"
    if [[ -n "${cmd}" ]]; then
      append_log "[health] ${component}=${state} reason=state_mismatch pid=${pid} cmd=${cmd}"
    else
      append_log "[health] ${component}=${state} reason=state_mismatch pid=${pid}"
    fi
    return 0
  fi

  append_log "[health] ${component}=${state} reason=stale_pid pid=${pid} pid_file=${pid_file}"
}

observe_states() {
  clear_pid_file_if_stale "${CODEX_LOOP_PID_FILE}"
  clear_pid_file_if_stale "${CODEX_AUTOFIX_PID_FILE}"

  OBS_MAIN_PID="$(read_pid_if_running "${MAIN_PID_FILE}" || true)"
  OBS_LOOP_PID="$(read_pid_if_running "${LOOP_PID_FILE}" || true)"
  OBS_KERNEL_PID="$(read_pid_if_running "${KERNEL_PID_FILE}" || true)"
  OBS_CODEX_LOOP_PID="$(read_pid_if_running "${CODEX_LOOP_PID_FILE}" || true)"
  OBS_CODEX_AUTOFIX_PID="$(read_pid_if_running "${CODEX_AUTOFIX_PID_FILE}" || true)"

  OBS_MAIN_HEALTH="$(main_health_state)"
  OBS_KERNEL_HEALTH="$(kernel_health_state)"
  OBS_LOOP_HEALTH="$(loop_health_state)"

  OBS_MAIN_SHA="$(git -C "${MAIN_ROOT}" rev-parse main 2>/dev/null || echo "unknown")"
  OBS_MAIN_DEPLOYED_SHA="$(cat "${MAIN_SHA_FILE}" 2>/dev/null || echo "unknown")"
  OBS_KERNEL_DEPLOYED_SHA="$(cat "${KERNEL_SHA_FILE}" 2>/dev/null || echo "unknown")"
  OBS_LAST_PROCESSED_SHA="$(cat "${LAST_PROCESSED_FILE}" 2>/dev/null || true)"

  OBS_CYCLE_PHASE="$(extract_json_string "${LOOP_STATE_FILE}" "cycle_phase" || echo "idle")"
  OBS_CYCLE_RESULT="$(extract_json_string "${LOOP_STATE_FILE}" "cycle_result" || echo "unknown")"
  OBS_LOOP_ERROR="$(extract_json_string "${LOOP_STATE_FILE}" "last_error" || true)"
  OBS_LAST_VALIDATED_SHA="$(cat "${LAST_VALIDATED_FILE}" 2>/dev/null || true)"
  observe_autofix_state
}

current_mode() {
  local now_epoch="$1"
  if (( now_epoch < COOLDOWN_UNTIL )); then
    echo "cooldown"
    return
  fi
  if is_validation_active && [[ "${OBS_MAIN_HEALTH}" == "healthy" && "${OBS_KERNEL_HEALTH}" == "healthy" && "${OBS_LOOP_HEALTH}" == "alive" ]]; then
    echo "validating"
    return
  fi
  if [[ "${OBS_MAIN_HEALTH}" == "healthy" && "${OBS_KERNEL_HEALTH}" == "healthy" && "${OBS_LOOP_HEALTH}" == "alive" ]]; then
    echo "healthy"
  else
    echo "degraded"
  fi
}

write_status_file() {
  local now_epoch now_utc mode last_error restart_count loop_alive
  local main_runs_window kernel_runs_window loop_runs_window
  now_epoch="$(date +%s)"
  now_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  restart_count="$(total_restart_count_window "${now_epoch}")"
  main_runs_window="$(restart_count_window_for_component "main" "${now_epoch}")"
  kernel_runs_window="$(restart_count_window_for_component "kernel" "${now_epoch}")"
  loop_runs_window="$(restart_count_window_for_component "loop" "${now_epoch}")"
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
  tmp_file="${STATUS_FILE}.tmp.$$.$RANDOM"
  cat > "${tmp_file}" <<EOF
{
  "ts_utc": "${now_utc}",
  "mode": "${MODE}",
  "main_pid": "${OBS_MAIN_PID}",
  "kernel_pid": "${OBS_KERNEL_PID}",
  "loop_pid": "${OBS_LOOP_PID}",
  "codex_loop_pid": "${OBS_CODEX_LOOP_PID}",
  "codex_autofix_pid": "${OBS_CODEX_AUTOFIX_PID}",
  "main_health": "${OBS_MAIN_HEALTH}",
  "kernel_health": "${OBS_KERNEL_HEALTH}",
  "loop_alive": ${loop_alive},
  "main_sha": "${OBS_MAIN_SHA}",
  "main_deployed_sha": "${OBS_MAIN_DEPLOYED_SHA}",
  "main_runs_window": ${main_runs_window},
  "kernel_deployed_sha": "${OBS_KERNEL_DEPLOYED_SHA}",
  "kernel_runs_window": ${kernel_runs_window},
  "loop_runs_window": ${loop_runs_window},
  "last_processed_sha": "${OBS_LAST_PROCESSED_SHA}",
  "last_validated_sha": "${OBS_LAST_VALIDATED_SHA}",
  "cycle_phase": "${OBS_CYCLE_PHASE}",
  "cycle_result": "${OBS_CYCLE_RESULT}",
  "loop_autofix_enabled": "${LOOP_AUTOFIX_ENABLED}",
  "last_error": "$(json_escape "${last_error}")",
  "restart_count_window": ${restart_count},
  "autofix_state": "${OBS_AUTOFIX_STATE}",
  "autofix_incident_id": "$(json_escape "${OBS_AUTOFIX_INCIDENT_ID}")",
  "autofix_last_reason": "$(json_escape "${OBS_AUTOFIX_LAST_REASON}")",
  "autofix_last_started_at": "${OBS_AUTOFIX_LAST_STARTED_AT}",
  "autofix_last_finished_at": "${OBS_AUTOFIX_LAST_FINISHED_AT}",
  "autofix_last_commit": "${OBS_AUTOFIX_LAST_COMMIT}",
  "autofix_runs_window": ${OBS_AUTOFIX_RUNS_WINDOW},
  "stale_state_recovered": ${OBS_STALE_STATE_RECOVERED},
  "stale_state_recovered_at": "${OBS_STALE_STATE_RECOVERED_AT}"
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
  trigger_autofix "${component}" "${LAST_ERROR}" || true
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

  append_restart_diagnostics "${component}" "${state}"
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

maybe_upgrade_for_sha_drift() {
  # Auto-restart healthy components whose deployed SHA differs from the latest
  # main SHA. This ensures code changes are picked up without manual intervention.
  local now_epoch
  now_epoch="$(date +%s)"

  # Skip during cooldown.
  if (( now_epoch < COOLDOWN_UNTIL )); then
    return 0
  fi

  # Keep runtime components that should track main SHA aligned.
  if [[ "${OBS_MAIN_HEALTH}" == "healthy" \
     && "${OBS_MAIN_DEPLOYED_SHA}" != "unknown" \
     && "${OBS_MAIN_SHA}" != "unknown" \
     && "${OBS_MAIN_DEPLOYED_SHA}" != "${OBS_MAIN_SHA}" ]]; then
    local count
    count="$(record_restart_attempt "main" "${now_epoch}")"
    if (( count > RESTART_MAX_IN_WINDOW )); then
      set_cooldown "main" "${count}"
      return 0
    fi
    append_log "[upgrade] main deployed=${OBS_MAIN_DEPLOYED_SHA:0:8} latest=${OBS_MAIN_SHA:0:8}; restarting"
    if restart_component "main"; then
      append_log "[upgrade] main restart success"
    else
      append_log "[upgrade] main restart failed"
    fi
    observe_states
  fi

  if [[ "${OBS_KERNEL_HEALTH}" == "healthy" \
     && "${OBS_KERNEL_DEPLOYED_SHA}" != "unknown" \
     && "${OBS_MAIN_SHA}" != "unknown" \
     && "${OBS_KERNEL_DEPLOYED_SHA}" != "${OBS_MAIN_SHA}" ]]; then
    local count
    count="$(record_restart_attempt "kernel" "${now_epoch}")"
    if (( count > RESTART_MAX_IN_WINDOW )); then
      set_cooldown "kernel" "${count}"
      return 0
    fi
    append_log "[upgrade] kernel deployed=${OBS_KERNEL_DEPLOYED_SHA:0:8} latest=${OBS_MAIN_SHA:0:8}; restarting"
    if restart_component "kernel"; then
      append_log "[upgrade] kernel restart success"
    else
      append_log "[upgrade] kernel restart failed"
    fi
    observe_states
  fi
}

run_tick() {
  local previous_mode
  previous_mode="$(extract_json_string "${STATUS_FILE}" "mode" || true)"

  cleanup_orphan_lark_agents
  observe_states
  reconcile_stale_loop_state
  observe_states

  restart_with_backoff "kernel" "${OBS_KERNEL_HEALTH}" || true
  observe_states

  restart_with_backoff "main" "${OBS_MAIN_HEALTH}" || true
  observe_states
  restart_with_backoff "loop" "${OBS_LOOP_HEALTH}" || true
  observe_states
  maybe_upgrade_for_sha_drift
  handle_autofix_success_restart
  observe_states

  write_status_file
  maybe_notify_mode_transition "${previous_mode}" "${MODE}"
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
  assert_main_config
  cleanup_orphan_lark_agents

  if ! acquire_lock; then
    die "Supervisor already running (lock: ${LOCK_DIR})"
  fi
  trap cleanup EXIT INT TERM
  echo "$$" > "${PID_FILE}"
  append_log "[supervisor] start tick=${TICK_SECONDS}s window=${RESTART_WINDOW_SECONDS}s max=${RESTART_MAX_IN_WINDOW} cooldown=${COOLDOWN_SECONDS}s"
  append_log "[supervisor] pid_dir=${PID_DIR}"
  append_log "[supervisor] main_config=$(lark_canonical_path "${MAIN_CONFIG_PATH}")"
  append_log "[supervisor] autofix enabled=${AUTOFIX_ENABLED} trigger=${AUTOFIX_TRIGGER} timeout=${AUTOFIX_TIMEOUT_SECONDS}s max=${AUTOFIX_MAX_IN_WINDOW}/${AUTOFIX_WINDOW_SECONDS}s cooldown=${AUTOFIX_COOLDOWN_SECONDS}s scope=${AUTOFIX_SCOPE}"
  append_log "[supervisor] loop_autofix_enabled=${LOOP_AUTOFIX_ENABLED}"

  while true; do
    run_tick
    sleep "${TICK_SECONDS}"
  done
}

is_supervisor_process() {
  local pid="$1"
  [[ -n "${pid}" ]] || return 1
  local cmd
  cmd="$(ps -p "${pid}" -o args= 2>/dev/null || true)"
  [[ "${cmd}" == *"supervisor.sh run"* ]]
}

clean_stale_supervisor() {
  local pid="$1"
  log_warn "Stale supervisor PID ${pid}; cleaning up"
  rm -f "${PID_FILE}"
  rm -rf "${LOCK_DIR}" 2>/dev/null || true
}

report_children_health() {
  observe_states
  local degraded=0

  if [[ "${OBS_MAIN_HEALTH}" != "healthy" ]]; then
    log_warn "  main: ${OBS_MAIN_HEALTH} (pid=${OBS_MAIN_PID:-none})"
    degraded=1
  else
    log_success "  main: healthy (pid=${OBS_MAIN_PID})"
  fi

  if [[ "${OBS_KERNEL_HEALTH}" != "healthy" ]]; then
    log_warn "  kernel: ${OBS_KERNEL_HEALTH} (pid=${OBS_KERNEL_PID:-none})"
    degraded=1
  else
    log_success "  kernel: healthy (pid=${OBS_KERNEL_PID})"
  fi
  if [[ "${OBS_LOOP_HEALTH}" != "alive" ]]; then
    log_warn "  loop: ${OBS_LOOP_HEALTH} (pid=${OBS_LOOP_PID:-none})"
    degraded=1
  else
    log_success "  loop: alive (pid=${OBS_LOOP_PID})"
  fi

  echo "  main  deployed: ${OBS_MAIN_DEPLOYED_SHA:0:8}  latest: ${OBS_MAIN_SHA:0:8}"
  if [[ "${OBS_MAIN_DEPLOYED_SHA:0:8}" != "${OBS_MAIN_SHA:0:8}" ]] && [[ "${OBS_MAIN_HEALTH}" == "healthy" ]]; then
    log_warn "  main is behind latest — will auto-upgrade on next tick"
  fi

  echo "  pid_dir: ${PID_DIR}"
  echo "  codex loop pid: ${OBS_CODEX_LOOP_PID:-none}"
  echo "  codex autofix pid: ${OBS_CODEX_AUTOFIX_PID:-none}"
  echo "  main config: $(lark_canonical_path "${MAIN_CONFIG_PATH}")"
  echo "  loop autofix: ${LOOP_AUTOFIX_ENABLED}"

  if (( degraded )); then
    log_warn "Supervisor is running but some components are down (use './lark.sh logs' to investigate)"
    return 1
  fi
  return 0
}

reconcile_children_once() {
  observe_states
  append_log "[start] reconcile-on-demand begin main=${OBS_MAIN_HEALTH} kernel=${OBS_KERNEL_HEALTH} loop=${OBS_LOOP_HEALTH}"

  if component_needs_restart "kernel" "${OBS_KERNEL_HEALTH}"; then
    append_log "[start] kernel=${OBS_KERNEL_HEALTH}; restarting"
    restart_component "kernel" || append_log "[start] kernel restart failed"
    observe_states
  fi

  if component_needs_restart "main" "${OBS_MAIN_HEALTH}"; then
    append_log "[start] main=${OBS_MAIN_HEALTH}; restarting"
    restart_component "main" || append_log "[start] main restart failed"
    observe_states
  fi

  if component_needs_restart "loop" "${OBS_LOOP_HEALTH}"; then
    append_log "[start] loop=${OBS_LOOP_HEALTH}; restarting"
    restart_component "loop" || append_log "[start] loop restart failed"
  fi
  observe_states

  maybe_upgrade_for_sha_drift
  observe_states
  write_status_file
  append_log "[start] reconcile-on-demand end main=${OBS_MAIN_HEALTH} kernel=${OBS_KERNEL_HEALTH} loop=${OBS_LOOP_HEALTH}"
}

start() {
  ensure_worktree
  ensure_dirs
  assert_main_config
  cleanup_orphan_lark_agents

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"

  if [[ -n "${pid}" ]] && is_process_running "${pid}"; then
    if ! is_supervisor_process "${pid}"; then
      clean_stale_supervisor "${pid}"
    else
      log_success "Supervisor already running (PID: ${pid})"
      log_info "Reconciling child components once..."
      reconcile_children_once || true
      report_children_health || true
      return 0
    fi
  elif [[ -n "${pid}" ]]; then
    clean_stale_supervisor "${pid}"
  fi

  # Clean orphaned lock from a previous unclean exit
  if [[ -d "${LOCK_DIR}" ]]; then
    local lock_pid
    lock_pid="$(awk -F= '/^pid=/{print $2}' "${LOCK_DIR}/owner" 2>/dev/null || true)"
    if [[ -z "${lock_pid}" ]] || ! is_process_running "${lock_pid}"; then
      log_warn "Removing orphaned lock (previous pid=${lock_pid:-unknown})"
      rm -rf "${LOCK_DIR}"
    fi
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
  "${MAIN_SH}" stop >> "${LOG_FILE}" 2>&1 || true
  "${KERNEL_SH}" stop >> "${LOG_FILE}" 2>&1 || true
}

stop_codex_processes() {
  stop_tracked_process "Loop auto-fix codex process" "${CODEX_LOOP_PID_FILE}"
  stop_tracked_process "Supervisor autofix codex process" "${CODEX_AUTOFIX_PID_FILE}"
}

stop() {
  ensure_worktree
  ensure_dirs
  stop_service "Lark supervisor" "${PID_FILE}" >> "${LOG_FILE}" 2>&1 || true
  stop_components
  # Final sweep after component shutdown to prevent loop/autofix races from leaving codex subprocesses behind.
  stop_codex_processes
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
  echo "pid_dir: ${PID_DIR}"
  echo "main_config: $(lark_canonical_path "${MAIN_CONFIG_PATH}")"
  echo "main: ${OBS_MAIN_HEALTH} pid=${OBS_MAIN_PID}"
  echo "kernel: ${OBS_KERNEL_HEALTH} pid=${OBS_KERNEL_PID}"
  echo "loop: ${OBS_LOOP_HEALTH} pid=${OBS_LOOP_PID}"
  echo "codex_loop_pid: ${OBS_CODEX_LOOP_PID}"
  echo "codex_autofix_pid: ${OBS_CODEX_AUTOFIX_PID}"
  echo "main_sha: ${OBS_MAIN_SHA}"
  echo "main_deployed_sha: ${OBS_MAIN_DEPLOYED_SHA}"
  echo "last_processed_sha: ${OBS_LAST_PROCESSED_SHA}"
  echo "last_validated_sha: ${OBS_LAST_VALIDATED_SHA}"
  echo "cycle_phase: ${OBS_CYCLE_PHASE}"
  echo "cycle_result: ${OBS_CYCLE_RESULT}"
  echo "loop_autofix_enabled: ${LOOP_AUTOFIX_ENABLED}"
  echo "restart_count_window: ${OBS_RESTART_COUNT_WINDOW}"
  echo "autofix_state: ${OBS_AUTOFIX_STATE}"
  echo "autofix_incident_id: ${OBS_AUTOFIX_INCIDENT_ID}"
  echo "autofix_last_reason: ${OBS_AUTOFIX_LAST_REASON}"
  echo "autofix_last_commit: ${OBS_AUTOFIX_LAST_COMMIT}"
  echo "autofix_runs_window: ${OBS_AUTOFIX_RUNS_WINDOW}"
  echo "stale_state_recovered: ${OBS_STALE_STATE_RECOVERED}"
  echo "stale_state_recovered_at: ${OBS_STALE_STATE_RECOVERED_AT}"
  echo "status_file: ${STATUS_FILE}"
}

logs() {
  ensure_worktree
  ensure_dirs
  touch \
    "${LOG_FILE}" \
    "${MAIN_ROOT}/logs/lark-main.log" \
    "${MAIN_ROOT}/logs/lark-kernel.log" \
    "${TEST_ROOT}/logs/lark-loop.log" \
    "${TEST_ROOT}/logs/lark-loop-agent.log"
  tail -n 200 -f \
    "${LOG_FILE}" \
    "${MAIN_ROOT}/logs/lark-main.log" \
    "${MAIN_ROOT}/logs/lark-kernel.log" \
    "${TEST_ROOT}/logs/lark-loop.log" \
    "${TEST_ROOT}/logs/lark-loop-agent.log"
}

doctor() {
  local failures warnings pid
  failures=0
  warnings=0

  echo "== lark doctor =="
  echo "main_root: ${MAIN_ROOT}"
  echo "test_root: ${TEST_ROOT}"
  echo "pid_dir: ${PID_DIR}"

  for cmd in git go; do
    if command_exists "${cmd}"; then
      echo "[ok] command: ${cmd}"
    else
      echo "[fail] missing command: ${cmd}"
      failures=$((failures + 1))
    fi
  done

  for script in "${MAIN_SH}" "${LOOP_AGENT_SH}" "${KERNEL_SH}" "${AUTOFIX_SH}"; do
    if [[ -x "${script}" ]]; then
      echo "[ok] script: ${script}"
    else
      echo "[fail] missing/non-executable script: ${script}"
      failures=$((failures + 1))
    fi
  done

  if [[ "${AUTOFIX_ENABLED}" == "1" ]]; then
    if command_exists codex; then
      echo "[ok] command: codex"
    else
      echo "[warn] codex not found in PATH (autofix may fail)"
      warnings=$((warnings + 1))
    fi
  fi

  if lark_test_worktree_registered "${MAIN_ROOT}" "${TEST_ROOT}"; then
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

  for pid_file in "${PID_FILE}" "${MAIN_PID_FILE}" "${KERNEL_PID_FILE}" "${LOOP_PID_FILE}" "${CODEX_LOOP_PID_FILE}" "${CODEX_AUTOFIX_PID_FILE}"; do
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

  echo "summary: failures=${failures} warnings=${warnings}"
  if (( failures > 0 )); then
    return 1
  fi
}

run_once() {
  ensure_worktree
  ensure_dirs
  assert_main_config
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
