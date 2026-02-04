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
  ta = test agent (alex-server + local auth DB + local self-heal loop watcher)

Notes:
  - For ma/ta, "start" always performs a restart (stop + start).
  - ta will ensure the persistent test worktree exists at .worktrees/test and sync .env
  - ta uses config at ~/.alex/test.yaml by default (override with TEST_CONFIG=/abs/path.yaml)
EOF
}

mode="${1:-}"
cmd="${2:-start}"

# For lark.sh, we always restart ma/ta on "start" (and default invocation).
if [[ "${cmd}" == "start" ]]; then
  case "${mode}" in
    ma|ta) cmd="restart" ;;
  esac
fi

case "${mode}" in
  ma)
    exec "${ROOT}/scripts/lark/main.sh" "${cmd}"
    ;;
  ta)
    # Ensure test worktree + .env exist before starting any test-side processes.
    "${ROOT}/scripts/lark/worktree.sh" ensure >/dev/null

    case "${cmd}" in
      start)
        "${ROOT}/scripts/lark/test.sh" start
        exec "${ROOT}/scripts/lark/loop-agent.sh" start
        ;;
      stop)
        "${ROOT}/scripts/lark/loop-agent.sh" stop || true
        exec "${ROOT}/scripts/lark/test.sh" stop || true
        ;;
      restart)
        "${ROOT}/scripts/lark/loop-agent.sh" stop || true
        "${ROOT}/scripts/lark/test.sh" stop || true
        "${ROOT}/scripts/lark/test.sh" start
        exec "${ROOT}/scripts/lark/loop-agent.sh" start
        ;;
      status)
        "${ROOT}/scripts/lark/test.sh" status || true
        exec "${ROOT}/scripts/lark/loop-agent.sh" status || true
        ;;
      logs)
        test_root="${ROOT}/.worktrees/test"
        mkdir -p "${test_root}/logs"
        touch \
          "${test_root}/logs/lark-test.log" \
          "${test_root}/logs/alex-service.log" \
          "${test_root}/logs/alex-llm.log" \
          "${test_root}/logs/alex-latency.log" \
          "${test_root}/logs/lark-loop.log" \
          "${test_root}/logs/lark-loop-agent.log"
        tail -n 200 -f \
          "${test_root}/logs/lark-test.log" \
          "${test_root}/logs/alex-service.log" \
          "${test_root}/logs/alex-llm.log" \
          "${test_root}/logs/alex-latency.log" \
          "${test_root}/logs/lark-loop.log" \
          "${test_root}/logs/lark-loop-agent.log"
        ;;
      *)
        exec "${ROOT}/scripts/lark/loop-agent.sh" "${cmd}"
        ;;
    esac
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
