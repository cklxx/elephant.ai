#!/usr/bin/env bash
# shellcheck shell=bash
# Robust .env file loader.
#
# Handles: comments, blank lines, `export` prefix, single/double quotes,
# inline comments, CR/LF line endings, and emits warnings for unparseable lines.

load_dotenv_file() {
  local dotenv_file="$1"
  local raw_line line key value
  local line_number=0

  [[ -f "$dotenv_file" ]] || return 0

  while IFS= read -r raw_line || [[ -n "$raw_line" ]]; do
    line_number=$((line_number + 1))
    line="${raw_line%$'\r'}"

    # trim leading/trailing whitespace
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"

    [[ -z "$line" ]] && continue
    [[ "${line:0:1}" == "#" ]] && continue

    # strip optional `export` prefix
    if [[ "$line" =~ ^export[[:space:]]+ ]]; then
      line="${line#export}"
      line="${line#"${line%%[![:space:]]*}"}"
    fi

    if [[ ! "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
      if declare -F log_warn >/dev/null 2>&1; then
        log_warn "Skipping unsupported .env entry at ${dotenv_file}:${line_number}"
      fi
      continue
    fi

    key="${BASH_REMATCH[1]}"
    value="${BASH_REMATCH[2]}"

    # trim value whitespace
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"

    # strip quotes
    if [[ "$value" =~ ^\"(.*)\"$ ]]; then
      value="${BASH_REMATCH[1]}"
    elif [[ "$value" =~ ^\'(.*)\'$ ]]; then
      value="${BASH_REMATCH[1]}"
    else
      # strip inline comments for unquoted values
      value="${value%%[[:space:]]#*}"
      value="${value%"${value##*[![:space:]]}"}"
    fi

    export "$key=$value"
  done < "$dotenv_file"
}
