#!/usr/bin/env bash
# shellcheck shell=bash
# One-command first-time project setup. Idempotent — safe to re-run.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO="${REPO_ROOT}/scripts/go-with-toolchain.sh"

source "${SCRIPT_DIR}/lib/common/logging.sh"
source "${SCRIPT_DIR}/lib/common/dotenv.sh"
source "${SCRIPT_DIR}/lib/common/env.sh"

cd "${REPO_ROOT}"

log_section "Checking prerequisites"

for cmd in go node npm; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    die "${cmd} is required but not found. Install it first."
  fi
done
log_success "Go, Node.js, npm found"

log_section "Environment"

SERVER_PORT="${SERVER_PORT:-8080}"

if [[ ! -f .env ]]; then
  log_info "Creating .env with defaults..."
  secret=$(generate_auth_secret)
  cat > .env << EOF
OPENAI_API_KEY=
NEXT_PUBLIC_API_URL=http://localhost:${SERVER_PORT}

# Authentication defaults for local development
AUTH_JWT_SECRET=${secret}
AUTH_DATABASE_URL=postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable
ALEX_SESSION_DATABASE_URL=postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable

# China Mirror Configuration (uncomment to enable)
# NPM_REGISTRY=https://registry.npmmirror.com/
# PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
EOF
  ensure_private_file_mode ".env"
  log_success "Created .env"
else
  log_success ".env already exists"
fi

log_section "Go dependencies"

"${GO}" mod download
log_success "Go modules downloaded"

log_section "Building alex CLI"

CGO_ENABLED=0 "${GO}" build -o alex ./cmd/alex
log_success "Built ./alex"

log_section "Web dependencies"

npm --prefix web install
log_success "web/node_modules installed"

log_section "Local runtime setup"

if [[ -x scripts/setup_local_runtime.sh ]]; then
  scripts/setup_local_runtime.sh || log_warn "setup_local_runtime.sh had issues (non-fatal)"
else
  log_info "No setup_local_runtime.sh found, skipping"
fi

log_section "Git hooks"

if [[ -f scripts/pre-push-hook.sh ]]; then
  cp scripts/pre-push-hook.sh .git/hooks/pre-push 2>/dev/null || true
  chmod +x .git/hooks/pre-push 2>/dev/null || true
  log_success "Pre-push hook installed"
else
  log_info "No pre-push-hook.sh found, skipping"
fi

echo ""
log_success "Setup complete. Run 'make doctor' to verify, or './dev.sh' to start developing."
echo ""
