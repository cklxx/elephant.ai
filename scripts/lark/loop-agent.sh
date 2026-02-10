#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/loop-agent.sh start|stop|restart|status|logs

Behavior:
  - Manages the local self-heal loop process (scripts/lark/loop.sh watch)
EOF
}

git_worktree_path_for_branch() {
  local want_branch_ref="$1" # e.g. refs/heads/main
  git worktree list --porcelain | awk -v want="${want_branch_ref}" '
    $1=="worktree"{p=$2}
    $1=="branch" && $2==want {print p; exit}
  '
}

ROOT="$(git_worktree_path_for_branch "refs/heads/main" || true)"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
fi
[[ -n "${ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

WORKTREE_SH="${ROOT}/scripts/lark/worktree.sh"
TEST_ROOT="${ROOT}/.worktrees/test"
MAIN_CONFIG_PATH="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG_PATH}")"

PID_FILE="${PID_DIR}/lark-loop.pid"
LOG_FILE="${TEST_ROOT}/logs/lark-loop-agent.log"
LOOP_SH="${ROOT}/scripts/lark/loop.sh"

ensure_worktree() {
  [[ -x "${WORKTREE_SH}" ]] || die "Missing ${WORKTREE_SH}"
  "${WORKTREE_SH}" ensure >/dev/null
}

start() {
  ensure_worktree
  mkdir -p "${PID_DIR}" "${TEST_ROOT}/logs"

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Loop agent already running (PID: ${pid})"
    return 0
  fi

  [[ -x "${LOOP_SH}" ]] || die "Missing ${LOOP_SH}"

  log_info "Starting loop agent (watch)..."
  nohup "${LOOP_SH}" watch >> "${LOG_FILE}" 2>&1 &
  echo "$!" > "${PID_FILE}"
  log_success "Loop agent started (PID: $!)"
}

stop() {
  ensure_worktree
  stop_service "Loop agent" "${PID_FILE}"
}

status() {
  ensure_worktree
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Loop agent running (PID: ${pid})"
  else
    log_warn "Loop agent not running"
  fi
}

cmd="${1:-start}"
shift || true

case "${cmd}" in
  start) start ;;
  stop) stop ;;
  restart) stop; start ;;
  status) status ;;
  logs)
    ensure_worktree
    tail -n 200 -f "${LOG_FILE}"
    ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac
