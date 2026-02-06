#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "[baseline] tracked files: $(git ls-files | wc -l | tr -d ' ')"
echo "[baseline] internal go files: $(find internal -name '*.go' | wc -l | tr -d ' ')"

echo "[baseline] top internal package fan-out"
find internal -type f -name '*.go' | awk -F/ '{print $2}' | sort | uniq -c | sort -nr | head -n 10

echo "[baseline] largest go files"
find internal cmd -type f -name '*.go' -print0 \
  | xargs -0 wc -l \
  | sort -nr \
  | head -n 10

echo "[baseline] domain imports requiring inversion"
rg -n "internal/(app|delivery|infra)" internal/domain -S || true
