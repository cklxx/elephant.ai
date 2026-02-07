#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=../lib/common/build.sh
source "${SCRIPT_DIR}/../lib/common/build.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/test.sh start|stop|restart|status|logs|build

Runs alex-server in standalone Lark WebSocket mode from the test worktree.

Env:
  TEST_CONFIG      Config path (default: ~/.alex/test.yaml)
  ALEX_LOG_DIR     Internal log dir override (default: <repo>/.worktrees/test/logs)
  FORCE_REBUILD=1  Force rebuild on start (default: 1)
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
BIN="${TEST_ROOT}/alex-server"
PID_FILE="${TEST_ROOT}/.pids/lark-test.pid"
BUILD_STAMP="${TEST_ROOT}/.pids/lark-test.build"
LOG_FILE="${TEST_ROOT}/logs/lark-test.log"
TEST_CONFIG="${TEST_CONFIG:-$HOME/.alex/test.yaml}"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${TEST_ROOT}/logs}"
FORCE_REBUILD="${FORCE_REBUILD:-1}"

# Readiness: grep for this log line to confirm the gateway has started.
READY_LOG_PATTERN="Lark gateway connecting"

ensure_worktree() {
  [[ -x "${WORKTREE_SH}" ]] || die "Missing ${WORKTREE_SH}"
  "${WORKTREE_SH}" ensure
  mkdir -p "${TEST_ROOT}/.pids" "${TEST_ROOT}/logs"
}

build() {
  ensure_worktree
  git -C "${TEST_ROOT}" switch test >/dev/null 2>&1 || true
  log_info "Building alex-server (test worktree)..."
  (cd "${TEST_ROOT}" && CGO_ENABLED=0 go build -o "${BIN}" ./cmd/alex-server)
  write_build_stamp "${BUILD_STAMP}" "$(build_fingerprint "${TEST_ROOT}")"
  git -C "${TEST_ROOT}" rev-parse HEAD > "${TEST_ROOT}/.pids/lark-test.sha" 2>/dev/null || true
  log_success "Built ${BIN}"
}

start() {
  [[ -f "${TEST_CONFIG}" ]] || die "Missing TEST_CONFIG: ${TEST_CONFIG}"

  ensure_worktree

  local current_fingerprint needs_build pid
  current_fingerprint="$(build_fingerprint "${TEST_ROOT}")"
  needs_build=0
  if [[ "${FORCE_REBUILD}" == "1" ]] || [[ ! -x "${BIN}" ]] || is_build_stale "${BUILD_STAMP}" "${current_fingerprint}"; then
    needs_build=1
  fi

  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    if [[ "${needs_build}" == "0" ]]; then
      log_success "Test Lark agent already running (PID: ${pid})"
      return 0
    fi
    log_info "Source changes detected; rebuilding and restarting test agent..."
    build
    needs_build=0
    stop
  fi

  if [[ "${needs_build}" == "1" ]]; then
    build
  else
    log_info "Reusing existing build (no changes detected)."
  fi

  log_info "Starting test Lark standalone agent..."
  (cd "${TEST_ROOT}" && ALEX_CONFIG_PATH="${TEST_CONFIG}" ALEX_LOG_DIR="${ALEX_LOG_DIR}" nohup "${BIN}" lark >> "${LOG_FILE}" 2>&1 & echo "$!" > "${PID_FILE}")

  pid="$(read_pid "${PID_FILE}" || true)"
  local i
  for i in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      log_error "Test Lark agent exited early (see ${LOG_FILE})"
      return 1
    fi
    if grep -q "${READY_LOG_PATTERN}" "${LOG_FILE}" 2>/dev/null; then
      log_success "Test Lark agent ready (PID: ${pid})"
      return 0
    fi
    sleep 1
  done

  log_warn "Test Lark agent running (PID: ${pid}) but readiness not confirmed within 30s (see ${LOG_FILE})"
  return 0
}

stop() {
  ensure_worktree
  stop_service "Test Lark agent" "${PID_FILE}"
}

restart() {
  [[ -f "${TEST_CONFIG}" ]] || die "Missing TEST_CONFIG: ${TEST_CONFIG}"
  ensure_worktree
  build
  stop
  FORCE_REBUILD=0 start
}

status() {
  ensure_worktree
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"

  if is_process_running "${pid}"; then
    log_success "Test Lark agent running (PID: ${pid})"
    return 0
  fi

  log_warn "Test Lark agent not running"
}

cmd="${1:-start}"
shift || true

case "${cmd}" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  status) status ;;
  logs)
    ensure_worktree
    touch "${LOG_FILE}" "${ALEX_LOG_DIR}/alex-service.log" "${ALEX_LOG_DIR}/alex-llm.log" "${ALEX_LOG_DIR}/alex-latency.log"
    tail -n 200 -f \
      "${LOG_FILE}" \
      "${ALEX_LOG_DIR}/alex-service.log" \
      "${ALEX_LOG_DIR}/alex-llm.log" \
      "${ALEX_LOG_DIR}/alex-latency.log"
    ;;
  build) build ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac
