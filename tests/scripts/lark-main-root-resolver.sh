#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../scripts/lib/common/git_worktree.sh
source "${ROOT}/scripts/lib/common/git_worktree.sh"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t lark-main-root-resolver)"
repo_root="${tmpdir}/repo"
anchor_dir="${repo_root}/scripts/lark"
main_worktree="${tmpdir}/main-worktree"

cleanup() {
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

mkdir -p "${repo_root}"
git -C "${repo_root}" init -q -b main 2>/dev/null || git -C "${repo_root}" init -q
current_branch="$(git -C "${repo_root}" symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
if [[ "${current_branch}" != "main" ]]; then
  git -C "${repo_root}" checkout -q -b main
fi

echo "resolver" > "${repo_root}/README.md"
git -C "${repo_root}" add README.md
git -C "${repo_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "init"
git -C "${repo_root}" checkout -q -b feature

mkdir -p "${anchor_dir}"
git -C "${repo_root}" worktree add -f "${main_worktree}" main >/dev/null
repo_root_resolved="$(git -C "${repo_root}" rev-parse --show-toplevel)"
main_worktree_resolved="$(git -C "${main_worktree}" rev-parse --show-toplevel)"

resolved="$(git_resolve_main_root "${anchor_dir}" || true)"
[[ "${resolved}" == "${main_worktree_resolved}" ]] || {
  echo "expected valid main worktree root: got '${resolved}', want '${main_worktree_resolved}'" >&2
  exit 1
}

rm -rf "${main_worktree}"
resolved_after_stale="$(git_resolve_main_root "${anchor_dir}" || true)"
[[ "${resolved_after_stale}" == "${repo_root_resolved}" ]] || {
  echo "expected fallback to repo root after stale main worktree: got '${resolved_after_stale}', want '${repo_root_resolved}'" >&2
  exit 1
}

if git_resolve_main_root "${tmpdir}" >/dev/null 2>&1; then
  echo "expected resolver failure outside git repository" >&2
  exit 1
fi

echo "ok"
