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

PID_FILE="${ROOT}/.pids/lark-loop.pid"
LOG_FILE="${ROOT}/logs/lark-loop-agent.log"
LOOP_SH="${ROOT}/scripts/lark/loop.sh"

mkdir -p "${ROOT}/.pids" "${ROOT}/logs"

start() {
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
  stop_service "Loop agent" "${PID_FILE}"
}

status() {
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
  logs) tail -n 200 -f "${LOG_FILE}" ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac

