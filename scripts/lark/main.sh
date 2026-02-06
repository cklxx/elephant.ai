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
  scripts/lark/main.sh start|stop|restart|status|logs|build

Env:
  MAIN_CONFIG   Config path (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
  MAIN_PORT     Healthcheck port override (default: from config; fallback 8080)
  ALEX_LOG_DIR  Internal log dir override (default: <repo>/logs)
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
PID_FILE="${ROOT}/.pids/lark-main.pid"
BUILD_STAMP="${ROOT}/.pids/lark-main.build"
LOG_FILE="${ROOT}/logs/lark-main.log"
MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
MAIN_PORT="${MAIN_PORT:-}"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${ROOT}/logs}"
FORCE_REBUILD="${FORCE_REBUILD:-0}"

mkdir -p "${ROOT}/.pids" "${ROOT}/logs" "${ALEX_LOG_DIR}"

infer_port_from_config() {
  local config_path="$1"
  [[ -f "${config_path}" ]] || return 0

  # Best-effort YAML parse:
  # - Look for top-level "server:" and read the first nested "port:".
  awk '
    function ltrim(s){sub(/^[ \t]+/, "", s); return s}
    function indent(s){match(s, /^[ \t]*/); return RLENGTH}
    BEGIN{server_indent=-1}
    {
      if ($0 ~ /^[ \t]*server:[ \t]*$/) {
        server_indent = indent($0)
        next
      }
      if (server_indent >= 0) {
        if (indent($0) <= server_indent && $0 ~ /^[ \t]*[A-Za-z0-9_-]+:[ \t]*/) {
          server_indent = -1
          next
        }
        if ($0 ~ /^[ \t]*port:[ \t]*/) {
          line = ltrim($0)
          sub(/^port:[ \t]*/, "", line)
          sub(/[ \t]#.*/, "", line)
          gsub(/^[\"\047]/, "", line)
          gsub(/[\"\047]$/, "", line)
          print line
          exit
        }
      }
    }
  ' "${config_path}"
}

sanitize_port() {
  local port="$1"
  if [[ "${port}" =~ ^[0-9]+$ ]]; then
    echo "${port}"
  fi
}

resolve_health_url() {
  local inferred_port health_port
  inferred_port="$(infer_port_from_config "${MAIN_CONFIG}" || true)"
  inferred_port="$(sanitize_port "${inferred_port}")"
  health_port="$(sanitize_port "${MAIN_PORT:-}")"
  if [[ -z "${health_port}" ]]; then
    health_port="${inferred_port:-8080}"
  fi
  echo "http://127.0.0.1:${health_port}/health"
}

discover_alex_server_pid_by_port() {
  local health_url port pid cmd
  health_url="$(resolve_health_url)"
  port="${health_url##*:}"
  port="${port%/health}"

  pid="$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t 2>/dev/null | head -n 1 || true)"
  [[ -n "${pid}" ]] || return 0
  cmd="$(ps -p "${pid}" -o command= 2>/dev/null || true)"
  if echo "${cmd}" | grep -q "alex-server"; then
    echo "${pid}"
  fi
}

adopt_pid_if_missing() {
  local pid health_url
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    return 0
  fi

  health_url="$(resolve_health_url)"
  if curl -sf "${health_url}" >/dev/null 2>&1; then
    pid="$(discover_alex_server_pid_by_port || true)"
    if [[ -n "${pid}" ]]; then
      echo "${pid}" > "${PID_FILE}"
      log_success "Adopted running main agent PID: ${pid}"
    fi
  fi
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
  log_success "Built ${BIN}"
}

start() {
  [[ -f "${MAIN_CONFIG}" ]] || die "Missing MAIN_CONFIG: ${MAIN_CONFIG}"

  maybe_setup_auth_db

  local health_url current_fingerprint needs_build
  health_url="$(resolve_health_url)"
  current_fingerprint="$(build_fingerprint "${ROOT}")"
  needs_build=0
  if [[ "${FORCE_REBUILD}" == "1" ]] || [[ ! -x "${BIN}" ]] || is_build_stale "${BUILD_STAMP}" "${current_fingerprint}"; then
    needs_build=1
  fi
  if curl -sf "${health_url}" >/dev/null 2>&1; then
    adopt_pid_if_missing || true
    local adopted_pid
    adopted_pid="$(read_pid "${PID_FILE}" || true)"
    if is_process_running "${adopted_pid}"; then
      if [[ "${needs_build}" == "0" ]]; then
        log_success "Main agent already healthy: ${health_url}"
        return 0
      fi
      log_info "Source changes detected; rebuilding and restarting main agent..."
      # Build first so we don't take down a healthy agent if compilation fails.
      build
      needs_build=0
      stop
    else
      # Health endpoint responds but it is NOT alex-server (port conflict).
      local foreign_port foreign_pid foreign_cmd
      foreign_port="${health_url##*:}"
      foreign_port="${foreign_port%%/*}"
      foreign_pid="$(lsof -nP -iTCP:"${foreign_port}" -sTCP:LISTEN -t 2>/dev/null | head -n 1 || true)"
      foreign_cmd="$(ps -p "${foreign_pid:-0}" -o command= 2>/dev/null || true)"
      log_error "Port ${foreign_port} is occupied by another process (PID: ${foreign_pid:-?}, cmd: ${foreign_cmd:-?})"
      log_error "Kill it or change the main agent port to resolve the conflict"
      return 1
    fi
  fi

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Main agent already running (PID: ${pid})"
    return 0
  fi

  if [[ "${needs_build}" == "1" ]]; then
    build
  else
    log_info "Reusing existing build (no changes detected)."
  fi
  log_info "Starting main agent..."
  ALEX_CONFIG_PATH="${MAIN_CONFIG}" ALEX_LOG_DIR="${ALEX_LOG_DIR}" nohup "${BIN}" >> "${LOG_FILE}" 2>&1 &
  echo "$!" > "${PID_FILE}"

  local i
  pid="$(read_pid "${PID_FILE}" || true)"
  for i in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      log_error "Main agent exited early (see ${LOG_FILE})"
      return 1
    fi
    if curl -sf "${health_url}" >/dev/null 2>&1; then
      log_success "Main agent healthy: ${health_url}"
      return 0
    fi
    sleep 1
  done

  log_error "Main agent failed to become healthy within 30s (see ${LOG_FILE})"
  return 1
}

stop() {
  adopt_pid_if_missing || true
  stop_service "Main agent" "${PID_FILE}"
}

restart() {
  [[ -f "${MAIN_CONFIG}" ]] || die "Missing MAIN_CONFIG: ${MAIN_CONFIG}"

  maybe_setup_auth_db
  build
  stop
  FORCE_REBUILD=0 start
}

status() {
  local health_url pid
  health_url="$(resolve_health_url)"

  adopt_pid_if_missing || true
  pid="$(read_pid "${PID_FILE}" || true)"

  if curl -sf "${health_url}" >/dev/null 2>&1; then
    if is_process_running "${pid}"; then
      log_success "Main agent healthy (PID: ${pid}) ${health_url}"
    else
      log_success "Main agent healthy ${health_url}"
    fi
    return 0
  fi

  if is_process_running "${pid}"; then
    log_warn "Main agent running but healthcheck failing (PID: ${pid}) ${health_url}"
  else
    log_warn "Main agent not running"
  fi
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
