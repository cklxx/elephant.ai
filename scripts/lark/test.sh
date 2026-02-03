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
  scripts/lark/test.sh start|stop|restart|status|logs|build

Behavior:
  - Ensures local auth DB is running (docker, migrations)
  - Ensures persistent test worktree exists at .worktrees/test and syncs .env
  - Builds and starts alex-server from the test worktree

Env:
  TEST_CONFIG          Config path (default: ~/.alex/test.yaml)
  TEST_PORT            Healthcheck port override (default: 8080)
  SKIP_LOCAL_AUTH_DB=1 Skip local auth DB auto-setup (default: 0)
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
SETUP_DB_SH="${ROOT}/scripts/setup_local_auth_db.sh"

TEST_ROOT="${ROOT}/.worktrees/test"
BIN="${TEST_ROOT}/alex-server"
PID_FILE="${ROOT}/.pids/lark-test.pid"
LOG_FILE="${ROOT}/logs/lark-test.log"
TEST_CONFIG="${TEST_CONFIG:-$HOME/.alex/test.yaml}"
TEST_PORT="${TEST_PORT:-8080}"
HEALTH_URL="http://127.0.0.1:${TEST_PORT}/health"

mkdir -p "${ROOT}/.pids" "${ROOT}/logs"

maybe_setup_auth_db() {
  if [[ "${SKIP_LOCAL_AUTH_DB:-0}" == "1" ]]; then
    log_info "Skipping local auth DB auto-setup (SKIP_LOCAL_AUTH_DB=1)"
    return 0
  fi

  if [[ -x "${SETUP_DB_SH}" ]]; then
    log_info "Ensuring local auth DB is ready..."
    "${SETUP_DB_SH}"
    return 0
  fi

  log_warn "Missing ${SETUP_DB_SH}; skipping DB setup"
  return 0
}

ensure_worktree() {
  [[ -x "${WORKTREE_SH}" ]] || die "Missing ${WORKTREE_SH}"
  "${WORKTREE_SH}" ensure
}

build() {
  ensure_worktree
  log_info "Building alex-server (test worktree)..."
  (cd "${TEST_ROOT}" && CGO_ENABLED=0 go build -o "${BIN}" ./cmd/alex-server)
  log_success "Built ${BIN}"
}

start() {
  [[ -f "${TEST_CONFIG}" ]] || die "Missing TEST_CONFIG: ${TEST_CONFIG}"

  maybe_setup_auth_db

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Test server already running (PID: ${pid})"
    return 0
  fi

  build
  log_info "Starting test server..."
  (cd "${TEST_ROOT}" && ALEX_CONFIG_PATH="${TEST_CONFIG}" nohup "${BIN}" >> "${LOG_FILE}" 2>&1 & echo "$!" > "${PID_FILE}")

  pid="$(read_pid "${PID_FILE}" || true)"
  local i
  for i in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      log_error "Test server exited early (see ${LOG_FILE})"
      return 1
    fi
    if curl -sf "${HEALTH_URL}" >/dev/null 2>&1; then
      log_success "Test server healthy: ${HEALTH_URL}"
      return 0
    fi
    sleep 1
  done

  log_error "Test server failed to become healthy within 30s (see ${LOG_FILE})"
  return 1
}

stop() {
  stop_service "Test server" "${PID_FILE}"
}

status() {
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Test server running (PID: ${pid}) ${HEALTH_URL}"
  else
    log_warn "Test server not running"
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

