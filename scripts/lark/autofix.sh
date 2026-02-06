#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common/logging.sh
source "${SCRIPT_DIR}/../lib/common/logging.sh"

usage() {
  cat <<'EOF'
Usage:
  scripts/lark/autofix.sh trigger --incident-id <id> --reason <text> --signature <sig> --main-sha <sha>

Behavior:
  - Runs codex-based autonomous repair from an isolated autofix worktree
  - Validates minimal recovery checks
  - Rebases and ff-only merges the autofix branch into main
  - Writes runtime state into .worktrees/test/tmp/lark-autofix.state.json

Env:
  LARK_MAIN_ROOT                                Main root override (tests only)
  LARK_AUTOFIX_CODEX_BIN                        Codex binary override (default: codex)
  LARK_AUTOFIX_MAIN_BRANCH                      Target branch for merge (default: main)
  LARK_AUTOFIX_BRANCH                           Autofix branch name (default: autofix)
  LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS       Max codex run time (default: 1800)
  LARK_SUPERVISOR_AUTOFIX_SCOPE                 Prompt scope hint (default: repo)
  LARK_AUTOFIX_REBASE_MAX_ATTEMPTS              Max codex conflict-repair attempts (default: 3)
  LARK_AUTOFIX_MERGE_RETRY_MAX                  Merge retries when main moves (default: 2)
EOF
}

git_worktree_path_for_branch() {
  local want_branch_ref="$1"
  git -C "${SCRIPT_DIR}" worktree list --porcelain | awk -v want="${want_branch_ref}" '
    $1=="worktree"{p=$2}
    $1=="branch" && $2==want {print p; exit}
  '
}

if [[ -n "${LARK_MAIN_ROOT:-}" ]]; then
  MAIN_ROOT="${LARK_MAIN_ROOT}"
else
  MAIN_ROOT="$(git_worktree_path_for_branch "refs/heads/main" || true)"
  if [[ -z "${MAIN_ROOT}" ]]; then
    MAIN_ROOT="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel 2>/dev/null || true)"
  fi
fi
[[ -n "${MAIN_ROOT}" ]] || die "Not a git repository (cannot resolve main worktree)"

TEST_ROOT="${MAIN_ROOT}/.worktrees/test"
AUTOFIX_ROOT="${MAIN_ROOT}/.worktrees/autofix"
TMP_DIR="${TEST_ROOT}/tmp"
LOG_DIR="${TEST_ROOT}/logs"

LOCK_DIR="${TMP_DIR}/lark-autofix.lock"
STATE_FILE="${TMP_DIR}/lark-autofix.state.json"
LOG_FILE="${LOG_DIR}/lark-autofix.log"

CODEX_BIN="${LARK_AUTOFIX_CODEX_BIN:-codex}"
MAIN_BRANCH="${LARK_AUTOFIX_MAIN_BRANCH:-main}"
AUTOFIX_BRANCH="${LARK_AUTOFIX_BRANCH:-autofix}"
AUTOFIX_TIMEOUT_SECONDS="${LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS:-1800}"
AUTOFIX_SCOPE="${LARK_SUPERVISOR_AUTOFIX_SCOPE:-repo}"
REBASE_MAX_ATTEMPTS="${LARK_AUTOFIX_REBASE_MAX_ATTEMPTS:-3}"
MERGE_RETRY_MAX="${LARK_AUTOFIX_MERGE_RETRY_MAX:-2}"

INCIDENT_ID=""
REASON=""
SIGNATURE=""
BASE_MAIN_SHA=""

STATE_STATUS="idle"
STATE_ERROR=""
STATE_STARTED_AT=""
STATE_FINISHED_AT=""
STATE_COMMIT=""
STATE_RESTART_REQUIRED="false"

