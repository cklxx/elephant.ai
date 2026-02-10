#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=lib/common/logging.sh
source "${SCRIPT_DIR}/lib/common/logging.sh"

MAIN_TEMPLATE="${ROOT}/examples/config/runtime-config.yaml"
TEST_TEMPLATE="${ROOT}/examples/config/runtime-test-config.yaml"
ENV_TEMPLATE="${ROOT}/.env.example"
ENV_FILE="${ROOT}/.env"

MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
TEST_CONFIG="${TEST_CONFIG:-${ALEX_TEST_CONFIG_PATH:-$HOME/.alex/test.yaml}}"

usage() {
  cat <<'EOF'
Usage:
  scripts/setup_local_runtime.sh [--main-config <path>] [--test-config <path>] [--no-test]

Behavior:
  - Ensure repo .env exists (copy from .env.example when missing)
  - Ensure main runtime config exists (default: ~/.alex/config.yaml)
  - Ensure test runtime config exists (default: ~/.alex/test.yaml)
EOF
}

ensure_parent_dir() {
  local path="$1"
  mkdir -p "$(dirname "${path}")"
}

ensure_env_file() {
  if [[ ! -f "${ENV_FILE}" ]]; then
    [[ -f "${ENV_TEMPLATE}" ]] || die "Missing template: ${ENV_TEMPLATE}"
    cp "${ENV_TEMPLATE}" "${ENV_FILE}"
    log_success "Created .env from template: ${ENV_FILE}"
  else
    log_info "Using existing .env: ${ENV_FILE}"
  fi

  if ! grep -Eq '^[[:space:]]*LLM_API_KEY=' "${ENV_FILE}"; then
    {
      echo ""
      echo "# Minimal required key for real LLM responses"
      echo "LLM_API_KEY="
    } >> "${ENV_FILE}"
    log_info "Added LLM_API_KEY placeholder to ${ENV_FILE}"
  fi
}

ensure_config_file() {
  local label="$1"
  local template="$2"
  local target="$3"

  [[ -f "${template}" ]] || die "Missing template: ${template}"
  ensure_parent_dir "${target}"

  if [[ -f "${target}" ]]; then
    log_info "Using existing ${label}: ${target}"
    return 0
  fi

  cp "${template}" "${target}"
  chmod 600 "${target}" 2>/dev/null || true
  log_success "Created ${label}: ${target}"
}

expand_home_path() {
  local path="$1"
  case "${path}" in
    "~")
      printf '%s\n' "${HOME}"
      ;;
    "~/"*)
      printf '%s/%s\n' "${HOME}" "${path#~/}"
      ;;
    *)
      printf '%s\n' "${path}"
      ;;
  esac
}

ENABLE_TEST_CONFIG=1
while [[ $# -gt 0 ]]; do
  case "$1" in
    --main-config)
      MAIN_CONFIG="${2:-}"
      shift 2
      ;;
    --test-config)
      TEST_CONFIG="${2:-}"
      shift 2
      ;;
    --no-test)
      ENABLE_TEST_CONFIG=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      die "Unknown argument: $1"
      ;;
  esac
done

MAIN_CONFIG="$(expand_home_path "${MAIN_CONFIG}")"
TEST_CONFIG="$(expand_home_path "${TEST_CONFIG}")"

ensure_env_file
ensure_config_file "main runtime config" "${MAIN_TEMPLATE}" "${MAIN_CONFIG}"
if [[ "${ENABLE_TEST_CONFIG}" == "1" ]]; then
  ensure_config_file "test runtime config" "${TEST_TEMPLATE}" "${TEST_CONFIG}"
fi
