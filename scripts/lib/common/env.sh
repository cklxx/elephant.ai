#!/usr/bin/env bash
# shellcheck shell=bash
# Common environment helpers (secret generation, file permissions, .env manipulation).

generate_auth_secret() {
  if command_exists python3; then
    python3 -c 'import secrets; print(secrets.token_hex(32))'
    return
  fi
  if command_exists python; then
    python -c 'import secrets; print(secrets.token_hex(32))'
    return
  fi
  if command_exists openssl; then
    openssl rand -hex 32
    return
  fi
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
}

ensure_private_file_mode() {
  local file_path="$1"
  if [[ -f "$file_path" ]]; then
    chmod 600 "$file_path" 2>/dev/null || log_warn "Unable to set 600 permissions on ${file_path}"
  fi
}

append_env_var_if_missing() {
  local key=$1
  local value=$2
  local env_file="${3:-.env}"
  if ! grep -q "^${key}=" "$env_file" 2>/dev/null; then
    printf "\n%s=%s\n" "$key" "$value" >> "$env_file"
    ensure_private_file_mode "$env_file"
    log_warn "Appended default ${key} to ${env_file}"
  fi
}
