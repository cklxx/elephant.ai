#!/usr/bin/env bash
###############################################################################
# elephant.ai - Local Development Helper
#
# Usage:
#   ./dev.sh                    # Start backend + web + lark (background)
#   ./dev.sh up|start           # Start backend + web only
#   ./dev.sh all-up             # Start backend + web + lark (separate stacks)
#   ./dev.sh sandbox-up         # Start sandbox + ACP only
#   ./dev.sh sandbox-down       # Stop sandbox + ACP only
#   ./dev.sh sandbox-status     # Show sandbox + ACP status only
#   ./dev.sh down|stop          # Stop backend + web
#   ./dev.sh status             # Show status + ports
#   ./dev.sh logs [server|web]  # Tail logs
#   ./dev.sh lark-up            # Start lark supervisor stack only
#   ./dev.sh lark-down          # Stop lark supervisor stack only
#   ./dev.sh lark-status        # Show lark supervisor status only
#   ./dev.sh lark-logs          # Tail lark supervisor logs
#   ./dev.sh logs-ui            # Start services and open diagnostics workbench
#   ./dev.sh test               # Go tests (CI parity)
#   ./dev.sh lint               # Go + web lint
#
# Env:
#   SERVER_PORT=8080            # Backend port override (default 8080)
#   WEB_PORT=3000               # Web port override (default 3000)
#   SANDBOX_PORT=18086          # Sandbox port override (default 18086)
#   SANDBOX_IMAGE=...           # Sandbox image override
#   SANDBOX_BASE_URL=...        # Sandbox base URL override (default http://localhost:18086)
#   SANDBOX_AUTO_INSTALL_CLI=1  # Auto-install Codex/Claude Code in sandbox (default 1)
#   START_ACP_WITH_SANDBOX=1    # Start ACP serve alongside sandbox (default 1)
#   ACP_RUN_MODE=sandbox|host   # Run ACP in sandbox container or on host (default sandbox)
#   ACP_PORT=0                  # ACP port override (0 = auto-pick)
#   ACP_HOST=127.0.0.1           # ACP bind host (default 127.0.0.1)
#   AUTO_STOP_CONFLICTING_PORTS=1 # Auto-stop our backend/web conflicts (default 1)
#   AUTH_JWT_SECRET=...         # Auth secret (default: dev-secret-change-me)
#   SKIP_LOCAL_AUTH_DB=1        # Skip local auth DB auto-setup (default 0)
#   ALEX_CGO_MODE=auto|on|off    # Auto-select CGO for builds (default auto)
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
readonly DEFAULT_SANDBOX_CONFIG_PATH="/root/.alex/config.yaml"

SERVER_PORT="${SERVER_PORT:-${DEFAULT_SERVER_PORT}}"
WEB_PORT="${WEB_PORT:-${DEFAULT_WEB_PORT}}"
SANDBOX_PORT="${SANDBOX_PORT:-${DEFAULT_SANDBOX_PORT}}"
SANDBOX_IMAGE="${SANDBOX_IMAGE:-${DEFAULT_SANDBOX_IMAGE}}"
SANDBOX_BASE_URL="${SANDBOX_BASE_URL:-http://localhost:${SANDBOX_PORT}}"
SANDBOX_CONTAINER_NAME="${SANDBOX_CONTAINER_NAME:-alex-sandbox}"
SANDBOX_AUTO_INSTALL_CLI="${SANDBOX_AUTO_INSTALL_CLI:-1}"
START_ACP_WITH_SANDBOX="${START_ACP_WITH_SANDBOX:-1}"
ACP_RUN_MODE="${ACP_RUN_MODE:-sandbox}"
ACP_PORT="${ACP_PORT:-0}"
ACP_HOST="${ACP_HOST:-${DEFAULT_ACP_HOST}}"
AUTO_STOP_CONFLICTING_PORTS="${AUTO_STOP_CONFLICTING_PORTS:-1}"

