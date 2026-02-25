#!/usr/bin/env bash
# shellcheck shell=bash
# Shared Lark identity lock helpers for main/test process isolation.

lark_canonical_path() {
  local path="$1"
  if [[ "${path}" != /* ]]; then
    path="$(pwd)/${path}"
  fi

  if [[ -e "${path}" ]] && command -v realpath >/dev/null 2>&1; then
    realpath "${path}"
    return 0
  fi

  local dir base
  dir="$(dirname "${path}")"
  base="$(basename "${path}")"
  if [[ -d "${dir}" ]]; then
    (
      cd "${dir}" || return 1
      printf '%s/%s\n' "$(pwd -P)" "${base}"
    )
    return 0
  fi

  printf '%s\n' "${path}"
}

lark_extract_channel_scalar() {
  local config_path="$1"
  local field="$2"
  awk -v target="${field}" '
    function trim(s) {
      sub(/^[ \t]+/, "", s)
      sub(/[ \t]+$/, "", s)
      return s
    }
    function strip_quotes(s) {
      if (s ~ /^".*"$/) {
        return substr(s, 2, length(s) - 2)
      }
      if (s ~ /^'\''.*'\''$/) {
        return substr(s, 2, length(s) - 2)
      }
      return s
    }
    {
      line = $0
      sub(/[[:space:]]+#.*$/, "", line)
      if (line ~ /^[[:space:]]*$/) {
        next
      }

      indent = match(line, /[^ ]/) - 1
      trimmed = trim(line)

      if (!in_channels && trimmed == "channels:") {
        in_channels = 1
        channels_indent = indent
        in_lark = 0
        next
      }

      if (in_channels && indent <= channels_indent && trimmed != "channels:") {
        in_channels = 0
        in_lark = 0
      }

      if (in_channels && !in_lark && trimmed == "lark:") {
        in_lark = 1
        lark_indent = indent
        next
      }

      if (in_lark && indent <= lark_indent) {
        in_lark = 0
      }

      if (in_lark && index(trimmed, target ":") == 1) {
        value = substr(trimmed, length(target) + 2)
        value = trim(value)
        if (value == "") {
          next
        }
        print strip_quotes(value)
        exit
      }
    }
  ' "${config_path}"
}

lark_expand_env_token() {
  local raw="${1:-}"
  if [[ -z "${raw}" ]]; then
    return 0
  fi

  if [[ "${raw}" =~ ^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$ ]]; then
    local name="${BASH_REMATCH[1]}"
    printf '%s' "${!name:-}"
    return 0
  fi
  if [[ "${raw}" =~ ^\$([A-Za-z_][A-Za-z0-9_]*)$ ]]; then
    local name="${BASH_REMATCH[1]}"
    printf '%s' "${!name:-}"
    return 0
  fi

  printf '%s' "${raw}"
}

lark_resolve_identity() {
  local config_path="$1"
  local canonical_path app_id_raw app_id base_domain_raw base_domain

  canonical_path="$(lark_canonical_path "${config_path}")"
  app_id_raw="$(lark_extract_channel_scalar "${config_path}" "app_id" || true)"
  app_id="$(lark_expand_env_token "${app_id_raw}")"
  base_domain_raw="$(lark_extract_channel_scalar "${config_path}" "base_domain" || true)"
  base_domain="$(lark_expand_env_token "${base_domain_raw}")"

  if [[ -n "${app_id}" ]]; then
    printf 'app_id=%s|base_domain=%s' "${app_id}" "${base_domain:-open.feishu.cn}"
    return 0
  fi

  printf 'config=%s' "${canonical_path}"
}

lark_default_main_config_path() {
  printf '%s\n' "${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
}

lark_shared_pid_dir() {
  local main_config="${1:-}"
  if [[ -n "${LARK_PID_DIR:-}" ]]; then
    printf '%s\n' "${LARK_PID_DIR}"
    return 0
  fi

  if [[ -z "${main_config}" ]]; then
    main_config="$(lark_default_main_config_path)"
  fi

  local canonical_main config_dir
  canonical_main="$(lark_canonical_path "${main_config}")"
  config_dir="$(dirname "${canonical_main}")"
  printf '%s/pids\n' "${config_dir}"
}

lark_identity_lock_file() {
  local main_root="$1"
  local identity="$2"
  local config_path="${3:-}"
  local digest pid_dir

  if command -v shasum >/dev/null 2>&1; then
    digest="$(printf '%s' "${identity}" | shasum -a 256 | awk "{print \$1}")"
  elif command -v sha256sum >/dev/null 2>&1; then
    digest="$(printf '%s' "${identity}" | sha256sum | awk "{print \$1}")"
  elif command -v openssl >/dev/null 2>&1; then
    digest="$(printf '%s' "${identity}" | openssl dgst -sha256 -r | awk "{print \$1}")"
  else
    die "missing hash tool (requires shasum/sha256sum/openssl) for lark identity lock"
  fi

  if [[ -n "${config_path}" || -n "${LARK_PID_DIR:-}" ]]; then
    pid_dir="$(lark_shared_pid_dir "${config_path}")"
  else
    pid_dir="${main_root}/pids"
  fi

  printf '%s/lark-identities/%s.lock\n' "${pid_dir}" "${digest}"
}

lark_lock_field() {
  local lock_file="$1"
  local key="$2"
  awk -F= -v k="${key}" '$1 == k { print substr($0, length($1) + 2); exit }' "${lock_file}" 2>/dev/null
}

lark_assert_identity_available() {
  local main_root="$1"
  local scope="$2"
  local config_path="$3"
  local owner_pid="${4:-}"
  local identity lock_file existing_pid existing_scope existing_config

  identity="$(lark_resolve_identity "${config_path}")"
  lock_file="$(lark_identity_lock_file "${main_root}" "${identity}" "${config_path}")"
  if [[ ! -f "${lock_file}" ]]; then
    return 0
  fi

  existing_pid="$(lark_lock_field "${lock_file}" "pid")"
  existing_scope="$(lark_lock_field "${lock_file}" "scope")"
  existing_config="$(lark_lock_field "${lock_file}" "config")"

  if [[ -z "${existing_pid}" ]]; then
    rm -f "${lock_file}" 2>/dev/null || true
    return 0
  fi

  if ! is_process_running "${existing_pid}"; then
    rm -f "${lock_file}" 2>/dev/null || true
    return 0
  fi

  if [[ -n "${owner_pid}" && "${existing_pid}" == "${owner_pid}" ]]; then
    return 0
  fi

  log_error "Lark identity conflict detected for ${scope}:"
  log_error "  identity=${identity}"
  log_error "  current_config=$(lark_canonical_path "${config_path}")"
  log_error "  owner_scope=${existing_scope:-unknown} owner_pid=${existing_pid} owner_config=${existing_config:-unknown}"
  log_error "Use independent YAML files (config.yaml + test.yaml) with independent channels.lark.app_id."
  return 1
}

lark_write_identity_lock() {
  local main_root="$1"
  local scope="$2"
  local config_path="$3"
  local owner_pid="$4"
  local identity lock_file canonical_config

  identity="$(lark_resolve_identity "${config_path}")"
  lock_file="$(lark_identity_lock_file "${main_root}" "${identity}" "${config_path}")"
  canonical_config="$(lark_canonical_path "${config_path}")"
  mkdir -p "$(dirname "${lock_file}")"

  cat > "${lock_file}" <<EOF
identity=${identity}
scope=${scope}
pid=${owner_pid}
config=${canonical_config}
updated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
}

lark_release_identity_lock() {
  local main_root="$1"
  local config_path="$2"
  local owner_pid="${3:-}"
  local identity lock_file existing_pid

  identity="$(lark_resolve_identity "${config_path}")"
  lock_file="$(lark_identity_lock_file "${main_root}" "${identity}" "${config_path}")"
  if [[ ! -f "${lock_file}" ]]; then
    return 0
  fi

  existing_pid="$(lark_lock_field "${lock_file}" "pid")"
  if [[ -n "${owner_pid}" && -n "${existing_pid}" && "${existing_pid}" != "${owner_pid}" ]]; then
    if is_process_running "${existing_pid}"; then
      return 0
    fi
  fi

  rm -f "${lock_file}" 2>/dev/null || true
}

lark_assert_config_paths_distinct() {
  local main_config="$1"
  local test_config="$2"
  local main_path test_path

  main_path="$(lark_canonical_path "${main_config}")"
  test_path="$(lark_canonical_path "${test_config}")"
  if [[ "${main_path}" == "${test_path}" ]]; then
    log_error "MAIN_CONFIG and TEST_CONFIG must point to different YAML files."
    log_error "  main=${main_path}"
    log_error "  test=${test_path}"
    return 1
  fi

  return 0
}

lark_assert_config_identities_distinct() {
  local main_config="$1"
  local test_config="$2"
  local main_identity test_identity

  main_identity="$(lark_resolve_identity "${main_config}")"
  test_identity="$(lark_resolve_identity "${test_config}")"
  if [[ "${main_identity}" == "${test_identity}" ]]; then
    log_error "MAIN_CONFIG and TEST_CONFIG resolve to the same Lark identity."
    log_error "  identity=${main_identity}"
    log_error "Use independent channels.lark.app_id to avoid duplicate replies."
    return 1
  fi

  return 0
}

lark_assert_main_test_isolation() {
  local main_config="$1"
  local test_config="$2"

  lark_assert_config_paths_distinct "${main_config}" "${test_config}" || return 1
  lark_assert_config_identities_distinct "${main_config}" "${test_config}" || return 1
  return 0
}
