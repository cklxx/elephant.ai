#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUPERVISOR_SH="${ROOT}/scripts/lark/supervisor.sh"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t lark-supervisor-smoke)"
main_root="${tmpdir}/repo"

run_supervisor() {
  env \
    LARK_MAIN_ROOT="${main_root}" \
    WORKTREE_SH="${main_root}/scripts/lark/worktree.sh" \
    MAIN_SH="${main_root}/scripts/lark/main.sh" \
    TEST_SH="${main_root}/scripts/lark/test.sh" \
    LOOP_AGENT_SH="${main_root}/scripts/lark/loop-agent.sh" \
    AUTOFIX_SH="${main_root}/scripts/lark/autofix.sh" \
    MAIN_CONFIG="${main_root}/config-main.yaml" \
    TEST_CONFIG="${main_root}/config-test.yaml" \
    LARK_SUPERVISOR_SKIP_HEALTHCHECK=1 \
    LARK_SUPERVISOR_TICK_SECONDS=1 \
    LARK_RESTART_MAX_IN_WINDOW=5 \
    LARK_RESTART_WINDOW_SECONDS=30 \
    LARK_COOLDOWN_SECONDS=3 \
    "${SUPERVISOR_SH}" "$@"
}

cleanup() {
  run_supervisor stop >/dev/null 2>&1 || true

  if [[ -d "${main_root}/.pids" ]]; then
    while IFS= read -r pid_file; do
      pid="$(cat "${pid_file}" 2>/dev/null || true)"
      if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
        kill "${pid}" 2>/dev/null || true
      fi
    done < <(find "${main_root}" -name '*.pid' -type f 2>/dev/null || true)
  fi

  rm -rf "${tmpdir}"
}
trap cleanup EXIT

mkdir -p "${main_root}/scripts/lark" "${main_root}/.pids" "${main_root}/logs" "${main_root}/.worktrees/test/.pids" "${main_root}/.worktrees/test/logs" "${main_root}/.worktrees/test/tmp"

cat > "${main_root}/scripts/lark/worktree.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

main_root="${LARK_MAIN_ROOT:?}"
test_root="${main_root}/.worktrees/test"
cmd="${1:-ensure}"

case "${cmd}" in
  ensure|sync-env)
    mkdir -p "${main_root}/.pids" "${main_root}/logs" "${test_root}/.pids" "${test_root}/logs" "${test_root}/tmp"
    ;;
  *)
    echo "unknown command: ${cmd}" >&2
    exit 2
    ;;
esac
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

make_stub "${main_root}/scripts/lark/main.sh" ".pids/lark-main.pid"
make_stub "${main_root}/scripts/lark/test.sh" ".worktrees/test/.pids/lark-test.pid"
make_stub "${main_root}/scripts/lark/loop-agent.sh" ".worktrees/test/.pids/lark-loop.pid"
cat > "${main_root}/scripts/lark/autofix.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF
chmod +x "${main_root}/scripts/lark/worktree.sh"
chmod +x "${main_root}/scripts/lark/autofix.sh"

cat > "${main_root}/config-main.yaml" <<'EOF'
server:
  port: 19080
EOF

cat > "${main_root}/config-test.yaml" <<'EOF'
server:
  port: 19081
EOF

git -C "${main_root}" init -q -b main 2>/dev/null || git -C "${main_root}" init -q
current_branch="$(git -C "${main_root}" symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
if [[ "${current_branch}" != "main" ]]; then
  git -C "${main_root}" checkout -q -b main
fi
echo "supervisor smoke" > "${main_root}/README.md"
git -C "${main_root}" add README.md
git -C "${main_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "init"

run_supervisor run-once >/dev/null

