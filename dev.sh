#!/usr/bin/env bash
###############################################################################
# elephant.ai - Local Development Helper
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
#   SANDBOX_PORT=18086          # Sandbox port override (default 18086)
#   SANDBOX_IMAGE=...           # Sandbox image override
#   SANDBOX_BASE_URL=...        # Sandbox base URL override (default http://localhost:18086)
#   START_ACP_WITH_SANDBOX=1    # Start ACP serve alongside sandbox (default 1)
#   ACP_PORT=0                  # ACP port override (0 = auto-pick)
#   ACP_HOST=127.0.0.1           # ACP bind host (default 127.0.0.1)
#   START_WITH_WATCH=1          # Backend hot reload (requires `air`)
#   AUTO_STOP_CONFLICTING_PORTS=1 # Auto-stop our backend/web conflicts (default 1)
#   AUTH_JWT_SECRET=...         # Auth secret (default: dev-secret-change-me)
#   SKIP_LOCAL_AUTH_DB=1        # Skip local auth DB auto-setup (default 0)
###############################################################################

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PID_DIR="${SCRIPT_DIR}/.pids"
readonly LOG_DIR="${SCRIPT_DIR}/logs"
readonly SERVER_PID_FILE="${PID_DIR}/server.pid"
readonly WEB_PID_FILE="${PID_DIR}/web.pid"
readonly ACP_PID_FILE="${PID_DIR}/acp.pid"
readonly ACP_PORT_FILE="${PID_DIR}/acp.port"
readonly SERVER_LOG="${LOG_DIR}/server.log"
readonly WEB_LOG="${LOG_DIR}/web.log"
readonly ACP_LOG="${LOG_DIR}/acp.log"

readonly DEFAULT_SERVER_PORT=8080
readonly DEFAULT_WEB_PORT=3000
readonly DEFAULT_SANDBOX_PORT=18086
readonly DEFAULT_SANDBOX_IMAGE="ghcr.io/agent-infra/sandbox:latest"
readonly DEFAULT_ACP_HOST="127.0.0.1"

SERVER_PORT="${SERVER_PORT:-${DEFAULT_SERVER_PORT}}"
WEB_PORT="${WEB_PORT:-${DEFAULT_WEB_PORT}}"
SANDBOX_PORT="${SANDBOX_PORT:-${DEFAULT_SANDBOX_PORT}}"
SANDBOX_IMAGE="${SANDBOX_IMAGE:-${DEFAULT_SANDBOX_IMAGE}}"
SANDBOX_BASE_URL="${SANDBOX_BASE_URL:-http://localhost:${SANDBOX_PORT}}"
SANDBOX_CONTAINER_NAME="${SANDBOX_CONTAINER_NAME:-alex-sandbox}"
START_ACP_WITH_SANDBOX="${START_ACP_WITH_SANDBOX:-1}"
ACP_PORT="${ACP_PORT:-0}"
ACP_HOST="${ACP_HOST:-${DEFAULT_ACP_HOST}}"
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

ensure_playwright_browsers() {
  log_info "Ensuring Playwright browsers..."
  if PLAYWRIGHT_LOG_DIR="${LOG_DIR}" "${SCRIPT_DIR}/scripts/ensure-playwright.sh"; then
    log_success "Playwright browsers ready"
  else
    log_error "Playwright browser install failed; see ${LOG_DIR}/playwright-install.log"
    exit 1
  fi
}

ensure_dirs() {
  mkdir -p "${PID_DIR}" "${LOG_DIR}"
}

pick_random_port() {
  if command_exists python3; then
    python3 - << 'PY'
import random
import socket

for _ in range(50):
    port = random.randint(20000, 45000)
    sock = socket.socket()
    try:
        sock.bind(("127.0.0.1", port))
    except OSError:
        continue
    sock.close()
    print(port)
    raise SystemExit(0)
raise SystemExit(1)
PY
    return $?
  fi

  local start=20000
  local end=45000
  local port
  for _ in {1..50}; do
    port=$((start + RANDOM % (end - start + 1)))
    if is_port_available "$port"; then
      echo "$port"
      return 0
    fi
  done

  return 1
}

ensure_acp_port() {
  local port="$ACP_PORT"

  if [[ -n "$port" && "$port" != "0" ]]; then
    if ! is_port_available "$port"; then
      die "ACP port ${port} is already in use; set ACP_PORT to a free port"
    fi
    echo "$port" >"$ACP_PORT_FILE"
    echo "$port"
    return 0
  fi

  if [[ -f "$ACP_PORT_FILE" ]]; then
    port="$(cat "$ACP_PORT_FILE" 2>/dev/null || true)"
    if [[ -n "$port" ]] && is_port_available "$port"; then
      echo "$port"
      return 0
    fi
  fi

  port="$(pick_random_port)" || return 1
  echo "$port" >"$ACP_PORT_FILE"
  echo "$port"
}

