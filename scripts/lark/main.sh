#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=../lib/common/build.sh
source "${SCRIPT_DIR}/../lib/common/build.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/main.sh start|stop|restart|status|logs|build

Runs alex-server in standalone Lark WebSocket mode (no HTTP server).

Env:
  MAIN_CONFIG   Config path (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
  LARK_PID_DIR  Shared pid dir override (default: <dirname(MAIN_CONFIG)>/pids)
  ALEX_LOG_DIR  Internal log dir override (default: <repo>/logs)
  LARK_NOTICE_STATE_FILE Notice binding state path (default: <repo>/.worktrees/test/tmp/lark-notice.state.json)
  FORCE_REBUILD=1  Force rebuild on start (default: 0)
  SKIP_LOCAL_AUTH_DB=1  Skip local auth DB auto-setup (default: 0)
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
MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG}")"
PID_FILE="${PID_DIR}/lark-main.pid"
BUILD_STAMP="${PID_DIR}/lark-main.build"
SHA_FILE="${PID_DIR}/lark-main.sha"
LOG_FILE="${ROOT}/logs/lark-main.log"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${ROOT}/logs}"
NOTICE_STATE_FILE="${LARK_NOTICE_STATE_FILE:-${ROOT}/.worktrees/test/tmp/lark-notice.state.json}"
FORCE_REBUILD="${FORCE_REBUILD:-0}"
BOOTSTRAP_SH="${ROOT}/scripts/setup_local_runtime.sh"
CLEANUP_ORPHANS_SH="${ROOT}/scripts/lark/cleanup_orphan_agents.sh"

# Readiness: grep for this log line to confirm the gateway has started.
READY_LOG_PATTERN="Lark gateway connecting"

mkdir -p "${PID_DIR}" "${ROOT}/logs" "${ALEX_LOG_DIR}"

load_dotenv() {
  local env_file="${ROOT}/.env"
  if [[ ! -f "${env_file}" ]]; then
    return 0
  fi

  set -a
  # shellcheck source=/dev/null
  source "${env_file}"
  set +a
}

ensure_local_bootstrap() {
  [[ -x "${BOOTSTRAP_SH}" ]] || die "Missing ${BOOTSTRAP_SH}"
  MAIN_CONFIG="${MAIN_CONFIG}" \
    TEST_CONFIG="${TEST_CONFIG:-$HOME/.alex/test.yaml}" \
    "${BOOTSTRAP_SH}" >/dev/null
}

cleanup_orphan_lark_agents() {
  if [[ -x "${CLEANUP_ORPHANS_SH}" ]]; then
    "${CLEANUP_ORPHANS_SH}" cleanup --scope all --quiet || true
  fi
}

print_runtime_binding() {
  log_info "Lark main config: $(lark_canonical_path "${MAIN_CONFIG}")"
  log_info "Lark main identity: $(lark_resolve_identity "${MAIN_CONFIG}")"
  log_info "Lark main pid dir: ${PID_DIR}"
}

maybe_setup_auth_db() {
  if [[ "${SKIP_LOCAL_AUTH_DB:-0}" == "1" ]]; then
    log_info "Skipping local auth DB auto-setup (SKIP_LOCAL_AUTH_DB=1)"
    return 0
  fi

  if [[ -x "${ROOT}/scripts/setup_local_auth_db.sh" ]]; then
    log_info "Ensuring local auth DB is ready..."
    "${ROOT}/scripts/setup_local_auth_db.sh"
    return 0
  fi

  log_warn "Missing ${ROOT}/scripts/setup_local_auth_db.sh; skipping DB setup"
  return 0
}

build() {
  log_info "Building alex-server (main)..."
  (cd "${ROOT}" && CGO_ENABLED=0 go build -o "${BIN}" ./cmd/alex-server)
  write_build_stamp "${BUILD_STAMP}" "$(build_fingerprint "${ROOT}")"
  git -C "${ROOT}" rev-parse HEAD > "${SHA_FILE}" 2>/dev/null || true
  log_success "Built ${BIN}"
}

