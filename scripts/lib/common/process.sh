#!/usr/bin/env bash
# shellcheck shell=bash
# Common process helpers.

# ---------------------------------------------------------------------------
# Shutdown timeout hierarchy (keep in sync with internal/devops/process/)
# ---------------------------------------------------------------------------
# Process-level SIGTERM grace period: 5 seconds
# Service-level shutdown (process + cleanup): 10 seconds
# Orchestrator total (all services): 30 seconds
readonly SIGTERM_GRACE_SECONDS="${SIGTERM_GRACE_SECONDS:-5}"
readonly SIGTERM_GRACE_ATTEMPTS=$(( SIGTERM_GRACE_SECONDS * 4 ))  # at 0.25s sleep
readonly SIGTERM_GRACE_SLEEP=0.25

read_pid() {
  local pid_file="$1"
  [[ -f "$pid_file" ]] && cat "$pid_file"
}

is_process_running() {
  local pid="${1:-}"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

# _resolve_pgid PID — prints the PGID for a process, or empty if unavailable.
# Used by stop_pid/stop_service to kill the entire process group, preventing
# orphan child processes (matches Go layer's Setpgid + kill(-pgid) behavior).
_resolve_pgid() {
  local pid="$1"
  local pgid
  pgid="$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d '[:space:]' || true)"
  # Don't use PGID 0 (kernel) or 1 (init) — those are not real process groups
  if [[ -n "$pgid" && "$pgid" != "0" && "$pgid" != "1" ]]; then
    printf '%s' "$pgid"
  fi
}

# _signal_process PID SIGNAL — sends SIGNAL to the process group if available,
# otherwise falls back to the individual PID.
_signal_process() {
  local pid="$1"
  local sig="$2"
  local pgid
  pgid="$(_resolve_pgid "$pid")"
  if [[ -n "$pgid" ]]; then
    kill "-${sig}" -- "-${pgid}" 2>/dev/null || kill "-${sig}" "$pid" 2>/dev/null || true
  else
    kill "-${sig}" "$pid" 2>/dev/null || true
  fi
}

stop_pid() {
  local pid="${1:-}"
  local label="${2:-process}"
  local attempts="${3:-${SIGTERM_GRACE_ATTEMPTS}}"
  local sleep_seconds="${4:-${SIGTERM_GRACE_SLEEP}}"

  if ! is_process_running "$pid"; then
    return 0
  fi

  log_info "Stopping ${label} (PID: ${pid})"
  _signal_process "$pid" "TERM"

  local i
  for i in $(seq 1 "$attempts"); do
    if ! is_process_running "$pid"; then
      return 0
    fi
    sleep "$sleep_seconds"
  done

  log_warn "${label} did not stop gracefully; force killing (PID: ${pid})"
  _signal_process "$pid" "KILL"
  return 0
}

stop_service() {
  local name="$1"
  local pid_file="$2"
  local attempts="${3:-${SIGTERM_GRACE_ATTEMPTS}}"
  local sleep_seconds="${4:-${SIGTERM_GRACE_SLEEP}}"
  local pid

  pid="$(read_pid "$pid_file" || true)"

  if is_process_running "$pid"; then
    log_info "Stopping ${name} (PID: ${pid})"
    _signal_process "$pid" "TERM"

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
    _signal_process "$pid" "KILL"
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

# ---------------------------------------------------------------------------
# PID metadata (compatible with Go process.Manager .meta JSON format)
# ---------------------------------------------------------------------------

# write_pid_meta PID_FILE PID — writes PID file and a companion .meta JSON
# file with the process command line. Matches the Go layer's writePIDState.
write_pid_meta() {
  local pid_file="$1"
  local pid="$2"
  local meta_file="${pid_file}.meta"

  printf '%s' "${pid}" > "${pid_file}"

  local cmd_line
  cmd_line="$(ps -ww -o command= -p "${pid}" 2>/dev/null || true)"
  if [[ -n "${cmd_line}" ]]; then
    # Normalize whitespace to match Go normalizeCommandLine
    cmd_line="$(echo "${cmd_line}" | xargs)"
    printf '{"command":"%s"}' "${cmd_line}" > "${meta_file}"
  fi
}

# cleanup_pid_meta PID_FILE — removes PID file and companion .meta file.
cleanup_pid_meta() {
  local pid_file="$1"
  rm -f "${pid_file}" "${pid_file}.meta"
}
