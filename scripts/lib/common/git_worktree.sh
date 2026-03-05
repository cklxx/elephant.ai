#!/usr/bin/env bash
# shellcheck shell=bash
# Common git worktree helpers.

git_worktree_path_for_branch() {
  local want_branch_ref="$1" # e.g. refs/heads/main
  local git_anchor="${2:-.}"
  local current_worktree=""
  local current_branch=""

  while IFS=' ' read -r key value; do
    case "${key}" in
      worktree)
        current_worktree="${value}"
        current_branch=""
        ;;
      branch)
        current_branch="${value}"
        if [[ "${current_branch}" == "${want_branch_ref}" ]] && git_is_worktree_dir "${current_worktree}"; then
          printf '%s\n' "${current_worktree}"
          return 0
        fi
        ;;
    esac
  done < <(git -C "${git_anchor}" worktree list --porcelain 2>/dev/null || true)

  return 1
}

git_repo_toplevel() {
  local git_anchor="${1:-.}"
  git -C "${git_anchor}" rev-parse --show-toplevel 2>/dev/null
}

git_is_worktree_dir() {
  local path="$1"
  [[ -n "${path}" ]] || return 1
  git -C "${path}" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

git_resolve_main_root() {
  local git_anchor="${1:-.}"
  local root=""

  root="$(git_worktree_path_for_branch "refs/heads/main" "${git_anchor}" || true)"
  if [[ -n "${root}" ]]; then
    printf '%s\n' "${root}"
    return 0
  fi

  root="$(git_repo_toplevel "${git_anchor}" || true)"
  if git_is_worktree_dir "${root}"; then
    printf '%s\n' "${root}"
    return 0
  fi

  return 1
}
