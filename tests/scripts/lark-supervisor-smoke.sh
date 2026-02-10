#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUPERVISOR_SH="${ROOT}/scripts/lark/supervisor.sh"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t lark-supervisor-smoke)"
main_root="${tmpdir}/repo"

run_supervisor() {
  env \
    LARK_MAIN_ROOT="${main_root}" \
    MAIN_SH="${main_root}/scripts/lark/main.sh" \
    TEST_SH="${main_root}/scripts/lark/test.sh" \
    LOOP_AGENT_SH="${main_root}/scripts/lark/loop-agent.sh" \
    AUTOFIX_SH="${main_root}/scripts/lark/autofix.sh" \
    LARK_SUPERVISOR_NOTIFY_SH="${main_root}/scripts/lark/notify.sh" \
    LARK_NOTICE_STATE_FILE="${main_root}/.worktrees/test/tmp/lark-notice.state.json" \
    MAIN_CONFIG="${main_root}/config-main.yaml" \
    TEST_CONFIG="${main_root}/config-test.yaml" \
    LARK_SUPERVISOR_SKIP_HEALTHCHECK=1 \
    LARK_SUPERVISOR_TICK_SECONDS=1 \
    LARK_VALIDATION_TEST_SUPPRESS_MAX_SECONDS=2 \
    LARK_STALE_LOOP_STATE_TIMEOUT_SECONDS=0 \
    LARK_RESTART_MAX_IN_WINDOW=5 \
    LARK_RESTART_WINDOW_SECONDS=30 \
    LARK_COOLDOWN_SECONDS=3 \
    "${SUPERVISOR_SH}" "$@"
}

run_supervisor_same_config() {
  env \
    LARK_MAIN_ROOT="${main_root}" \
    MAIN_SH="${main_root}/scripts/lark/main.sh" \
    TEST_SH="${main_root}/scripts/lark/test.sh" \
    LOOP_AGENT_SH="${main_root}/scripts/lark/loop-agent.sh" \
    AUTOFIX_SH="${main_root}/scripts/lark/autofix.sh" \
    LARK_SUPERVISOR_NOTIFY_SH="${main_root}/scripts/lark/notify.sh" \
    LARK_NOTICE_STATE_FILE="${main_root}/.worktrees/test/tmp/lark-notice.state.json" \
    MAIN_CONFIG="${main_root}/config-main.yaml" \
    TEST_CONFIG="${main_root}/config-main.yaml" \
    LARK_SUPERVISOR_SKIP_HEALTHCHECK=1 \
    LARK_SUPERVISOR_TICK_SECONDS=1 \
    LARK_VALIDATION_TEST_SUPPRESS_MAX_SECONDS=2 \
    LARK_STALE_LOOP_STATE_TIMEOUT_SECONDS=0 \
    LARK_RESTART_MAX_IN_WINDOW=5 \
    LARK_RESTART_WINDOW_SECONDS=30 \
    LARK_COOLDOWN_SECONDS=3 \
    "${SUPERVISOR_SH}" "$@"
}

run_supervisor_same_identity() {
  env \
    LARK_MAIN_ROOT="${main_root}" \
    MAIN_SH="${main_root}/scripts/lark/main.sh" \
    TEST_SH="${main_root}/scripts/lark/test.sh" \
    LOOP_AGENT_SH="${main_root}/scripts/lark/loop-agent.sh" \
    AUTOFIX_SH="${main_root}/scripts/lark/autofix.sh" \
    LARK_SUPERVISOR_NOTIFY_SH="${main_root}/scripts/lark/notify.sh" \
    LARK_NOTICE_STATE_FILE="${main_root}/.worktrees/test/tmp/lark-notice.state.json" \
    MAIN_CONFIG="${main_root}/config-main-lark.yaml" \
    TEST_CONFIG="${main_root}/config-test-lark.yaml" \
    LARK_SUPERVISOR_SKIP_HEALTHCHECK=1 \
    LARK_SUPERVISOR_TICK_SECONDS=1 \
    LARK_VALIDATION_TEST_SUPPRESS_MAX_SECONDS=2 \
    LARK_STALE_LOOP_STATE_TIMEOUT_SECONDS=0 \
    LARK_RESTART_MAX_IN_WINDOW=5 \
    LARK_RESTART_WINDOW_SECONDS=30 \
    LARK_COOLDOWN_SECONDS=3 \
    "${SUPERVISOR_SH}" "$@"
}