status_file="${main_root}/.worktrees/test/tmp/lark-supervisor.status.json"
[[ -f "${status_file}" ]] || { echo "missing status file: ${status_file}" >&2; exit 1; }
for key in ts_utc mode main_pid test_pid loop_pid main_health test_health loop_alive main_sha last_processed_sha last_validated_sha cycle_phase cycle_result last_error restart_count_window autofix_state autofix_incident_id autofix_last_reason autofix_last_started_at autofix_last_finished_at autofix_last_commit autofix_runs_window; do
  grep -q "\"${key}\"" "${status_file}" || { echo "missing key in status: ${key}" >&2; exit 1; }
done
grep -q '"mode": "healthy"' "${status_file}" || { echo "expected healthy mode after run-once" >&2; exit 1; }

main_pid="$(cat "${main_root}/.pids/lark-main.pid")"
test_pid="$(cat "${main_root}/.worktrees/test/.pids/lark-test.pid")"
loop_pid="$(cat "${main_root}/.worktrees/test/.pids/lark-loop.pid")"
for pid in "${main_pid}" "${test_pid}" "${loop_pid}"; do
  kill -0 "${pid}" 2>/dev/null || { echo "expected managed pid alive: ${pid}" >&2; exit 1; }
done

run_supervisor start >/dev/null
sleep 1
supervisor_pid_1="$(cat "${main_root}/.worktrees/test/.pids/lark-supervisor.pid")"
kill -0 "${supervisor_pid_1}" 2>/dev/null || { echo "supervisor pid not alive after start: ${supervisor_pid_1}" >&2; exit 1; }

run_supervisor start >/dev/null
supervisor_pid_2="$(cat "${main_root}/.worktrees/test/.pids/lark-supervisor.pid")"
if [[ "${supervisor_pid_1}" != "${supervisor_pid_2}" ]]; then
  echo "expected idempotent start to keep same supervisor pid" >&2
  exit 1
fi

run_supervisor doctor >/dev/null
run_supervisor stop >/dev/null

if [[ -f "${main_root}/.worktrees/test/.pids/lark-supervisor.pid" ]]; then
  echo "expected supervisor pid file removed after stop" >&2
  exit 1
fi

for pid_file in \
  "${main_root}/.pids/lark-main.pid" \
  "${main_root}/.worktrees/test/.pids/lark-test.pid" \
  "${main_root}/.worktrees/test/.pids/lark-loop.pid"; do
  if [[ -f "${pid_file}" ]]; then
    echo "expected component pid file removed after stop: ${pid_file}" >&2
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
test_pid="$(cat "${main_root}/.worktrees/test/.pids/lark-test.pid" 2>/dev/null || true)"
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
rm -f "${main_root}/.worktrees/test/.pids/lark-test.pid"

# Run a supervisor tick — it should NOT restart test because validation is active.
run_supervisor run-once >/dev/null

# Check: test should still be down (no pid file or stale pid).
if [[ -f "${main_root}/.worktrees/test/.pids/lark-test.pid" ]]; then
  suppressed_pid="$(cat "${main_root}/.worktrees/test/.pids/lark-test.pid" 2>/dev/null || true)"
  if [[ -n "${suppressed_pid}" ]] && kill -0 "${suppressed_pid}" 2>/dev/null; then
    echo "expected test NOT restarted during validation phase, but got pid=${suppressed_pid}" >&2
    exit 1
  fi
fi

# Clean up: reset loop state to idle so final stop works.
cat > "${loop_state_file}" <<JSEOF
{
  "ts_utc": "2026-01-01T00:00:00Z",
  "base_sha": "",
  "cycle_phase": "idle",
  "cycle_result": "unknown",
  "main_sha": "",
  "last_processed_sha": "",
  "last_validated_sha": "",
  "validating_sha": "",
  "last_error": ""
}
JSEOF

# Verify the log contains the skip message.
supervisor_log="${main_root}/.worktrees/test/logs/lark-supervisor.log"
if [[ -f "${supervisor_log}" ]]; then
  if ! grep -q "skip test restart during validation" "${supervisor_log}"; then
    echo "expected 'skip test restart during validation' in supervisor log" >&2
    exit 1
  fi
fi

echo "ok"