resolve_acp_binary() {
  if [[ -n "${ACP_BIN:-}" && -x "${ACP_BIN}" ]]; then
    echo "${ACP_BIN}"
    return 0
  fi

  if [[ -x "${SCRIPT_DIR}/alex" ]]; then
    echo "${SCRIPT_DIR}/alex"
    return 0
  fi

  if ! command_exists "${SCRIPT_DIR}/scripts/go-with-toolchain.sh"; then
    return 1
  fi

  log_info "Building CLI (./cmd/alex)..."
  "${SCRIPT_DIR}/scripts/go-with-toolchain.sh" build -o "${SCRIPT_DIR}/alex" ./cmd/alex >/dev/null
  if [[ -x "${SCRIPT_DIR}/alex" ]]; then
    echo "${SCRIPT_DIR}/alex"
    return 0
  fi

  return 1
}

start_acp_daemon() {
  if [[ "${START_ACP_WITH_SANDBOX}" != "1" ]]; then
    return 0
  fi

  ensure_dirs

  local pid
  pid="$(read_pid "$ACP_PID_FILE" || true)"
  if is_process_running "$pid"; then
    log_info "ACP already running (PID: ${pid})"
    return 0
  fi

  local port
  port="$(ensure_acp_port)" || die "Failed to allocate ACP port"
  ACP_PORT="$port"

  local alex_bin
  alex_bin="$(resolve_acp_binary)" || die "alex CLI not available (need ./alex or go toolchain)"

  log_info "Starting ACP daemon on ${ACP_HOST}:${ACP_PORT}..."
  (
    trap '[[ -n "${child_pid:-}" ]] && kill "$child_pid" 2>/dev/null || true; exit 0' TERM INT
    while true; do
      "${alex_bin}" acp serve --host "${ACP_HOST}" --port "${ACP_PORT}" >>"${ACP_LOG}" 2>&1 &
      child_pid=$!
      wait "$child_pid"
      sleep 1
    done
  ) &

  echo $! >"${ACP_PID_FILE}"
}

