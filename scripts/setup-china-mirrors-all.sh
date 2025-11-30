#!/bin/bash
###############################################################################
# One-Click Setup for All China Mirrors
#
# This script configures all available mirrors for faster builds in China:
# - Docker registry mirrors (Linux only, requires sudo)
# - NPM registry in .env
# - PyPI index in .env
###############################################################################

set -euo pipefail

# Colors
readonly C_RED='\033[0;31m'
readonly C_GREEN='\033[0;32m'
readonly C_YELLOW='\033[1;33m'
readonly C_BLUE='\033[0;34m'
readonly C_CYAN='\033[0;36m'
readonly C_RESET='\033[0m'

log_info() {
    echo -e "${C_BLUE}▸${C_RESET} $*"
}

log_success() {
    echo -e "${C_GREEN}✓${C_RESET} $*"
}

log_error() {
    echo -e "${C_RED}✗${C_RESET} $*" >&2
}

log_warn() {
    echo -e "${C_YELLOW}⚠${C_RESET} $*"
}

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly AUTH_DB_MIRROR_IMAGE="docker.m.daocloud.io/library/postgres:15"

echo -e "${C_CYAN}"
cat << 'EOF'
    ___    __    _______  __  __
   /   |  / /   / ____/ |/ / / /
  / /| | / /   / __/  |   / / /
 / ___ |/ /___/ /___ /   | / /___
/_/  |_/_____/_____//_/|_|/_____/

China Mirrors Setup
EOF
echo -e "${C_RESET}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cd "$PROJECT_ROOT"

###############################################################################
# Step 1: Configure .env for China Sandbox and mirrors
###############################################################################

echo -e "${C_BLUE}Step 1/2: Configuring China sandbox and mirrors in .env${C_RESET}"
echo ""

ENV_FILE=".env"
ENV_EXAMPLE=".env.example"

if [[ ! -f "$ENV_FILE" ]]; then
    if [[ -f "$ENV_EXAMPLE" ]]; then
        log_info "Creating .env from .env.example"
        cp "$ENV_EXAMPLE" "$ENV_FILE"
        log_success ".env created"
    else
        log_error ".env.example not found"
        exit 1
    fi
fi

# Check if China sandbox image is already configured
SANDBOX_IMAGE_CONFIGURED=$(grep -c "^SANDBOX_IMAGE=" "$ENV_FILE" || true)
NPM_CONFIGURED=$(grep -c "^NPM_REGISTRY=" "$ENV_FILE" || true)
PIP_CONFIGURED=$(grep -c "^PIP_INDEX_URL=" "$ENV_FILE" || true)
AUTH_DB_IMAGE_CONFIGURED=$(grep -c "^AUTH_DB_IMAGE=" "$ENV_FILE" || true)

if [[ $SANDBOX_IMAGE_CONFIGURED -gt 0 ]]; then
    log_success "SANDBOX_IMAGE already configured in .env"
else
    # Remove commented lines if they exist
    sed -i.bak '/^# SANDBOX_IMAGE=/d' "$ENV_FILE" 2>/dev/null || true
    sed -i.bak '/^# SANDBOX_SECURITY_OPT=/d' "$ENV_FILE" 2>/dev/null || true

    # Add China sandbox configuration
    echo "" >> "$ENV_FILE"
    echo "# China Mirror Configuration" >> "$ENV_FILE"
    echo "# Use pre-built China sandbox image (fastest option - no build required!)" >> "$ENV_FILE"
    echo "SANDBOX_IMAGE=enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest" >> "$ENV_FILE"
    echo "SANDBOX_SECURITY_OPT=seccomp=unconfined" >> "$ENV_FILE"
    log_success "SANDBOX_IMAGE configured - using Volcengine pre-built image"

    # Clean up backup files
    rm -f "${ENV_FILE}.bak"
fi

