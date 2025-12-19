#!/usr/bin/env bash
###############################################################################
# Spinner - Local Development Helper
#
# Usage:
#   ./dev.sh                    # Start backend + web (background)
#   ./dev.sh up|start           # Same as default
#   ./dev.sh down|stop          # Stop backend + web
#   ./dev.sh status             # Show status + ports
#   ./dev.sh logs [server|web]  # Tail logs
#   ./dev.sh test               # Go + web tests
#   ./dev.sh lint               # Go + web lint
#
# Env:
#   SERVER_PORT=8080            # Backend port override (default 8080)
#   WEB_PORT=3000               # Web port override (default 3000)
#   START_WITH_WATCH=1          # Backend hot reload (requires `air`)
#   AUTO_STOP_CONFLICTING_PORTS=1 # Auto-stop processes using WEB_PORT (default 1)
#   AUTH_JWT_SECRET=...         # Auth secret (default: dev-secret-change-me)
###############################################################################

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PID_DIR="${SCRIPT_DIR}/.pids"
readonly LOG_DIR="${SCRIPT_DIR}/logs"
readonly SERVER_PID_FILE="${PID_DIR}/server.pid"
readonly WEB_PID_FILE="${PID_DIR}/web.pid"
readonly SERVER_LOG="${LOG_DIR}/server.log"
readonly WEB_LOG="${LOG_DIR}/web.log"

readonly DEFAULT_SERVER_PORT=8080
readonly DEFAULT_WEB_PORT=3000

SERVER_PORT="${SERVER_PORT:-${DEFAULT_SERVER_PORT}}"
WEB_PORT="${WEB_PORT:-${DEFAULT_WEB_PORT}}"
START_WITH_WATCH="${START_WITH_WATCH:-1}"
AUTO_STOP_CONFLICTING_PORTS="${AUTO_STOP_CONFLICTING_PORTS:-1}"

load_dotenv() {
  local env_file="${SCRIPT_DIR}/.env"
  if [[ ! -f "$env_file" ]]; then
    return 0
  fi

  set -a
  # shellcheck source=/dev/null
  source "$env_file"
  set +a
}

load_dotenv

export AUTH_JWT_SECRET="${AUTH_JWT_SECRET:-dev-secret-change-me}"

readonly C_RED='\033[0;31m'
readonly C_GREEN='\033[0;32m'
readonly C_YELLOW='\033[1;33m'
readonly C_BLUE='\033[0;34m'
readonly C_RESET='\033[0m'

log_info() { echo -e "${C_BLUE}▸${C_RESET} $*"; }
log_success() { echo -e "${C_GREEN}✓${C_RESET} $*"; }
log_warn() { echo -e "${C_YELLOW}⚠${C_RESET} $*"; }
log_error() { echo -e "${C_RED}✗${C_RESET} $*" >&2; }

