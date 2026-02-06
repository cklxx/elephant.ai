#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

TARGET_DIR="internal/agent/domain"
echo "Checking architecture boundaries in ${TARGET_DIR}..."

declare -a banned_imports=(
  "alex/internal/agent/app"
  "alex/internal/infra"
  "alex/internal/server"
  "alex/internal/channels"
  "alex/internal/llm"
  "alex/internal/memory"
  "alex/internal/async"
  "alex/internal/jsonx"
  "alex/internal/external/workspace"
  "alex/internal/tools/builtin/pathutil"
  "alex/internal/utils/id"
  "alex/internal/utils/clilatency"
)

violations=""
for import_path in "${banned_imports[@]}"; do
  matches="$(rg -n "^[[:space:]]*\"${import_path}\"" "$TARGET_DIR" --glob '*.go' --glob '!**/*_test.go' || true)"
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
