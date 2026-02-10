#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../scripts/lib/common/logging.sh
source "${ROOT}/scripts/lib/common/logging.sh"
# shellcheck source=../../scripts/lib/common/process.sh
source "${ROOT}/scripts/lib/common/process.sh"
# shellcheck source=../../scripts/lark/identity_lock.sh
source "${ROOT}/scripts/lark/identity_lock.sh"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t lark-identity-lock)"
cleanup() {
  if [[ -n "${holder_pid:-}" ]] && kill -0 "${holder_pid}" 2>/dev/null; then
    kill "${holder_pid}" 2>/dev/null || true
    wait "${holder_pid}" 2>/dev/null || true
  fi
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

mkdir -p "${tmpdir}/pids"

main_cfg="${tmpdir}/config.yaml"
test_cfg="${tmpdir}/test.yaml"
test_same_identity_cfg="${tmpdir}/test-same-identity.yaml"

cat > "${main_cfg}" <<'EOF'
channels:
  lark:
    app_id: "cli_main_app"
    base_domain: "open.feishu.cn"
EOF

cat > "${test_cfg}" <<'EOF'
channels:
  lark:
    app_id: "cli_test_app"
    base_domain: "open.feishu.cn"
EOF

cat > "${test_same_identity_cfg}" <<'EOF'
channels:
  lark:
    app_id: "cli_main_app"
    base_domain: "open.feishu.cn"
EOF

lark_assert_main_test_isolation "${main_cfg}" "${test_cfg}"

if lark_assert_main_test_isolation "${main_cfg}" "${main_cfg}" >/dev/null 2>&1; then
  echo "expected same config path to fail isolation check" >&2
  exit 1
fi

if lark_assert_main_test_isolation "${main_cfg}" "${test_same_identity_cfg}" >/dev/null 2>&1; then
  echo "expected same lark identity to fail isolation check" >&2
  exit 1
fi

sleep 300 &
holder_pid="$!"
lark_write_identity_lock "${tmpdir}" "main" "${main_cfg}" "${holder_pid}"

if lark_assert_identity_available "${tmpdir}" "test" "${main_cfg}" "$$" >/dev/null 2>&1; then
  echo "expected conflicting live owner pid to block identity reuse" >&2
  exit 1
fi

lark_assert_identity_available "${tmpdir}" "main" "${main_cfg}" "${holder_pid}"

kill "${holder_pid}" 2>/dev/null || true
wait "${holder_pid}" 2>/dev/null || true
holder_pid=""
lark_assert_identity_available "${tmpdir}" "test" "${main_cfg}" "$$"

lark_write_identity_lock "${tmpdir}" "main" "${main_cfg}" "$$"
lark_release_identity_lock "${tmpdir}" "${main_cfg}" "$$"
lock_path="$(lark_identity_lock_file "${tmpdir}" "$(lark_resolve_identity "${main_cfg}")" "${main_cfg}")"
if [[ -f "${lock_path}" ]]; then
  echo "expected lock file removed after explicit release" >&2
  exit 1
fi

echo "ok"