die() {
  log_error "$*"
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

ensure_dirs() {
  mkdir -p "${PID_DIR}" "${LOG_DIR}"
}

read_pid() {
  local pid_file="$1"
  [[ -f "$pid_file" ]] && cat "$pid_file"
}

is_process_running() {
  local pid="${1:-}"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

stop_pid() {
  local pid="${1:-}"
  local label="${2:-process}"

  if ! is_process_running "$pid"; then
    return 0
  fi

  log_info "Stopping ${label} (PID: ${pid})"
  kill "$pid" 2>/dev/null || true

  for _ in {1..20}; do
    if ! is_process_running "$pid"; then
      return 0
    fi
    sleep 0.25
  done

  log_warn "${label} did not stop gracefully; force killing (PID: ${pid})"
  kill -9 "$pid" 2>/dev/null || true
  return 0
}

stop_port_listeners() {
  local port="$1"
  local label="$2"

  if ! command_exists lsof; then
    log_warn "lsof not found; cannot auto-stop port ${port} listeners"
    return 0
  fi

  local pids
  pids="$(lsof -ti tcp:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  [[ -n "$pids" ]] || return 0

  log_warn "${label} port ${port} is already in use; attempting to stop listeners"
  for pid in $pids; do
    local cmd
    cmd="$(ps -o command= -p "$pid" 2>/dev/null || true)"
    if [[ -n "$cmd" ]]; then
      log_warn "Stopping PID ${pid}: ${cmd}"
    else
      log_warn "Stopping PID ${pid}"
    fi
    stop_pid "$pid" "${label} port ${port} listener"
  done
}

stop_service() {
  local name="$1"
  local pid_file="$2"
  local pid
  pid="$(read_pid "$pid_file" || true)"

  if is_process_running "$pid"; then
    log_info "Stopping ${name} (PID: ${pid})"
    kill "$pid" 2>/dev/null || true

    for _ in {1..20}; do
      if ! is_process_running "$pid"; then
        rm -f "$pid_file"
        log_success "${name} stopped"
        return 0
      fi
      sleep 0.25
    done

    log_warn "${name} did not stop gracefully; force killing"
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$pid_file"
    log_success "${name} stopped"
    return 0
  fi

  if [[ -f "$pid_file" ]]; then
    log_warn "${name} PID file exists but process not running; cleaning up"
    rm -f "$pid_file"
    return 0
  fi

  log_info "${name} is not running"
}

is_port_available() {
  local port="$1"
  ! lsof -ti tcp:"$port" -sTCP:LISTEN >/dev/null 2>&1
}

assert_port_available() {
  local port="$1"
  local name="$2"
  if ! is_port_available "$port"; then
    local upper_name
    upper_name="$(printf '%s' "$name" | tr '[:lower:]' '[:upper:]')"
    die "${name} port ${port} is already in use. Stop the process or set ${upper_name}_PORT."
  fi
}

wait_for_health() {
  local url="$1"
  local name="$2"
  local attempts=30

  if ! command_exists curl; then
    log_warn "curl not found; skipping ${name} readiness check"
    return 0
  fi

  log_info "Waiting for ${name} to be ready..."
  for i in $(seq 1 "$attempts"); do
    if curl -sf --noproxy '*' "$url" >/dev/null 2>&1; then
      log_success "${name} is ready"
      return 0
    fi
    if [[ "$i" -eq "$attempts" ]]; then
      log_error "${name} did not become ready in ${attempts}s"
      return 1
    fi
    sleep 1
  done
}

cleanup_next_dev_lock() {
  local lock_file="${SCRIPT_DIR}/web/.next/dev/lock"
  if [[ -f "$lock_file" ]]; then
    log_warn "Removing stale Next.js dev lock: ${lock_file}"
    rm -f "$lock_file"
  fi
}

build_server() {
  log_info "Building backend (./cmd/alex-server)..."
  "${SCRIPT_DIR}/scripts/go-with-toolchain.sh" build -o "${SCRIPT_DIR}/alex-server" ./cmd/alex-server
  [[ -x "${SCRIPT_DIR}/alex-server" ]] || die "Backend build succeeded but ./alex-server is not executable"
  log_success "Backend built: ./alex-server"
}

start_server() {
  ensure_dirs

  local pid
  pid="$(read_pid "$SERVER_PID_FILE" || true)"
  if is_process_running "$pid"; then
    log_info "Backend already running (PID: ${pid})"
    return 0
  fi

  assert_port_available "$SERVER_PORT" "server"
  build_server

  log_info "Starting backend on :${SERVER_PORT}..."
  if [[ "${START_WITH_WATCH}" == "1" ]] && command_exists air; then
    PORT="${SERVER_PORT}" \
      ALEX_SERVER_PORT="${SERVER_PORT}" \
      ALEX_SERVER_MODE="deploy" \
      ALEX_LOG_DIR="${LOG_DIR}" \
      air \
      --build.cmd "${SCRIPT_DIR}/scripts/go-with-toolchain.sh build -o ${SCRIPT_DIR}/alex-server ./cmd/alex-server" \
      --build.bin "${SCRIPT_DIR}/alex-server" \
      >"${SERVER_LOG}" 2>&1 &
  else
    PORT="${SERVER_PORT}" \
      ALEX_SERVER_PORT="${SERVER_PORT}" \
      ALEX_SERVER_MODE="deploy" \
      ALEX_LOG_DIR="${LOG_DIR}" \
      "${SCRIPT_DIR}/alex-server" \
      >"${SERVER_LOG}" 2>&1 &
  fi

  echo $! >"${SERVER_PID_FILE}"
  wait_for_health "http://localhost:${SERVER_PORT}/health" "backend" || true
  log_success "Backend started (PID: $(cat "${SERVER_PID_FILE}"))"
}

start_web() {
  ensure_dirs

  local pid
  pid="$(read_pid "$WEB_PID_FILE" || true)"
  if is_process_running "$pid"; then
    log_info "Web already running (PID: ${pid})"
    return 0
  fi

  cleanup_next_dev_lock

  if [[ "${AUTO_STOP_CONFLICTING_PORTS}" == "1" ]] && ! is_port_available "$WEB_PORT"; then
    stop_port_listeners "$WEB_PORT" "web"
  fi
  assert_port_available "$WEB_PORT" "web"

  if [[ ! -d "${SCRIPT_DIR}/web/node_modules" ]]; then
    log_warn "web/node_modules not found; run: (cd web && npm install)"
  fi

  log_info "Starting web on :${WEB_PORT}..."
  PORT="${WEB_PORT}" \
    NEXT_PUBLIC_API_URL="http://localhost:${SERVER_PORT}" \
    npm --prefix "${SCRIPT_DIR}/web" run dev \
    >"${WEB_LOG}" 2>&1 &

  echo $! >"${WEB_PID_FILE}"
  wait_for_health "http://localhost:${WEB_PORT}" "web" || true
  log_success "Web started (PID: $(cat "${WEB_PID_FILE}"))"
}

cmd_up() {
  start_server
  start_web
  log_success "Dev services are running: backend=http://localhost:${SERVER_PORT} web=http://localhost:${WEB_PORT}"
}

cmd_down() {
  stop_service "Web" "${WEB_PID_FILE}"
  cleanup_next_dev_lock
  stop_service "Backend" "${SERVER_PID_FILE}"
}

cmd_status() {
  local server_pid web_pid
  server_pid="$(read_pid "$SERVER_PID_FILE" || true)"
  web_pid="$(read_pid "$WEB_PID_FILE" || true)"

  if is_process_running "$server_pid"; then
    log_success "Backend: running (PID: ${server_pid}) http://localhost:${SERVER_PORT}"
  else
    log_warn "Backend: stopped"
  fi

  if is_process_running "$web_pid"; then
    log_success "Web: running (PID: ${web_pid}) http://localhost:${WEB_PORT}"
  else
    log_warn "Web: stopped"
  fi
}

cmd_logs() {
  local target="${1:-all}"
  ensure_dirs

  case "$target" in
    server|backend)
      tail -n 200 -f "${SERVER_LOG}"
      ;;
    web)
      tail -n 200 -f "${WEB_LOG}"
      ;;
    all)
      tail -n 200 -f "${SERVER_LOG}" "${WEB_LOG}"
      ;;
    *)
      die "Unknown logs target: ${target} (expected: server|web|all)"
      ;;
  esac
}