source "${SCRIPT_DIR}/scripts/lib/common/logging.sh"
source "${SCRIPT_DIR}/scripts/lib/common/process.sh"
source "${SCRIPT_DIR}/scripts/lib/common/ports.sh"
source "${SCRIPT_DIR}/scripts/lib/common/http.sh"
source "${SCRIPT_DIR}/scripts/lib/common/sandbox.sh"
source "${SCRIPT_DIR}/scripts/lib/common/cgo.sh"
source "${SCRIPT_DIR}/scripts/lib/acp_host.sh"

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

readonly BOOTSTRAP_MARKER="${PID_DIR}/bootstrap.done"
readonly LARK_SUPERVISOR_SH="${SCRIPT_DIR}/scripts/lark/supervisor.sh"

ensure_local_bootstrap() {
  if [[ -f "${BOOTSTRAP_MARKER}" ]]; then
    return 0
  fi
  local bootstrap_sh="${SCRIPT_DIR}/scripts/setup_local_runtime.sh"
  if [[ ! -x "${bootstrap_sh}" ]]; then
    die "Missing bootstrap script: ${bootstrap_sh}"
  fi
  MAIN_CONFIG="${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}" \
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

sandbox_host_config_path() {
  if [[ -n "${ALEX_CONFIG_PATH:-}" ]]; then
    echo "${ALEX_CONFIG_PATH}"
    return 0
  fi
  echo "${HOME}/.alex/config.yaml"
}

rewrite_base_url_for_sandbox() {
  local url="$1"
  if [[ -z "$url" ]]; then
    return 0
  fi
  local rewritten="$url"
  if [[ "$rewritten" =~ ^https?://(localhost|127\\.0\\.0\\.1|0\\.0\\.0\\.0)(:|/|$) ]]; then
    rewritten="${rewritten/://localhost/:\/\/host.docker.internal}"
    rewritten="${rewritten/://127.0.0.1/:\/\/host.docker.internal}"
    rewritten="${rewritten/://0.0.0.0/:\/\/host.docker.internal}"
  fi
  echo "$rewritten"
}

read_config_base_url() {
  local config_path="$1"
  if [[ -z "$config_path" || ! -f "$config_path" ]]; then
    return 0
  fi
  local value
  value="$(awk -F: '/^[[:space:]]*base_url[[:space:]]*:/ {sub(/^[^:]*:[[:space:]]*/, "", $0); print $0; exit}' "$config_path")"
  value="${value#\"}"
  value="${value%\"}"
  value="${value#\'}"
  value="${value%\'}"
  echo "$value"
}

acp_should_run_in_sandbox() {
  if [[ "${START_ACP_WITH_SANDBOX}" != "1" ]]; then
    return 1
  fi
  if [[ "${ACP_RUN_MODE}" != "sandbox" ]]; then
    return 1
  fi
  if ! is_local_sandbox_url; then
    return 1
  fi
  if ! command_exists docker; then
    return 1
  fi
  return 0
}

sandbox_has_acp_port_mapping() {
  local port="$1"
  if [[ -z "$port" ]]; then
    return 1
  fi
  docker port "${SANDBOX_CONTAINER_NAME}" "${port}/tcp" >/dev/null 2>&1
}

detect_sandbox_acp_port() {
  docker port "${SANDBOX_CONTAINER_NAME}" 2>/dev/null | awk -v sp="${SANDBOX_PORT}" '
    match($0, /:([0-9]+)$/, m) {
      port = m[1];
      if (port != sp) {
        print port;
        exit;
      }
    }
  '
}

collect_sandbox_env_flags() {
  SANDBOX_ENV_FLAGS=()
  SANDBOX_HAS_LLM_ENV=0
  local base_url_override=""
  local keys=(
    LLM_PROVIDER
    LLM_MODEL
    LLM_SMALL_PROVIDER
    LLM_SMALL_MODEL
    LLM_VISION_MODEL
    LLM_BASE_URL
    OPENAI_API_KEY
    OPENAI_BASE_URL
    ANTHROPIC_API_KEY
    ANTHROPIC_BASE_URL
    CODEX_API_KEY
    CODEX_BASE_URL
    ARK_API_KEY
    LLM_BASE_URL
    SEEDREAM_TEXT_ENDPOINT_ID
    SEEDREAM_IMAGE_ENDPOINT_ID
    SEEDREAM_TEXT_MODEL
    SEEDREAM_IMAGE_MODEL
    SEEDREAM_VISION_MODEL
    SEEDREAM_VIDEO_MODEL
  )
  local key val
  for key in "${keys[@]}"; do
    val="${!key-}"
    if [[ -n "$val" ]]; then
      if [[ "$key" == *"_BASE_URL" || "$key" == "LLM_BASE_URL" ]]; then
        val="$(rewrite_base_url_for_sandbox "$val")"
        base_url_override="$val"
      fi
      SANDBOX_ENV_FLAGS+=("-e" "${key}=${val}")
      SANDBOX_HAS_LLM_ENV=1
    fi
  done

  if [[ -z "$base_url_override" ]]; then
    local host_config base_url
    host_config="$(sandbox_host_config_path)"
    base_url="$(read_config_base_url "$host_config")"
    if [[ -n "$base_url" ]]; then
      local rewritten
      rewritten="$(rewrite_base_url_for_sandbox "$base_url")"
      if [[ -n "$rewritten" && "$rewritten" != "$base_url" ]]; then
        SANDBOX_ENV_FLAGS+=("-e" "LLM_BASE_URL=${rewritten}")
      fi
    fi
  fi
}

ensure_sandbox_acp_config() {
  SANDBOX_CONFIG_UPDATED=0
  SANDBOX_CONFIG_FOUND=0
  local host_config
  host_config="$(sandbox_host_config_path)"
  if [[ -z "$host_config" || ! -f "$host_config" ]]; then
    return 0
  fi
  SANDBOX_CONFIG_FOUND=1
  docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc 'mkdir -p /root/.alex' >/dev/null
  docker cp "${host_config}" "${SANDBOX_CONTAINER_NAME}:/tmp/alex-config.yaml" >/dev/null
  local result
  result="$(
    docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc '
      if command -v cmp >/dev/null 2>&1 && [ -f '"${DEFAULT_SANDBOX_CONFIG_PATH}"' ]; then
        if cmp -s '"${DEFAULT_SANDBOX_CONFIG_PATH}"' /tmp/alex-config.yaml; then
          rm -f /tmp/alex-config.yaml
          echo same
          exit 0
        fi
      fi
      mv /tmp/alex-config.yaml '"${DEFAULT_SANDBOX_CONFIG_PATH}"'
      chmod 600 '"${DEFAULT_SANDBOX_CONFIG_PATH}"'
      echo updated
    '
  )"
  if [[ "$result" == "updated" ]]; then
    log_info "Updated sandbox alex config from host."
    SANDBOX_CONFIG_UPDATED=1
  fi
}

ensure_sandbox_acp_binary() {
  SANDBOX_ACP_BIN_UPDATED=0
  local arch
  arch="$(docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc 'uname -m' 2>/dev/null || true)"
  local goarch="amd64"
  case "$arch" in
    aarch64|arm64)
      goarch="arm64"
      ;;
    x86_64|amd64)
      goarch="amd64"
      ;;
  esac

  local alex_bin
  local had_alex=0
  if docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc 'command -v alex >/dev/null 2>&1'; then
    had_alex=1
  fi
  local out_bin="${PID_DIR}/alex-linux-${goarch}"
  local rebuild=0
  if [[ ! -x "${out_bin}" ]]; then
    rebuild=1
  elif find "${SCRIPT_DIR}/cmd" "${SCRIPT_DIR}/internal" -name '*.go' -newer "${out_bin}" | head -n 1 | grep -q .; then
    rebuild=1
  fi
  if [[ "${rebuild}" == "1" ]]; then
    if ! command_exists "${SCRIPT_DIR}/scripts/go-with-toolchain.sh"; then
      die "go toolchain not available to build linux alex binary"
    fi
    log_info "Building linux alex (${goarch}) for sandbox..."
    GOOS=linux GOARCH="${goarch}" CGO_ENABLED=0 \
      "${SCRIPT_DIR}/scripts/go-with-toolchain.sh" build -o "${out_bin}" ./cmd/alex >/dev/null
  fi
  alex_bin="${out_bin}"

  log_info "Copying alex CLI into sandbox..."
  docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc 'mkdir -p /usr/local/bin' >/dev/null
  docker cp "${alex_bin}" "${SANDBOX_CONTAINER_NAME}:/usr/local/bin/alex" >/dev/null
  docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc 'chmod +x /usr/local/bin/alex' >/dev/null
  if [[ "${rebuild}" == "1" || "${had_alex}" == "0" ]]; then
    SANDBOX_ACP_BIN_UPDATED=1
  fi
}

