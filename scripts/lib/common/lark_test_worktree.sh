#!/usr/bin/env bash
# shellcheck shell=bash

lark_test_root_path() {
  local main_root="$1"
  printf '%s/.worktrees/test' "${main_root}"
}

lark_normalize_path() {
  local path="$1"
  local dir base
  dir="$(dirname "${path}")"
  base="$(basename "${path}")"
  if [[ -d "${dir}" ]]; then
    (
      cd "${dir}" >/dev/null 2>&1 || exit 1
      printf '%s/%s\n' "$(pwd -P)" "${base}"
    )
    return 0
  fi
  printf '%s\n' "${path}"
}

lark_test_worktree_registered() {
  local main_root="$1"
  local test_root="$2"
  local want current
  want="$(lark_normalize_path "${test_root}")"

  while IFS=' ' read -r key value; do
    if [[ "${key}" == "worktree" ]]; then
      current="$(lark_normalize_path "${value}")"
      if [[ "${current}" == "${want}" ]]; then
        return 0
      fi
    fi
  done < <(git -C "${main_root}" worktree list --porcelain)

  return 1
}

lark_sync_test_worktree_env() {
  local main_root="$1"
  local test_root
  local src
  local dst
  local src_example

  test_root="$(lark_test_root_path "${main_root}")"
  src="${main_root}/.env"
  dst="${test_root}/.env"

  if [[ ! -f "${src}" ]]; then
    src_example="${main_root}/.env.example"
    [[ -f "${src_example}" ]] || die "Missing ${src} and ${src_example}"
    cp -f "${src_example}" "${src}"
    log_warn "Created ${src} from ${src_example}; set LLM_API_KEY before running real LLM tasks"
  fi
  [[ -d "${test_root}" ]] || die "Missing test worktree at ${test_root}"

  cp -f "${src}" "${dst}"
  log_success "Synced .env -> ${dst}"
}

lark_is_git_worktree_dir() {
  local path="$1"
  git -C "${path}" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

lark_is_valid_test_worktree() {
  local main_root="$1"
  local test_root
  test_root="$(lark_test_root_path "${main_root}")"

  if ! lark_is_git_worktree_dir "${test_root}"; then
    return 1
  fi

  # In production repo go.mod is expected. In tiny smoke repos it may not exist.
  if [[ -f "${main_root}/go.mod" && ! -f "${test_root}/go.mod" ]]; then
    return 1
  fi

  return 0
}

lark_stale_test_admin_dir() {
  local main_root="$1"
  local git_common_dir
  git_common_dir="$(git -C "${main_root}" rev-parse --git-common-dir)"
  if [[ "${git_common_dir}" != /* ]]; then
    git_common_dir="${main_root}/${git_common_dir}"
  fi
  printf '%s/worktrees/test' "${git_common_dir}"
}

lark_ensure_test_worktree() {
  local main_root="$1"
  local test_root
  local has_worktree=0
  local backup_root=""

  test_root="$(lark_test_root_path "${main_root}")"
  mkdir -p "${main_root}/.worktrees"

  if lark_test_worktree_registered "${main_root}" "${test_root}"; then
    has_worktree=1
  fi

  if [[ ${has_worktree} -eq 1 ]]; then
    if lark_is_valid_test_worktree "${main_root}"; then
      log_info "Test worktree exists: ${test_root}"
    else
      log_warn "Stale/partial test worktree detected; pruning: ${test_root}"
      git -C "${main_root}" worktree prune || true
      local admin_dir
      admin_dir="$(lark_stale_test_admin_dir "${main_root}")"
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
    git -C "${main_root}" worktree add -f --detach "${test_root}" main

    if ! lark_is_valid_test_worktree "${main_root}"; then
      die "Test worktree invalid after create: ${test_root}"
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

  lark_sync_test_worktree_env "${main_root}"
}
