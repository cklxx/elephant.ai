#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/cleanup_orphan_agents.sh list [--scope main|test|all]
  scripts/lark/cleanup_orphan_agents.sh cleanup [--scope main|test|all] [--quiet]

Behavior:
  - Finds running local Lark agent processes (`.../alex-server lark`)
  - Keeps managed PIDs from `<shared_pid_dir>/lark-main.pid` and `<shared_pid_dir>/lark-test.pid`
  - Stops orphan processes that are no longer tracked by PID files
EOF
}

git_worktree_path_for_branch() {
  local want_branch_ref="$1" # e.g. refs/heads/main
  git worktree list --porcelain | awk -v want="${want_branch_ref}" '
    $1=="worktree"{p=$2}
    $1=="branch" && $2==want {print p; exit}
  '
}

ROOT="$(git_worktree_path_for_branch "refs/heads/main" || true)"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
fi
[[ -n "${ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

TEST_ROOT="${ROOT}/.worktrees/test"
MAIN_CONFIG_PATH="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG_PATH}")"
MAIN_PID_FILE="${PID_DIR}/lark-main.pid"
TEST_PID_FILE="${PID_DIR}/lark-test.pid"

SCOPE="all"
QUIET=0

read_pid_if_running() {
  local pid_file="$1"
  local pid
  pid="$(read_pid "${pid_file}" || true)"
  if is_process_running "${pid}"; then
    echo "${pid}"
  fi
}

collect_candidates() {
  local scope="$1"
  # Match both new-style (`alex-server` without args) and legacy (`alex-server lark`).
  ps -axo pid= -o command= | awk \
    -v main_cmd="${ROOT}/alex-server" \
    -v main_cmd_legacy="${ROOT}/alex-server lark" \
    -v test_cmd="${TEST_ROOT}/alex-server" \
    -v test_cmd_legacy="${TEST_ROOT}/alex-server lark" \
    -v scope="${scope}" '
    {
      pid = $1
      $1 = ""
      sub(/^ /, "", $0)
      cmd = $0

      if ((scope == "all" || scope == "main") && (index(cmd, main_cmd_legacy) == 1 || index(cmd, main_cmd) == 1)) {
        print pid "\tmain\t" cmd
      }
      if ((scope == "all" || scope == "test") && (index(cmd, test_cmd_legacy) == 1 || index(cmd, test_cmd) == 1)) {
        print pid "\ttest\t" cmd
      }
    }
  '
}

is_tracked_pid() {
  local pid="$1"
  local kind="$2"
  local tracked_main="$3"
  local tracked_test="$4"
  case "${kind}" in
    main)
      [[ -n "${tracked_main}" && "${pid}" == "${tracked_main}" ]]
      ;;
    test)
      [[ -n "${tracked_test}" && "${pid}" == "${tracked_test}" ]]
      ;;
    *)
      return 1
      ;;
  esac
}

list_candidates() {
  collect_candidates "${SCOPE}"
}

cleanup_orphans() {
  local tracked_main tracked_test
  local total kept killed
  tracked_main="$(read_pid_if_running "${MAIN_PID_FILE}" || true)"
  tracked_test="$(read_pid_if_running "${TEST_PID_FILE}" || true)"

  total=0
  kept=0
  killed=0

  while IFS=$'\t' read -r pid kind cmd; do
    [[ -n "${pid}" ]] || continue
    total=$((total + 1))

    if is_tracked_pid "${pid}" "${kind}" "${tracked_main}" "${tracked_test}"; then
      kept=$((kept + 1))
      if [[ "${QUIET}" != "1" ]]; then
        log_info "Keeping managed ${kind} agent PID ${pid}"
      fi
      continue
    fi

    if [[ "${QUIET}" != "1" ]]; then
      log_warn "Stopping orphan ${kind} agent PID ${pid}"
    fi
    stop_pid "${pid}" "orphan ${kind} agent" 12 0.25 >/dev/null 2>&1 || true
    killed=$((killed + 1))
  done < <(collect_candidates "${SCOPE}")

  if [[ "${QUIET}" != "1" ]]; then
    log_info "Orphan cleanup summary: scope=${SCOPE} total=${total} kept=${kept} killed=${killed}"
  fi
}

cmd="${1:-cleanup}"
shift || true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)
      SCOPE="${2:-}"
      shift 2
      ;;
    --quiet)
      QUIET=1
      shift
      ;;
    help|-h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      die "Unknown arg: $1"
      ;;
  esac
done

case "${SCOPE}" in
  main|test|all)
    ;;
  *)
    usage
    die "Invalid --scope: ${SCOPE}"
    ;;
esac

case "${cmd}" in
  list)
    list_candidates
    ;;
  cleanup)
    cleanup_orphans
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    usage
    die "Unknown command: ${cmd}"
    ;;
esac
