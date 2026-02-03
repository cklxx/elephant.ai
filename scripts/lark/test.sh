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
  TEST_PORT            Healthcheck port override (default: from config; fallback 8080)
  ALEX_LOG_DIR         Internal log dir override (default: <repo>/.worktrees/test/logs)
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
PID_FILE="${TEST_ROOT}/.pids/lark-test.pid"
LOG_FILE="${TEST_ROOT}/logs/lark-test.log"
TEST_CONFIG="${TEST_CONFIG:-$HOME/.alex/test.yaml}"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${TEST_ROOT}/logs}"

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
  mkdir -p "${TEST_ROOT}/.pids" "${TEST_ROOT}/logs"
}

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

build() {
  ensure_worktree
  # Keep the test worktree aligned with the latest main snapshot before building.
  git -C "${TEST_ROOT}" switch test >/dev/null 2>&1 || true
  git -C "${TEST_ROOT}" reset --hard main >/dev/null 2>&1 || true
  log_info "Building alex-server (test worktree)..."
  (cd "${TEST_ROOT}" && CGO_ENABLED=0 go build -o "${BIN}" ./cmd/alex-server)
  log_success "Built ${BIN}"
}

sanitize_port() {
  local port="$1"
  if [[ "${port}" =~ ^[0-9]+$ ]]; then
    echo "${port}"
  fi
}

resolve_health_url() {
  local inferred_port health_port
  inferred_port="$(infer_port_from_config "${TEST_CONFIG}" || true)"
  inferred_port="$(sanitize_port "${inferred_port}")"
  health_port="$(sanitize_port "${TEST_PORT:-}")"
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
      log_success "Adopted running test server PID: ${pid}"
    fi
  fi
}

start() {
  [[ -f "${TEST_CONFIG}" ]] || die "Missing TEST_CONFIG: ${TEST_CONFIG}"

  maybe_setup_auth_db
  ensure_worktree

  local health_url
  health_url="$(resolve_health_url)"
  if curl -sf "${health_url}" >/dev/null 2>&1; then
    adopt_pid_if_missing || true
    log_success "Test server already healthy: ${health_url}"
    return 0
  fi

  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    log_success "Test server already running (PID: ${pid})"
    return 0
  fi

  build

  log_info "Starting test server..."
  (cd "${TEST_ROOT}" && ALEX_CONFIG_PATH="${TEST_CONFIG}" ALEX_LOG_DIR="${ALEX_LOG_DIR}" nohup "${BIN}" >> "${LOG_FILE}" 2>&1 & echo "$!" > "${PID_FILE}")

  pid="$(read_pid "${PID_FILE}" || true)"
  local i
  for i in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      log_error "Test server exited early (see ${LOG_FILE})"
      return 1
    fi
    if curl -sf "${health_url}" >/dev/null 2>&1; then
      log_success "Test server healthy: ${health_url}"
      return 0
    fi
    sleep 1
  done

  log_error "Test server failed to become healthy within 30s (see ${LOG_FILE})"
  return 1
}

stop() {
  ensure_worktree
  adopt_pid_if_missing || true
  stop_service "Test server" "${PID_FILE}"
}

status() {
  ensure_worktree
  local health_url pid
  health_url="$(resolve_health_url)"

  adopt_pid_if_missing || true
  pid="$(read_pid "${PID_FILE}" || true)"

  if curl -sf "${health_url}" >/dev/null 2>&1; then
    if is_process_running "${pid}"; then
      log_success "Test server healthy (PID: ${pid}) ${health_url}"
    else
      log_success "Test server healthy ${health_url}"
    fi
    return 0
  fi

  if is_process_running "${pid}"; then
    log_warn "Test server running but healthcheck failing (PID: ${pid}) ${health_url}"
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
