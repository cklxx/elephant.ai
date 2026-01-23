#!/usr/bin/env bash
# shellcheck shell=bash
# Common port helpers.

is_port_available() {
  local port="$1"
  ! lsof -ti tcp:"$port" -sTCP:LISTEN >/dev/null 2>&1
}

kill_process_on_port() {
  local port="$1"

  if ! command_exists lsof; then
    log_warn "lsof not found; cannot kill listeners on port ${port}"
    return 0
  fi

  local pids
  pids=$(lsof -i ":$port" -sTCP:LISTEN -t 2>/dev/null || true)

  if [[ -n "$pids" ]]; then
    log_warn "Port $port is in use, killing processes: $pids"
    echo "$pids" | xargs kill -9 2>/dev/null || true
    sleep 1
  fi
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
