#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOOP_SH="${ROOT}/scripts/lark/loop.sh"

sandbox="$(mktemp -d 2>/dev/null || mktemp -d -t lark-loop-codex-pid)"
cleanup() {
  rm -rf "${sandbox}"
}
trap cleanup EXIT

main_root="${sandbox}/main"
test_root="${sandbox}/test"
pid_dir="${sandbox}/pids"
tmp_dir="${sandbox}/tmp"
bin_dir="${sandbox}/bin"
loop_log="${sandbox}/lark-loop.log"
fail_summary="${sandbox}/lark-loop.fail.txt"

mkdir -p "${main_root}" "${test_root}" "${pid_dir}" "${tmp_dir}" "${bin_dir}"
touch "${loop_log}" "${fail_summary}"

# Create a tiny git repo so auto_fix commit checks can run safely.
git -C "${test_root}" init -q
echo "seed" > "${test_root}/README.md"
git -C "${test_root}" add README.md
git -C "${test_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "init"

cat > "${bin_dir}/codex" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
sleep 2
EOF
chmod +x "${bin_dir}/codex"

set -- help
# shellcheck source=/dev/null
source "${LOOP_SH}" >/dev/null

MAIN_ROOT="${main_root}"
TEST_ROOT="${test_root}"
PID_DIR="${pid_dir}"
CODEX_LOOP_PID_FILE="${PID_DIR}/lark-codex-loop.pid"
TMP_DIR="${tmp_dir}"
LOOP_LOG="${loop_log}"
FAIL_SUMMARY="${fail_summary}"
LOOP_AUTOFIX_ENABLED=1

PATH="${bin_dir}:${PATH}"

auto_fix "fast" "abc12345" &
runner_pid="$!"

seen_pid_file=0
for _ in $(seq 1 40); do
  if [[ -f "${CODEX_LOOP_PID_FILE}" ]]; then
    seen_pid_file=1
    break
  fi
  sleep 0.1
done

if [[ "${seen_pid_file}" -ne 1 ]]; then
  echo "expected codex pid file to be recorded while auto_fix is running" >&2
  wait "${runner_pid}" || true
  exit 1
fi

codex_pid="$(cat "${CODEX_LOOP_PID_FILE}" 2>/dev/null || true)"
if [[ -z "${codex_pid}" ]] || ! kill -0 "${codex_pid}" 2>/dev/null; then
  echo "expected recorded codex pid to be alive: ${codex_pid}" >&2
  wait "${runner_pid}" || true
  exit 1
fi

wait "${runner_pid}"

if [[ -f "${CODEX_LOOP_PID_FILE}" ]]; then
  echo "expected codex pid file cleaned after auto_fix completion" >&2
  exit 1
fi

if kill -0 "${codex_pid}" 2>/dev/null; then
  echo "expected codex pid to exit after auto_fix completion: ${codex_pid}" >&2
  exit 1
fi

echo "lark loop codex pid lifecycle: PASS"
