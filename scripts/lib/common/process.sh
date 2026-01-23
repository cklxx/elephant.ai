#!/usr/bin/env bash
# shellcheck shell=bash
# Common process helpers.

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
  local attempts="${3:-20}"
  local sleep_seconds="${4:-0.25}"

  if ! is_process_running "$pid"; then
    return 0
  fi

  log_info "Stopping ${label} (PID: ${pid})"
  kill "$pid" 2>/dev/null || true

  local i
  for i in $(seq 1 "$attempts"); do
    if ! is_process_running "$pid"; then
      return 0
    fi
    sleep "$sleep_seconds"
  done

  log_warn "${label} did not stop gracefully; force killing (PID: ${pid})"
  kill -9 "$pid" 2>/dev/null || true
  return 0
}

stop_service() {
  local name="$1"
  local pid_file="$2"
  local attempts="${3:-20}"
  local sleep_seconds="${4:-0.25}"
  local pid

  pid="$(read_pid "$pid_file" || true)"

  if is_process_running "$pid"; then
    log_info "Stopping ${name} (PID: ${pid})"
    kill "$pid" 2>/dev/null || true

    local i
    for i in $(seq 1 "$attempts"); do
      if ! is_process_running "$pid"; then
        rm -f "$pid_file"
        log_success "${name} stopped"
        return 0
      fi
      sleep "$sleep_seconds"
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
