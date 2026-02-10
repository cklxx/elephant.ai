#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

SUPERVISOR_SH="${ROOT}/scripts/lark/supervisor.sh"
WORKTREE_SH="${ROOT}/scripts/lark/worktree.sh"
LOOP_SH="${ROOT}/scripts/lark/loop.sh"

usage() {
  cat <<'EOF'
Usage:
  ./lark.sh up
  ./lark.sh down
  ./lark.sh restart
  ./lark.sh status
  ./lark.sh logs
  ./lark.sh doctor
  ./lark.sh cycle --base-sha <sha>

Aliases:
  ./lark.sh start   -> up
  ./lark.sh stop    -> down

Notes:
  - This is the only supported entrypoint for local lark autonomous iteration.
  - up does exactly two things: ensure test worktree/.env, then start supervisor.
  - loop gate auto-fix is controlled by LARK_LOOP_AUTOFIX_ENABLED (default: 0).
  - Deprecated (compat for one cycle): ./lark.sh ma ..., ./lark.sh ta ...
EOF
}

ensure_worktree() {
  "${WORKTREE_SH}" ensure >/dev/null
}

run_cycle() {
  local base_sha=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --base-sha)
        base_sha="${2:-}"
        shift 2
        ;;
      *)
        usage
        echo "Unknown cycle arg: $1" >&2
        exit 2
        ;;
    esac
  done
  if [[ -z "${base_sha}" ]]; then
    usage
    echo "--base-sha is required for cycle" >&2
    exit 2
  fi
  ensure_worktree
  exec "${LOOP_SH}" run --base-sha "${base_sha}"
}

forward_deprecated() {
  local mode="$1"
  shift || true
  local cmd="${1:-start}"
  if [[ $# -gt 0 ]]; then
    shift
  fi
  echo "[DEPRECATED] ./lark.sh ${mode} ... is deprecated. Use ./lark.sh up|down|restart|status|logs|doctor|cycle." >&2

  case "${cmd}" in
    start|up)
      exec "${ROOT}/lark.sh" up "$@"
      ;;
    stop|down)
      exec "${ROOT}/lark.sh" down "$@"
      ;;
    restart)
      exec "${ROOT}/lark.sh" restart "$@"
      ;;
    status)
      exec "${ROOT}/lark.sh" status "$@"
      ;;
    logs)
      exec "${ROOT}/lark.sh" logs "$@"
      ;;
    doctor)
      exec "${ROOT}/lark.sh" doctor "$@"
      ;;
    cycle)
      exec "${ROOT}/lark.sh" cycle "$@"
      ;;
    *)
      usage
      echo "Deprecated alias ./lark.sh ${mode} supports: start|stop|restart|status|logs|doctor|cycle" >&2
      exit 2
      ;;
  esac
}

cmd="${1:-up}"
shift || true

case "${cmd}" in
  up|start)
    ensure_worktree
    exec "${SUPERVISOR_SH}" start
    ;;
  down|stop)
    exec "${SUPERVISOR_SH}" stop
    ;;
  restart)
    ensure_worktree
    exec "${SUPERVISOR_SH}" restart
    ;;
  status)
    exec "${SUPERVISOR_SH}" status
    ;;
  logs)
    exec "${SUPERVISOR_SH}" logs
    ;;
  doctor)
    exec "${SUPERVISOR_SH}" doctor
    ;;
  cycle)
    run_cycle "$@"
    ;;
  ma|ta)
    forward_deprecated "${cmd}" "$@"
    ;;
  help|-h|--help|"")
    usage
    exit 0
    ;;
  *)
    usage
    echo "Unknown command: ${cmd}" >&2
    exit 2
    ;;
esac
