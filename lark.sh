#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'EOF'
Usage:
  ./lark.sh ma [start|stop|restart|status|logs|build]
  ./lark.sh ta [start|stop|restart|status|logs]

Meaning:
  ma = main agent (alex-server + local auth DB)
  ta = test agent (local self-heal loop watcher)

Notes:
  - ta will ensure the persistent test worktree exists at .worktrees/test and sync .env
  - If you also want to start the test server (optional), use:
      ./scripts/lark/test.sh start
EOF
}

mode="${1:-}"
cmd="${2:-start}"

case "${mode}" in
  ma)
    exec "${ROOT}/scripts/lark/main.sh" "${cmd}"
    ;;
  ta)
    # Ensure test worktree + .env exist before starting the loop.
    "${ROOT}/scripts/lark/worktree.sh" ensure >/dev/null
    exec "${ROOT}/scripts/lark/loop-agent.sh" "${cmd}"
    ;;
  help|-h|--help|"")
    usage
    exit 0
    ;;
  *)
    usage
    echo "Unknown mode: ${mode}" >&2
    exit 2
    ;;
esac

