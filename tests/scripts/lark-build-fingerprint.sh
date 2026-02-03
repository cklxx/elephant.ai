#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../scripts/lib/common/build.sh
source "${ROOT}/scripts/lib/common/build.sh"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t lark-build-fp)"
cleanup() { rm -rf "${tmpdir}"; }
trap cleanup EXIT

git -C "${tmpdir}" init -q
printf "hello\n" > "${tmpdir}/file.txt"
git -C "${tmpdir}" add file.txt
git -C "${tmpdir}" -c user.name="test" -c user.email="test@example.com" commit -m "init" -q

fp_clean="$(build_fingerprint "${tmpdir}")"
stamp="${tmpdir}/.stamp"
write_build_stamp "${stamp}" "${fp_clean}"
if is_build_stale "${stamp}" "${fp_clean}"; then
  echo "expected clean fingerprint to be fresh" >&2
  exit 1
fi

touch "${tmpdir}/untracked.txt"
fp_untracked="$(build_fingerprint "${tmpdir}")"
if [[ "${fp_untracked}" == "${fp_clean}" ]]; then
  echo "expected untracked file to change fingerprint" >&2
  exit 1
fi

rm -f "${tmpdir}/untracked.txt"
printf "change\n" >> "${tmpdir}/file.txt"
fp_dirty="$(build_fingerprint "${tmpdir}")"
if [[ "${fp_dirty}" == "${fp_clean}" ]]; then
  echo "expected dirty changes to change fingerprint" >&2
  exit 1
fi
if ! is_build_stale "${stamp}" "${fp_dirty}"; then
  echo "expected dirty fingerprint to be stale" >&2
  exit 1
fi

git -C "${tmpdir}" add file.txt
git -C "${tmpdir}" -c user.name="test" -c user.email="test@example.com" commit -m "update" -q
ref_before="$(build_ref_fingerprint "${tmpdir}" "HEAD")"
head_before="$(awk -F= '/^head=/{print $2}' <<< "${ref_before}")"

printf "more\n" >> "${tmpdir}/file.txt"
git -C "${tmpdir}" add file.txt
git -C "${tmpdir}" -c user.name="test" -c user.email="test@example.com" commit -m "update-2" -q
ref_after="$(build_ref_fingerprint "${tmpdir}" "HEAD")"
head_after="$(awk -F= '/^head=/{print $2}' <<< "${ref_after}")"

if [[ -z "${head_before}" || -z "${head_after}" || "${head_before}" == "${head_after}" ]]; then
  echo "expected HEAD fingerprint to update after commit" >&2
  exit 1
fi

echo "ok"
