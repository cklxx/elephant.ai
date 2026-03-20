#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

exit_code=0

# Check function: verifies a target directory does not import banned packages.
check_boundary() {
  local target_dir="$1"
  local label="$2"
  shift 2
  local -a banned=("$@")

  if [[ ! -d "$target_dir" ]]; then
    return
  fi

  echo "Checking architecture boundaries in ${target_dir}..."

  local violations=""
  for import_path in "${banned[@]}"; do
    # Match both direct imports and subpackages, with or without import aliases.
    # Examples:
    #   "alex/internal/app"
    #   "alex/internal/app/foo"
    #   appctx "alex/internal/app/context"
    local matches
    matches="$(rg -n "^[[:space:]]*([[:alnum:]_]+[[:space:]]+)?\"${import_path}(/[^\\\"]*)?\"" "$target_dir" --glob '*.go' --glob '!**/*_test.go' || true)"
    if [[ -n "$matches" ]]; then
      violations+="${matches}"$'\n'
    fi
  done

  if [[ -n "$violations" ]]; then
    echo "  ❌ ${label} boundary violations detected:"
    printf "%s" "$violations"
    exit_code=1
  else
    echo "  ✓ ${label} imports respect architecture boundaries."
  fi
}

# internal/domain/ — cannot import app, delivery, infra
check_boundary "internal/domain" "Domain" \
  "alex/internal/app" \
  "alex/internal/delivery" \
  "alex/internal/infra"

# internal/core/ — cannot import app, delivery, infra, framework
check_boundary "internal/core" "Core" \
  "alex/internal/app" \
  "alex/internal/delivery" \
  "alex/internal/infra" \
  "alex/internal/framework"

exit "$exit_code"
