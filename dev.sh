#!/usr/bin/env bash
###############################################################################
# elephant.ai - Local Development Helper
#
# Usage:
#   ./dev.sh                    # Start lark (default)
#   ./dev.sh lark [cmd]         # Lark stack (default cmd=up)
#   ./dev.sh up|start           # Start backend + web only
#   ./dev.sh up --lark          # Start backend + web + lark
#   ./dev.sh down|stop          # Stop backend + web
#   ./dev.sh status             # Show status + ports
#   ./dev.sh logs [server|web]  # Tail logs
#   ./dev.sh logs-ui            # Start services and open diagnostics workbench
#   ./dev.sh test               # Go tests (CI parity)
#   ./dev.sh lint               # Go + web lint
#
# Env:
#   SERVER_PORT=8080            # Backend port override (default 8080)
#   WEB_PORT=3000               # Web port override (default 3000)
#   AUTO_STOP_CONFLICTING_PORTS=1 # Auto-stop our backend/web conflicts (default 1)
#   AUTH_JWT_SECRET=...         # Auth secret (default: dev-secret-change-me)
#   ALEX_CGO_MODE=auto|on|off    # Auto-select CGO for builds (default auto)
###############################################################################

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly LOG_DIR="${SCRIPT_DIR}/logs"
readonly SERVER_LOG="${LOG_DIR}/server.log"
readonly WEB_LOG="${LOG_DIR}/web.log"

readonly DEFAULT_SERVER_PORT=8080
readonly DEFAULT_WEB_PORT=3000

SERVER_PORT="${SERVER_PORT:-${DEFAULT_SERVER_PORT}}"
WEB_PORT="${WEB_PORT:-${DEFAULT_WEB_PORT}}"
AUTO_STOP_CONFLICTING_PORTS="${AUTO_STOP_CONFLICTING_PORTS:-1}"

source "${SCRIPT_DIR}/scripts/lib/common/logging.sh"
source "${SCRIPT_DIR}/scripts/lib/common/process.sh"
source "${SCRIPT_DIR}/scripts/lib/common/ports.sh"
source "${SCRIPT_DIR}/scripts/lib/common/http.sh"
source "${SCRIPT_DIR}/scripts/lib/common/cgo.sh"

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

canonicalize_path() {
  local path="$1"
  if [[ "${path}" != /* ]]; then
    path="$(pwd)/${path}"
  fi

  if [[ -e "${path}" ]] && command_exists realpath; then
    realpath "${path}"
    return 0
  fi

  local dir base
  dir="$(dirname "${path}")"
  base="$(basename "${path}")"
  if [[ -d "${dir}" ]]; then
    (
      cd "${dir}" || return 1
      printf '%s/%s\n' "$(pwd -P)" "${base}"
    )
    return 0
  fi

  printf '%s\n' "${path}"
}

resolve_main_config_path() {
  local config_path="${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}"
  canonicalize_path "${config_path}"
}

resolve_shared_pid_dir() {
  local main_config_path="$1"
  if [[ -n "${LARK_PID_DIR:-}" ]]; then
    printf '%s\n' "${LARK_PID_DIR}"
    return 0
  fi
  printf '%s/pids\n' "$(dirname "${main_config_path}")"
}

readonly MAIN_CONFIG_PATH="$(resolve_main_config_path)"
readonly PID_DIR="$(resolve_shared_pid_dir "${MAIN_CONFIG_PATH}")"
readonly SERVER_PID_FILE="${PID_DIR}/server.pid"
readonly WEB_PID_FILE="${PID_DIR}/web.pid"
readonly LARK_SUPERVISOR_PID_FILE="${PID_DIR}/lark-supervisor.pid"
readonly LARK_CAFFEINATE_PID_FILE="${PID_DIR}/lark-caffeinate.pid"

readonly BOOTSTRAP_MARKER="${PID_DIR}/bootstrap.done"
readonly LARK_ENTRY_SH="${SCRIPT_DIR}/lark.sh"

ensure_local_bootstrap() {
  if [[ -f "${BOOTSTRAP_MARKER}" ]]; then
    return 0
  fi
  local bootstrap_sh="${SCRIPT_DIR}/scripts/setup_local_runtime.sh"
  if [[ ! -x "${bootstrap_sh}" ]]; then
    die "Missing bootstrap script: ${bootstrap_sh}"
  fi
  MAIN_CONFIG="${MAIN_CONFIG_PATH}" \
    TEST_CONFIG="${ALEX_TEST_CONFIG_PATH:-$HOME/.alex/test.yaml}" \
    "${bootstrap_sh}" >/dev/null
  ensure_dirs
  date -Iseconds > "${BOOTSTRAP_MARKER}"
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
  local backend_bin="${SCRIPT_DIR}/alex-web"
  local exe

  exe="$(pid_executable_path "$pid" || true)"
  [[ -n "$exe" && "$exe" == "$backend_bin" ]]
}

is_alex_server_pid() {
  local pid="$1"
  local exe

  exe="$(pid_executable_path "$pid" || true)"
  [[ -n "$exe" && ( "$(basename "$exe")" == "alex-server" || "$(basename "$exe")" == "alex-web" ) ]]
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

assert_port_available() {
  local port="$1"
  local name="$2"
  if ! is_port_available "$port"; then
    die_port_in_use "$port" "$name"
  fi
}

cleanup_next_dev_lock() {
  local lock_file="${SCRIPT_DIR}/web/.next/dev/lock"
  if [[ -f "$lock_file" ]]; then
    log_warn "Removing stale Next.js dev lock: ${lock_file}"
    rm -f "$lock_file"
  fi
}

collect_web_process_candidates() {
  ps -axo pid= -o pgid= -o command= | awk -v root="${SCRIPT_DIR}/web" '
    {
      pid=$1
      pgid=$2
      $1=""
      $2=""
      sub(/^  */, "", $0)
      cmd=$0
      if (index(cmd, "npm --prefix " root " run dev") > 0) {
        print pid "\t" pgid "\t" cmd
        next
      }
      if (index(cmd, root) > 0 && index(cmd, "next dev") > 0) {
        print pid "\t" pgid "\t" cmd
      }
    }
  '
}

cleanup_orphan_web_processes() {
  local tracked_pid tracked_pgid pid pgid cmd
  tracked_pid="$(read_pid "$WEB_PID_FILE" || true)"
  tracked_pgid=""
  if is_process_running "$tracked_pid"; then
    tracked_pgid="$(ps -p "$tracked_pid" -o pgid= 2>/dev/null | tr -d '[:space:]' || true)"
  fi

  while IFS=$'\t' read -r pid pgid cmd; do
    [[ -n "${pid}" ]] || continue
    if [[ -n "${tracked_pid}" && "${pid}" == "${tracked_pid}" ]]; then
      continue
    fi
    if [[ -n "${tracked_pgid}" && "${pgid}" == "${tracked_pgid}" ]]; then
      continue
    fi
    log_warn "Stopping orphan web process PID ${pid}"
    stop_pid "${pid}" "orphan web process" 12 0.25 >/dev/null 2>&1 || true
  done < <(collect_web_process_candidates)
}

build_server() {
  log_info "Building backend (./cmd/alex-web + ./cmd/alex-server)..."
  cgo_apply_mode
  if [[ "${CGO_ENABLED:-}" == "1" ]]; then
    log_info "CGO enabled for build (ALEX_CGO_MODE=$(cgo_mode))"
  else
    log_info "CGO disabled for build (ALEX_CGO_MODE=$(cgo_mode))"
  fi
  "${SCRIPT_DIR}/scripts/go-with-toolchain.sh" build -o "${SCRIPT_DIR}/alex-web" ./cmd/alex-web
  [[ -x "${SCRIPT_DIR}/alex-web" ]] || die "Backend build succeeded but ./alex-web is not executable"
  "${SCRIPT_DIR}/scripts/go-with-toolchain.sh" build -o "${SCRIPT_DIR}/alex-server" ./cmd/alex-server
  [[ -x "${SCRIPT_DIR}/alex-server" ]] || die "Backend build succeeded but ./alex-server is not executable"
  log_success "Backend built: ./alex-web + ./alex-server"
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
  PORT="${SERVER_PORT}" \
    ALEX_SERVER_PORT="${SERVER_PORT}" \
    ALEX_SERVER_MODE="deploy" \
    ALEX_LOG_DIR="${LOG_DIR}" \
    "${SCRIPT_DIR}/alex-web" \
    >"${SERVER_LOG}" 2>&1 &

  echo $! >"${SERVER_PID_FILE}"
  wait_for_health "http://localhost:${SERVER_PORT}/health" "backend" || true
  log_success "Backend started (PID: $(cat "${SERVER_PID_FILE}"))"
}

