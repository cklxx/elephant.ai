#!/bin/bash
###############################################################################
# Test China Mirror Configuration
#
# This script verifies that the China mirror configuration is working correctly
# by checking the build arguments passed to Docker.
###############################################################################

set -euo pipefail

# Colors
readonly C_GREEN='\033[0;32m'
readonly C_YELLOW='\033[1;33m'
readonly C_BLUE='\033[0;34m'
readonly C_RESET='\033[0m'

echo -e "${C_BLUE}Testing China Mirror Configuration${C_RESET}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check Docker registry mirrors first
echo -e "${C_BLUE}Docker Registry Mirrors:${C_RESET}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if command -v docker >/dev/null 2>&1; then
    if docker info >/dev/null 2>&1; then
        MIRRORS=$(docker info 2>/dev/null | grep -A 10 "Registry Mirrors:" | tail -n +2 | grep -v "^--" | grep "https://" || true)
        if [[ -n "$MIRRORS" ]]; then
            echo -e "${C_GREEN}✓${C_RESET} Docker registry mirrors configured:"
            echo "$MIRRORS" | while read -r line; do
                echo "  $line"
            done
        else
            echo -e "${C_YELLOW}○${C_RESET} No Docker registry mirrors configured"
            echo ""
            echo "To configure Docker mirrors, run:"
            echo "  ./scripts/setup-docker-mirrors.sh  # Linux only"
            echo ""
            echo "Or configure manually in Docker Desktop (macOS/Windows)"
        fi
    else
        echo -e "${C_YELLOW}⚠${C_RESET} Docker is installed but not running"
    fi
else
    echo -e "${C_YELLOW}○${C_RESET} Docker not installed"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if .env exists
if [[ ! -f .env ]]; then
    echo -e "${C_YELLOW}Warning: .env file not found${C_RESET}"
    echo "Create .env from .env.example and configure mirrors"
    echo ""
else

# Source .env
set -a
source .env
set +a

echo "Current Configuration:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check China sandbox configuration
if [[ -n "${SANDBOX_IMAGE:-}" ]]; then
    echo -e "${C_GREEN}✓${C_RESET} SANDBOX_IMAGE: ${SANDBOX_IMAGE}"
    echo -e "  Using pre-built sandbox image (no build required)"
    if [[ -n "${SANDBOX_SECURITY_OPT:-}" ]]; then
        echo -e "${C_GREEN}✓${C_RESET} SANDBOX_SECURITY_OPT: ${SANDBOX_SECURITY_OPT}"
    fi
    echo ""
else
    echo -e "${C_YELLOW}○${C_RESET} SANDBOX_IMAGE: not set (will build from Dockerfile)"
    echo ""

    # Check NPM registry (only relevant when building)
    if [[ -n "${NPM_REGISTRY:-}" ]]; then
        echo -e "${C_GREEN}✓${C_RESET} NPM Registry: ${NPM_REGISTRY}"
    else
        echo -e "${C_YELLOW}○${C_RESET} NPM Registry: (using default: https://registry.npmjs.org/)"
    fi

    # Check PIP index (only relevant when building)
    if [[ -n "${PIP_INDEX_URL:-}" ]]; then
        echo -e "${C_GREEN}✓${C_RESET} PIP Index URL: ${PIP_INDEX_URL}"
    else
        echo -e "${C_YELLOW}○${C_RESET} PIP Index URL: (using default: https://pypi.org/simple)"
    fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test docker-compose config
if command -v docker-compose >/dev/null 2>&1 || docker compose version >/dev/null 2>&1; then
    echo "Docker Compose Build Arguments:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    if docker compose version >/dev/null 2>&1; then
        docker compose config | grep -A3 "sandbox:" | grep -A2 "args:" || true
    elif command -v docker-compose >/dev/null 2>&1; then
        docker-compose config | grep -A3 "sandbox:" | grep -A2 "args:" || true
    fi
else
    echo -e "${C_YELLOW}Warning: docker-compose not found${C_RESET}"
fi

fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${C_BLUE}Recommended Configuration for China:${C_RESET}"
echo ""
echo "  🚀 Fastest: SANDBOX_IMAGE=enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest"
echo "  🐳 Docker:  https://docker.mirrors.ustc.edu.cn"
echo "  📦 NPM:     https://registry.npmmirror.com/"
echo "  🐍 PIP:     https://pypi.tuna.tsinghua.edu.cn/simple"
echo ""
echo -e "${C_BLUE}Quick Setup (One Command):${C_RESET}"
echo ""
echo "  ./scripts/setup-china-mirrors-all.sh"
echo ""
echo -e "${C_BLUE}Manual Setup:${C_RESET}"
echo ""
echo "  1. Enable pre-built China sandbox (recommended - fastest!):"
echo "     Add to .env:"
echo "       SANDBOX_IMAGE=enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest"
echo "       SANDBOX_SECURITY_OPT=seccomp=unconfined"
echo ""
echo "  2. Or configure Docker mirrors + build with npm/pip mirrors:"
echo "     ./scripts/setup-docker-mirrors.sh  # Docker (Linux only)"
echo "     Add to .env: NPM_REGISTRY=https://registry.npmmirror.com/"
echo "     Add to .env: PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple"
echo ""
echo "For more information, see: docs/deployment/CHINA_MIRRORS.md"
echo ""
