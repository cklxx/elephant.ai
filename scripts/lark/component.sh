#!/usr/bin/env bash
# shellcheck shell=bash
###############################################################################
# Shared Lark component lifecycle library.
#
# Each Lark component script (main.sh, test.sh, kernel.sh) sources this file
# and configures a handful of variables, then delegates to these functions.
#
# Required variables (set by caller before calling any function):
#   COMPONENT_NAME        — human-readable label, e.g. "Lark agent", "Test Lark agent"
#   COMPONENT_TAG         — short tag for identity lock, e.g. "main", "test", "kernel"
#   ROOT                  — repo root (main worktree)
#   BIN                   — path to the compiled binary
#   CONFIG_PATH           — config file path for this component
#   PID_DIR               — shared PID directory
#   PID_FILE              — PID file path
#   BUILD_STAMP           — build fingerprint stamp file
#   SHA_FILE              — deployed SHA file
#   LOG_FILE              — process stdout/stderr log
#   ALEX_LOG_DIR          — structured log directory
#   READY_LOG_PATTERN     — log line pattern indicating readiness
#   FORCE_REBUILD         — "1" to force rebuild
#   BOOTSTRAP_SH          — path to setup_local_runtime.sh
#
# Optional variables:
#   COMPONENT_BUILD_ROOT  — directory to build in (default: ROOT)
#   COMPONENT_BUILD_LABEL — label for build output (default: COMPONENT_TAG)
#   COMPONENT_BIN_ARGS    — extra args to pass to the binary (default: empty)
#   COMPONENT_EXTRA_ENV   — extra env vars for process launch (default: empty)
#   NOTICE_STATE_FILE     — notice binding state path (default: unset)
#   CLEANUP_ORPHANS_SH    — orphan cleanup script path (default: unset)
#   SKIP_LOCAL_AUTH_DB     — "1" to skip auth DB setup (default: "0")
#   USE_IDENTITY_LOCK     — "1" to use identity lock (default: "1")
#   PRE_BUILD_HOOK        — function name to call before build (default: unset)
###############################################################################

COMPONENT_BUILD_ROOT="${COMPONENT_BUILD_ROOT:-}"
COMPONENT_BUILD_LABEL="${COMPONENT_BUILD_LABEL:-}"
COMPONENT_BIN_ARGS="${COMPONENT_BIN_ARGS:-}"
COMPONENT_EXTRA_ENV="${COMPONENT_EXTRA_ENV:-}"
USE_IDENTITY_LOCK="${USE_IDENTITY_LOCK:-1}"
SKIP_LOCAL_AUTH_DB="${SKIP_LOCAL_AUTH_DB:-0}"

# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

component_ensure_dirs() {
  mkdir -p "${PID_DIR}" "${ALEX_LOG_DIR}" "$(dirname "${LOG_FILE}")"
}

component_resolve_defaults() {
  COMPONENT_BUILD_ROOT="${COMPONENT_BUILD_ROOT:-${ROOT:-}}"
  COMPONENT_BUILD_LABEL="${COMPONENT_BUILD_LABEL:-${COMPONENT_TAG:-}}"
}