start_web() {
  ensure_dirs
  cleanup_orphan_web_processes

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
  local with_lark=0 arg
  for arg in "$@"; do
    case "${arg}" in
      --lark)
        with_lark=1
        ;;
      *)
        die "Unknown up flag: ${arg} (expected: --lark)"
        ;;
    esac
  done

  ensure_local_bootstrap
  log_section "Backend"
  start_server
  log_section "Web"
  start_web
  echo ""
  log_section "Services"
  log_success "Backend     http://localhost:${SERVER_PORT}"
  log_success "Web         http://localhost:${WEB_PORT}"
  echo ""
  log_section "Dev Tools"
  log_info "Evaluation           http://localhost:${WEB_PORT}/evaluation"
  log_info "Sessions             http://localhost:${WEB_PORT}/sessions"
  log_info "Diagnostics         http://localhost:${WEB_PORT}/dev/diagnostics"
  log_info "Configuration       http://localhost:${WEB_PORT}/dev/configuration"
  log_info "Operations          http://localhost:${WEB_PORT}/dev/operations"

  if (( with_lark )); then
    echo ""
    cmd_lark up
  fi
}

cmd_down() {
  stop_service "Web" "${WEB_PID_FILE}"
  cleanup_next_dev_lock
  if [[ "${AUTO_STOP_CONFLICTING_PORTS}" == "1" ]] && ! is_port_available "$WEB_PORT"; then
    stop_port_listeners "$WEB_PORT" "web"
  fi
  stop_service "Backend" "${SERVER_PID_FILE}"
  stop_alex_server_listeners "$SERVER_PORT"
}

cmd_down_all() {
  cmd_down
  rm -f "${BOOTSTRAP_MARKER}"
}

require_lark_entrypoint() {
  [[ -x "${LARK_ENTRY_SH}" ]] || die "Missing ${LARK_ENTRY_SH}"
}

warn_lark_alias_deprecated() {
  local alias="$1"
  local canonical="$2"
  log_warn "Deprecated command './dev.sh ${alias}'. Use './dev.sh lark ${canonical}' instead."
}

lark_caffeinate_supported() {
  [[ "$(uname -s)" == "Darwin" ]] || return 1
  command_exists caffeinate
}

stop_lark_caffeinate_guard() {
  local pid
  pid="$(read_pid "${LARK_CAFFEINATE_PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    stop_pid "${pid}" "Lark caffeinate guard" 4 0.1 >/dev/null 2>&1 || true
  fi
  rm -f "${LARK_CAFFEINATE_PID_FILE}"
}

start_lark_caffeinate_guard() {
  lark_caffeinate_supported || return 0

  local supervisor_pid
  supervisor_pid="$(read_pid "${LARK_SUPERVISOR_PID_FILE}" || true)"
  if ! is_process_running "${supervisor_pid}"; then
    log_warn "Skip caffeinate guard: Lark supervisor is not running yet"
    return 0
  fi

  local guard_pid
  guard_pid="$(read_pid "${LARK_CAFFEINATE_PID_FILE}" || true)"
  if is_process_running "${guard_pid}"; then
    return 0
  fi

  rm -f "${LARK_CAFFEINATE_PID_FILE}"
  nohup caffeinate -s -w "${supervisor_pid}" >/dev/null 2>&1 &
  echo "$!" > "${LARK_CAFFEINATE_PID_FILE}"
  log_info "Lark caffeinate guard enabled (pid=$(cat "${LARK_CAFFEINATE_PID_FILE}" 2>/dev/null || true), supervisor_pid=${supervisor_pid})"
}

cmd_lark() {
  local subcmd="${1:-up}"
  shift || true
  require_lark_entrypoint
  case "${subcmd}" in
    up|start|restart|cycle)
      ensure_local_bootstrap
      ;;
  esac
  log_section "Lark"
  "${LARK_ENTRY_SH}" "${subcmd}" "$@"
  case "${subcmd}" in
    up|start|restart)
      start_lark_caffeinate_guard
      ;;
    down|stop)
      stop_lark_caffeinate_guard
      ;;
  esac
}

cmd_lark_up() {
  warn_lark_alias_deprecated "lark-up" "up"
  cmd_lark up
}

cmd_lark_down() {
  warn_lark_alias_deprecated "lark-down" "down"
  cmd_lark down
}

cmd_lark_status() {
  warn_lark_alias_deprecated "lark-status" "status"
  cmd_lark status
}

cmd_lark_logs() {
  warn_lark_alias_deprecated "lark-logs" "logs"
  cmd_lark logs
}

cmd_all_up() {
  cmd_up --lark
}

cmd_all_down() {
  cmd_lark down
  cmd_down
}

