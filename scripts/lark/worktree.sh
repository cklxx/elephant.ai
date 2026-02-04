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
  git -C "${SCRIPT_DIR}" worktree list --porcelain | awk -v want="${want_branch_ref}" '
    $1=="worktree"{p=$2}
    $1=="branch" && $2==want {print p; exit}
  '
}

main_root="$(git_worktree_path_for_branch "refs/heads/main" || true)"
if [[ -z "${main_root}" ]]; then
  main_root="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel 2>/dev/null || true)"
fi
[[ -n "${main_root}" ]] || die "Not a git repository (cannot resolve main worktree)"

test_root="${main_root}/.worktrees/test"


is_git_worktree_dir() {
  local path="$1"
  git -C "${path}" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

is_valid_test_worktree() {
  is_git_worktree_dir "${test_root}" && [[ -f "${test_root}/go.mod" ]]
}

stale_test_admin_dir() {
  local git_common_dir
  git_common_dir="$(git -C "${main_root}" rev-parse --git-common-dir)"
  if [[ "${git_common_dir}" != /* ]]; then
    git_common_dir="${main_root}/${git_common_dir}"
  fi
  printf '%s/worktrees/test' "${git_common_dir}"
}

ensure() {
  mkdir -p "${main_root}/.worktrees"

  local has_worktree=0
  local backup_root=""

  if git -C "${main_root}" worktree list --porcelain | awk -v p="${test_root}" '$1=="worktree" && $2==p {found=1} END{exit found?0:1}'; then
    has_worktree=1
  fi

  if [[ ${has_worktree} -eq 1 ]]; then
    if is_valid_test_worktree; then
      log_info "Test worktree exists: ${test_root}"
    else
      log_warn "Stale/partial test worktree detected; pruning: ${test_root}"
      git -C "${main_root}" worktree prune || true
      local admin_dir
      admin_dir="$(stale_test_admin_dir)"
      if [[ -d "${admin_dir}" ]]; then
        log_warn "Removing stale worktree admin dir: ${admin_dir}"
        rm -rf "${admin_dir}"
      fi
      has_worktree=0
    fi
  fi

  if [[ ${has_worktree} -eq 0 ]]; then
    if [[ -d "${test_root}" ]]; then
      backup_root="${main_root}/.worktrees/test-orphan-$(date -u +%Y%m%d%H%M%S)"
      log_warn "Non-worktree directory at ${test_root}; moving to ${backup_root}"
      mv "${test_root}" "${backup_root}"
    fi
    log_info "Creating test worktree: ${test_root}"
    # -B ensures "test" exists and points to main without failing if it already exists.
    git -C "${main_root}" worktree add -B test "${test_root}" main
    if ! is_valid_test_worktree; then
      die "Test worktree missing go.mod after create: ${test_root}"
    fi
    if [[ -n "${backup_root}" ]]; then
      if [[ -d "${backup_root}/logs" && ! -e "${test_root}/logs" ]]; then
        mv "${backup_root}/logs" "${test_root}/logs"
      fi
      if [[ -d "${backup_root}/tmp" && ! -e "${test_root}/tmp" ]]; then
        mv "${backup_root}/tmp" "${test_root}/tmp"
      fi
    fi
  fi

}

cmd="${1:-ensure}"
shift || true

case "${cmd}" in
  ensure) ensure ;;
  help|-h|--help) usage ;;
  *) usage; die "Unknown command: ${cmd}" ;;
esac
