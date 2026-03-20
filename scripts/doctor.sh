#!/usr/bin/env bash
# shellcheck shell=bash
# Diagnostic checker — verifies all development prerequisites.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/lib/common/logging.sh"

errors=0

pass() { echo -e "${C_GREEN}  ✓${C_RESET} $1"; }
fail() { echo -e "${C_RED}  ✗${C_RESET} $1"; errors=$((errors + 1)); }
info() { echo -e "${C_BLUE}  ▸${C_RESET} $1"; }

check_command() {
  local cmd="$1"
  local label="${2:-$1}"
  local show_version="${3:-}"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "${label} not found"
    return
  fi
  if [[ -n "$show_version" ]]; then
    local version
    # go uses 'go version', everything else uses '--version'
    if [[ "$cmd" == "go" ]]; then
      version="$(go version 2>&1 | head -1 || true)"
    else
      version="$("$cmd" --version 2>&1 | head -1 || true)"
    fi
    pass "${label}: ${version}"
  else
    pass "${label} found"
  fi
}

check_port() {
  local port="$1"
  local label="${2:-port $1}"
  if lsof -ti "tcp:${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    local pids
    pids="$(lsof -ti "tcp:${port}" -sTCP:LISTEN 2>/dev/null | tr '\n' ',' | sed 's/,$//')"
    fail "${label} (:${port}) in use by PID(s): ${pids}"
  else
    pass "${label} (:${port}) is free"
  fi
}

echo ""
echo -e "${C_CYAN}Elephant.ai Doctor${C_RESET}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# --- Toolchain ---
echo ""
echo -e "${C_CYAN}Toolchain${C_RESET}"

check_command go "Go" show
check_command node "Node.js" show
check_command npm "npm" show
check_command make "make"

# --- Optional tools ---
echo ""
echo -e "${C_CYAN}Optional Tools${C_RESET}"

if command -v docker >/dev/null 2>&1; then
  pass "Docker found"
else
  info "Docker not found (needed for docker-compose deployments and local auth DB)"
fi

if command -v psql >/dev/null 2>&1; then
  pass "psql found"
else
  info "psql not found (needed for auth database migrations)"
fi

if command -v golangci-lint >/dev/null 2>&1 || [[ -x "${REPO_ROOT}/scripts/run-golangci-lint.sh" ]]; then
  pass "golangci-lint available"
else
  info "golangci-lint not found (lint will use repo wrapper to install)"
fi

# --- Environment ---
echo ""
echo -e "${C_CYAN}Environment${C_RESET}"

if [[ -f "${REPO_ROOT}/.env" ]]; then
  pass ".env file exists"
  # Check required keys
  for key in OPENAI_API_KEY AUTH_JWT_SECRET; do
    if grep -q "^${key}=" "${REPO_ROOT}/.env" 2>/dev/null; then
      local_val="$(grep "^${key}=" "${REPO_ROOT}/.env" | head -1 | cut -d= -f2-)"
      if [[ -n "$local_val" ]]; then
        pass "${key} is set"
      else
        info "${key} is present but empty"
      fi
    else
      info "${key} not in .env"
    fi
  done
else
  fail ".env file not found (run: make setup)"
fi

# --- Ports ---
echo ""
echo -e "${C_CYAN}Ports${C_RESET}"

check_port 8080 "Backend"
check_port 3000 "Frontend"

# --- Binary ---
echo ""
echo -e "${C_CYAN}Build Artifacts${C_RESET}"

if [[ -x "${REPO_ROOT}/alex" ]]; then
  pass "alex binary exists"
else
  info "alex binary not built yet (run: make build)"
fi

if [[ -d "${REPO_ROOT}/web/node_modules" ]]; then
  pass "web/node_modules exists"
else
  info "web dependencies not installed (run: npm --prefix web install)"
fi

# --- Summary ---
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [[ $errors -eq 0 ]]; then
  echo -e "${C_GREEN}All checks passed.${C_RESET}"
else
  echo -e "${C_RED}${errors} issue(s) found.${C_RESET}"
  exit 1
fi
echo ""