cmd_all_status() {
  cmd_status
  echo ""
  cmd_lark status
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

probe_http_status() {
  local url="$1"
  if ! command_exists curl; then
    return 1
  fi
  curl -sS --noproxy '*' -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true
}

dev_logs_index_ready() {
  local status
  status="$(probe_http_status "http://localhost:${SERVER_PORT}/api/dev/logs/index?limit=1")"
  case "$status" in
    200)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

dev_logs_ui_ready() {
  local status
  status="$(probe_http_status "http://localhost:${WEB_PORT}/dev/diagnostics")"
  case "$status" in
    200|301|302|307|308)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

cmd_logs_ui() {
  cmd_up

  if command_exists curl; then
    if ! dev_logs_index_ready; then
      log_warn "Dev log index endpoint unavailable; restarting backend to refresh routes..."
      stop_service "Backend" "${SERVER_PID_FILE}"
      stop_alex_server_listeners "$SERVER_PORT"
      start_server
    fi

    if ! dev_logs_index_ready; then
      die "Dev log index is still unavailable at http://localhost:${SERVER_PORT}/api/dev/logs/index (expected 200). Ensure runtime environment is development and backend uses latest code."
    fi

    if ! dev_logs_ui_ready; then
      log_warn "Diagnostics workbench page unavailable; restarting web..."
      stop_service "Web" "${WEB_PID_FILE}"
      cleanup_next_dev_lock
      if [[ "${AUTO_STOP_CONFLICTING_PORTS}" == "1" ]] && ! is_port_available "$WEB_PORT"; then
        stop_port_listeners "$WEB_PORT" "web"
      fi
      start_web
    fi
  else
    log_warn "curl not found; skipping logs-ui readiness probes"
  fi

  local url="http://localhost:${WEB_PORT}/dev/diagnostics"
  log_success "Diagnostics workbench ready: ${url}"

  if command_exists open; then
    open "${url}" >/dev/null 2>&1 || true
    return 0
  fi
  if command_exists xdg-open; then
    xdg-open "${url}" >/dev/null 2>&1 || true
    return 0
  fi
  log_info "Open this URL in your browser: ${url}"
}

cmd_test() {
  log_info "Running Go tests (CI parity)..."
  if [[ -z "${CGO_ENABLED:-}" ]]; then
    if [[ "${ALEX_CGO_MODE:-auto}" == "on" ]]; then
      export CGO_ENABLED=1
    elif [[ "$(uname -s)" == "Darwin" ]]; then
      export CGO_ENABLED=0
    else
      cgo_apply_mode
    fi
  fi
  "${SCRIPT_DIR}/scripts/go-with-toolchain.sh" test -race -covermode=atomic -coverprofile=coverage.out ./...
  log_success "Go tests passed"
}

cmd_setup_cgo() {
  "${SCRIPT_DIR}/scripts/setup_cgo_sqlite.sh"
}

cmd_lint() {
  log_info "Running Go lint..."
  "${SCRIPT_DIR}/scripts/run-golangci-lint.sh" run ./...
  log_success "Go lint passed"

  log_info "Running web lint..."
  local web_dir="${SCRIPT_DIR}/web"
  if [[ ! -x "${web_dir}/node_modules/.bin/eslint" ]]; then
    log_warn "web/node_modules missing or eslint not found; installing web deps..."
    npm --prefix "${web_dir}" ci
  fi
  npm --prefix "${web_dir}" run lint
  log_success "Web lint passed"
}

usage() {
  cat <<EOF
elephant.ai dev helper

Usage:
  ./dev.sh                 # Start lark (default)
  ./dev.sh [command]

Commands:
  lark [cmd]     Manage lark stack (default: up)
                 cmd: up|down|restart|status|logs|doctor|cycle
  up|start       Start backend + web only (background)
  up --lark      Start backend + web + lark
  down|stop      Stop backend + web
  down-all       Stop everything and reset bootstrap
  status         Show status + ports
  logs           Tail logs (optional: server|web)
  logs-ui        Start services and open the diagnostics workbench
  test           Run Go tests (CI parity)
  lint           Run Go + web lint
  setup-cgo      Install CGO sqlite dependencies

Legacy aliases still accepted:
  all-up | all-down | all-status | lark-up | lark-down | lark-status | lark-logs
EOF
}

cmd="${1:-lark}"
shift || true

case "$cmd" in
  up|start) cmd_up "$@" ;;
  all-up) cmd_all_up ;;
  all-down) cmd_all_down ;;
  all-status) cmd_all_status ;;
  down|stop) cmd_down ;;
  down-all|stop-all) cmd_down_all ;;
  status) cmd_status ;;
  logs) cmd_logs "${@:-all}" ;;
  lark) cmd_lark "$@" ;;
  lark-up) cmd_lark_up ;;
  lark-down) cmd_lark_down ;;
  lark-status) cmd_lark_status ;;
  lark-logs) cmd_lark_logs ;;
  logs-ui|log-ui|analyze-logs) cmd_logs_ui ;;
  test) cmd_test ;;
  lint) cmd_lint ;;
  setup-cgo) cmd_setup_cgo ;;
  help|-h|--help) usage ;;
  *) die "Unknown command: ${cmd} (run ./dev.sh help)" ;;
esac