json_escape() {
  printf '%s' "${1:-}" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

ensure_dirs() {
  mkdir -p "${TMP_DIR}" "${LOG_DIR}"
}

append_log() {
  ensure_dirs
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" >> "${LOG_FILE}"
}

write_state_file() {
  ensure_dirs
  local now_utc
  now_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  local tmp_file
  tmp_file="${STATE_FILE}.tmp"
  cat > "${tmp_file}" <<EOF
{
  "ts_utc": "${now_utc}",
  "autofix_state": "${STATE_STATUS}",
  "autofix_incident_id": "$(json_escape "${INCIDENT_ID}")",
  "autofix_last_reason": "$(json_escape "${REASON}")",
  "autofix_last_started_at": "${STATE_STARTED_AT}",
  "autofix_last_finished_at": "${STATE_FINISHED_AT}",
  "autofix_last_commit": "${STATE_COMMIT}",
  "autofix_error": "$(json_escape "${STATE_ERROR}")",
  "autofix_signature": "$(json_escape "${SIGNATURE}")",
  "autofix_main_sha": "${BASE_MAIN_SHA}",
  "autofix_restart_required": "${STATE_RESTART_REQUIRED}"
}
EOF
  mv "${tmp_file}" "${STATE_FILE}"
}

fail_state() {
  local message="$1"
  STATE_STATUS="failed"
  STATE_ERROR="${message}"
  STATE_RESTART_REQUIRED="false"
  STATE_FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  append_log "[autofix] failed: ${message}"
  write_state_file
}

acquire_lock() {
  if mkdir "${LOCK_DIR}" 2>/dev/null; then
    printf 'pid=%s started_at=%s incident=%s\n' "$$" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${INCIDENT_ID}" > "${LOCK_DIR}/owner"
    return 0
  fi
  return 1
}

release_lock() {
  rm -rf "${LOCK_DIR}" 2>/dev/null || true
}

cleanup() {
  release_lock
}

has_conflict_markers() {
  local file
  while IFS= read -r file; do
    [[ -n "${file}" ]] || continue
    if rg -n '<<<<<<<|=======|>>>>>>>' "${AUTOFIX_ROOT}/${file}" >/dev/null 2>&1; then
      return 0
    fi
  done < <(git -C "${AUTOFIX_ROOT}" diff --name-only --diff-filter=U || true)
  return 1
}

ensure_autofix_worktree() {
  local has_worktree=0
  local backup_root=""
  mkdir -p "${MAIN_ROOT}/.worktrees"

  if git -C "${MAIN_ROOT}" worktree list --porcelain | awk -v p="${AUTOFIX_ROOT}" '$1=="worktree" && $2==p {found=1} END{exit found?0:1}'; then
    has_worktree=1
  fi

  if [[ ${has_worktree} -eq 1 ]]; then
    if git -C "${AUTOFIX_ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1 && [[ -f "${AUTOFIX_ROOT}/go.mod" ]]; then
      :
    else
      append_log "[autofix] stale autofix worktree detected, pruning"
      git -C "${MAIN_ROOT}" worktree prune || true
      has_worktree=0
    fi
  fi

  if [[ ${has_worktree} -eq 0 ]]; then
    if [[ -d "${AUTOFIX_ROOT}" ]]; then
      backup_root="${MAIN_ROOT}/.worktrees/autofix-orphan-$(date -u +%Y%m%d%H%M%S)"
      mv "${AUTOFIX_ROOT}" "${backup_root}"
    fi
    git -C "${MAIN_ROOT}" worktree add -B "${AUTOFIX_BRANCH}" "${AUTOFIX_ROOT}" "${MAIN_BRANCH}" >> "${LOG_FILE}" 2>&1
  fi

  if [[ -f "${MAIN_ROOT}/.env" ]]; then
    cp -f "${MAIN_ROOT}/.env" "${AUTOFIX_ROOT}/.env"
  fi

  git -C "${AUTOFIX_ROOT}" checkout -B "${AUTOFIX_BRANCH}" "${MAIN_BRANCH}" >> "${LOG_FILE}" 2>&1
  git -C "${AUTOFIX_ROOT}" reset --hard "${MAIN_BRANCH}" >> "${LOG_FILE}" 2>&1
}

run_with_timeout() {
  local timeout_seconds="$1"
  shift

  "$@" &
  local pid=$!
  local started_at
  started_at="$(date +%s)"

  while kill -0 "${pid}" 2>/dev/null; do
    local now
    now="$(date +%s)"
    if (( now - started_at >= timeout_seconds )); then
      kill "${pid}" 2>/dev/null || true
      sleep 1
      kill -9 "${pid}" 2>/dev/null || true
      wait "${pid}" 2>/dev/null || true
      return 124
    fi
    sleep 1
  done

  wait "${pid}"
}

collect_context() {
  local out_file="$1"
  local supervisor_status loop_state doctor_output
  supervisor_status="$(cat "${TEST_ROOT}/tmp/lark-supervisor.status.json" 2>/dev/null || true)"
  loop_state="$(cat "${TEST_ROOT}/tmp/lark-loop.state.json" 2>/dev/null || true)"
  doctor_output="$("${MAIN_ROOT}/scripts/lark/supervisor.sh" doctor 2>&1 || true)"

  cat > "${out_file}" <<EOF
You are recovering elephant.ai local lark autonomous system health.

Incident:
- incident_id: ${INCIDENT_ID}
- base_main_sha: ${BASE_MAIN_SHA}
- signature: ${SIGNATURE}
- reason: ${REASON}
- allowed_scope: ${AUTOFIX_SCOPE}

Requirements:
1) Prioritize restoring supervisor/main/test/loop health.
2) Make minimal, maintainable changes.
3) You may modify any files in this repository if needed.
4) Do not ask for interactive confirmation.
5) After edits, leave repo in a committable state.

