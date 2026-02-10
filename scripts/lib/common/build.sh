#!/usr/bin/env bash
# shellcheck shell=bash
# Build fingerprint helpers for scripts.

hash_stdin() {
  cksum | awk '{print $1 ":" $2}'
}

build_untracked_hash() {
  local root="$1"
  (
    cd "${root}"
    git ls-files --others --exclude-standard -z \
      | grep -zv -e '^logs/' -e '^\.pids/' -e '^pids/' -e '^eval-server/' -e '^\.worktrees/' \
      | xargs -0 cksum 2>/dev/null
  ) | hash_stdin
}

build_fingerprint() {
  local root="${1:-.}"
  local head diff_hash staged_hash untracked_hash

  head="$(git -C "${root}" rev-parse HEAD 2>/dev/null || true)"
  diff_hash="$(git -C "${root}" diff --no-ext-diff -- . | hash_stdin)"
  staged_hash="$(git -C "${root}" diff --cached --no-ext-diff -- . | hash_stdin)"
  untracked_hash="$(build_untracked_hash "${root}")"

  printf "head=%s\ndiff=%s\nstaged=%s\nuntracked=%s\n" \
    "${head}" "${diff_hash}" "${staged_hash}" "${untracked_hash}"
}

build_ref_fingerprint() {
  local root="$1"
  local ref="$2"
  local head

  head="$(git -C "${root}" rev-parse "${ref}" 2>/dev/null || true)"
  printf "ref=%s\nhead=%s\n" "${ref}" "${head}"
}

read_build_stamp() {
  local stamp_file="$1"
  [[ -f "${stamp_file}" ]] && cat "${stamp_file}"
}

write_build_stamp() {
  local stamp_file="$1"
  local content="$2"
  printf '%s' "${content}" > "${stamp_file}"
}

is_build_stale() {
  local stamp_file="$1"
  local current="$2"
  local previous

  previous="$(read_build_stamp "${stamp_file}")"
  if [[ -z "${current}" || -z "${previous}" ]]; then
    return 0
  fi
  [[ "${current}" != "${previous}" ]]
}
