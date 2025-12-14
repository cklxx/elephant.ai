#!/bin/bash
###############################################################################
# Setup Docker Registry Mirrors for China
#
# This script configures Docker to use China-based registry mirrors for faster
# image pulls. It only works on Linux systems with systemd.
#
# For macOS/Windows, please configure Docker Desktop manually.
###############################################################################

set -euo pipefail

# Colors
readonly C_RED='\033[0;31m'
readonly C_GREEN='\033[0;32m'
readonly C_YELLOW='\033[1;33m'
readonly C_BLUE='\033[0;34m'
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

die() {
    log_error "$*"
    exit 1
}

# Check if running on Linux
if [[ "$(uname -s)" != "Linux" ]]; then
    log_warn "This script only works on Linux systems"
    echo ""
    echo "For macOS/Windows, please configure Docker Desktop manually:"
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
    exit 0
fi

# Check if Docker is installed
if ! command -v docker >/dev/null 2>&1; then
    die "Docker is not installed. Please install Docker first."
fi

# Check if running as root or with sudo
if [[ $EUID -ne 0 ]]; then
    log_warn "This script requires root privileges. Attempting to use sudo..."

    # Check if sudo is available
    if ! command -v sudo >/dev/null 2>&1; then
        die "sudo is not available. Please run this script as root."
    fi

    # Re-run the script with sudo
    exec sudo "$0" "$@"
fi

echo -e "${C_BLUE}Setting up Docker Registry Mirrors${C_RESET}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

DAEMON_JSON="/etc/docker/daemon.json"
BACKUP_FILE="${DAEMON_JSON}.backup.$(date +%Y%m%d_%H%M%S)"

# Backup existing configuration
if [[ -f "$DAEMON_JSON" ]]; then
    log_info "Backing up existing configuration to $BACKUP_FILE"
    cp "$DAEMON_JSON" "$BACKUP_FILE"
    log_success "Backup created"
else
    log_info "No existing configuration found, creating new one"
fi

# Create new configuration
log_info "Configuring Docker registry mirrors..."

# Tencent Cloud registry mirror (recommended for CN lightweight servers)
MIRRORS='[
    "https://mirror.ccs.tencentyun.com"
  ]'

# Create or update daemon.json
if [[ -f "$DAEMON_JSON" ]]; then
    # Parse existing config and add registry-mirrors
    if command -v jq >/dev/null 2>&1; then
        # Use jq if available for safer JSON manipulation
        jq --argjson mirrors "$MIRRORS" '. + {"registry-mirrors": $mirrors}' "$DAEMON_JSON" > "${DAEMON_JSON}.tmp"
        mv "${DAEMON_JSON}.tmp" "$DAEMON_JSON"
    else
        # Fallback: simple replacement (may not handle all edge cases)
        log_warn "jq not found, using basic configuration merge"
        cat > "$DAEMON_JSON" << EOF
{
  "registry-mirrors": $MIRRORS
}
EOF
    fi
else
    # Create new file
    mkdir -p /etc/docker
    cat > "$DAEMON_JSON" << EOF
{
  "registry-mirrors": $MIRRORS
}
EOF
fi

log_success "Configuration updated"

# Validate JSON
if command -v jq >/dev/null 2>&1; then
    if ! jq empty "$DAEMON_JSON" 2>/dev/null; then
        log_error "Invalid JSON configuration"
        if [[ -f "$BACKUP_FILE" ]]; then
            log_info "Restoring backup..."
            cp "$BACKUP_FILE" "$DAEMON_JSON"
        fi
        die "Configuration failed. Please check $DAEMON_JSON"
    fi
fi

# Reload and restart Docker
log_info "Reloading Docker daemon..."

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
    systemctl restart docker

    # Wait for Docker to be ready
    sleep 2

    if systemctl is-active --quiet docker; then
        log_success "Docker daemon restarted successfully"
    else
        log_error "Docker failed to restart"
        if [[ -f "$BACKUP_FILE" ]]; then
            log_info "Restoring backup..."
            cp "$BACKUP_FILE" "$DAEMON_JSON"
            systemctl daemon-reload
            systemctl restart docker
        fi
        die "Failed to restart Docker. Check logs with: journalctl -xeu docker"
    fi
else
    log_warn "systemctl not found. Please restart Docker manually."
fi

# Verify configuration
echo ""
log_info "Verifying configuration..."
echo ""

if docker info > /dev/null 2>&1; then
    echo -e "${C_GREEN}Registry Mirrors:${C_RESET}"
    docker info | grep -A 10 "Registry Mirrors" || log_warn "Could not find Registry Mirrors in docker info"
else
    log_error "Docker is not responding"
    die "Configuration may have failed"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${C_GREEN}✓ Docker registry mirrors configured successfully!${C_RESET}"
echo ""
echo "Configured mirrors:"
echo "  - https://mirror.ccs.tencentyun.com"
echo ""
echo "Backup saved to: $BACKUP_FILE"
echo ""
echo "You can now build and pull images with faster registry access:"
echo "  ./deploy.sh"
echo ""