Supervisor status snapshot:
${supervisor_status}

Loop state snapshot:
${loop_state}

Doctor output:
${doctor_output}

Recent supervisor logs (tail 200):
$(tail -n 200 "${TEST_ROOT}/logs/lark-supervisor.log" 2>/dev/null || true)

Recent loop logs (tail 200):
$(tail -n 200 "${TEST_ROOT}/logs/lark-loop.log" 2>/dev/null || true)
EOF
}

run_codex_prompt_file() {
  local prompt_file="$1"
  if ! command -v "${CODEX_BIN}" >/dev/null 2>&1; then
    return 127
  fi

  run_with_timeout "${AUTOFIX_TIMEOUT_SECONDS}" bash -lc "cd \"${AUTOFIX_ROOT}\" && \"${CODEX_BIN}\" exec --dangerously-bypass-approvals-and-sandbox - < \"${prompt_file}\"" >> "${LOG_FILE}" 2>&1
}

resolve_rebase_conflicts() {
  local attempt
  for attempt in $(seq 1 "${REBASE_MAX_ATTEMPTS}"); do
    local conflicted
    conflicted="$(git -C "${AUTOFIX_ROOT}" diff --name-only --diff-filter=U | tr '\n' ' ')"
    if [[ -n "${conflicted}" ]]; then
      append_log "[autofix] rebase conflict attempt ${attempt}/${REBASE_MAX_ATTEMPTS}; files=${conflicted}"
      local prompt_file
      prompt_file="$(mktemp)"
      cat > "${prompt_file}" <<EOF
You are resolving git rebase conflicts in elephant.ai.

Incident:
- incident_id: ${INCIDENT_ID}
- reason: ${REASON}
- conflicted_files: ${conflicted}

Rules:
1) Resolve conflicts correctly and keep intended behavior.
2) Remove all conflict markers.
3) Keep changes minimal.
4) Stage resolved files.
EOF

      if ! run_codex_prompt_file "${prompt_file}"; then
        rm -f "${prompt_file}"
        continue
      fi
      rm -f "${prompt_file}"

      if has_conflict_markers; then
        append_log "[autofix] conflict markers remain after codex attempt"
        continue
      fi

      git -C "${AUTOFIX_ROOT}" add -A
    fi

    if GIT_EDITOR=true git -C "${AUTOFIX_ROOT}" rebase --continue >> "${LOG_FILE}" 2>&1; then
      append_log "[autofix] rebase conflict resolved"
      return 0
    fi
  done

  git -C "${AUTOFIX_ROOT}" rebase --abort >> "${LOG_FILE}" 2>&1 || true
  return 1
}

