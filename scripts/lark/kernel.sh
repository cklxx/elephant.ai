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
  cat <<'USAGE'
Usage:
  scripts/lark/kernel.sh start|stop|restart|status|logs|build

Runs alex-server in standalone kernel daemon mode.

Env:
  MAIN_CONFIG        Config path (default: $ALEX_CONFIG_PATH or ~/.alex/config.yaml)
  LARK_PID_DIR       Shared pid dir override (default: <dirname(MAIN_CONFIG)>/pids)
  ALEX_LOG_DIR       Internal log dir override (default: <repo>/logs)
  LARK_NOTICE_STATE_FILE Notice binding state path (default: <repo>/.worktrees/test/tmp/lark-notice.state.json)
  FORCE_REBUILD=1    Force rebuild on start (default: 0)
  SKIP_LOCAL_AUTH_DB=1  Skip local auth DB auto-setup (default: 0)
USAGE
}

# ---------------------------------------------------------------------------
# Resolve paths
# ---------------------------------------------------------------------------

ROOT="$(git_resolve_main_root "${SCRIPT_DIR}" || true)"
[[ -n "${ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
NOTICE_STATE_FILE="${LARK_NOTICE_STATE_FILE:-${ROOT}/.worktrees/test/tmp/lark-notice.state.json}"

# ---------------------------------------------------------------------------
# Component configuration
# ---------------------------------------------------------------------------

COMPONENT_NAME="Kernel daemon"
COMPONENT_TAG="kernel"
CONFIG_PATH="${MAIN_CONFIG}"
BIN="${ROOT}/alex-server"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG}")"
PID_FILE="${PID_DIR}/lark-kernel.pid"
BUILD_STAMP="${PID_DIR}/lark-kernel.build"
SHA_FILE="${PID_DIR}/lark-kernel.sha"
LOG_FILE="${ROOT}/logs/lark-kernel.log"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${ROOT}/logs}"
READY_LOG_PATTERN="Kernel daemon running"
FORCE_REBUILD="${FORCE_REBUILD:-0}"
BOOTSTRAP_SH="${ROOT}/scripts/setup_local_runtime.sh"
COMPONENT_BIN_ARGS="kernel-daemon"
COMPONENT_USAGE_FN="usage"
# Kernel doesn't use identity lock (shares config with main)
USE_IDENTITY_LOCK="0"
# Kernel checks an extra log file for readiness
COMPONENT_READY_EXTRA_LOG="${ALEX_LOG_DIR}/alex-kernel.log"
COMPONENT_LOG_EXTRAS="${ALEX_LOG_DIR}/alex-kernel.log"

component_ensure_dirs

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------

component_dispatch "${1:-start}"
