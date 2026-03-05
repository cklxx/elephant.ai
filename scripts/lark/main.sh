#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"
# shellcheck source=../lib/common/process.sh
source "${SCRIPT_DIR}/../lib/common/process.sh"
# shellcheck source=../lib/common/build.sh
source "${SCRIPT_DIR}/../lib/common/build.sh"
# shellcheck source=../lib/common/dotenv.sh
source "${SCRIPT_DIR}/../lib/common/dotenv.sh"
# shellcheck source=../lib/common/git_worktree.sh
source "${SCRIPT_DIR}/../lib/common/git_worktree.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"
# shellcheck source=./component.sh
source "${SCRIPT_DIR}/component.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/main.sh start|stop|restart|status|logs|build

Runs alex-server in standalone Lark WebSocket mode (no HTTP server).

Env:
  MAIN_CONFIG   Config path (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
  LARK_PID_DIR  Shared pid dir override (default: <dirname(MAIN_CONFIG)>/pids)
  ALEX_LOG_DIR  Internal log dir override (default: <repo>/logs)
  LARK_NOTICE_STATE_FILE Notice binding state path (default: <repo>/.worktrees/test/tmp/lark-notice.state.json)
  FORCE_REBUILD=1  Force rebuild on start (default: 0)
  SKIP_LOCAL_AUTH_DB=1  Skip local auth DB auto-setup (default: 0)
EOF
}

# ---------------------------------------------------------------------------
# Resolve paths
# ---------------------------------------------------------------------------

ROOT="$(git_resolve_main_root "${SCRIPT_DIR}" || true)"
[[ -n "${ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"

# ---------------------------------------------------------------------------
# Component configuration
# ---------------------------------------------------------------------------

COMPONENT_NAME="Lark agent"
COMPONENT_TAG="main"
CONFIG_PATH="${MAIN_CONFIG}"
BIN="${ROOT}/alex-server"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG}")"
PID_FILE="${PID_DIR}/lark-main.pid"
BUILD_STAMP="${PID_DIR}/lark-main.build"
SHA_FILE="${PID_DIR}/lark-main.sha"
LOG_FILE="${ROOT}/logs/lark-main.log"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${ROOT}/logs}"
READY_LOG_PATTERN="Lark gateway connecting"
FORCE_REBUILD="${FORCE_REBUILD:-0}"
BOOTSTRAP_SH="${ROOT}/scripts/setup_local_runtime.sh"
CLEANUP_ORPHANS_SH="${ROOT}/scripts/lark/cleanup_orphan_agents.sh"
NOTICE_STATE_FILE="${LARK_NOTICE_STATE_FILE:-${ROOT}/.worktrees/test/tmp/lark-notice.state.json}"
COMPONENT_USAGE_FN="usage"
COMPONENT_LOG_EXTRAS="${ALEX_LOG_DIR}/alex-service.log ${ALEX_LOG_DIR}/alex-llm.log ${ALEX_LOG_DIR}/alex-latency.log"

component_ensure_dirs

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------

component_dispatch "${1:-start}"
