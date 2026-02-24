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
# shellcheck source=../lib/common/lark_test_worktree.sh
source "${SCRIPT_DIR}/../lib/common/lark_test_worktree.sh"
# shellcheck source=./identity_lock.sh
source "${SCRIPT_DIR}/identity_lock.sh"
# shellcheck source=./component.sh
source "${SCRIPT_DIR}/component.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/test.sh start|stop|restart|status|logs|build

Runs alex-server in standalone Lark WebSocket mode from the test worktree.

Behavior:
  - Ensures persistent test worktree exists at .worktrees/test and syncs .env
  - Builds and starts alex-server from the test worktree

Env:
  TEST_CONFIG          Config path (default: ~/.alex/test.yaml)
  LARK_PID_DIR         Shared pid dir override (default: <dirname(MAIN_CONFIG)>/pids)
  ALEX_LOG_DIR         Internal log dir override (default: <repo>/.worktrees/test/logs)
  LARK_NOTICE_STATE_FILE Notice binding state path (default: <repo>/.worktrees/test/tmp/lark-notice.state.json)
  FORCE_REBUILD=1      Force rebuild on start (default: 1)
  LARK_TEST_SYNC_DIRTY_OVERLAY=1 Apply main worktree tracked dirty diff to test runtime after SHA align (default: 1)
  SKIP_LOCAL_AUTH_DB=1 Skip local auth DB auto-setup (default: 0)
EOF
}

# ---------------------------------------------------------------------------
# Resolve paths
# ---------------------------------------------------------------------------

