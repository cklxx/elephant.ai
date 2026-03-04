#!/usr/bin/env bash
set -euo pipefail

# Advisory file-placement and naming precheck for changed files.
# Default target: staged changes; fallback: working tree changes.

ROOT_DIR="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT_DIR"

if git diff --cached --quiet --exit-code; then
  CHANGES_CMD=(git diff --name-status --diff-filter=ACMR)
  SOURCE_LABEL="working tree"
else
  CHANGES_CMD=(git diff --cached --name-status --diff-filter=ACMR)
  SOURCE_LABEL="staged"
fi

CHANGES=()
while IFS= read -r line; do
  CHANGES+=("$line")
done < <("${CHANGES_CMD[@]}")

if [[ ${#CHANGES[@]} -eq 0 ]]; then
  echo "reuse-precheck: no changed files in ${SOURCE_LABEL}."
  exit 0
fi

errors=0

is_kebab_md() {
  local base="$1"
  [[ "$base" =~ ^[a-z0-9]+(-[a-z0-9]+)*\.md$ ]]
}

is_dated_file() {
  local base="$1"
  [[ "$base" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z0-9-]+\.(md|yaml)$ ]]
}

is_task_yaml_name() {
  local base="$1"
  [[ "$base" =~ ^[a-z0-9]+_[a-z0-9_]+_[0-9]{8}\.yaml$ ]]
}

check_added_file() {
  local path="$1"
  local base
  base="$(basename "$path")"

  case "$path" in
    internal/*|cmd/*|tests/*|scripts/*|configs/*|tasks/*|docs/*|artifacts/*|tmp/*|.tmp/*|.elephant/tasks/*)
      ;;
    *)
      echo "[ERROR] new file outside governed roots: $path"
      ((errors++))
      ;;
  esac

  case "$path" in
    internal/*)
      case "$path" in
        internal/app/*|internal/domain/*|internal/infra/*|internal/delivery/*|internal/shared/*|internal/devops/*|internal/testutil/*)
          ;;
        *)
          echo "[ERROR] internal path must use approved first-level namespace (app/domain/infra/delivery/shared/devops/testutil): $path"
          ((errors++))
          ;;
      esac
      ;;
  esac

  case "$path" in
    *.go)
      case "$path" in
        internal/*|cmd/*|tests/*|scripts/*) ;;
        *)
          echo "[ERROR] new Go file must live in internal/cmd/tests/scripts: $path"
          ((errors++))
          ;;
      esac
      ;;
    *.sh)
      case "$path" in
        scripts/*) ;;
        *)
          echo "[ERROR] new shell script must live in scripts/: $path"
          ((errors++))
          ;;
      esac
      ;;
    *.py)
      case "$path" in
        scripts/*) ;;
        *)
          echo "[ERROR] new python script must live in scripts/: $path"
          ((errors++))
          ;;
      esac
      ;;
    docs/reference/*.md|docs/guides/*.md)
      if [[ "$base" != "README.md" ]] && ! is_kebab_md "$base"; then
        echo "[ERROR] doc filename must be kebab-case markdown: $path"
        ((errors++))
      fi
      ;;
    docs/plans/*)
      case "$base" in
        README.md|*.yaml) ;;
        *)
          if ! is_dated_file "$base"; then
            echo "[ERROR] plan file must start with YYYY-MM-DD- and use .md/.yaml: $path"
            ((errors++))
          fi
          ;;
      esac
      ;;
    tasks/*.yaml)
      if [[ "$path" != *.status.yaml ]] && ! is_task_yaml_name "$base"; then
        echo "[ERROR] task YAML should match <domain>_<purpose>_<YYYYMMDD>.yaml: $path"
        ((errors++))
      fi
      ;;
  esac
}

check_status_sidecar() {
  local path="$1"
  if [[ "$path" == *.status.yaml ]]; then
    case "$path" in
      tasks/*.status.yaml|.elephant/tasks/*.status.yaml) ;;
      *)
        echo "[ERROR] status sidecar must be under tasks/ or .elephant/tasks/: $path"
        ((errors++))
        ;;
    esac
  fi
}

echo "reuse-precheck: scanning ${#CHANGES[@]} changed file(s) from ${SOURCE_LABEL}."
for row in "${CHANGES[@]}"; do
  status="${row%%$'\t'*}"
  path="${row#*$'\t'}"

  # Rename lines are "R100\told\tnew"; use destination path.
  if [[ "$status" == R* ]]; then
    path="${row##*$'\t'}"
    status="R"
  fi

  check_status_sidecar "$path"

  if [[ "$status" == "A" ]]; then
    check_added_file "$path"
  fi
done

if [[ $errors -gt 0 ]]; then
  echo "reuse-precheck: failed with ${errors} error(s)."
  exit 1
fi

echo "reuse-precheck: passed."
