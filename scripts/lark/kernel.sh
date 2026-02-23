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
  cat <<'USAGE'
Usage:
  scripts/lark/kernel.sh start|stop|restart|status|logs|build

Runs alex-server in standalone kernel daemon mode.

Env:
  MAIN_CONFIG        Config path (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
  LARK_PID_DIR       Shared pid dir override (default: <dirname(MAIN_CONFIG)>/pids)
  ALEX_LOG_DIR       Internal log dir override (default: <repo>/logs)
  FORCE_REBUILD=1    Force rebuild on start (default: 0)
  SKIP_LOCAL_AUTH_DB=1  Skip local auth DB auto-setup (default: 0)
USAGE
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
PID_FILE="${PID_DIR}/lark-kernel.pid"
BUILD_STAMP="${PID_DIR}/lark-kernel.build"
SHA_FILE="${PID_DIR}/lark-kernel.sha"
LOG_FILE="${ROOT}/logs/lark-kernel.log"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${ROOT}/logs}"
FORCE_REBUILD="${FORCE_REBUILD:-0}"
BOOTSTRAP_SH="${ROOT}/scripts/setup_local_runtime.sh"
READY_LOG_PATTERN="Kernel daemon running"

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

  local current_fingerprint needs_build pid
  local alex_kernel_log main_log_lines_start alex_log_lines_start
  current_fingerprint="$(build_fingerprint "${ROOT}")"
  needs_build=0
  if [[ "${FORCE_REBUILD}" == "1" ]] || [[ ! -x "${BIN}" ]] || is_build_stale "${BUILD_STAMP}" "${current_fingerprint}"; then
    needs_build=1
  fi

  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    if [[ "${needs_build}" == "0" ]]; then
      log_success "Kernel daemon already running (PID: ${pid})"
      return 0
    fi
    log_info "Source changes detected; rebuilding and restarting kernel daemon..."
    build
    needs_build=0
    stop
  fi

  if [[ "${needs_build}" == "1" ]]; then
    build
  else
    log_info "Reusing existing build (no changes detected)."
  fi

  log_info "Starting kernel daemon..."
  alex_kernel_log="${ALEX_LOG_DIR}/alex-kernel.log"
  touch "${LOG_FILE}" "${alex_kernel_log}"
  main_log_lines_start="$(wc -l < "${LOG_FILE}" | tr -d '[:space:]')"
  alex_log_lines_start="$(wc -l < "${alex_kernel_log}" | tr -d '[:space:]')"

  ALEX_CONFIG_PATH="${MAIN_CONFIG}" ALEX_LOG_DIR="${ALEX_LOG_DIR}" nohup "${BIN}" kernel-daemon >> "${LOG_FILE}" 2>&1 &
  echo "$!" > "${PID_FILE}"

  pid="$(read_pid "${PID_FILE}" || true)"
  for _ in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      log_error "Kernel daemon exited early (see ${LOG_FILE})"
      return 1
    fi
    if tail -n "+$((main_log_lines_start + 1))" "${LOG_FILE}" 2>/dev/null | grep -q "${READY_LOG_PATTERN}" \
      || tail -n "+$((alex_log_lines_start + 1))" "${alex_kernel_log}" 2>/dev/null | grep -q "${READY_LOG_PATTERN}"; then
      log_success "Kernel daemon ready (PID: ${pid})"
      return 0
    fi
    sleep 1
  done

  log_info "Kernel daemon running (PID: ${pid}); readiness log not observed within 30s (logs: ${LOG_FILE}, ${alex_kernel_log})"
  return 0
}

stop() {
  stop_service "Kernel daemon" "${PID_FILE}"
}

restart() {
  load_dotenv
  ensure_local_bootstrap
  [[ -f "${MAIN_CONFIG}" ]] || die "Missing MAIN_CONFIG: ${MAIN_CONFIG}"

  build
  stop
  FORCE_REBUILD=0 start
}

status() {
  log_info "Kernel config: $(lark_canonical_path "${MAIN_CONFIG}")"
  log_info "Kernel pid dir: ${PID_DIR}"

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"

  if is_process_running "${pid}"; then
    log_success "Kernel daemon running (PID: ${pid})"
    return 0
  fi

  log_warn "Kernel daemon not running"
}

cmd="${1:-start}"
shift || true

case "${cmd}" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  status) status ;;
  logs)
    touch "${LOG_FILE}" "${ALEX_LOG_DIR}/alex-kernel.log"
    tail -n 200 -f "${LOG_FILE}" "${ALEX_LOG_DIR}/alex-kernel.log"
    ;;
  build) build ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac
