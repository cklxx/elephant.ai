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
  scripts/lark/main.sh start|stop|restart|status|logs|build

Env:
  MAIN_CONFIG   Config path (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
  MAIN_PORT     Healthcheck port (default: 8080)
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

BIN="${ROOT}/alex-server"
PID_FILE="${ROOT}/.pids/lark-main.pid"
LOG_FILE="${ROOT}/logs/lark-main.log"
MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
MAIN_PORT="${MAIN_PORT:-8080}"
HEALTH_URL="http://127.0.0.1:${MAIN_PORT}/health"

mkdir -p "${ROOT}/.pids" "${ROOT}/logs"

build() {
  log_info "Building alex-server (main)..."
  (cd "${ROOT}" && CGO_ENABLED=0 go build -o "${BIN}" ./cmd/alex-server)
  log_success "Built ${BIN}"
}

start() {
  [[ -f "${MAIN_CONFIG}" ]] || die "Missing MAIN_CONFIG: ${MAIN_CONFIG}"

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Main agent already running (PID: ${pid})"
    return 0
  fi

  build
  log_info "Starting main agent..."
  ALEX_CONFIG_PATH="${MAIN_CONFIG}" nohup "${BIN}" >> "${LOG_FILE}" 2>&1 &
  echo "$!" > "${PID_FILE}"

  local i
  for i in $(seq 1 30); do
    if curl -sf "${HEALTH_URL}" >/dev/null 2>&1; then
      log_success "Main agent healthy: ${HEALTH_URL}"
      return 0
    fi
    sleep 1
  done

  log_error "Main agent failed to become healthy within 30s (see ${LOG_FILE})"
  return 1
}

stop() {
  stop_service "Main agent" "${PID_FILE}"
}

status() {
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Main agent running (PID: ${pid}) ${HEALTH_URL}"
  else
    log_warn "Main agent not running"
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
  build) build ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac

