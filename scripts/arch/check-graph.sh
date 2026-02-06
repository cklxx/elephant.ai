#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

POLICY_FILE="${1:-configs/arch/policy.yaml}"
EXCEPTIONS_FILE="${2:-configs/arch/exceptions.yaml}"

if [[ ! -f "$POLICY_FILE" ]]; then
  echo "missing policy file: $POLICY_FILE" >&2
  exit 1
fi
if [[ ! -f "$EXCEPTIONS_FILE" ]]; then
  echo "missing exceptions file: $EXCEPTIONS_FILE" >&2
  exit 1
fi

POLICY_JSON="$(
  ruby -r yaml -r json -e 'puts JSON.generate(YAML.safe_load(File.read(ARGV[0]), permitted_classes: [], aliases: true) || {})' \
    "$POLICY_FILE"
)"
EXCEPTIONS_JSON="$(
  ruby -r yaml -r json -e 'puts JSON.generate(YAML.safe_load(File.read(ARGV[0]), permitted_classes: [], aliases: true) || {})' \
    "$EXCEPTIONS_FILE"
)"

REPORT_PATH="$(echo "$POLICY_JSON" | jq -r '.report.path // "artifacts/arch-report.json"')"
MAX_FANOUT="$(echo "$POLICY_JSON" | jq -r '.thresholds.max_fanout // 40')"
MAX_FILE_LOC="$(echo "$POLICY_JSON" | jq -r '.thresholds.max_file_loc // 2200')"
MAX_PACKAGE_FILES="$(echo "$POLICY_JSON" | jq -r '.thresholds.max_package_files // 60')"

mkdir -p "$(dirname "$REPORT_PATH")"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

touch "$TMP_DIR/allowed.tsv"
touch "$TMP_DIR/import_exc_active.tsv" "$TMP_DIR/fanout_exc_active.tsv" "$TMP_DIR/file_loc_exc_active.tsv" "$TMP_DIR/package_size_exc_active.tsv"
touch "$TMP_DIR/import_violations.jsonl" "$TMP_DIR/fanout_violations.jsonl" "$TMP_DIR/file_loc_violations.jsonl" "$TMP_DIR/package_size_violations.jsonl" "$TMP_DIR/expired_exceptions.jsonl"

echo "$POLICY_JSON" | jq -r '.layers | to_entries[] | .key as $src | (.value // [])[] | "\($src)\t\(.)"' > "$TMP_DIR/allowed.tsv"

today="$(date +%F)"

append_json_line() {
  local file="$1"
  local json="$2"
  printf '%s\n' "$json" >> "$file"
}

while IFS=$'\t' read -r from to owner reason expires_at; do
  [[ -z "${from}" || -z "${to}" ]] && continue
  if [[ -n "$expires_at" && "$today" > "$expires_at" ]]; then
    append_json_line "$TMP_DIR/expired_exceptions.jsonl" "$(jq -nc \
      --arg type "imports" \
      --arg from "$from" \
      --arg to "$to" \
      --arg owner "$owner" \
      --arg reason "$reason" \
      --arg expires_at "$expires_at" \
      '{type:$type,from:$from,to:$to,owner:$owner,reason:$reason,expires_at:$expires_at}')"
  else
    printf '%s\t%s\n' "$from" "$to" >> "$TMP_DIR/import_exc_active.tsv"
  fi
done < <(echo "$EXCEPTIONS_JSON" | jq -r '.imports[]? | [(.from // ""), (.to // ""), (.owner // ""), (.reason // ""), (.expires_at // "")] | @tsv')

while IFS=$'\t' read -r pkg owner reason expires_at; do
  [[ -z "${pkg}" ]] && continue
  if [[ -n "$expires_at" && "$today" > "$expires_at" ]]; then
    append_json_line "$TMP_DIR/expired_exceptions.jsonl" "$(jq -nc \
      --arg type "fanout" \
      --arg package "$pkg" \
      --arg owner "$owner" \
      --arg reason "$reason" \
      --arg expires_at "$expires_at" \
      '{type:$type,package:$package,owner:$owner,reason:$reason,expires_at:$expires_at}')"
  else
    printf '%s\n' "$pkg" >> "$TMP_DIR/fanout_exc_active.tsv"
  fi
done < <(echo "$EXCEPTIONS_JSON" | jq -r '.fanout[]? | [(.package // ""), (.owner // ""), (.reason // ""), (.expires_at // "")] | @tsv')

while IFS=$'\t' read -r path owner reason expires_at; do
  [[ -z "${path}" ]] && continue
  if [[ -n "$expires_at" && "$today" > "$expires_at" ]]; then
    append_json_line "$TMP_DIR/expired_exceptions.jsonl" "$(jq -nc \
      --arg type "file_loc" \
      --arg path "$path" \
      --arg owner "$owner" \
      --arg reason "$reason" \
      --arg expires_at "$expires_at" \
      '{type:$type,path:$path,owner:$owner,reason:$reason,expires_at:$expires_at}')"
  else
    printf '%s\n' "$path" >> "$TMP_DIR/file_loc_exc_active.tsv"
  fi
done < <(echo "$EXCEPTIONS_JSON" | jq -r '.file_loc[]? | [(.path // ""), (.owner // ""), (.reason // ""), (.expires_at // "")] | @tsv')

while IFS=$'\t' read -r pkg owner reason expires_at; do
  [[ -z "${pkg}" ]] && continue
  if [[ -n "$expires_at" && "$today" > "$expires_at" ]]; then
    append_json_line "$TMP_DIR/expired_exceptions.jsonl" "$(jq -nc \
      --arg type "package_size" \
      --arg package "$pkg" \
      --arg owner "$owner" \
      --arg reason "$reason" \
      --arg expires_at "$expires_at" \
      '{type:$type,package:$package,owner:$owner,reason:$reason,expires_at:$expires_at}')"
  else
    printf '%s\n' "$pkg" >> "$TMP_DIR/package_size_exc_active.tsv"
  fi
done < <(echo "$EXCEPTIONS_JSON" | jq -r '.package_size[]? | [(.package // ""), (.owner // ""), (.reason // ""), (.expires_at // "")] | @tsv')

layer_of() {
  local import_path="$1"
  case "$import_path" in
    alex/internal/delivery|alex/internal/delivery/*) echo "delivery" ;;
    alex/internal/app|alex/internal/app/*) echo "app" ;;
    alex/internal/domain|alex/internal/domain/*) echo "domain" ;;
    alex/internal/infra|alex/internal/infra/*) echo "infra" ;;
    alex/internal/shared|alex/internal/shared/*) echo "shared" ;;
    *) echo "" ;;
  esac
}

is_allowed_layer() {
  local src="$1"
  local dst="$2"
  grep -Fxq "$src"$'\t'"$dst" "$TMP_DIR/allowed.tsv"
}

has_import_exception() {
  local src="$1"
  local dst="$2"
  grep -Fxq "$src"$'\t'"$dst" "$TMP_DIR/import_exc_active.tsv"
}

has_fanout_exception() {
  local pkg="$1"
  grep -Fxq "$pkg" "$TMP_DIR/fanout_exc_active.tsv"
}

has_file_loc_exception() {
  local path="$1"
  grep -Fxq "$path" "$TMP_DIR/file_loc_exc_active.tsv"
}

has_package_size_exception() {
  local pkg="$1"
  grep -Fxq "$pkg" "$TMP_DIR/package_size_exc_active.tsv"
}

total_packages=0
while IFS= read -r pkg; do
  total_packages=$((total_packages + 1))
  src_layer="$(layer_of "$pkg")"
  [[ -z "$src_layer" ]] && continue

  imports="$(go list -f '{{range .Imports}}{{println .}}{{end}}' "$pkg" | rg '^alex/internal/' || true)"
  if [[ -n "$imports" ]]; then
    printf '%s\n' "$imports" | sort -u > "$TMP_DIR/pkg-imports.txt"
  else
    : > "$TMP_DIR/pkg-imports.txt"
  fi

  fanout="$(wc -l < "$TMP_DIR/pkg-imports.txt" | tr -d ' ')"
  if (( fanout > MAX_FANOUT )) && ! has_fanout_exception "$pkg"; then
    append_json_line "$TMP_DIR/fanout_violations.jsonl" "$(jq -nc \
      --arg package "$pkg" \
      --argjson fanout "$fanout" \
      --argjson max_fanout "$MAX_FANOUT" \
      '{package:$package,fanout:$fanout,max_fanout:$max_fanout}')"
  fi

  while IFS= read -r dep; do
    [[ -z "$dep" ]] && continue
    dst_layer="$(layer_of "$dep")"
    [[ -z "$dst_layer" ]] && continue
    if ! is_allowed_layer "$src_layer" "$dst_layer" && ! has_import_exception "$pkg" "$dep"; then
      append_json_line "$TMP_DIR/import_violations.jsonl" "$(jq -nc \
        --arg from "$pkg" \
        --arg from_layer "$src_layer" \
        --arg to "$dep" \
        --arg to_layer "$dst_layer" \
        '{from:$from,from_layer:$from_layer,to:$to,to_layer:$to_layer}')"
    fi
  done < "$TMP_DIR/pkg-imports.txt"
done < <(go list ./internal/...)

while IFS=$'\t' read -r lines path; do
  [[ -z "$path" || "$path" == "total" ]] && continue
  if (( lines > MAX_FILE_LOC )) && ! has_file_loc_exception "$path"; then
    append_json_line "$TMP_DIR/file_loc_violations.jsonl" "$(jq -nc \
      --arg path "$path" \
      --argjson loc "$lines" \
      --argjson max_loc "$MAX_FILE_LOC" \
      '{path:$path,loc:$loc,max_loc:$max_loc}')"
  fi
done < <(find internal -type f -name '*.go' -print0 | xargs -0 wc -l | awk '$2 != "total" {print $1"\t"$2}')

while IFS= read -r pkg; do
  pkg_dir="$(go list -f '{{.Dir}}' "$pkg")"
  pkg_files="$(find "$pkg_dir" -maxdepth 1 -type f -name '*.go' | wc -l | tr -d ' ')"
  if (( pkg_files > MAX_PACKAGE_FILES )) && ! has_package_size_exception "$pkg"; then
    append_json_line "$TMP_DIR/package_size_violations.jsonl" "$(jq -nc \
      --arg package "$pkg" \
      --argjson file_count "$pkg_files" \
      --argjson max_file_count "$MAX_PACKAGE_FILES" \
      '{package:$package,file_count:$file_count,max_file_count:$max_file_count}')"
  fi
done < <(go list ./internal/...)

json_array_or_empty() {
  local file="$1"
  if [[ -s "$file" ]]; then
    jq -s '.' "$file"
  else
    echo '[]'
  fi
}

import_violations="$(json_array_or_empty "$TMP_DIR/import_violations.jsonl")"
fanout_violations="$(json_array_or_empty "$TMP_DIR/fanout_violations.jsonl")"
file_loc_violations="$(json_array_or_empty "$TMP_DIR/file_loc_violations.jsonl")"
package_size_violations="$(json_array_or_empty "$TMP_DIR/package_size_violations.jsonl")"
expired_exceptions="$(json_array_or_empty "$TMP_DIR/expired_exceptions.jsonl")"

import_count="$(echo "$import_violations" | jq 'length')"
fanout_count="$(echo "$fanout_violations" | jq 'length')"
file_loc_count="$(echo "$file_loc_violations" | jq 'length')"
package_size_count="$(echo "$package_size_violations" | jq 'length')"
expired_count="$(echo "$expired_exceptions" | jq 'length')"
violation_count=$((import_count + fanout_count + file_loc_count + package_size_count + expired_count))

generated_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
jq -n \
  --arg generated_at "$generated_at" \
  --arg policy_file "$POLICY_FILE" \
  --arg exceptions_file "$EXCEPTIONS_FILE" \
  --argjson total_packages "$total_packages" \
  --argjson max_fanout "$MAX_FANOUT" \
  --argjson max_file_loc "$MAX_FILE_LOC" \
  --argjson max_package_files "$MAX_PACKAGE_FILES" \
  --argjson import_violations "$import_violations" \
  --argjson fanout_violations "$fanout_violations" \
  --argjson file_loc_violations "$file_loc_violations" \
  --argjson package_size_violations "$package_size_violations" \
  --argjson expired_exceptions "$expired_exceptions" \
  --argjson violation_count "$violation_count" \
  '{
    generated_at: $generated_at,
    policy_file: $policy_file,
    exceptions_file: $exceptions_file,
    thresholds: {
      max_fanout: $max_fanout,
      max_file_loc: $max_file_loc,
      max_package_files: $max_package_files
    },
    stats: {
      total_internal_packages: $total_packages,
      total_violations: $violation_count
    },
    violations: {
      imports: $import_violations,
      fanout: $fanout_violations,
      file_loc: $file_loc_violations,
      package_size: $package_size_violations,
      expired_exceptions: $expired_exceptions
    }
  }' > "$REPORT_PATH"

if (( violation_count > 0 )); then
  echo "❌ Architecture policy violations detected: $violation_count"
  echo "Report: $REPORT_PATH"
  exit 1
fi

echo "✓ Architecture policy checks passed."
echo "Report: $REPORT_PATH"