start_acp_daemon_in_sandbox() {
  if ! acp_should_run_in_sandbox; then
    return 0
  fi

  ensure_dirs
  collect_sandbox_env_flags

  if [[ -z "${ACP_PORT}" || "${ACP_PORT}" == "0" ]]; then
    local port
    port="$(ensure_acp_port)" || die "Failed to allocate ACP port"
    ACP_PORT="$port"
  fi

  ensure_sandbox_acp_binary
  ensure_sandbox_acp_config
  local force_restart=0
  if [[ "${SANDBOX_CONFIG_UPDATED}" == "1" || "${SANDBOX_ACP_BIN_UPDATED:-0}" == "1" ]]; then
    force_restart=1
  fi

  log_info "Starting ACP daemon inside sandbox on 0.0.0.0:${ACP_PORT}..."
  docker exec "${SANDBOX_ENV_FLAGS[@]}" -e ACP_PORT="${ACP_PORT}" -e ACP_FORCE_RESTART="${force_restart}" "${SANDBOX_CONTAINER_NAME}" sh -lc '
    if [ -f /tmp/acp.pid ] && kill -0 $(cat /tmp/acp.pid) 2>/dev/null; then
      if [ "${ACP_FORCE_RESTART}" = "1" ]; then
        kill "$(cat /tmp/acp.pid)" 2>/dev/null || true
        rm -f /tmp/acp.pid
      else
        exit 0
      fi
    fi
    if [ -d /workspace ]; then
      cd /workspace
    fi
    nohup /usr/local/bin/alex acp serve --host 0.0.0.0 --port "${ACP_PORT}" >/tmp/acp.log 2>&1 &
    echo $! >/tmp/acp.pid
  '
}

