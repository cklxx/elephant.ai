#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUPERVISOR_SH="${ROOT}/scripts/lark/supervisor.sh"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t lark-supervisor-test-gates-main)"
main_root="${tmpdir}/repo"

run_supervisor() {
  env \
    LARK_MAIN_ROOT="${main_root}" \
    MAIN_SH="${main_root}/scripts/lark/main.sh" \
    TEST_SH="${main_root}/scripts/lark/test.sh" \
    LOOP_AGENT_SH="${main_root}/scripts/lark/loop-agent.sh" \
    AUTOFIX_SH="${main_root}/scripts/lark/autofix.sh" \
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

cleanup() {
  run_supervisor stop >/dev/null 2>&1 || true
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

mkdir -p "${main_root}/scripts/lark" "${main_root}/pids" "${main_root}/logs"

make_main_or_loop_stub() {
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

cat > "${main_root}/scripts/lark/test.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

main_root="${LARK_MAIN_ROOT:?}"
pid_file="${main_root}/pids/lark-test.pid"
fail_flag="${main_root}/tmp/fail-test-start"
mkdir -p "$(dirname "${pid_file}")"

start_proc() {
  if [[ -f "${fail_flag}" ]]; then
    exit 1
  fi
  local pid
  pid="$(cat "${pid_file}" 2>/dev/null || true)"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
    return 0
  fi
  sleep 300 &
  echo "$!" > "${pid_file}"
}

stop_proc() {
  local pid
  pid="$(cat "${pid_file}" 2>/dev/null || true)"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
    kill "${pid}" 2>/dev/null || true
    wait "${pid}" 2>/dev/null || true
  fi
  rm -f "${pid_file}"
}

cmd="${1:-start}"
case "${cmd}" in
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
    echo "unknown command: ${cmd}" >&2
    exit 2
    ;;
esac
EOF
chmod +x "${main_root}/scripts/lark/test.sh"

make_main_or_loop_stub "${main_root}/scripts/lark/main.sh" "pids/lark-main.pid"
make_main_or_loop_stub "${main_root}/scripts/lark/loop-agent.sh" "pids/lark-loop.pid"
cat > "${main_root}/scripts/lark/autofix.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF
chmod +x "${main_root}/scripts/lark/autofix.sh"

cat > "${main_root}/config-main.yaml" <<'EOF'
server:
  port: 19080
EOF
cat > "${main_root}/config-test.yaml" <<'EOF'
server:
  port: 19081
EOF
cat > "${main_root}/.env" <<'EOF'
LLM_API_KEY=test
EOF

git -C "${main_root}" init -q -b main 2>/dev/null || git -C "${main_root}" init -q
current_branch="$(git -C "${main_root}" symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
if [[ "${current_branch}" != "main" ]]; then
  git -C "${main_root}" checkout -q -b main
fi
echo "supervisor gate main by test" > "${main_root}/README.md"
git -C "${main_root}" add README.md .env
git -C "${main_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "init"

mkdir -p "${main_root}/tmp"
touch "${main_root}/tmp/fail-test-start"

run_supervisor run-once >/dev/null

if [[ -f "${main_root}/pids/lark-main.pid" ]]; then
  main_pid="$(cat "${main_root}/pids/lark-main.pid" 2>/dev/null || true)"
  if [[ -n "${main_pid}" ]] && kill -0 "${main_pid}" 2>/dev/null; then
    echo "expected main not started when test start fails" >&2
    exit 1
  fi
fi

supervisor_log="${main_root}/.worktrees/test/logs/lark-supervisor.log"
if [[ -f "${supervisor_log}" ]]; then
  if ! grep -q "skip main restart: test is not healthy" "${supervisor_log}"; then
    echo "expected skip-main log when test is unhealthy" >&2
    exit 1
  fi
fi

rm -f "${main_root}/tmp/fail-test-start"
run_supervisor run-once >/dev/null

test_pid="$(cat "${main_root}/pids/lark-test.pid" 2>/dev/null || true)"
main_pid="$(cat "${main_root}/pids/lark-main.pid" 2>/dev/null || true)"

if [[ -z "${test_pid}" ]] || ! kill -0 "${test_pid}" 2>/dev/null; then
  echo "expected test started after removing failure marker" >&2
  exit 1
fi
if [[ -z "${main_pid}" ]] || ! kill -0 "${main_pid}" 2>/dev/null; then
  echo "expected main started after test recovered" >&2
  exit 1
fi

echo "lark supervisor gates main on test health: PASS"