cmd_test() {
  log_info "Running Go tests..."
  "${SCRIPT_DIR}/scripts/go-with-toolchain.sh" test ./... -count=1
  log_success "Go tests passed"

  log_info "Running web tests..."
  npm --prefix "${SCRIPT_DIR}/web" test
  log_success "Web tests passed"
}

cmd_lint() {
  log_info "Running Go lint..."
  "${SCRIPT_DIR}/scripts/run-golangci-lint.sh" run ./...
  log_success "Go lint passed"

  log_info "Running web lint..."
  npm --prefix "${SCRIPT_DIR}/web" run lint
  log_success "Web lint passed"
}

usage() {
  cat <<EOF
Spinner dev helper

Usage:
  ./dev.sh [command]

Commands:
  up|start   Start backend + web (background)
  down|stop  Stop backend + web
  status     Show status + ports
  logs       Tail logs (optional: server|web)
  test       Run Go + web tests
  lint       Run Go + web lint
EOF
}

cmd="${1:-up}"
shift || true

case "$cmd" in
  up|start) cmd_up ;;
  down|stop) cmd_down ;;
  status) cmd_status ;;
  logs) cmd_logs "${@:-all}" ;;
  test) cmd_test ;;
  lint) cmd_lint ;;
  help|-h|--help) usage ;;
  *) die "Unknown command: ${cmd} (run ./dev.sh help)" ;;
esac