stop_acp_daemon_in_sandbox() {
  if ! acp_should_run_in_sandbox; then
    return 0
  fi
  if ! docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    return 0
  fi
  docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc '
    if [ -f /tmp/acp.pid ]; then
      pid=$(cat /tmp/acp.pid)
      kill "$pid" 2>/dev/null || true
      rm -f /tmp/acp.pid
    fi
  ' >/dev/null 2>&1 || true
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

start_sandbox() {
  if ! is_local_sandbox_url; then
    wait_for_health "${SANDBOX_BASE_URL}/v1/docs" "sandbox"
    return $?
  fi

  if ! command_exists docker; then
    log_error "docker not found; cannot start sandbox"
    return 1
  fi

  ensure_dirs
  local workspace_dir=""
  workspace_dir="$(sandbox_workspace_dir || true)"
  if [[ -n "$workspace_dir" && ! -d "$workspace_dir" ]]; then
    log_warn "Sandbox workspace dir ${workspace_dir} not found; skipping mount"
    workspace_dir=""
  fi
  local container_running=0
  if docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    container_running=1
  fi
  local container_exists=0
  if docker ps -a --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    container_exists=1
  fi

  local run_acp_in_sandbox=0
  if acp_should_run_in_sandbox; then
    run_acp_in_sandbox=1
  fi

  if [[ "$container_exists" == "1" && -n "$workspace_dir" ]] && ! sandbox_has_workspace_mount "$workspace_dir"; then
    log_warn "Sandbox container missing workspace mount; recreating..."
    if [[ "$container_running" == "1" ]]; then
      docker stop "${SANDBOX_CONTAINER_NAME}" >/dev/null
    fi
    docker rm "${SANDBOX_CONTAINER_NAME}" >/dev/null
    container_exists=0
    container_running=0
  fi

  if [[ "$run_acp_in_sandbox" == "1" ]]; then
    local port
    if [[ -n "${ACP_PORT}" && "${ACP_PORT}" != "0" ]]; then
      if [[ "$container_running" == "0" ]]; then
        port="$(ensure_acp_port)" || die "Failed to allocate ACP port"
        ACP_PORT="$port"
      else
        if [[ ! -f "$ACP_PORT_FILE" ]]; then
          echo "${ACP_PORT}" >"$ACP_PORT_FILE"
        fi
      fi
    elif [[ "$container_running" == "1" ]]; then
      if ! load_acp_port_file >/dev/null; then
        port="$(detect_sandbox_acp_port || true)"
        if [[ -n "$port" ]]; then
          ACP_PORT="$port"
          echo "${port}" >"$ACP_PORT_FILE"
        fi
      fi
    fi

    if [[ -z "${ACP_PORT}" || "${ACP_PORT}" == "0" ]]; then
      port="$(ensure_acp_port)" || die "Failed to allocate ACP port"
      ACP_PORT="$port"
    fi
  fi

  if [[ "$container_exists" == "1" && "$container_running" == "0" && "$run_acp_in_sandbox" == "1" ]]; then
    if ! sandbox_has_acp_port_mapping "${ACP_PORT}"; then
      log_warn "Sandbox container missing ACP port mapping; recreating..."
      docker rm "${SANDBOX_CONTAINER_NAME}" >/dev/null
      container_exists=0
    fi
  fi

  local acp_container_host="host.docker.internal"
  local acp_addr="${acp_container_host}:${ACP_PORT}"

  if [[ "$container_running" == "1" ]]; then
    if [[ "$run_acp_in_sandbox" == "1" ]] && ! sandbox_has_acp_port_mapping "${ACP_PORT}"; then
      log_warn "Sandbox container missing ACP port mapping; recreating..."
      docker stop "${SANDBOX_CONTAINER_NAME}" >/dev/null
      docker rm "${SANDBOX_CONTAINER_NAME}" >/dev/null
      container_running=0
      container_exists=0
    else
      if sandbox_ready; then
        log_success "Sandbox running (container ${SANDBOX_CONTAINER_NAME})"
      else
        wait_for_health "http://localhost:${SANDBOX_PORT}/v1/docs" "sandbox"
      fi
      ensure_sandbox_cli_tools
      if [[ "$run_acp_in_sandbox" == "1" ]]; then
        start_acp_daemon_in_sandbox
      else
        start_acp_daemon_host
        log_info "ACP server injected at ${acp_addr}"
      fi
      return $?
    fi
  fi

  if docker ps -a --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    log_info "Starting sandbox container ${SANDBOX_CONTAINER_NAME}..."
    docker start "${SANDBOX_CONTAINER_NAME}" >/dev/null
  else
    log_info "Starting sandbox container ${SANDBOX_CONTAINER_NAME} on :${SANDBOX_PORT}..."
    local port_flags=("-p" "${SANDBOX_PORT}:8080")
    if [[ "$run_acp_in_sandbox" == "1" ]]; then
      port_flags+=("-p" "${ACP_PORT}:${ACP_PORT}")
    fi
    local volume_flags=()
    if [[ -n "$workspace_dir" ]]; then
      volume_flags+=("-v" "${workspace_dir}:/workspace")
    fi
    docker run -d --name "${SANDBOX_CONTAINER_NAME}" \
      --add-host "host.docker.internal:host-gateway" \
      -e "ACP_SERVER_HOST=${acp_container_host}" \
      -e "ACP_SERVER_PORT=${ACP_PORT}" \
      -e "ACP_SERVER_ADDR=${acp_addr}" \
      "${port_flags[@]}" \
      "${volume_flags[@]}" \
      "${SANDBOX_IMAGE}" >/dev/null
  fi

  wait_for_health "http://localhost:${SANDBOX_PORT}/v1/docs" "sandbox"
  ensure_sandbox_cli_tools
  if [[ "$run_acp_in_sandbox" == "1" ]]; then
    start_acp_daemon_in_sandbox
  else
    start_acp_daemon_host
    log_info "ACP server injected at ${acp_addr}"
  fi
}

stop_sandbox() {
  if ! is_local_sandbox_url; then
    return 0
  fi
  if ! command_exists docker; then
    return 0
  fi
  if acp_should_run_in_sandbox; then
    stop_acp_daemon_in_sandbox
  else
    stop_acp_daemon_host
  fi
  if docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    log_info "Stopping sandbox container ${SANDBOX_CONTAINER_NAME}..."
    docker stop "${SANDBOX_CONTAINER_NAME}" >/dev/null
  fi
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
  log_info "Setting up local auth DB..."
  if "${SCRIPT_DIR}/scripts/setup_local_auth_db.sh" >"${LOG_DIR}/setup_auth_db.log" 2>&1; then
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
  cgo_apply_mode
  if [[ "${CGO_ENABLED:-}" == "1" ]]; then
    log_info "CGO enabled for build (ALEX_CGO_MODE=$(cgo_mode))"
  else
    log_info "CGO disabled for build (ALEX_CGO_MODE=$(cgo_mode))"
  fi
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
  local acp_executor_addr=""
  if acp_executor_addr="$(resolve_acp_executor_addr)"; then
    log_info "Using ACP executor at ${acp_executor_addr}"
  fi
  PORT="${SERVER_PORT}" \
    ALEX_SERVER_PORT="${SERVER_PORT}" \
    ALEX_SERVER_MODE="deploy" \
    ALEX_LOG_DIR="${LOG_DIR}" \
    ACP_EXECUTOR_ADDR="${acp_executor_addr}" \
    "${SCRIPT_DIR}/alex-server" \
    >"${SERVER_LOG}" 2>&1 &

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
  ensure_local_bootstrap
  log_section "Sandbox"
  start_sandbox
  log_section "Auth DB"
  maybe_setup_auth_db
  log_section "Backend"
  start_server
  log_section "Web"
  start_web
  echo ""
  log_section "Services"
  log_success "Backend     http://localhost:${SERVER_PORT}"
  log_success "Web         http://localhost:${WEB_PORT}"
  if sandbox_ready; then
    log_success "Sandbox     ${SANDBOX_BASE_URL}"
  else
    log_warn  "Sandbox     ${SANDBOX_BASE_URL} (unavailable)"
  fi
  echo ""
  log_section "Dev Tools"
  log_info "Evaluation           http://localhost:${WEB_PORT}/evaluation"
  log_info "Sessions             http://localhost:${WEB_PORT}/sessions"
  log_info "Diagnostics         http://localhost:${WEB_PORT}/dev/diagnostics"
  log_info "Configuration       http://localhost:${WEB_PORT}/dev/configuration"
  log_info "Operations          http://localhost:${WEB_PORT}/dev/operations"
}

cmd_sandbox_up() {
  ensure_local_bootstrap
  start_sandbox
  log_success "Sandbox ready: ${SANDBOX_BASE_URL}"
}

cmd_sandbox_down() {
  stop_sandbox
  log_success "Sandbox stopped"
}

cmd_sandbox_status() {
  if sandbox_ready; then
    log_success "Sandbox: ready ${SANDBOX_BASE_URL}"
  else
    log_warn "Sandbox: unavailable ${SANDBOX_BASE_URL}"
  fi

  local acp_pid acp_port
  acp_pid="$(read_pid "$ACP_PID_FILE" || true)"
  acp_port="$(cat "$ACP_PORT_FILE" 2>/dev/null || true)"
  if acp_should_run_in_sandbox; then
    if docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
      if docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc 'test -f /tmp/acp.pid && kill -0 $(cat /tmp/acp.pid) 2>/dev/null'; then
        log_success "ACP: running (sandbox) http://localhost:${acp_port}"
      else
        log_warn "ACP: stopped (sandbox)"
      fi
    else
      log_warn "ACP: sandbox container not running"
    fi
  else
    if is_process_running "$acp_pid"; then
      log_success "ACP: running (PID: ${acp_pid}) ${ACP_HOST}:${acp_port}"
    else
      log_warn "ACP: stopped"
    fi
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
  log_info "Keeping sandbox/authdb running (use ./dev.sh down-all to stop everything)"
}

cmd_down_all() {
  cmd_down
  stop_sandbox
  rm -f "${BOOTSTRAP_MARKER}"
}

require_lark_supervisor() {
  [[ -x "${LARK_SUPERVISOR_SH}" ]] || die "Missing ${LARK_SUPERVISOR_SH}"
}

cmd_lark_up() {
  ensure_local_bootstrap
  require_lark_supervisor
  log_section "Lark"
  "${LARK_SUPERVISOR_SH}" start
}

cmd_lark_down() {
  require_lark_supervisor
  "${LARK_SUPERVISOR_SH}" stop
}

cmd_lark_status() {
  require_lark_supervisor
  "${LARK_SUPERVISOR_SH}" status
}

cmd_lark_logs() {
  require_lark_supervisor
  "${LARK_SUPERVISOR_SH}" logs
}

cmd_all_up() {
  cmd_up
  echo ""
  cmd_lark_up
}

cmd_all_down() {
  cmd_lark_down
  cmd_down
}

cmd_all_status() {
  cmd_status
  echo ""
  cmd_lark_status
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
  if acp_should_run_in_sandbox; then
    if docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
      if docker exec "${SANDBOX_CONTAINER_NAME}" sh -lc 'test -f /tmp/acp.pid && kill -0 $(cat /tmp/acp.pid) 2>/dev/null'; then
        log_success "ACP: running (sandbox) http://localhost:${acp_port}"
      else
        log_warn "ACP: stopped (sandbox)"
      fi
    else
      log_warn "ACP: sandbox container not running"
    fi
  else
    if is_process_running "$acp_pid"; then
      log_success "ACP: running (PID: ${acp_pid}) ${ACP_HOST}:${acp_port}"
    else
      log_warn "ACP: stopped"
    fi
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
  ./dev.sh [command]

Commands:
  up|start       Start backend + web only (background)
  all-up         Start backend + web + lark (isolated stacks)
  all-down       Stop backend + web + lark (keeps sandbox/authdb running)
  all-status     Show status for backend/web/sandbox + lark
  sandbox-up     Start sandbox + ACP only
  sandbox-down   Stop sandbox + ACP only
  sandbox-status Show sandbox + ACP status
  down|stop      Stop backend + web (keeps sandbox/authdb running)
  down-all       Stop everything including sandbox + authdb
  status         Show status + ports
  logs           Tail logs (optional: server|web)
  lark-up        Start lark supervisor stack only
  lark-down      Stop lark supervisor stack only
  lark-status    Show lark supervisor status
  lark-logs      Tail lark supervisor logs
  logs-ui        Start services and open the diagnostics workbench
  test           Run Go tests (CI parity)
  lint           Run Go + web lint
  setup-cgo      Install CGO sqlite dependencies
EOF
}

cmd="${1:-all-up}"
shift || true

case "$cmd" in
  up|start) cmd_up ;;
  all-up) cmd_all_up ;;
  all-down) cmd_all_down ;;
  all-status) cmd_all_status ;;
  sandbox-up) cmd_sandbox_up ;;
  sandbox-down) cmd_sandbox_down ;;
  sandbox-status) cmd_sandbox_status ;;
  down|stop) cmd_down ;;
  down-all|stop-all) cmd_down_all ;;
  status) cmd_status ;;
  logs) cmd_logs "${@:-all}" ;;
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