component_require_config() {
  local missing=()
  local required_vars=(
    COMPONENT_NAME
    COMPONENT_TAG
    ROOT
    BIN
    CONFIG_PATH
    PID_DIR
    PID_FILE
    BUILD_STAMP
    SHA_FILE
    LOG_FILE
    ALEX_LOG_DIR
    READY_LOG_PATTERN
    FORCE_REBUILD
    BOOTSTRAP_SH
  )

  for key in "${required_vars[@]}"; do
    if [[ -z "${!key:-}" ]]; then
      missing+=("${key}")
    fi
  done

  if (( ${#missing[@]} > 0 )); then
    die "Missing component config: ${missing[*]}"
  fi

  component_resolve_defaults
}

component_ensure_bootstrap() {
  [[ -x "${BOOTSTRAP_SH}" ]] || die "Missing ${BOOTSTRAP_SH}"
  MAIN_CONFIG="${MAIN_CONFIG:-${ALEX_CONFIG_PATH:-$HOME/.alex/config.yaml}}" \
    TEST_CONFIG="${TEST_CONFIG:-$HOME/.alex/test.yaml}" \
    "${BOOTSTRAP_SH}" >/dev/null
}

component_cleanup_orphans() {
  if [[ -n "${CLEANUP_ORPHANS_SH:-}" && -x "${CLEANUP_ORPHANS_SH}" ]]; then
    "${CLEANUP_ORPHANS_SH}" cleanup --scope main --quiet || true
  fi
}

component_print_binding() {
  log_info "${COMPONENT_NAME} config: $(lark_canonical_path "${CONFIG_PATH}")"
  if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
    log_info "${COMPONENT_NAME} identity: $(lark_resolve_identity "${CONFIG_PATH}")"
  fi
  log_info "${COMPONENT_NAME} pid dir: ${PID_DIR}"
}

component_maybe_setup_auth_db() {
  if [[ "${SKIP_LOCAL_AUTH_DB}" == "1" ]]; then
    log_info "Skipping local auth DB auto-setup (SKIP_LOCAL_AUTH_DB=1)"
    return 0
  fi

  local db_script="${ROOT}/scripts/setup_local_auth_db.sh"
  if [[ -x "${db_script}" ]]; then
    log_info "Ensuring local auth DB is ready..."
    "${db_script}"
    return 0
  fi

  log_warn "Missing ${db_script}; skipping DB setup"
  return 0
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

component_build() {
  component_require_config
  if [[ -n "${PRE_BUILD_HOOK:-}" ]]; then
    "${PRE_BUILD_HOOK}"
  fi
  build_alex_server_binary "${COMPONENT_BUILD_ROOT}" "${BIN}" "${BUILD_STAMP}" "${SHA_FILE}" "${COMPONENT_BUILD_LABEL}"
}

# ---------------------------------------------------------------------------
# Start
# ---------------------------------------------------------------------------

component_start() {
  component_require_config
  load_dotenv_file "${ROOT}/.env"
  component_ensure_bootstrap
  [[ -f "${CONFIG_PATH}" ]] || die "Missing config: ${CONFIG_PATH}"
  component_print_binding
  component_cleanup_orphans
  component_maybe_setup_auth_db
  component_ensure_dirs

  if [[ -n "${NOTICE_STATE_FILE:-}" ]]; then
    mkdir -p "$(dirname "${NOTICE_STATE_FILE}")"
  fi

  local current_fingerprint needs_build pid
  current_fingerprint="$(build_fingerprint "${COMPONENT_BUILD_ROOT}")"
  needs_build=0
  if [[ "${FORCE_REBUILD}" == "1" ]] || [[ ! -x "${BIN}" ]] || is_build_stale "${BUILD_STAMP}" "${current_fingerprint}"; then
    needs_build=1
  fi

  pid="$(read_pid "${PID_FILE}" || true)"
  if is_process_running "${pid}"; then
    if [[ "${needs_build}" == "0" ]]; then
      if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
        lark_write_identity_lock "${ROOT}" "${COMPONENT_TAG}" "${CONFIG_PATH}" "${pid}"
      fi
      log_success "${COMPONENT_NAME} already running (PID: ${pid}, config: $(lark_canonical_path "${CONFIG_PATH}"))"
      return 0
    fi
    log_info "Source changes detected; rebuilding and restarting..."
    component_build
    needs_build=0
    component_stop
  fi

  # Identity lock
  if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
    local lock_owner_pid="${pid}"
    if ! is_process_running "${lock_owner_pid}"; then
      lock_owner_pid="$$"
    fi
    lark_assert_identity_available "${ROOT}" "${COMPONENT_TAG}" "${CONFIG_PATH}" "${lock_owner_pid}" || die "Lark identity is already owned by another process"
    lark_write_identity_lock "${ROOT}" "${COMPONENT_TAG}-starting" "${CONFIG_PATH}" "$$"
  fi

  if [[ "${needs_build}" == "1" ]]; then
    component_build
  else
    log_info "Reusing existing build (no changes detected)."
  fi

  log_info "Starting ${COMPONENT_NAME}..."

  # Build env array
  local env_prefix="ALEX_CONFIG_PATH=${CONFIG_PATH} ALEX_LOG_DIR=${ALEX_LOG_DIR}"
  if [[ -n "${NOTICE_STATE_FILE:-}" ]]; then
    env_prefix="${env_prefix} LARK_NOTICE_STATE_FILE=${NOTICE_STATE_FILE}"
  fi
  if [[ -n "${COMPONENT_EXTRA_ENV}" ]]; then
    env_prefix="${env_prefix} ${COMPONENT_EXTRA_ENV}"
  fi

  # shellcheck disable=SC2086
  eval "${env_prefix}" nohup "${BIN}" ${COMPONENT_BIN_ARGS} >> "${LOG_FILE}" 2>&1 &
  write_pid_meta "${PID_FILE}" "$!"

  pid="$(read_pid "${PID_FILE}" || true)"
  if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
    lark_write_identity_lock "${ROOT}" "${COMPONENT_TAG}" "${CONFIG_PATH}" "${pid}"
  fi

  _component_wait_ready "${pid}"
}

_component_wait_ready() {
  local pid="$1"
  local check_files=("${LOG_FILE}")

  # For kernel, also check the structured log
  if [[ -n "${COMPONENT_READY_EXTRA_LOG:-}" ]]; then
    check_files+=("${COMPONENT_READY_EXTRA_LOG}")
  fi

  for _ in $(seq 1 30); do
    if ! is_process_running "${pid}"; then
      if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
        lark_release_identity_lock "${ROOT}" "${CONFIG_PATH}" "${pid}"
      fi
      log_error "${COMPONENT_NAME} exited early (see ${LOG_FILE})"
      return 1
    fi
    for lf in "${check_files[@]}"; do
      if [[ -f "${lf}" ]] && grep -q "${READY_LOG_PATTERN}" "${lf}" 2>/dev/null; then
        log_success "${COMPONENT_NAME} ready (PID: ${pid}, config: $(lark_canonical_path "${CONFIG_PATH}"))"
        return 0
      fi
    done
    sleep 1
  done

  log_warn "${COMPONENT_NAME} running (PID: ${pid}) but readiness not confirmed within 30s (see ${LOG_FILE})"
  return 0
}

# ---------------------------------------------------------------------------
# Stop
# ---------------------------------------------------------------------------

component_stop() {
  component_require_config
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"
  stop_service "${COMPONENT_NAME}" "${PID_FILE}"
  if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
    lark_release_identity_lock "${ROOT}" "${CONFIG_PATH}" "${pid}" || true
  fi
}

# ---------------------------------------------------------------------------
# Restart
# ---------------------------------------------------------------------------

component_restart() {
  component_require_config
  load_dotenv_file "${ROOT}/.env"
  component_ensure_bootstrap
  [[ -f "${CONFIG_PATH}" ]] || die "Missing config: ${CONFIG_PATH}"
  component_cleanup_orphans
  component_maybe_setup_auth_db

  if [[ -n "${PRE_BUILD_HOOK:-}" ]]; then
    "${PRE_BUILD_HOOK}"
  fi

  component_build
  component_stop
  FORCE_REBUILD=0 component_start
}

# ---------------------------------------------------------------------------
# Status
# ---------------------------------------------------------------------------

component_status() {
  component_require_config
  component_print_binding
  local pid
  pid="$(read_pid "${PID_FILE}" || true)"

  if is_process_running "${pid}"; then
    if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
      lark_write_identity_lock "${ROOT}" "${COMPONENT_TAG}" "${CONFIG_PATH}" "${pid}"
    fi
    log_success "${COMPONENT_NAME} running (PID: ${pid}, config: $(lark_canonical_path "${CONFIG_PATH}"))"
    return 0
  fi

  if [[ "${USE_IDENTITY_LOCK}" == "1" ]]; then
    lark_release_identity_lock "${ROOT}" "${CONFIG_PATH}" "${pid}" || true
  fi
  log_warn "${COMPONENT_NAME} not running"
}

# ---------------------------------------------------------------------------
# Logs
# ---------------------------------------------------------------------------

component_logs() {
  component_require_config
  local log_files=("${LOG_FILE}")
  if [[ -n "${COMPONENT_LOG_EXTRAS:-}" ]]; then
    # shellcheck disable=SC2206
    log_files+=(${COMPONENT_LOG_EXTRAS})
  fi
  for f in "${log_files[@]}"; do
    touch "${f}"
  done
  tail -n 200 -f "${log_files[@]}"
}

# ---------------------------------------------------------------------------
# Command dispatcher
# ---------------------------------------------------------------------------

component_dispatch() {
  local cmd="${1:-start}"
  shift || true

  case "${cmd}" in
    start)   component_start ;;
    stop)    component_stop ;;
    restart) component_restart ;;
    status)  component_status ;;
    logs)    component_logs ;;
    build)   component_build ;;
    help|-h|--help) "${COMPONENT_USAGE_FN:-true}" ;;
    *)
      if [[ -n "${COMPONENT_USAGE_FN:-}" ]]; then
        "${COMPONENT_USAGE_FN}"
      fi
      die "Unknown command: ${cmd}"
      ;;
  esac
}
