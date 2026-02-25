#!/usr/bin/env bash
# shellcheck shell=bash
# Common logging helpers for scripts.

if [[ -z "${C_RED:-}" ]]; then C_RED='\033[0;31m'; fi
if [[ -z "${C_GREEN:-}" ]]; then C_GREEN='\033[0;32m'; fi
if [[ -z "${C_YELLOW:-}" ]]; then C_YELLOW='\033[1;33m'; fi
if [[ -z "${C_BLUE:-}" ]]; then C_BLUE='\033[0;34m'; fi
if [[ -z "${C_CYAN:-}" ]]; then C_CYAN='\033[0;36m'; fi
if [[ -z "${C_RESET:-}" ]]; then C_RESET='\033[0m'; fi

log_info() { echo -e "${C_BLUE}▸${C_RESET} $*"; }
log_success() { echo -e "${C_GREEN}✓${C_RESET} $*"; }
log_warn() { echo -e "${C_YELLOW}⚠${C_RESET} $*"; }
log_error() { echo -e "${C_RED}✗${C_RESET} $*" >&2; }
log_section() { echo -e "\n${C_CYAN}── $* ──${C_RESET}"; }

die() {
  log_error "$*"
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}
