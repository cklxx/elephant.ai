#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOOP_SH="${ROOT}/scripts/lark/loop.sh"
TEST_SANDBOX=""

prepare_loop_context() {
  local sandbox="$1"
  MAIN_ROOT="${sandbox}/main"
  TEST_ROOT="${sandbox}/test"
  mkdir -p "${MAIN_ROOT}" "${TEST_ROOT}"

  LOG_DIR="${sandbox}"
  TMP_DIR="${sandbox}"
  LOOP_LOG="${sandbox}/lark-loop.log"
  FAIL_SUMMARY="${sandbox}/lark-loop.fail.txt"
  SCENARIO_JSON="${sandbox}/lark-scenarios.json"
  SCENARIO_MD="${sandbox}/lark-scenarios.md"
  LOOP_STATE="${sandbox}/lark-loop.state.json"
  LAST_FILE="${sandbox}/lark-loop.last"
  LAST_VALIDATED_FILE="${sandbox}/lark-loop.last-validated"
  : > "${LOOP_LOG}"
}

install_stubs() {
  git() {
    local cmd=""
    if [[ "${1:-}" == "-C" ]]; then
      shift 2
    fi
    cmd="${1:-}"
    case "${cmd}" in
      switch|reset|merge)
        return 0
        ;;
      rev-parse)
        printf '%s\n' "abc1234567890"
        return 0
        ;;
      add|commit|diff)
        return 0
        ;;
      *)
        return 0
        ;;
    esac
  }

  write_loop_state() { :; }
  lark_ensure_test_worktree() { :; }
  stop_test_agent() { :; }
  restore_test_to_validated() { :; }
  restart_test_agent() { :; }
  restart_main_agent() { :; }
  merge_into_main_ff_only() { return 0; }
}

run_with_autofix_disabled() {
  local sandbox="$1"
  local auto_fix_calls=0

  prepare_loop_context "${sandbox}"
  install_stubs

  MAX_CYCLES=3
  MAX_CYCLES_SLOW=1
  LOOP_AUTOFIX_ENABLED=0

  run_fast_gate() { return 1; }
  run_slow_gate() { return 0; }
  auto_fix() { auto_fix_calls=$((auto_fix_calls + 1)); }

  if run_cycle_locked "abc1234567890"; then
    echo "expected run_cycle_locked to fail when fast gate fails and auto-fix is disabled" >&2
    return 1
  fi

  if [[ "${auto_fix_calls}" -ne 0 ]]; then
    echo "expected auto_fix not to run when LOOP_AUTOFIX_ENABLED=0" >&2
    return 1
  fi

  if ! grep -q "auto-fix disabled" "${LOOP_LOG}"; then
    echo "expected disabled auto-fix log entry" >&2
    return 1
  fi
}

run_with_autofix_enabled() {
  local sandbox="$1"
  local auto_fix_calls=0
  local fast_calls=0

  prepare_loop_context "${sandbox}"
  install_stubs

  MAX_CYCLES=3
  MAX_CYCLES_SLOW=1
  LOOP_AUTOFIX_ENABLED=1

  run_fast_gate() {
    fast_calls=$((fast_calls + 1))
    if [[ "${fast_calls}" -eq 1 ]]; then
      return 1
    fi
    return 0
  }
  run_slow_gate() { return 0; }
  auto_fix() { auto_fix_calls=$((auto_fix_calls + 1)); }

  run_cycle_locked "abc1234567890"

  if [[ "${auto_fix_calls}" -lt 1 ]]; then
    echo "expected auto_fix to run when LOOP_AUTOFIX_ENABLED=1" >&2
    return 1
  fi
}

main() {
  TEST_SANDBOX="$(mktemp -d)"
  trap 'rm -rf "${TEST_SANDBOX}"' EXIT

  set -- help
  # shellcheck source=/dev/null
  source "${LOOP_SH}" >/dev/null

  run_with_autofix_disabled "${TEST_SANDBOX}/disabled"
  run_with_autofix_enabled "${TEST_SANDBOX}/enabled"

  echo "lark loop autofix toggle regression: PASS"
}

main "$@"