stop_acp_daemon() {
  local pid
  pid="$(read_pid "$ACP_PID_FILE" || true)"
  if ! is_process_running "$pid"; then
    [[ -f "$ACP_PID_FILE" ]] && rm -f "$ACP_PID_FILE"
    return 0
  fi

  log_info "Stopping ACP daemon (PID: ${pid})"
  kill "$pid" 2>/dev/null || true
  for _ in {1..20}; do
    if ! is_process_running "$pid"; then
      rm -f "$ACP_PID_FILE"
      return 0
    fi
    sleep 0.25
  done
  log_warn "ACP daemon did not stop gracefully; force killing (PID: ${pid})"
  kill -9 "$pid" 2>/dev/null || true
  rm -f "$ACP_PID_FILE"
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

port_listener_pids() {
  local port="$1"

  if ! command_exists lsof; then
    return 0
  fi

  lsof -ti tcp:"$port" -sTCP:LISTEN 2>/dev/null || true
}

pid_executable_path() {
  local pid="$1"

  if ! command_exists lsof; then
    return 1
  fi

  lsof -nP -p "$pid" 2>/dev/null | awk '$4 ~ /txt/ {print $9; exit}'
}

is_our_backend_pid() {
  local pid="$1"
  local backend_bin="${SCRIPT_DIR}/alex-server"
  local exe

  exe="$(pid_executable_path "$pid" || true)"
  [[ -n "$exe" && "$exe" == "$backend_bin" ]]
}

is_alex_server_pid() {
  local pid="$1"
  local exe

  exe="$(pid_executable_path "$pid" || true)"
  [[ -n "$exe" && "$(basename "$exe")" == "alex-server" ]]
}

alex_server_listener_pids() {
  local port="$1"
  local pid

  for pid in $(port_listener_pids "$port"); do
    if is_alex_server_pid "$pid"; then
      echo "$pid"
    fi
  done
}

our_backend_listener_pids() {
  local port="$1"
  local pid

  for pid in $(port_listener_pids "$port"); do
    if is_our_backend_pid "$pid"; then
      echo "$pid"
    fi
  done
}

maybe_stop_backend_supervisor() {
  local pid="$1"
  local ppid
  local parent_comm
  local parent_cmd

  ppid="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"
  [[ -n "$ppid" && "$ppid" != "0" && "$ppid" != "1" ]] || return 0

  parent_comm="$(ps -o comm= -p "$ppid" 2>/dev/null || true)"
  [[ "$parent_comm" == "air" ]] || return 0

  parent_cmd="$(ps -o command= -p "$ppid" 2>/dev/null || true)"
  if [[ "$parent_cmd" == *"alex-server"* ]] || [[ "$parent_cmd" == *"cmd/alex-server"* ]]; then
    stop_pid "$ppid" "backend supervisor (air)"
  fi
}

stop_alex_server_listeners() {
  local port="$1"

  if ! command_exists lsof; then
    log_warn "lsof not found; cannot auto-stop server port ${port} listeners"
    return 0
  fi

  local pids=()
  local pid
  while IFS= read -r pid; do
    [[ -n "$pid" ]] && pids+=("$pid")
  done < <(alex_server_listener_pids "$port")

  ((${#pids[@]} > 0)) || return 0

  log_warn "server port ${port} is already in use by alex-server; attempting to stop listener(s)"
  for pid in "${pids[@]}"; do
    local cmd
    cmd="$(ps -o command= -p "$pid" 2>/dev/null || true)"
    if [[ -n "$cmd" ]]; then
      log_warn "Stopping PID ${pid}: ${cmd}"
    else
      log_warn "Stopping PID ${pid}"
    fi

    maybe_stop_backend_supervisor "$pid"
    stop_pid "$pid" "backend port ${port} listener"
  done
}

die_port_in_use() {
  local port="$1"
  local name="$2"
  local upper_name
  upper_name="$(printf '%s' "$name" | tr '[:lower:]' '[:upper:]')"

  log_error "${name} port ${port} is already in use. Stop the process or set ${upper_name}_PORT."
  if command_exists lsof; then
    local listeners
    listeners="$(lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
    if [[ -n "$listeners" ]]; then
      log_error "Listeners:"
      printf '%s\n' "$listeners" | sed 's/^/  /' >&2
    fi
  fi
  exit 1
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
    die_port_in_use "$port" "$name"
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

is_local_sandbox_url() {
  case "$SANDBOX_BASE_URL" in
    http://localhost:*|http://127.0.0.1:*|http://0.0.0.0:*|https://localhost:*|https://127.0.0.1:*|https://0.0.0.0:*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

start_sandbox() {
  if ! is_local_sandbox_url; then
    wait_for_health "${SANDBOX_BASE_URL}/v1/docs" "sandbox"
    return $?
  fi

  if ! command_exists docker; then
    log_error "docker not found; cannot start sandbox"
    return 1
  fi

  start_acp_daemon
  local acp_container_host="host.docker.internal"
  local acp_addr="${acp_container_host}:${ACP_PORT}"

  if docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    log_info "Sandbox already running (container ${SANDBOX_CONTAINER_NAME})"
    wait_for_health "http://localhost:${SANDBOX_PORT}/v1/docs" "sandbox"
    log_info "ACP server injected at ${acp_addr}"
    return $?
  fi

  if docker ps -a --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    log_info "Starting sandbox container ${SANDBOX_CONTAINER_NAME}..."
    docker start "${SANDBOX_CONTAINER_NAME}" >/dev/null
  else
    log_info "Starting sandbox container ${SANDBOX_CONTAINER_NAME} on :${SANDBOX_PORT}..."
    docker run -d --name "${SANDBOX_CONTAINER_NAME}" \
      --add-host "host.docker.internal:host-gateway" \
      -e "ACP_SERVER_HOST=${acp_container_host}" \
      -e "ACP_SERVER_PORT=${ACP_PORT}" \
      -e "ACP_SERVER_ADDR=${acp_addr}" \
      -p "${SANDBOX_PORT}:8080" \
      "${SANDBOX_IMAGE}" >/dev/null
  fi

  wait_for_health "http://localhost:${SANDBOX_PORT}/v1/docs" "sandbox"
  log_info "ACP server injected at ${acp_addr}"
}

stop_sandbox() {
  if ! is_local_sandbox_url; then
    return 0
  fi
  if ! command_exists docker; then
    return 0
  fi
  if docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    log_info "Stopping sandbox container ${SANDBOX_CONTAINER_NAME}..."
    docker stop "${SANDBOX_CONTAINER_NAME}" >/dev/null
  fi
  stop_acp_daemon
}

sandbox_ready() {
  if ! command_exists curl; then
    return 1
  fi
  curl -sf --noproxy '*' "${SANDBOX_BASE_URL}/v1/docs" >/dev/null 2>&1
}

auth_db_host() {
  local db_url="$1"
  local rest hostport host

  rest="${db_url#*://}"
  rest="${rest##*@}"
  hostport="${rest%%/*}"
  hostport="${hostport%%\?*}"

  if [[ "$hostport" == \[* ]]; then
    host="${hostport#\[}"
    host="${host%%]*}"
  else
    host="${hostport%%:*}"
  fi

  printf '%s' "$host"
}

is_local_auth_db_host() {
  case "$1" in
    localhost|127.0.0.1|::1)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

maybe_setup_auth_db() {
  local db_url host

  if [[ "${SKIP_LOCAL_AUTH_DB:-0}" == "1" ]]; then
    log_info "Skipping local auth DB auto-setup (SKIP_LOCAL_AUTH_DB=1)"
    return 0
  fi

  db_url="${AUTH_DATABASE_URL:-}"
  if [[ -z "$db_url" ]]; then
    return 0
  fi

  host="$(auth_db_host "$db_url")"
  if [[ -z "$host" ]] || ! is_local_auth_db_host "$host"; then
    return 0
  fi

  if ! command_exists docker; then
    log_warn "docker not found; skipping local auth DB auto-setup"
    return 0
  fi

  ensure_dirs
  log_info "Setting up local auth DB via scripts/setup_local_auth_db.sh..."
  if "${SCRIPT_DIR}/scripts/setup_local_auth_db.sh"; then
    log_success "Local auth DB ready"
  else
    log_warn "Local auth DB setup failed; auth may be disabled"
    log_warn "See ${LOG_DIR}/setup_auth_db.log for details"
  fi
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

  if ! is_port_available "$SERVER_PORT"; then
    local backend_pids=()
    while IFS= read -r pid; do
      [[ -n "$pid" ]] && backend_pids+=("$pid")
    done < <(our_backend_listener_pids "$SERVER_PORT")

    if ((${#backend_pids[@]} == 1)); then
      log_warn "Backend already listening on :${SERVER_PORT} (PID: ${backend_pids[0]}); restoring PID file"
      echo "${backend_pids[0]}" >"${SERVER_PID_FILE}"
      return 0
    fi

    if [[ "${AUTO_STOP_CONFLICTING_PORTS}" == "1" ]]; then
      stop_alex_server_listeners "$SERVER_PORT"
    fi
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
  start_sandbox
  maybe_setup_auth_db
  start_server
  start_web
  log_success "Dev services are running: backend=http://localhost:${SERVER_PORT} web=http://localhost:${WEB_PORT} sandbox=${SANDBOX_BASE_URL}"
}

cmd_down() {
  stop_service "Web" "${WEB_PID_FILE}"
  cleanup_next_dev_lock
  if [[ "${AUTO_STOP_CONFLICTING_PORTS}" == "1" ]] && ! is_port_available "$WEB_PORT"; then
    stop_port_listeners "$WEB_PORT" "web"
  fi
  stop_service "Backend" "${SERVER_PID_FILE}"
  stop_alex_server_listeners "$SERVER_PORT"
  stop_sandbox
}

cmd_status() {
  local server_pid web_pid
  server_pid="$(read_pid "$SERVER_PID_FILE" || true)"
  web_pid="$(read_pid "$WEB_PID_FILE" || true)"

  if is_process_running "$server_pid"; then
    log_success "Backend: running (PID: ${server_pid}) http://localhost:${SERVER_PORT}"
  else
    local backend_pids=()
    local pid
    while IFS= read -r pid; do
      [[ -n "$pid" ]] && backend_pids+=("$pid")
    done < <(alex_server_listener_pids "$SERVER_PORT")

    if ((${#backend_pids[@]} == 1)); then
      log_success "Backend: running (PID: ${backend_pids[0]}) http://localhost:${SERVER_PORT} (PID file missing)"
    elif ((${#backend_pids[@]} > 1)); then
      log_warn "Backend: multiple listeners on :${SERVER_PORT} (PID file missing)"
    else
      log_warn "Backend: stopped"
    fi
  fi

  if is_process_running "$web_pid"; then
    log_success "Web: running (PID: ${web_pid}) http://localhost:${WEB_PORT}"
  else
    log_warn "Web: stopped"
  fi

  if sandbox_ready; then
    log_success "Sandbox: ready ${SANDBOX_BASE_URL}"
  else
    log_warn "Sandbox: unavailable ${SANDBOX_BASE_URL}"
  fi

  local acp_pid acp_port
  acp_pid="$(read_pid "$ACP_PID_FILE" || true)"
  acp_port="$(cat "$ACP_PORT_FILE" 2>/dev/null || true)"
  if is_process_running "$acp_pid"; then
    log_success "ACP: running (PID: ${acp_pid}) ${ACP_HOST}:${acp_port}"
  else
    log_warn "ACP: stopped"
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

  ensure_playwright_browsers

  log_info "Running web tests..."
  NEXT_DISABLE_GOOGLE_FONTS=1 npm --prefix "${SCRIPT_DIR}/web" test
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
elephant.ai dev helper

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
