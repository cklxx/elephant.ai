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

ensure_sandbox_cli_tools() {
  if [[ "${SANDBOX_AUTO_INSTALL_CLI:-1}" != "1" ]]; then
    return 0
  fi
  if ! command_exists docker; then
    return 0
  fi
  if ! docker ps --format '{{.Names}}' | grep -qx "${SANDBOX_CONTAINER_NAME}"; then
    return 0
  fi

  local env_flags=()
  local npm_registry="${NPM_CONFIG_REGISTRY:-${NPM_REGISTRY:-}}"
  if [[ -n "${npm_registry}" ]]; then
    env_flags+=("-e" "NPM_CONFIG_REGISTRY=${npm_registry}")
  fi
  local key val
  for key in HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy; do
    val="${!key-}"
    if [[ -n "${val}" ]]; then
      env_flags+=("-e" "${key}=${val}")
    fi
  done

  if ! docker exec ${env_flags[@]+"${env_flags[@]}"} "${SANDBOX_CONTAINER_NAME}" sh -lc 'command -v npm >/dev/null 2>&1'; then
    log_warn "Sandbox npm not found; skipping Codex/Claude Code install."
    return 0
  fi

  log_info "Ensuring Codex + Claude Code inside sandbox..."
  if ! docker exec ${env_flags[@]+"${env_flags[@]}"} "${SANDBOX_CONTAINER_NAME}" sh -lc '
    fail=0
    if ! command -v codex >/dev/null 2>&1; then
      npm i -g @openai/codex || fail=1
    fi
    if ! command -v claude >/dev/null 2>&1; then
      npm i -g @anthropic-ai/claude-code || fail=1
    fi
    exit "$fail"
  '; then
    log_warn "Sandbox CLI install failed; verify npm/Node connectivity."
  fi
}
