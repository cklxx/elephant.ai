#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=../lib/common/build.sh
source "${SCRIPT_DIR}/../lib/common/build.sh"
# shellcheck source=../lib/common/lark_test_worktree.sh
source "${SCRIPT_DIR}/../lib/common/lark_test_worktree.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/test.sh start|stop|restart|status|logs|build

Runs alex-server in standalone Lark WebSocket mode from the test worktree.

Behavior:
  - Ensures local auth DB is running (docker, migrations)
  - Ensures persistent test worktree exists at .worktrees/test and syncs .env
  - Builds and starts alex-server from the test worktree

Env:
  TEST_CONFIG          Config path (default: ~/.alex/test.yaml)
  LARK_PID_DIR         Shared pid dir override (default: <dirname(MAIN_CONFIG)>/pids)
  ALEX_LOG_DIR         Internal log dir override (default: <repo>/.worktrees/test/logs)
  LARK_NOTICE_STATE_FILE Notice binding state path (default: <repo>/.worktrees/test/tmp/lark-notice.state.json)
  FORCE_REBUILD=1      Force rebuild on start (default: 1)
  SKIP_LOCAL_AUTH_DB=1 Skip local auth DB auto-setup (default: 0)
  LARK_REQUIRE_DOCKER=1 Ensure docker sandbox is running before startup (default: 1)
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

SETUP_DB_SH="${ROOT}/scripts/setup_local_auth_db.sh"

TEST_ROOT="${ROOT}/.worktrees/test"
MAIN_CONFIG_PATH_FOR_PID="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG_PATH_FOR_PID}")"
BIN="${TEST_ROOT}/alex-server"
PID_FILE="${PID_DIR}/lark-test.pid"
BUILD_STAMP="${PID_DIR}/lark-test.build"
SHA_FILE="${PID_DIR}/lark-test.sha"
LOG_FILE="${TEST_ROOT}/logs/lark-test.log"
TEST_CONFIG="${TEST_CONFIG:-$HOME/.alex/test.yaml}"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${TEST_ROOT}/logs}"
NOTICE_STATE_FILE="${LARK_NOTICE_STATE_FILE:-${ROOT}/.worktrees/test/tmp/lark-notice.state.json}"
FORCE_REBUILD="${FORCE_REBUILD:-1}"
LARK_REQUIRE_DOCKER="${LARK_REQUIRE_DOCKER:-1}"
DEV_SH="${ROOT}/dev.sh"
BOOTSTRAP_SH="${ROOT}/scripts/setup_local_runtime.sh"
CLEANUP_ORPHANS_SH="${ROOT}/scripts/lark/cleanup_orphan_agents.sh"

# Readiness: grep for this log line to confirm the gateway has started.
READY_LOG_PATTERN="Lark gateway connecting"

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
  MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}" \
    TEST_CONFIG="${TEST_CONFIG}" \
    "${BOOTSTRAP_SH}" >/dev/null
}

ensure_lark_sandbox() {
  if [[ "${LARK_REQUIRE_DOCKER}" != "1" ]]; then
    return 0
  fi
  command -v docker >/dev/null 2>&1 || die "docker not found but Lark mode requires sandbox Docker (set LARK_REQUIRE_DOCKER=0 to bypass)"
  [[ -x "${DEV_SH}" ]] || die "Missing ${DEV_SH}"
  log_info "Ensuring docker sandbox for lark mode..."
  (cd "${ROOT}" && "${DEV_SH}" sandbox-up)
}

cleanup_orphan_lark_agents() {
  if [[ -x "${CLEANUP_ORPHANS_SH}" ]]; then
    "${CLEANUP_ORPHANS_SH}" cleanup --scope all --quiet || true
  fi
}

print_runtime_binding() {
  log_info "Lark test config: $(lark_canonical_path "${TEST_CONFIG}")"
  log_info "Lark test identity: $(lark_resolve_identity "${TEST_CONFIG}")"
  log_info "Lark test pid dir: ${PID_DIR}"
}

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
  lark_ensure_test_worktree "${ROOT}"
  mkdir -p "${PID_DIR}" "${TEST_ROOT}/logs"
}

sync_test_runtime_to_main() {
  local main_sha test_sha
  main_sha="$(git -C "${ROOT}" rev-parse main 2>/dev/null || true)"
  [[ -n "${main_sha}" ]] || die "Failed to resolve main SHA"

  log_info "Aligning test worktree runtime to main (${main_sha:0:8})"
  git -C "${TEST_ROOT}" reset --hard "${main_sha}" >/dev/null 2>&1
  if ! git -C "${TEST_ROOT}" switch --detach "${main_sha}" >/dev/null 2>&1; then
    git -C "${TEST_ROOT}" checkout --detach "${main_sha}" >/dev/null 2>&1 || true
  fi

  test_sha="$(git -C "${TEST_ROOT}" rev-parse HEAD 2>/dev/null || true)"
  if [[ "${test_sha}" != "${main_sha}" ]]; then
    die "Failed to align test worktree to main (main=${main_sha} test=${test_sha})"
  fi
}

