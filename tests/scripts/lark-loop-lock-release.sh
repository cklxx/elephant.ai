#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOOP_SH="${ROOT}/scripts/lark/loop.sh"

run_lock_release_regression() {
  local sandbox="$1"
  local calls_file="${sandbox}/calls.log"
  local output_file="${sandbox}/output.log"

  set -- help
  # shellcheck source=/dev/null
  source "${LOOP_SH}" >/dev/null

  require_tools() { :; }

  init_test_paths() {
    LOG_DIR="${sandbox}"
    TMP_DIR="${sandbox}"
    LOCK_DIR="${sandbox}/lark-loop.lock"
    LAST_FILE="${sandbox}/lark-loop.last"
    LAST_VALIDATED_FILE="${sandbox}/lark-loop.last-validated"
    LOOP_LOG="${sandbox}/lark-loop.log"
    : > "${LOOP_LOG}"
  }

  run_cycle_locked() {
    printf '%s\n' "cycle:$1" >> "${calls_file}"
    return 0
  }

  run_cycle "sha-one" >> "${output_file}" 2>&1

  if [[ -d "${sandbox}/lark-loop.lock" ]]; then
    echo "expected lock directory to be removed after first run_cycle" >&2
    return 1
  fi

  run_cycle "sha-two" >> "${output_file}" 2>&1

  local call_count
  call_count="$(wc -l < "${calls_file}")"
  if [[ "${call_count}" -ne 2 ]]; then
    echo "expected run_cycle_locked to execute twice, got ${call_count}" >&2
    cat "${output_file}" >&2 || true
    return 1
  fi

  if grep -q "Loop already running" "${output_file}"; then
    echo "unexpected self-lock warning after sequential run_cycle invocations" >&2
    cat "${output_file}" >&2 || true
    return 1
  fi
}

run_stale_lock_recovery_regression() {
  local sandbox="$1"
  local calls_file="${sandbox}/stale-calls.log"
  local output_file="${sandbox}/stale-output.log"
  local stale_pid=999999

  set -- help
  # shellcheck source=/dev/null
  source "${LOOP_SH}" >/dev/null

  require_tools() { :; }

  init_test_paths() {
    LOG_DIR="${sandbox}"
    TMP_DIR="${sandbox}"
    LOCK_DIR="${sandbox}/lark-loop.lock"
    LAST_FILE="${sandbox}/lark-loop.last"
    LAST_VALIDATED_FILE="${sandbox}/lark-loop.last-validated"
    LOOP_LOG="${sandbox}/lark-loop.log"
    : > "${LOOP_LOG}"
  }

  run_cycle_locked() {
    printf '%s\n' "cycle:$1" >> "${calls_file}"
    return 0
  }

  mkdir -p "${sandbox}/lark-loop.lock"
  printf '%s\n' "pid=${stale_pid} started_at=2026-02-09T00:00:00Z" > "${sandbox}/lark-loop.lock/owner"

  run_cycle "sha-stale" >> "${output_file}" 2>&1

  local call_count
  call_count="$(wc -l < "${calls_file}")"
  if [[ "${call_count}" -ne 1 ]]; then
    echo "expected stale lock recovery to execute run_cycle_locked once" >&2
    cat "${output_file}" >&2 || true
    return 1
  fi

  if grep -q "Loop already running" "${output_file}"; then
    echo "stale lock should be recovered instead of reporting running lock" >&2
    cat "${output_file}" >&2 || true
    return 1
  fi
}

main() {
  local sandbox
  sandbox="$(mktemp -d)"
  trap 'rm -rf "${sandbox}"' EXIT

  run_lock_release_regression "${sandbox}"
  run_stale_lock_recovery_regression "${sandbox}"
  echo "lark loop lock release regression: PASS"
}

main "$@"