cleanup() {
  run_supervisor stop >/dev/null 2>&1 || true

  if [[ -d "${main_root}/pids" ]]; then
    while IFS= read -r pid_file; do
      pid="$(cat "${pid_file}" 2>/dev/null || true)"
      if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
        kill "${pid}" 2>/dev/null || true
      fi
    done < <(find "${main_root}" -name '*.pid' -type f 2>/dev/null || true)
  fi

  if [[ "${KEEP_TMPDIR:-0}" == "1" ]]; then
    echo "keeping tmpdir for debug: ${tmpdir}" >&2
    return 0
  fi

  rm -rf "${tmpdir}"
}
trap cleanup EXIT

mkdir -p "${main_root}/scripts/lark" "${main_root}/pids" "${main_root}/logs" "${main_root}/.worktrees/test/logs" "${main_root}/.worktrees/test/tmp"

notice_state_file="${main_root}/.worktrees/test/tmp/lark-notice.state.json"
notify_calls_file="${main_root}/.worktrees/test/tmp/notify.calls.log"
cat > "${notice_state_file}" <<'EOF'
{
  "chat_id": "oc_supervisor_notice",
  "set_by_user_id": "ou_tester",
  "set_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-01T00:00:00Z"
}
EOF

make_stub() {
  local file="$1"
  local rel_pid="$2"
  cat > "${file}" <<EOF
#!/usr/bin/env bash
set -euo pipefail

main_root="\${LARK_MAIN_ROOT:?}"
pid_file="\${main_root}/${rel_pid}"
mkdir -p "\$(dirname "\${pid_file}")"

start_proc() {
  local pid
  pid="\$(cat "\${pid_file}" 2>/dev/null || true)"
  if [[ -n "\${pid}" ]] && kill -0 "\${pid}" 2>/dev/null; then
    return 0
  fi
  sleep 300 &
  echo "\$!" > "\${pid_file}"
}

stop_proc() {
  local pid
  pid="\$(cat "\${pid_file}" 2>/dev/null || true)"
  if [[ -n "\${pid}" ]] && kill -0 "\${pid}" 2>/dev/null; then
    kill "\${pid}" 2>/dev/null || true
    wait "\${pid}" 2>/dev/null || true
  fi
  rm -f "\${pid_file}"
}

cmd="\${1:-start}"
case "\${cmd}" in
  start)
    start_proc
    ;;
  restart)
    stop_proc
    start_proc
    ;;
  stop)
    stop_proc
    ;;
  status|logs)
    ;;
  *)
    echo "unknown command: \${cmd}" >&2
    exit 2
    ;;
esac
EOF
  chmod +x "${file}"
}

make_stub "${main_root}/scripts/lark/main.sh" "pids/lark-main.pid"
make_stub "${main_root}/scripts/lark/test.sh" "pids/lark-test.pid"
make_stub "${main_root}/scripts/lark/loop-agent.sh" "pids/lark-loop.pid"
cat > "${main_root}/scripts/lark/autofix.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF
chmod +x "${main_root}/scripts/lark/autofix.sh"
cat > "${main_root}/scripts/lark/notify.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

main_root="${LARK_MAIN_ROOT:?}"
calls_file="${main_root}/.worktrees/test/tmp/notify.calls.log"

cmd="${1:-}"
shift || true
[[ "${cmd}" == "send" ]] || exit 2

chat_id=""
text=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --chat-id)
      chat_id="${2:-}"
      shift 2
      ;;
    --text)
      text="${2:-}"
      shift 2
      ;;
    --config)
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

printf 'chat_id=%s\ntext=%s\n---\n' "${chat_id}" "${text}" >> "${calls_file}"
EOF
chmod +x "${main_root}/scripts/lark/notify.sh"

cat > "${main_root}/config-main.yaml" <<'EOF'
server:
  port: 19080
EOF

cat > "${main_root}/config-test.yaml" <<'EOF'
server:
  port: 19081
EOF

cat > "${main_root}/config-main-lark.yaml" <<'EOF'
channels:
  lark:
    app_id: "cli_shared_app"
EOF

cat > "${main_root}/config-test-lark.yaml" <<'EOF'
channels:
  lark:
    app_id: "cli_shared_app"
EOF

git -C "${main_root}" init -q -b main 2>/dev/null || git -C "${main_root}" init -q
current_branch="$(git -C "${main_root}" symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
if [[ "${current_branch}" != "main" ]]; then
  git -C "${main_root}" checkout -q -b main
fi
echo "supervisor smoke" > "${main_root}/README.md"
cat > "${main_root}/.env" <<'EOF'
LLM_API_KEY=test
EOF
git -C "${main_root}" add README.md .env
git -C "${main_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "init"

run_supervisor run-once >/dev/null

status_file="${main_root}/.worktrees/test/tmp/lark-supervisor.status.json"
[[ -f "${status_file}" ]] || { echo "missing status file: ${status_file}" >&2; exit 1; }
for key in ts_utc mode main_pid test_pid loop_pid main_health test_health loop_alive main_sha last_processed_sha last_validated_sha cycle_phase cycle_result loop_autofix_enabled last_error restart_count_window autofix_state autofix_incident_id autofix_last_reason autofix_last_started_at autofix_last_finished_at autofix_last_commit autofix_runs_window; do
  grep -q "\"${key}\"" "${status_file}" || { echo "missing key in status: ${key}" >&2; exit 1; }
done
for key in validation_suppressed_since validation_suppress_timeout_seconds stale_state_recovered stale_state_recovered_at codex_loop_pid codex_autofix_pid; do
  grep -q "\"${key}\"" "${status_file}" || { echo "missing key in status: ${key}" >&2; exit 1; }
done
grep -q '"mode": "healthy"' "${status_file}" || { echo "expected healthy mode after run-once" >&2; exit 1; }

main_pid="$(cat "${main_root}/pids/lark-main.pid")"
test_pid="$(cat "${main_root}/pids/lark-test.pid")"
loop_pid="$(cat "${main_root}/pids/lark-loop.pid")"
for pid in "${main_pid}" "${test_pid}" "${loop_pid}"; do
  kill -0 "${pid}" 2>/dev/null || { echo "expected managed pid alive: ${pid}" >&2; exit 1; }
done

run_supervisor start >/dev/null
sleep 1
supervisor_pid_1="$(cat "${main_root}/pids/lark-supervisor.pid")"
kill -0 "${supervisor_pid_1}" 2>/dev/null || { echo "supervisor pid not alive after start: ${supervisor_pid_1}" >&2; exit 1; }

run_supervisor start >/dev/null
supervisor_pid_2="$(cat "${main_root}/pids/lark-supervisor.pid")"
if [[ "${supervisor_pid_1}" != "${supervisor_pid_2}" ]]; then
  echo "expected idempotent start to keep same supervisor pid" >&2
  exit 1
fi

run_supervisor doctor >/dev/null
if run_supervisor_same_config doctor >/dev/null 2>&1; then
  echo "expected doctor to fail when MAIN_CONFIG and TEST_CONFIG point to the same yaml" >&2
  exit 1
fi
if run_supervisor_same_identity doctor >/dev/null 2>&1; then
  echo "expected doctor to fail when MAIN_CONFIG and TEST_CONFIG share same lark identity" >&2
  exit 1
fi

# Ensure stop path kills tracked codex subprocesses too.
sleep 300 &
codex_loop_pid="$!"
echo "${codex_loop_pid}" > "${main_root}/pids/lark-codex-loop.pid"
sleep 300 &
codex_autofix_pid="$!"
echo "${codex_autofix_pid}" > "${main_root}/pids/lark-codex-autofix.pid"

run_supervisor stop >/dev/null

if [[ -f "${main_root}/pids/lark-supervisor.pid" ]]; then
  echo "expected supervisor pid file removed after stop" >&2
  exit 1
fi

for pid_file in \
  "${main_root}/pids/lark-main.pid" \
  "${main_root}/pids/lark-test.pid" \
  "${main_root}/pids/lark-loop.pid"; do
  if [[ -f "${pid_file}" ]]; then
    echo "expected component pid file removed after stop: ${pid_file}" >&2
    exit 1
  fi
done

if kill -0 "${codex_loop_pid}" 2>/dev/null; then
  echo "expected loop codex pid killed on stop: ${codex_loop_pid}" >&2
  exit 1
fi
wait "${codex_loop_pid}" 2>/dev/null || true
if kill -0 "${codex_autofix_pid}" 2>/dev/null; then
  echo "expected autofix codex pid killed on stop: ${codex_autofix_pid}" >&2
  exit 1
fi
wait "${codex_autofix_pid}" 2>/dev/null || true

for pid_file in \
  "${main_root}/pids/lark-codex-loop.pid" \
  "${main_root}/pids/lark-codex-autofix.pid"; do
  if [[ -f "${pid_file}" ]]; then
    echo "expected codex pid file removed after stop: ${pid_file}" >&2
    exit 1
  fi
done

# --- Validation-phase suppression test ---
# Start all components, then simulate a validation phase and kill the test
# process.  run-once should NOT restart test because the phase is active.
loop_state_file="${main_root}/.worktrees/test/tmp/lark-loop.state.json"

# Start components fresh.
run_supervisor run-once >/dev/null

# Verify test is alive before we test suppression.
test_pid="$(cat "${main_root}/pids/lark-test.pid" 2>/dev/null || true)"
kill -0 "${test_pid}" 2>/dev/null || { echo "expected test pid alive before suppression test" >&2; exit 1; }

