#!/usr/bin/env bash
# shellcheck shell=bash
# ACP host helpers (shared by dev.sh and deploy.sh).

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

load_acp_port() {
  local pid="${1:-}"
  local port=""

  if [[ -f "$ACP_PORT_FILE" ]]; then
    port="$(cat "$ACP_PORT_FILE" 2>/dev/null || true)"
  fi

  if [[ -z "$port" && -n "$pid" ]]; then
    if command_exists lsof; then
      local addr
      addr="$(lsof -nP -a -p "$pid" -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $9; exit}')"
      port="${addr##*:}"
    fi
  fi

  if [[ -n "$port" ]]; then
    ACP_PORT="$port"
    echo "$port"
    return 0
  fi

  return 1
}

load_acp_port_file() {
  local port=""
  if [[ -f "$ACP_PORT_FILE" ]]; then
    port="$(cat "$ACP_PORT_FILE" 2>/dev/null || true)"
  fi

  if [[ -n "$port" ]]; then
    ACP_PORT="$port"
    return 0
  fi

  return 1
}

resolve_acp_executor_addr() {
  local host="${ACP_HOST:-${DEFAULT_ACP_HOST}}"
  local port="${ACP_PORT:-0}"

  if [[ -z "$port" || "$port" == "0" ]]; then
    if load_acp_port; then
      port="$ACP_PORT"
    fi
  fi

  if [[ -z "$port" || "$port" == "0" ]]; then
    return 1
  fi

  echo "http://${host}:${port}"
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

  local toolchain="${SCRIPT_DIR}/scripts/go-with-toolchain.sh"
  if [[ -x "$toolchain" ]]; then
    log_info "Building CLI (./cmd/alex) with toolchain..."
    if "$toolchain" build -o "${SCRIPT_DIR}/alex" ./cmd/alex >/dev/null; then
      if [[ -x "${SCRIPT_DIR}/alex" ]]; then
        echo "${SCRIPT_DIR}/alex"
        return 0
      fi
    else
      log_warn "Toolchain build failed; falling back to make build"
    fi
  fi

  log_info "Building CLI (./cmd/alex) with make..."
  if ! make build 2>&1 | tee "${LOG_DIR}/build-acp.log"; then
    log_error "CLI build failed, check logs/build-acp.log"
    return 1
  fi

  if [[ -x "${SCRIPT_DIR}/alex" ]]; then
    echo "${SCRIPT_DIR}/alex"
    return 0
  fi

  return 1
}

start_acp_daemon_host() {
  if [[ "${START_ACP_WITH_SANDBOX}" != "1" ]]; then
    return 0
  fi

  mkdir -p "$PID_DIR" "$LOG_DIR"

  local pid
  pid="$(read_pid "$ACP_PID_FILE" || true)"
  if is_process_running "$pid"; then
    load_acp_port "$pid" || log_warn "ACP running but port unknown; set ACP_PORT or remove ${ACP_PORT_FILE}"
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

stop_acp_daemon_host() {
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
