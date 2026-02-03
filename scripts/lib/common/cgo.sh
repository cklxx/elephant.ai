#!/usr/bin/env bash
# shellcheck shell=bash
# CGO detection helpers for sqlite-vec builds.

if ! command -v command_exists >/dev/null 2>&1; then
  command_exists() {
    command -v "$1" >/dev/null 2>&1
  }
fi

cgo_mode() {
  local mode="${ALEX_CGO_MODE:-auto}"
  case "$mode" in
    on|off|auto) ;; 
    *) mode="auto" ;;
  esac
  echo "$mode"
}

cgo_has_compiler() {
  command_exists clang || command_exists gcc
}

cgo_darwin_sdk_path() {
  if ! command_exists xcrun; then
    return 1
  fi
  local sdk
  sdk="$(xcrun --show-sdk-path 2>/dev/null || true)"
  if [[ -n "$sdk" ]]; then
    echo "$sdk"
    return 0
  fi
  return 1
}

cgo_sqlite_header_present() {
  local uname_s
  uname_s="$(uname -s)"
  if [[ "$uname_s" == "Darwin" ]]; then
    local sdk
    sdk="$(cgo_darwin_sdk_path || true)"
    if [[ -n "$sdk" && -f "${sdk}/usr/include/sqlite3.h" ]]; then
      return 0
    fi
    if [[ -f "/usr/include/sqlite3.h" ]]; then
      return 0
    fi
  else
    if [[ -f "/usr/include/sqlite3.h" || -f "/usr/local/include/sqlite3.h" ]]; then
      return 0
    fi
  fi
  if command_exists pkg-config; then
    if pkg-config --exists sqlite3; then
      return 0
    fi
  fi
  return 1
}

cgo_sqlite_ready() {
  if ! cgo_has_compiler; then
    return 1
  fi
  local uname_s
  uname_s="$(uname -s)"
  if [[ "$uname_s" == "Darwin" ]]; then
    if ! command_exists xcode-select; then
      return 1
    fi
    if ! xcode-select -p >/dev/null 2>&1; then
      return 1
    fi
  fi
  if ! cgo_sqlite_header_present; then
    return 1
  fi
  return 0
}

cgo_apply_mode() {
  if [[ -n "${CGO_ENABLED:-}" ]]; then
    return 0
  fi
  local mode
  mode="$(cgo_mode)"
  case "$mode" in
    on)
      export CGO_ENABLED=1
      ;;
    off)
      export CGO_ENABLED=0
      ;;
    auto)
      if cgo_sqlite_ready; then
        export CGO_ENABLED=1
      else
        export CGO_ENABLED=0
      fi
      ;;
  esac
}
