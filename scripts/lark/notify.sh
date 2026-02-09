#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"

usage() {
  cat <<'USAGE'
Usage:
  scripts/lark/notify.sh send --chat-id <chat_id> --text <message> [--config <config.yaml>]

Env:
  LARK_NOTIFY_APP_ID        Override app id
  LARK_NOTIFY_APP_SECRET    Override app secret
  LARK_NOTIFY_BASE_DOMAIN   Override base domain (default: https://open.feishu.cn)
USAGE
}

json_escape() {
  printf '%s' "${1:-}" | sed ':a;N;$!ba;s/\\/\\\\/g;s/"/\\"/g;s/\n/\\n/g'
}

parse_lark_config_value() {
  local config_path="$1"
  local key="$2"
  [[ -f "${config_path}" ]] || return 1

  awk -v want_key="${key}" '
    function indent(line,    m) {
      m = match(line, /[^ ]/)
      if (m == 0) return length(line)
      return m - 1
    }
    {
      line = $0
      if (line ~ /^[[:space:]]*#/ || line ~ /^[[:space:]]*$/) next
      i = indent(line)

      if (line ~ /^[[:space:]]*channels:[[:space:]]*$/) {
        in_channels = 1
        channels_indent = i
        next
      }
      if (in_channels && i <= channels_indent && line !~ /^[[:space:]]*channels:[[:space:]]*$/) {
        in_channels = 0
        in_lark = 0
      }

      if (in_channels && line ~ /^[[:space:]]*lark:[[:space:]]*$/) {
        in_lark = 1
        lark_indent = i
        next
      }
      if (in_lark && i <= lark_indent && line !~ /^[[:space:]]*lark:[[:space:]]*$/) {
        in_lark = 0
      }

      if (in_lark && line ~ "^[[:space:]]*" want_key ":[[:space:]]*") {
        sub("^[[:space:]]*" want_key ":[[:space:]]*", "", line)
        sub(/[[:space:]]+#.*/, "", line)
        gsub(/^"|"$/, "", line)
        gsub(/^\047|\047$/, "", line)
        print line
        exit
      }
    }
  ' "${config_path}"
}

resolve_credentials() {
  local config_path="$1"

  app_id="${LARK_NOTIFY_APP_ID:-}"
  app_secret="${LARK_NOTIFY_APP_SECRET:-}"
  base_domain="${LARK_NOTIFY_BASE_DOMAIN:-}"

  if [[ -z "${app_id}" ]]; then
    app_id="$(parse_lark_config_value "${config_path}" "app_id" || true)"
  fi
  if [[ -z "${app_secret}" ]]; then
    app_secret="$(parse_lark_config_value "${config_path}" "app_secret" || true)"
  fi
  if [[ -z "${base_domain}" ]]; then
    base_domain="$(parse_lark_config_value "${config_path}" "base_domain" || true)"
  fi

  if [[ -z "${base_domain}" ]]; then
    base_domain="https://open.feishu.cn"
  fi
  base_domain="${base_domain%/}"

  if [[ -z "${app_id}" || -z "${app_secret}" ]]; then
    return 1
  fi
  return 0
}

extract_json_number() {
  local json="$1"
  printf '%s' "${json}" | tr -d '\n' | sed -nE 's/.*"code"[[:space:]]*:[[:space:]]*([0-9]+).*/\1/p' | head -n1
}

extract_json_string() {
  local json="$1"
  local key="$2"
  printf '%s' "${json}" | tr -d '\n' | sed -nE 's/.*"'"${key}"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' | head -n1
}

fetch_access_token() {
  local payload response code token

  payload=$(printf '{"app_id":"%s","app_secret":"%s"}' "$(json_escape "${app_id}")" "$(json_escape "${app_secret}")")
  response="$(curl -sS -X POST "${base_domain}/open-apis/auth/v3/app_access_token/internal" \
    -H 'Content-Type: application/json; charset=utf-8' \
    -d "${payload}")"

  code="$(extract_json_number "${response}")"
  if [[ -n "${code}" && "${code}" != "0" ]]; then
    return 1
  fi

  token="$(extract_json_string "${response}" "tenant_access_token")"
  if [[ -z "${token}" ]]; then
    token="$(extract_json_string "${response}" "app_access_token")"
  fi
  if [[ -z "${token}" ]]; then
    return 1
  fi

  printf '%s' "${token}"
}

send_message() {
  local chat_id="$1"
  local text="$2"
  local token payload response code

  token="$(fetch_access_token)"

  local content
  content=$(printf '{"text":"%s"}' "$(json_escape "${text}")")
  payload=$(printf '{"receive_id":"%s","msg_type":"text","content":"%s"}' \
    "$(json_escape "${chat_id}")" \
    "$(json_escape "${content}")")

  response="$(curl -sS -X POST "${base_domain}/open-apis/im/v1/messages?receive_id_type=chat_id" \
    -H 'Content-Type: application/json; charset=utf-8' \
    -H "Authorization: Bearer ${token}" \
    -d "${payload}")"

  code="$(extract_json_number "${response}")"
  if [[ -n "${code}" && "${code}" != "0" ]]; then
    return 1
  fi
  return 0
}

cmd="${1:-}"
shift || true

if [[ "${cmd}" != "send" ]]; then
  usage
  die "unknown command: ${cmd:-}"
fi

chat_id=""
text=""
config_path="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --chat-id)
      chat_id="${2:-}"
      shift 2
      ;;
    --text)
      text="${2:-}"
      shift 2
      ;;
    --config)
      config_path="${2:-}"
      shift 2
      ;;
    help|-h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      die "unknown argument: $1"
      ;;
  esac
done

chat_id="$(printf '%s' "${chat_id}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
text="$(printf '%s' "${text}")"
[[ -n "${chat_id}" ]] || die "--chat-id is required"
[[ -n "${text}" ]] || die "--text is required"

if ! resolve_credentials "${config_path}"; then
  die "missing lark app credentials (env or ${config_path})"
fi

if ! send_message "${chat_id}" "${text}"; then
  die "failed to send lark message"
fi

log_success "Lark notice sent to ${chat_id}"