start() {
  load_dotenv
  ensure_local_bootstrap
  [[ -f "${MAIN_CONFIG}" ]] || die "Missing MAIN_CONFIG: ${MAIN_CONFIG}"
  print_runtime_binding
  cleanup_orphan_lark_agents

  maybe_setup_auth_db

  local current_fingerprint needs_build pid
  mkdir -p "$(dirname "${NOTICE_STATE_FILE}")"
  current_fingerprint="$(build_fingerprint "${ROOT}")"
  needs_build=0
  if [[ "${FORCE_REBUILD}" == "1" ]] || [[ ! -x "${BIN}" ]] || is_build_stale "${BUILD_STAMP}" "${current_fingerprint}"; then
    needs_build=1
  fi

  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    if [[ "${needs_build}" == "0" ]]; then
      lark_write_identity_lock "${ROOT}" "main" "${MAIN_CONFIG}" "${pid}"
      log_success "Lark agent already running (PID: ${pid}, config: $(lark_canonical_path "${MAIN_CONFIG}"))"
      return 0
    fi
    log_info "Source changes detected; rebuilding and restarting..."
    build
    needs_build=0
    stop
  fi

  local lock_owner_pid
  lock_owner_pid="${pid}"
  if ! is_process_running "${lock_owner_pid}"; then
    lock_owner_pid="$$"
  fi
  lark_assert_identity_available "${ROOT}" "main" "${MAIN_CONFIG}" "${lock_owner_pid}" || die "Lark identity is already owned by another process"
  lark_write_identity_lock "${ROOT}" "main-starting" "${MAIN_CONFIG}" "$$"

  if [[ "${needs_build}" == "1" ]]; then
    build
  else
    log_info "Reusing existing build (no changes detected)."
  fi

  log_info "Starting Lark standalone agent..."
  ALEX_CONFIG_PATH="${MAIN_CONFIG}" ALEX_LOG_DIR="${ALEX_LOG_DIR}" LARK_NOTICE_STATE_FILE="${NOTICE_STATE_FILE}" nohup "${BIN}" >> "${LOG_FILE}" 2>&1 &
  echo "$!" > "${PID_FILE}"

  pid="$(read_pid "${PID_FILE}" || true)"
  lark_write_identity_lock "${ROOT}" "main" "${MAIN_CONFIG}" "${pid}"
  for _ in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      lark_release_identity_lock "${ROOT}" "${MAIN_CONFIG}" "${pid}"
      log_error "Lark agent exited early (see ${LOG_FILE})"
      return 1
    fi
    if grep -q "${READY_LOG_PATTERN}" "${LOG_FILE}" 2>/dev/null; then
      log_success "Lark agent ready (PID: ${pid}, config: $(lark_canonical_path "${MAIN_CONFIG}"))"
      return 0
    fi
    sleep 1
  done

  log_warn "Lark agent running (PID: ${pid}) but readiness not confirmed within 30s (see ${LOG_FILE})"
  return 0
}

stop() {
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  stop_service "Lark agent" "${PID_FILE}"
  lark_release_identity_lock "${ROOT}" "${MAIN_CONFIG}" "${pid}" || true
}

restart() {
  load_dotenv
  ensure_local_bootstrap
  [[ -f "${MAIN_CONFIG}" ]] || die "Missing MAIN_CONFIG: ${MAIN_CONFIG}"
  cleanup_orphan_lark_agents

  maybe_setup_auth_db
  build
  stop
  FORCE_REBUILD=0 start
}

status() {
  print_runtime_binding
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"

  if is_process_running "${pid}"; then
    lark_write_identity_lock "${ROOT}" "main" "${MAIN_CONFIG}" "${pid}"
    log_success "Lark agent running (PID: ${pid}, config: $(lark_canonical_path "${MAIN_CONFIG}"))"
    return 0
  fi

  lark_release_identity_lock "${ROOT}" "${MAIN_CONFIG}" "${pid}" || true
  log_warn "Lark agent not running"
}

cmd="${1:-start}"
shift || true

case "${cmd}" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  status) status ;;
  logs)
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
