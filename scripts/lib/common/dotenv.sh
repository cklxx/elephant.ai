#!/usr/bin/env bash
# shellcheck shell=bash
# Common .env loading helpers.

load_dotenv_file() {
  local env_file="$1"
  if [[ ! -f "$env_file" ]]; then
    return 0
  fi

  set -a
  # shellcheck source=/dev/null
  source "$env_file"
  set +a
}