ROOT="$(git_worktree_path_for_branch "refs/heads/main" || true)"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
fi
[[ -n "${ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

TEST_ROOT="${ROOT}/.worktrees/test"
MAIN_CONFIG_PATH_FOR_PID="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}"
TEST_CONFIG="${TEST_CONFIG:-$HOME/.alex/test.yaml}"
SYNC_DIRTY_OVERLAY="${LARK_TEST_SYNC_DIRTY_OVERLAY:-1}"

# ---------------------------------------------------------------------------
# Test worktree sync helpers (unique to test.sh)
# ---------------------------------------------------------------------------

ensure_worktree() {
  lark_ensure_test_worktree "${ROOT}"
  mkdir -p "${PID_DIR}" "${TEST_ROOT}/logs"
}

sync_test_runtime_to_main() {
	local main_sha test_sha
	main_sha="$(git -C "${ROOT}" rev-parse main 2>/dev/null || true)"
	[[ -n "${main_sha}" ]] || die "Failed to resolve main SHA"

  log_info "Aligning test worktree runtime to main (${main_sha:0:8})"
  git -C "${TEST_ROOT}" reset --hard "${main_sha}" >/dev/null 2>&1
  if ! git -C "${TEST_ROOT}" switch --detach "${main_sha}" >/dev/null 2>&1; then
    git -C "${TEST_ROOT}" checkout --detach "${main_sha}" >/dev/null 2>&1 || true
  fi

	test_sha="$(git -C "${TEST_ROOT}" rev-parse HEAD 2>/dev/null || true)"
	if [[ "${test_sha}" != "${main_sha}" ]]; then
		die "Failed to align test worktree to main (main=${main_sha} test=${test_sha})"
	fi

	apply_main_dirty_overlay
}

apply_main_dirty_overlay() {
	if [[ "${SYNC_DIRTY_OVERLAY}" != "1" ]]; then
		return 0
	fi

	local overlay_patch
	overlay_patch="$(mktemp "${TMPDIR:-/tmp}/lark-test-overlay.XXXXXX.patch")"
	if ! git -C "${ROOT}" diff --binary --no-ext-diff HEAD -- . ":(exclude).worktrees" > "${overlay_patch}"; then
		rm -f "${overlay_patch}"
		die "Failed to generate main worktree overlay patch"
	fi

	if [[ ! -s "${overlay_patch}" ]]; then
		rm -f "${overlay_patch}"
		sync_main_untracked_overlay
		return 0
	fi

	log_info "Applying main working-tree overlay to test runtime"
	if ! git -C "${TEST_ROOT}" apply --3way --whitespace=nowarn "${overlay_patch}" >/dev/null 2>&1; then
		rm -f "${overlay_patch}"
		die "Failed to apply main worktree overlay patch to test runtime"
	fi
	rm -f "${overlay_patch}"
	sync_main_untracked_overlay
}

sync_main_untracked_overlay() {
	local path copied
	copied=0
	while IFS= read -r path; do
		[[ -n "${path}" ]] || continue
		case "${path}" in
			.worktrees/*) continue ;;
		esac
		case "${path}" in
			*.go|*.mod|*.sum|*.yaml|*.yml|*.json|*.sh|*.md|*.txt|*.proto|*.toml) ;;
			*) continue ;;
		esac
		[[ -f "${ROOT}/${path}" ]] || continue
		mkdir -p "${TEST_ROOT}/$(dirname "${path}")"
		cp -f "${ROOT}/${path}" "${TEST_ROOT}/${path}"
		copied=$((copied + 1))
	done < <(git -C "${ROOT}" ls-files --others --exclude-standard -- . ":(exclude).worktrees")

	if (( copied > 0 )); then
		log_info "Copied ${copied} untracked source file(s) to test runtime"
	fi
}

test_pre_build() {
  ensure_worktree
  sync_test_runtime_to_main
}

# ---------------------------------------------------------------------------
# Component configuration
# ---------------------------------------------------------------------------

COMPONENT_NAME="Test Lark agent"
COMPONENT_TAG="test"
CONFIG_PATH="${TEST_CONFIG}"
BIN="${TEST_ROOT}/alex-server"
PID_DIR="$(lark_shared_pid_dir "${MAIN_CONFIG_PATH_FOR_PID}")"
PID_FILE="${PID_DIR}/lark-test.pid"
BUILD_STAMP="${PID_DIR}/lark-test.build"
SHA_FILE="${PID_DIR}/lark-test.sha"
LOG_FILE="${TEST_ROOT}/logs/lark-test.log"
ALEX_LOG_DIR="${ALEX_LOG_DIR:-${TEST_ROOT}/logs}"
READY_LOG_PATTERN="Lark gateway connecting"
FORCE_REBUILD="${FORCE_REBUILD:-1}"
BOOTSTRAP_SH="${ROOT}/scripts/setup_local_runtime.sh"
CLEANUP_ORPHANS_SH="${ROOT}/scripts/lark/cleanup_orphan_agents.sh"
NOTICE_STATE_FILE="${LARK_NOTICE_STATE_FILE:-${ROOT}/.worktrees/test/tmp/lark-notice.state.json}"
COMPONENT_BUILD_ROOT="${TEST_ROOT}"
COMPONENT_BUILD_LABEL="test worktree"
PRE_BUILD_HOOK="test_pre_build"
COMPONENT_USAGE_FN="usage"
COMPONENT_LOG_EXTRAS="${ALEX_LOG_DIR}/alex-service.log ${ALEX_LOG_DIR}/alex-llm.log ${ALEX_LOG_DIR}/alex-latency.log"

component_ensure_dirs

# ---------------------------------------------------------------------------
# Override stop to ensure worktree exists first
# ---------------------------------------------------------------------------

_original_component_stop() { component_stop; }

test_component_stop() {
  ensure_worktree
  _original_component_stop
}

test_component_status() {
  ensure_worktree
  component_status
}

test_component_logs() {
  ensure_worktree
  component_logs
}

# ---------------------------------------------------------------------------
# Dispatch (with test-specific overrides)
# ---------------------------------------------------------------------------

cmd="${1:-start}"
shift || true

case "${cmd}" in
  start)   component_start ;;
  stop)    test_component_stop ;;
  restart) component_restart ;;
  status)  test_component_status ;;
  logs)    test_component_logs ;;
  build)   component_build ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac
