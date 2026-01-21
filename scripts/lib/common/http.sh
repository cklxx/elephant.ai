#!/usr/bin/env bash
# shellcheck shell=bash
# Common HTTP helpers.

wait_for_health() {
  local url="$1"
  local name="$2"
  local attempts="${3:-30}"

  if ! command_exists curl; then
    log_warn "curl not found; skipping ${name} readiness check"
    return 0
  fi

  log_info "Waiting for ${name} to be ready..."
  local i
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
