#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOOP_SH="${ROOT}/scripts/lark/loop.sh"

sandbox="$(mktemp -d 2>/dev/null || mktemp -d -t lark-loop-merge-main-head)"
cleanup() {
  rm -rf "${sandbox}"
}
trap cleanup EXIT

main_root="${sandbox}/main"
test_root="${main_root}/.worktrees/test"
loop_log="${sandbox}/lark-loop.log"

mkdir -p "${main_root}"

git -C "${main_root}" init -q -b main 2>/dev/null || git -C "${main_root}" init -q
current_branch="$(git -C "${main_root}" symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
if [[ "${current_branch}" != "main" ]]; then
  git -C "${main_root}" checkout -q -b main
fi

echo "base" > "${main_root}/README.md"
git -C "${main_root}" add README.md
git -C "${main_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "init"
base_sha="$(git -C "${main_root}" rev-parse HEAD)"

git -C "${main_root}" worktree add --detach "${test_root}" main >/dev/null

if git -C "${test_root}" symbolic-ref -q HEAD >/dev/null 2>&1; then
  echo "expected detached HEAD in test worktree" >&2
  exit 1
fi

echo "candidate" >> "${test_root}/README.md"
git -C "${test_root}" add README.md
git -C "${test_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "candidate change"
candidate_sha="$(git -C "${test_root}" rev-parse HEAD)"

touch "${loop_log}"
set -- help
# shellcheck source=/dev/null
source "${LOOP_SH}" >/dev/null

MAIN_ROOT="${main_root}"
TEST_ROOT="${test_root}"
LOOP_LOG="${loop_log}"

merge_into_main_ff_only "${base_sha}"

new_main_sha="$(git -C "${main_root}" rev-parse HEAD)"
if [[ "${new_main_sha}" != "${candidate_sha}" ]]; then
  echo "expected ff-only merge to promote test HEAD (main=${new_main_sha} candidate=${candidate_sha})" >&2
  exit 1
fi

echo "lark loop merge from test head: PASS"
