#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

TARGET_DIR="internal/domain"
echo "Checking architecture boundaries in ${TARGET_DIR}..."

declare -a banned_imports=(
  "alex/internal/app"
  "alex/internal/delivery"
  "alex/internal/infra"
)

violations=""
for import_path in "${banned_imports[@]}"; do
  # Match both direct imports and subpackages, with or without import aliases.
  # Examples:
  #   "alex/internal/app"
  #   "alex/internal/app/foo"
  #   appctx "alex/internal/app/context"
  matches="$(rg -n "^[[:space:]]*([[:alnum:]_]+[[:space:]]+)?\"${import_path}(/[^\\\"]*)?\"" "$TARGET_DIR" --glob '*.go' --glob '!**/*_test.go' || true)"
  if [[ -n "$matches" ]]; then
    violations+="${matches}"$'\n'
  fi
done

if [[ -n "$violations" ]]; then
  echo "❌ Architecture boundary violations detected:"
  printf "%s" "$violations"
  exit 1
fi

echo "✓ Domain imports respect architecture boundaries."
