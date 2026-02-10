#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AUTOFIX_SH="${ROOT}/scripts/lark/autofix.sh"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t lark-autofix-smoke)"

cleanup() {
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

init_repo() {
  local repo_root="$1"
  mkdir -p \
    "${repo_root}/scripts/lark" \
    "${repo_root}/scripts/lib/common" \
    "${repo_root}/tests/scripts" \
    "${repo_root}/.worktrees/test/tmp" \
    "${repo_root}/.worktrees/test/logs"

  cat > "${repo_root}/go.mod" <<'EOF'
module autofix-smoke

go 1.22
EOF

  cat > "${repo_root}/lark.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-status}"
case "${cmd}" in
  doctor) echo "doctor ok" ;;
  *) echo "ok" ;;
esac
EOF

  cat > "${repo_root}/scripts/lark/supervisor.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-doctor}"
case "${cmd}" in
  doctor) echo "supervisor doctor ok" ;;
  *) echo "ok" ;;
esac
EOF

  cat > "${repo_root}/scripts/lark/dummy.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "dummy"
EOF

  cat > "${repo_root}/scripts/lib/common/dummy.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "common"
EOF

  cat > "${repo_root}/tests/scripts/lark-supervisor-smoke.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "ok"
EOF

  echo "base" > "${repo_root}/target.txt"

  chmod +x \
    "${repo_root}/lark.sh" \
    "${repo_root}/scripts/lark/supervisor.sh" \
    "${repo_root}/scripts/lark/dummy.sh" \
    "${repo_root}/scripts/lib/common/dummy.sh" \
    "${repo_root}/tests/scripts/lark-supervisor-smoke.sh"

  git -C "${repo_root}" init -q -b main 2>/dev/null || git -C "${repo_root}" init -q
  if [[ "$(git -C "${repo_root}" symbolic-ref --quiet --short HEAD 2>/dev/null || true)" != "main" ]]; then
    git -C "${repo_root}" checkout -q -b main
  fi
  git -C "${repo_root}" add .
  git -C "${repo_root}" -c user.name="test" -c user.email="test@example.com" commit -q -m "init"
}

make_codex_success() {
  local bin="$1"
  cat > "${bin}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
sleep 2
git_dir="$(git rev-parse --git-dir 2>/dev/null || true)"
if [[ -n "${git_dir}" && ( -d "${git_dir}/rebase-merge" || -d "${git_dir}/rebase-apply" ) ]]; then
  echo "resolved-success" > target.txt
else
  echo "autofix-success" > target.txt
fi
EOF
  chmod +x "${bin}"
}

make_codex_failure() {
  local bin="$1"
  cat > "${bin}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
exit 1
EOF
  chmod +x "${bin}"
}

make_codex_conflict() {
  local bin="$1"
  cat > "${bin}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
git_dir="$(git rev-parse --git-dir 2>/dev/null || true)"
if [[ -n "${git_dir}" && ( -d "${git_dir}/rebase-merge" || -d "${git_dir}/rebase-apply" ) ]]; then
  echo "resolved-conflict" > target.txt
  exit 0
fi
echo "autofix-conflict" > target.txt
if [[ -n "${MAIN_ROOT_FOR_TEST:-}" ]]; then
  echo "main-conflict" > "${MAIN_ROOT_FOR_TEST}/target.txt"
  git -C "${MAIN_ROOT_FOR_TEST}" add target.txt
  git -C "${MAIN_ROOT_FOR_TEST}" -c user.name="test" -c user.email="test@example.com" commit -q -m "advance main"
fi
EOF
  chmod +x "${bin}"
}

assert_state() {
  local state_file="$1"
  local expected_state="$2"
  [[ -f "${state_file}" ]] || { echo "missing state file: ${state_file}" >&2; exit 1; }
  grep -q "\"autofix_state\": \"${expected_state}\"" "${state_file}" || {
    echo "expected autofix_state=${expected_state} in ${state_file}" >&2
    cat "${state_file}" >&2
    exit 1
  }
}

assert_no_codex_pid_file() {
  local repo_root="$1"
  local pid_file="${repo_root}/pids/lark-codex-autofix.pid"
  if [[ -f "${pid_file}" ]]; then
    local pid
    pid="$(cat "${pid_file}" 2>/dev/null || true)"
    echo "expected no codex autofix pid file after trigger run: ${pid_file} pid=${pid}" >&2
    exit 1
  fi
}

