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
