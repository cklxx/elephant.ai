#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=../lib/common/git_worktree.sh
source "${SCRIPT_DIR}/../lib/common/git_worktree.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/cleanup_orphan_agents.sh list [--scope main|all]
  scripts/lark/cleanup_orphan_agents.sh cleanup [--scope main|all] [--quiet]

Behavior:
  - Finds running local main Lark agent processes (`<main_root>/alex-server [lark]`)
  - Keeps managed PID from `<shared_pid_dir>/lark-main.pid`
  - Stops orphan main processes that are no longer tracked by PID files
EOF
}

ROOT="$(git_resolve_main_root "${SCRIPT_DIR}" || true)"
[[ -n "${ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

MAIN_CONFIG_PATH="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG_PATH}")"
MAIN_PID_FILE="${PID_DIR}/lark-main.pid"

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
  # Match both new-style (`alex-server` without args) and legacy (`alex-server lark`),
  # but avoid matching other subcommands.
  ps -axo pid= -o command= | awk \
    -v main_cmd="${ROOT}/alex-server" \
    -v main_cmd_legacy="${ROOT}/alex-server lark" \
    -v scope="${scope}" '
    {
      pid = $1
      $1 = ""
      sub(/^ /, "", $0)
      cmd = $0

      if ((scope == "all" || scope == "main") && (cmd == main_cmd_legacy || cmd == main_cmd)) {
        print pid "\tmain\t" cmd
      }
    }
  '
}

is_tracked_pid() {
  local pid="$1"
  local kind="$2"
  local tracked_main="$3"
  case "${kind}" in
    main)
      [[ -n "${tracked_main}" && "${pid}" == "${tracked_main}" ]]
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
  local tracked_main
  local total kept killed
  tracked_main="$(read_pid_if_running "${MAIN_PID_FILE}" || true)"

  total=0
  kept=0
  killed=0

  while IFS=$'\t' read -r pid kind cmd; do
    [[ -n "${pid}" ]] || continue
    total=$((total + 1))

    if is_tracked_pid "${pid}" "${kind}" "${tracked_main}"; then
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
  main|all)
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