# Configure Postgres image mirror for auth-db
if [[ $AUTH_DB_IMAGE_CONFIGURED -gt 0 ]]; then
    CURRENT_AUTH_DB_IMAGE=$(grep "^AUTH_DB_IMAGE=" "$ENV_FILE" | tail -n 1 | cut -d= -f2-)
    if [[ "$CURRENT_AUTH_DB_IMAGE" == "$AUTH_DB_MIRROR_IMAGE" ]]; then
        log_success "AUTH_DB_IMAGE already set to China mirror: $CURRENT_AUTH_DB_IMAGE"
    elif [[ "$CURRENT_AUTH_DB_IMAGE" == "postgres:15" || -z "$CURRENT_AUTH_DB_IMAGE" ]]; then
        # Replace default value with mirror
        sed -i.bak "s|^AUTH_DB_IMAGE=.*$|AUTH_DB_IMAGE=$AUTH_DB_MIRROR_IMAGE|" "$ENV_FILE"
        rm -f "${ENV_FILE}.bak"
        log_success "AUTH_DB_IMAGE updated to China mirror: $AUTH_DB_MIRROR_IMAGE"
    else
        log_success "AUTH_DB_IMAGE already customized: $CURRENT_AUTH_DB_IMAGE"
    fi
else
    echo "AUTH_DB_IMAGE=$AUTH_DB_MIRROR_IMAGE" >> "$ENV_FILE"
    log_success "AUTH_DB_IMAGE configured: $AUTH_DB_MIRROR_IMAGE"
fi

# Note: NPM and PIP mirrors are only needed when building, not when using pre-built image
if [[ $SANDBOX_IMAGE_CONFIGURED -eq 0 ]]; then
    log_info "Note: NPM/PyPI mirrors not needed when using pre-built SANDBOX_IMAGE"
    log_info "      (Pre-built image is used instead of building)"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

###############################################################################
# Step 2: Configure Docker Registry Mirrors
###############################################################################

echo -e "${C_BLUE}Step 2/2: Configuring Docker registry mirrors${C_RESET}"
echo ""

if [[ "$(uname -s)" == "Linux" ]]; then
    # On Linux, run the Docker mirrors setup script
    if [[ -x "$SCRIPT_DIR/setup-docker-mirrors.sh" ]]; then
        log_info "Running Docker mirrors setup script..."
        echo ""
        "$SCRIPT_DIR/setup-docker-mirrors.sh"
    else
        log_error "setup-docker-mirrors.sh not found or not executable"
        log_warn "Please run: chmod +x scripts/setup-docker-mirrors.sh"
    fi
else
    # On macOS/Windows, provide instructions
    log_info "Detected $(uname -s) - Docker Desktop configuration required"
    echo ""
    echo "Please configure Docker Desktop manually:"
    echo ""
    echo "1. Open Docker Desktop"
    echo "2. Go to Settings → Docker Engine"
    echo "3. Add the following to the JSON configuration:"
    echo ""
    cat << 'EOF'
{
  "registry-mirrors": [
    "https://mirror.ccs.tencentyun.com"
  ]
}
EOF
    echo ""
    echo "4. Click 'Apply & Restart'"
    echo ""
    log_warn "Please complete the above steps manually"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

###############################################################################
# Verification
###############################################################################

echo -e "${C_BLUE}Verifying Configuration${C_RESET}"
echo ""

if [[ -x "$SCRIPT_DIR/test-china-mirrors.sh" ]]; then
    "$SCRIPT_DIR/test-china-mirrors.sh"
else
    log_warn "test-china-mirrors.sh not found or not executable"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${C_GREEN}✓ Configuration Complete!${C_RESET}"
echo ""
echo "Next steps:"
echo "  1. Verify configuration: ./scripts/test-china-mirrors.sh"
echo "  2. Start services: ./deploy.sh start"
echo ""
echo "For more information, see: docs/deployment/CHINA_MIRRORS.md"
echo ""