# Simulate a validation phase by writing loop state with cycle_phase=fast_gate.
cat > "${loop_state_file}" <<JSEOF
{
  "ts_utc": "2026-01-01T00:00:00Z",
  "base_sha": "abc1234",
  "cycle_phase": "fast_gate",
  "cycle_result": "running",
  "main_sha": "",
  "last_processed_sha": "",
  "last_validated_sha": "",
  "validating_sha": "abc1234",
  "last_error": ""
}
JSEOF

# Kill the test process to simulate it being down during validation.
kill "${test_pid}" 2>/dev/null || true
wait "${test_pid}" 2>/dev/null || true
rm -f "${main_root}/pids/lark-test.pid"

# Run a supervisor tick — it should NOT restart test because validation is active.
run_supervisor run-once >/dev/null

# Check: test should still be down (no pid file or stale pid).
if [[ -f "${main_root}/pids/lark-test.pid" ]]; then
  suppressed_pid="$(cat "${main_root}/pids/lark-test.pid" 2>/dev/null || true)"
  if [[ -n "${suppressed_pid}" ]] && kill -0 "${suppressed_pid}" 2>/dev/null; then
    echo "expected test NOT restarted during validation phase, but got pid=${suppressed_pid}" >&2
    exit 1
  fi
fi

# Check: mode should be "validating" (not degraded) while test is intentionally suppressed.
if ! grep -q '"mode": "validating"' "${status_file}"; then
  echo "expected supervisor mode=validating during validation suppression" >&2
  exit 1
fi

# Verify the log contains the suppression skip message.
supervisor_log="${main_root}/.worktrees/test/logs/lark-supervisor.log"
if [[ -f "${supervisor_log}" ]]; then
  if ! grep -q "skip test restart during validation" "${supervisor_log}"; then
    echo "expected 'skip test restart during validation' in supervisor log" >&2
    exit 1
  fi
fi

# Recovery path: when suppression exceeds timeout, supervisor should force test restart.
sleep 3
run_supervisor run-once >/dev/null

recovered_test_pid="$(cat "${main_root}/pids/lark-test.pid" 2>/dev/null || true)"
if [[ -z "${recovered_test_pid}" ]] || ! kill -0 "${recovered_test_pid}" 2>/dev/null; then
  echo "expected test restarted after validation suppression timeout" >&2
  exit 1
fi

if [[ -f "${supervisor_log}" ]]; then
  if ! grep -q "validation suppression timeout reached" "${supervisor_log}"; then
    echo "expected validation suppression timeout log entry" >&2
    exit 1
  fi
fi

# Stale phase recovery: if loop is down but loop-state remains validating, supervisor
# should reset the phase to idle and recover restart behavior.
loop_pid="$(cat "${main_root}/pids/lark-loop.pid" 2>/dev/null || true)"
if [[ -n "${loop_pid}" ]]; then
  kill "${loop_pid}" 2>/dev/null || true
  wait "${loop_pid}" 2>/dev/null || true
fi
rm -f "${main_root}/pids/lark-loop.pid"

test_pid="$(cat "${main_root}/pids/lark-test.pid" 2>/dev/null || true)"
if [[ -n "${test_pid}" ]]; then
  kill "${test_pid}" 2>/dev/null || true
  wait "${test_pid}" 2>/dev/null || true
fi
rm -f "${main_root}/pids/lark-test.pid"

cat > "${loop_state_file}" <<JSEOF
{
  "ts_utc": "2026-01-01T00:00:00Z",
  "base_sha": "abc1234",
  "cycle_phase": "fast_gate",
  "cycle_result": "running",
  "main_sha": "",
  "last_processed_sha": "",
  "last_validated_sha": "",
  "validating_sha": "abc1234",
  "last_error": ""
}
JSEOF

run_supervisor run-once >/dev/null

if ! grep -q '"cycle_phase": "idle"' "${loop_state_file}"; then
  echo "expected stale loop state recovery to reset cycle_phase=idle" >&2
  exit 1
fi

if ! grep -q '"stale_state_recovered": true' "${status_file}"; then
  echo "expected status stale_state_recovered=true after stale loop state recovery" >&2
  exit 1
fi

post_recover_test_pid="$(cat "${main_root}/pids/lark-test.pid" 2>/dev/null || true)"
if [[ -z "${post_recover_test_pid}" ]] || ! kill -0 "${post_recover_test_pid}" 2>/dev/null; then
  echo "expected test restarted after stale state recovery" >&2
  exit 1
fi

echo "ok"
