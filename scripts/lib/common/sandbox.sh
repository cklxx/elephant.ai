#!/usr/bin/env bash
# shellcheck shell=bash
# Shared sandbox helpers.

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

sandbox_workspace_dir() {
  if [[ -n "${SANDBOX_WORKSPACE_DIR:-}" ]]; then
    echo "${SANDBOX_WORKSPACE_DIR}"
    return 0
  fi
  if [[ -n "${SCRIPT_DIR:-}" ]]; then
    echo "${SCRIPT_DIR}"
    return 0
  fi
  return 1
}

sandbox_has_workspace_mount() {
  local host_dir="$1"
  if [[ -z "$host_dir" ]]; then
    return 0
  fi
  if ! command_exists docker; then
    return 1
  fi
  docker inspect --format '{{range .Mounts}}{{printf "%s %s\n" .Source .Destination}}{{end}}' "${SANDBOX_CONTAINER_NAME}" 2>/dev/null | awk -v src="$host_dir" '$2 == "/workspace" && $1 == src {found=1} END {exit found ? 0 : 1}'
}