run_success_case() {
  local case_dir repo_root codex_bin state_file commit_count runner_pid codex_pid pid_file seen
  case_dir="${tmpdir}/success"
  repo_root="${case_dir}/repo"
  codex_bin="${case_dir}/codex-success"
  mkdir -p "${case_dir}"
  init_repo "${repo_root}"
  make_codex_success "${codex_bin}"

  env \
    LARK_MAIN_ROOT="${repo_root}" \
    LARK_AUTOFIX_CODEX_BIN="${codex_bin}" \
    LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS=120 \
    "${AUTOFIX_SH}" trigger \
      --incident-id "success-1" \
      --reason "restart storm" \
      --signature "sig-success" \
      --main-sha "$(git -C "${repo_root}" rev-parse main)" &
  runner_pid="$!"

  pid_file="${repo_root}/pids/lark-codex-autofix.pid"
  seen=0
  for _ in $(seq 1 50); do
    if [[ -f "${pid_file}" ]]; then
      seen=1
      break
    fi
    sleep 0.1
  done
  if [[ "${seen}" -ne 1 ]]; then
    echo "expected codex autofix pid file while trigger is running" >&2
    wait "${runner_pid}" || true
    exit 1
  fi

  codex_pid="$(cat "${pid_file}" 2>/dev/null || true)"
  if [[ -z "${codex_pid}" ]] || ! kill -0 "${codex_pid}" 2>/dev/null; then
    echo "expected codex autofix pid alive while trigger is running: ${codex_pid}" >&2
    wait "${runner_pid}" || true
    exit 1
  fi

  wait "${runner_pid}"

  state_file="${repo_root}/.worktrees/test/tmp/lark-autofix.state.json"
  assert_state "${state_file}" "succeeded"
  assert_no_codex_pid_file "${repo_root}"
  grep -q '"autofix_restart_required": "true"' "${state_file}" || { echo "expected restart_required=true" >&2; exit 1; }
  commit_count="$(git -C "${repo_root}" rev-list --count main)"
  if (( commit_count < 2 )); then
    echo "expected main to include autofix commit (count=${commit_count})" >&2
    exit 1
  fi
}

run_failure_case() {
  local case_dir repo_root codex_bin state_file commit_count
  case_dir="${tmpdir}/failure"
  repo_root="${case_dir}/repo"
  codex_bin="${case_dir}/codex-failure"
  mkdir -p "${case_dir}"
  init_repo "${repo_root}"
  make_codex_failure "${codex_bin}"

  set +e
  env \
    LARK_MAIN_ROOT="${repo_root}" \
    LARK_AUTOFIX_CODEX_BIN="${codex_bin}" \
    LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS=120 \
    "${AUTOFIX_SH}" trigger \
      --incident-id "failure-1" \
      --reason "restart storm" \
      --signature "sig-failure" \
      --main-sha "$(git -C "${repo_root}" rev-parse main)"
  local rc=$?
  set -e
  if [[ ${rc} -eq 0 ]]; then
    echo "expected failure case to return non-zero" >&2
    exit 1
  fi

  state_file="${repo_root}/.worktrees/test/tmp/lark-autofix.state.json"
  assert_state "${state_file}" "failed"
  assert_no_codex_pid_file "${repo_root}"
  commit_count="$(git -C "${repo_root}" rev-list --count main)"
  if (( commit_count != 1 )); then
    echo "expected main unchanged after failed autofix (count=${commit_count})" >&2
    exit 1
  fi
}

run_conflict_case() {
  local case_dir repo_root codex_bin state_file
  case_dir="${tmpdir}/conflict"
  repo_root="${case_dir}/repo"
  codex_bin="${case_dir}/codex-conflict"
  mkdir -p "${case_dir}"
  init_repo "${repo_root}"
  make_codex_conflict "${codex_bin}"

  env \
    LARK_MAIN_ROOT="${repo_root}" \
    LARK_AUTOFIX_CODEX_BIN="${codex_bin}" \
    LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS=120 \
    MAIN_ROOT_FOR_TEST="${repo_root}" \
    "${AUTOFIX_SH}" trigger \
      --incident-id "conflict-1" \
      --reason "restart storm" \
      --signature "sig-conflict" \
      --main-sha "$(git -C "${repo_root}" rev-parse main)"

  state_file="${repo_root}/.worktrees/test/tmp/lark-autofix.state.json"
  assert_state "${state_file}" "succeeded"
  assert_no_codex_pid_file "${repo_root}"
  if [[ "$(cat "${repo_root}/target.txt")" != "resolved-conflict" ]]; then
    echo "expected conflict resolution content in main target.txt" >&2
    exit 1
  fi
}

run_success_case
run_failure_case
run_conflict_case

echo "ok"