build() {
  ensure_worktree
  sync_test_runtime_to_main
  log_info "Building alex-server (test worktree)..."
  (cd "${TEST_ROOT}" && CGO_ENABLED=0 go build -o "${BIN}" ./cmd/alex-server)
  write_build_stamp "${BUILD_STAMP}" "$(build_fingerprint "${TEST_ROOT}")"
  git -C "${TEST_ROOT}" rev-parse HEAD > "${SHA_FILE}" 2>/dev/null || true
  log_success "Built ${BIN}"
}

start() {
  load_dotenv
  ensure_local_bootstrap
  [[ -f "${TEST_CONFIG}" ]] || die "Missing TEST_CONFIG: ${TEST_CONFIG}"
  print_runtime_binding
  ensure_lark_sandbox
  cleanup_orphan_lark_agents

  maybe_setup_auth_db
  ensure_worktree
  sync_test_runtime_to_main
  mkdir -p "$(dirname "${NOTICE_STATE_FILE}")"

  local current_fingerprint needs_build pid
  current_fingerprint="$(build_fingerprint "${TEST_ROOT}")"
  needs_build=0
  if [[ "${FORCE_REBUILD}" == "1" ]] || [[ ! -x "${BIN}" ]] || is_build_stale "${BUILD_STAMP}" "${current_fingerprint}"; then
    needs_build=1
  fi

  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    if [[ "${needs_build}" == "0" ]]; then
      lark_write_identity_lock "${ROOT}" "test" "${TEST_CONFIG}" "${pid}"
      log_success "Test Lark agent already running (PID: ${pid}, config: $(lark_canonical_path "${TEST_CONFIG}"))"
      return 0
    fi
    log_info "Source changes detected; rebuilding and restarting test agent..."
    build
    needs_build=0
    stop
  fi

  local lock_owner_pid
  lock_owner_pid="${pid}"
  if ! is_process_running "${lock_owner_pid}"; then
    lock_owner_pid="$$"
  fi
  lark_assert_identity_available "${ROOT}" "test" "${TEST_CONFIG}" "${lock_owner_pid}" || die "Lark identity is already owned by another process"
  lark_write_identity_lock "${ROOT}" "test-starting" "${TEST_CONFIG}" "$$"

  if [[ "${needs_build}" == "1" ]]; then
    build
  else
    log_info "Reusing existing build (no changes detected)."
  fi

  log_info "Starting test Lark standalone agent..."
  (
    cd "${TEST_ROOT}"
    ALEX_CONFIG_PATH="${TEST_CONFIG}" ALEX_LOG_DIR="${ALEX_LOG_DIR}" LARK_NOTICE_STATE_FILE="${NOTICE_STATE_FILE}" nohup "${BIN}" lark >> "${LOG_FILE}" 2>&1 &
    echo "$!" > "${PID_FILE}"
  )

  pid="$(read_pid "${PID_FILE}" || true)"
  lark_write_identity_lock "${ROOT}" "test" "${TEST_CONFIG}" "${pid}"
  for _ in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      lark_release_identity_lock "${ROOT}" "${TEST_CONFIG}" "${pid}"
      log_error "Test Lark agent exited early (see ${LOG_FILE})"
      return 1
    fi
    if grep -q "${READY_LOG_PATTERN}" "${LOG_FILE}" 2>/dev/null; then
      log_success "Test Lark agent ready (PID: ${pid}, config: $(lark_canonical_path "${TEST_CONFIG}"))"
      return 0
    fi
    sleep 1
  done

  log_warn "Test Lark agent running (PID: ${pid}) but readiness not confirmed within 30s (see ${LOG_FILE})"
  return 0
}

stop() {
  ensure_worktree
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  stop_service "Test Lark agent" "${PID_FILE}"
  lark_release_identity_lock "${ROOT}" "${TEST_CONFIG}" "${pid}" || true
}

restart() {
  load_dotenv
  ensure_local_bootstrap
  [[ -f "${TEST_CONFIG}" ]] || die "Missing TEST_CONFIG: ${TEST_CONFIG}"
  cleanup_orphan_lark_agents

  maybe_setup_auth_db
  ensure_worktree
  build
  stop
  FORCE_REBUILD=0 start
}

status() {
  ensure_worktree
  print_runtime_binding
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"

  if is_process_running "${pid}"; then
    lark_write_identity_lock "${ROOT}" "test" "${TEST_CONFIG}" "${pid}"
    log_success "Test Lark agent running (PID: ${pid}, config: $(lark_canonical_path "${TEST_CONFIG}"))"
    return 0
  fi

  lark_release_identity_lock "${ROOT}" "${TEST_CONFIG}" "${pid}" || true
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