run_validation() {
  append_log "[autofix] running validation checks"
  (cd "${AUTOFIX_ROOT}" && bash -n lark.sh scripts/lark/*.sh scripts/lib/common/*.sh)
  (cd "${AUTOFIX_ROOT}" && ./tests/scripts/lark-supervisor-smoke.sh)
  (cd "${AUTOFIX_ROOT}" && ./lark.sh doctor)
}

commit_if_needed() {
  local before after
  before="$(git -C "${AUTOFIX_ROOT}" rev-parse HEAD)"
  if ! git -C "${AUTOFIX_ROOT}" diff --quiet || ! git -C "${AUTOFIX_ROOT}" diff --cached --quiet; then
    git -C "${AUTOFIX_ROOT}" add -A
    git -C "${AUTOFIX_ROOT}" -c user.name="lark-autofix" -c user.email="lark-autofix@local" \
      commit -m "fix(autofix): incident ${INCIDENT_ID}" >> "${LOG_FILE}" 2>&1
  fi
  after="$(git -C "${AUTOFIX_ROOT}" rev-parse HEAD)"
  if [[ "${before}" == "${after}" ]]; then
    return 1
  fi
  STATE_COMMIT="${after}"
  return 0
}

rebase_and_merge_into_main() {
  local retry
  for retry in $(seq 1 "${MERGE_RETRY_MAX}"); do
    git -C "${AUTOFIX_ROOT}" switch "${AUTOFIX_BRANCH}" >> "${LOG_FILE}" 2>&1

    if ! git -C "${AUTOFIX_ROOT}" rebase "${MAIN_BRANCH}" >> "${LOG_FILE}" 2>&1; then
      if ! resolve_rebase_conflicts; then
        append_log "[autofix] rebase conflict unresolved"
        return 1
      fi
    fi

    STATE_COMMIT="$(git -C "${AUTOFIX_ROOT}" rev-parse HEAD)"
    if git -C "${MAIN_ROOT}" merge --ff-only "${AUTOFIX_BRANCH}" >> "${LOG_FILE}" 2>&1; then
      append_log "[autofix] ff-only merge success: ${STATE_COMMIT}"
      return 0
    fi

    append_log "[autofix] ff-only merge failed (main moved), retry ${retry}/${MERGE_RETRY_MAX}"
  done
  return 1
}

parse_args() {
  local cmd="${1:-}"
  shift || true
  [[ "${cmd}" == "trigger" ]] || { usage; die "Unknown command: ${cmd:-}"; }

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --incident-id)
        INCIDENT_ID="${2:-}"
        shift 2
        ;;
      --reason)
        REASON="${2:-}"
        shift 2
        ;;
      --signature)
        SIGNATURE="${2:-}"
        shift 2
        ;;
      --main-sha)
        BASE_MAIN_SHA="${2:-}"
        shift 2
        ;;
      *)
        usage
        die "Unknown arg: $1"
        ;;
    esac
  done

  [[ -n "${INCIDENT_ID}" ]] || die "--incident-id is required"
  [[ -n "${REASON}" ]] || die "--reason is required"
  [[ -n "${SIGNATURE}" ]] || die "--signature is required"
  if [[ -z "${BASE_MAIN_SHA}" ]]; then
    BASE_MAIN_SHA="$(git -C "${MAIN_ROOT}" rev-parse "${MAIN_BRANCH}" 2>/dev/null || true)"
  fi
  [[ -n "${BASE_MAIN_SHA}" ]] || die "--main-sha is required"
}

main() {
  parse_args "$@"
  ensure_dirs

  if ! acquire_lock; then
    append_log "[autofix] already running; skipping incident=${INCIDENT_ID}"
    exit 0
  fi
  trap cleanup EXIT INT TERM

  STATE_STATUS="running"
  STATE_ERROR=""
  STATE_RESTART_REQUIRED="false"
  STATE_STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  STATE_FINISHED_AT=""
  STATE_COMMIT=""
  write_state_file

  append_log "[autofix] start incident=${INCIDENT_ID} signature=${SIGNATURE}"

  if ! ensure_autofix_worktree; then
    fail_state "failed to ensure autofix worktree"
    exit 1
  fi

  local prompt_file
  prompt_file="$(mktemp)"
  collect_context "${prompt_file}"

  if ! run_codex_prompt_file "${prompt_file}"; then
    rm -f "${prompt_file}"
    fail_state "codex execution failed or timed out"
    exit 1
  fi
  rm -f "${prompt_file}"

  if ! commit_if_needed; then
    fail_state "codex completed without creating changes"
    exit 1
  fi

  if ! run_validation; then
    fail_state "validation failed after codex changes"
    exit 1
  fi

  if ! rebase_and_merge_into_main; then
    fail_state "failed to rebase/merge autofix branch into ${MAIN_BRANCH}"
    exit 1
  fi

  STATE_STATUS="succeeded"
  STATE_ERROR=""
  STATE_RESTART_REQUIRED="true"
  STATE_FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_state_file
  append_log "[autofix] success incident=${INCIDENT_ID} commit=${STATE_COMMIT}"
}

main "$@"
