#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/worktree.sh ensure
  scripts/lark/worktree.sh sync-env

Behavior:
  - Ensures a persistent test worktree exists at .worktrees/test on branch "test"
  - Copies .env from the main worktree into the test worktree (.worktrees/test/.env)
EOF
}

git_worktree_path_for_branch() {
  local want_branch_ref="$1" # e.g. refs/heads/main
  git worktree list --porcelain | awk -v want="${want_branch_ref}" '
    $1=="worktree"{p=$2}
    $1=="branch" && $2==want {print p; exit}
  '
}

main_root="$(git_worktree_path_for_branch "refs/heads/main" || true)"
if [[ -z "${main_root}" ]]; then
  main_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
fi
[[ -n "${main_root}" ]] || die "Not a git repository (cannot resolve main worktree)"

test_root="${main_root}/.worktrees/test"

sync_env() {
  local src="${main_root}/.env"
  local dst="${test_root}/.env"
  [[ -f "${src}" ]] || die "Missing ${src} (create it from .env.example)"
  [[ -d "${test_root}" ]] || die "Missing test worktree at ${test_root} (run: $0 ensure)"

  cp -f "${src}" "${dst}"
  log_success "Synced .env -> ${dst}"
}

ensure() {
  mkdir -p "${main_root}/.worktrees"

  if git -C "${main_root}" worktree list --porcelain | awk -v p="${test_root}" '$1=="worktree" && $2==p {found=1} END{exit found?0:1}'; then
    log_info "Test worktree exists: ${test_root}"
  else
    log_info "Creating test worktree: ${test_root}"
    # -B ensures "test" exists and points to main without failing if it already exists.
    git -C "${main_root}" worktree add -B test "${test_root}" main
  fi

  sync_env
}

cmd="${1:-ensure}"
shift || true

case "${cmd}" in
  ensure) ensure ;;
  sync-env) sync_env ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac

