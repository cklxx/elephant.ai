#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

TOP_N="${1:-40}"

echo "== Log call hotspots (top ${TOP_N}) =="
rg -n 'logger\.(Debug|Info|Warn|Error)\(|\blog\.(Print|Printf|Println|Fatal|Fatalf|Panic|Panicf)\(' \
  internal cmd scripts --glob '*.go' \
  | awk -F: '{print $1}' \
  | sort \
  | uniq -c \
  | sort -nr \
  | head -n "${TOP_N}"

echo
echo "== Logger level distribution =="
for level in Debug Info Warn Error; do
  count="$(rg -n "logger\.${level}\(" internal cmd scripts --glob '*.go' | wc -l | tr -d ' ')"
  printf "%-5s %s\n" "${level}" "${count}"
done

echo
echo "== Messages with manual [Prefix] in logger text (potential redundancy) =="
rg -n 'logger\.(Debug|Info|Warn|Error)\("\[[^"]+\]' internal cmd scripts --glob '*.go' || true
