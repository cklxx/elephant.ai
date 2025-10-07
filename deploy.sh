#!/bin/bash
###############################################################################
# ALEX SSE Service - Ultra-Simplified Local Deployment
#
# Design Philosophy:
#   - One script, one purpose: local development
#   - Fast, reliable, debuggable
#   - Clear output, proper cleanup
#   - Port conflict detection
#
# Usage:
#   ./deploy.sh          # Start services
#   ./deploy.sh down     # Stop services
#   ./deploy.sh status   # Check status
#   ./deploy.sh logs     # Tail logs
###############################################################################

set -euo pipefail

# Configuration
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SERVER_PORT=8080
readonly WEB_PORT=3000
readonly PID_DIR="${SCRIPT_DIR}/.pids"
readonly LOG_DIR="${SCRIPT_DIR}/logs"
readonly SERVER_PID_FILE="${PID_DIR}/server.pid"
readonly WEB_PID_FILE="${PID_DIR}/web.pid"
readonly SERVER_LOG="${LOG_DIR}/server.log"
readonly WEB_LOG="${LOG_DIR}/web.log"

# Colors
readonly C_RED='\033[0;31m'
readonly C_GREEN='\033[0;32m'
readonly C_YELLOW='\033[1;33m'
readonly C_BLUE='\033[0;34m'
readonly C_CYAN='\033[0;36m'
readonly C_RESET='\033[0m'

###############################################################################
# Utilities
###############################################################################

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

banner() {
    echo -e "${C_CYAN}"
    cat << 'EOF'
    ___    __    _______  __  __
   /   |  / /   / ____/ |/ / / /
  / /| | / /   / __/  |   / / /
 / ___ |/ /___/ /___ /   | / /___
/_/  |_/_____/_____//_/|_|/_____/

AI Programming Agent - Local Dev
EOF
    echo -e "${C_RESET}"
}

die() {
    log_error "$*"
    exit 1
}

###############################################################################
# Process Management
###############################################################################

is_port_available() {
    local port=$1
    ! lsof -i ":$port" -sTCP:LISTEN -t >/dev/null 2>&1
}

kill_process_on_port() {
    local port=$1
    local pids
    pids=$(lsof -i ":$port" -sTCP:LISTEN -t 2>/dev/null || true)

    if [[ -n "$pids" ]]; then
        log_warn "Port $port is in use, killing processes: $pids"
        echo "$pids" | xargs kill -9 2>/dev/null || true
        sleep 1
    fi
}

read_pid() {
    local pid_file=$1
    if [[ -f "$pid_file" ]]; then
        cat "$pid_file"
    fi
}

is_process_running() {
    local pid=$1
    [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

stop_service() {
    local name=$1
    local pid_file=$2
    local pid

    pid=$(read_pid "$pid_file")

    if is_process_running "$pid"; then
        log_info "Stopping $name (PID: $pid)"
        kill "$pid" 2>/dev/null || true

        # Wait for graceful shutdown
        for i in {1..10}; do
            if ! is_process_running "$pid"; then
                log_success "$name stopped"
                rm -f "$pid_file"
                return 0
            fi
            sleep 0.5
        done

        # Force kill if still running
        log_warn "$name didn't stop gracefully, force killing"
        kill -9 "$pid" 2>/dev/null || true
        rm -f "$pid_file"
    elif [[ -f "$pid_file" ]]; then
        log_warn "$name PID file exists but process not running, cleaning up"
        rm -f "$pid_file"
    else
        log_info "$name is not running"
    fi
}

###############################################################################
# Health Checks
###############################################################################

wait_for_health() {
    local url=$1
    local name=$2
    local max_attempts=30

    log_info "Waiting for $name to be ready..."

    for i in $(seq 1 $max_attempts); do
        if curl -sf "$url" >/dev/null 2>&1; then
            log_success "$name is ready!"
            return 0
        fi

        if [[ $i -eq $max_attempts ]]; then
            log_error "$name failed to start within ${max_attempts}s"
            return 1
        fi

        sleep 1
    done
}

###############################################################################
# Environment Setup
###############################################################################

setup_environment() {
    # Create directories
    mkdir -p "$PID_DIR" "$LOG_DIR"

    # Check prerequisites
    command -v go >/dev/null 2>&1 || die "Go not installed"
    command -v node >/dev/null 2>&1 || die "Node.js not installed"
    command -v npm >/dev/null 2>&1 || die "npm not installed"

    # Check .env
    if [[ ! -f .env ]]; then
        log_warn ".env not found, creating default"
        cat > .env << 'EOF'
OPENAI_API_KEY=
OPENAI_BASE_URL=https://openrouter.ai/api/v1
ALEX_MODEL=anthropic/claude-3.5-sonnet
ALEX_VERBOSE=false
EOF
    fi

    # Source environment
    set -a
    source .env
    set +a

    if [[ -z "${OPENAI_API_KEY:-}" ]]; then
        log_warn "OPENAI_API_KEY not set in .env"
    else
        log_success "API key configured: ${OPENAI_API_KEY:0:12}..."
    fi

    # Verify .env.development exists
    if [[ ! -f web/.env.development ]]; then
        log_warn ".env.development not found, creating it"
        echo "NEXT_PUBLIC_API_URL=http://localhost:$SERVER_PORT" > web/.env.development
        log_success "Created web/.env.development"
    fi
}

###############################################################################
# Build & Deploy
###############################################################################

build_backend() {
    log_info "Building backend..."

    if ! make server-build 2>&1 | tee "$LOG_DIR/build.log"; then
        log_error "Backend build failed, check logs/build.log"
        tail -20 "$LOG_DIR/build.log"
        return 1
    fi

    if [[ ! -f ./alex-server ]]; then
        die "alex-server binary not found after build"
    fi

    log_success "Backend built: ./alex-server"
}

install_frontend_deps() {
    log_info "Installing frontend dependencies..."

    cd web
    if ! npm install 2>&1 | tee "$LOG_DIR/npm-install.log"; then
        log_error "npm install failed, check logs/npm-install.log"
        tail -20 "$LOG_DIR/npm-install.log"
        cd ..
        return 1
    fi
    cd ..

    log_success "Frontend dependencies installed"
}

start_backend() {
    # Ensure port is available
    if ! is_port_available "$SERVER_PORT"; then
        kill_process_on_port "$SERVER_PORT"
    fi

    log_info "Starting backend on :$SERVER_PORT..."

    # Rotate logs
    if [[ -f "$SERVER_LOG" ]]; then
        mv "$SERVER_LOG" "$SERVER_LOG.old"
    fi

    # Start server in background
    ./alex-server > "$SERVER_LOG" 2>&1 &
    local pid=$!
    echo "$pid" > "$SERVER_PID_FILE"

    log_success "Backend started (PID: $pid)"

    # Wait for health
    if ! wait_for_health "http://localhost:$SERVER_PORT/health" "Backend"; then
        log_error "Recent logs from $SERVER_LOG:"
        tail -30 "$SERVER_LOG"
        stop_service "Backend" "$SERVER_PID_FILE"
        return 1
    fi
}

start_frontend() {
    # Ensure port is available
    if ! is_port_available "$WEB_PORT"; then
        kill_process_on_port "$WEB_PORT"
    fi

    log_info "Starting frontend on :$WEB_PORT..."

    # Rotate logs
    if [[ -f "$WEB_LOG" ]]; then
        mv "$WEB_LOG" "$WEB_LOG.old"
    fi

    # Clear Next.js cache to avoid webpack issues
    rm -rf web/.next

    # Start frontend in background
    cd web
    PORT=$WEB_PORT npm run dev > "$WEB_LOG" 2>&1 &
    local pid=$!
    cd ..
    echo "$pid" > "$WEB_PID_FILE"

    log_success "Frontend started (PID: $pid)"

    # Give it a moment to start
    sleep 2
}

###############################################################################
# Commands
###############################################################################

cmd_start() {
    banner

    # Stop any existing services
    stop_service "Backend" "$SERVER_PID_FILE"
    stop_service "Frontend" "$WEB_PID_FILE"

    # Setup
    setup_environment

    # Build & start
    build_backend || die "Backend build failed"
    install_frontend_deps || die "Frontend dependency installation failed"
    start_backend || die "Backend failed to start"
    start_frontend || die "Frontend failed to start"

    # Success message
    echo ""
    echo -e "${C_GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${C_RESET}"
    echo -e "${C_GREEN}  ALEX SSE Service Running${C_RESET}"
    echo -e "${C_GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${C_RESET}"
    echo ""
    echo -e "  ${C_CYAN}Web UI:${C_RESET}  http://localhost:$WEB_PORT"
    echo -e "  ${C_CYAN}API:${C_RESET}     http://localhost:$SERVER_PORT"
    echo -e "  ${C_CYAN}Health:${C_RESET}  http://localhost:$SERVER_PORT/health"
    echo ""
    echo -e "${C_YELLOW}Commands:${C_RESET}"
    echo -e "  ./deploy.sh logs     # Tail logs"
    echo -e "  ./deploy.sh status   # Check status"
    echo -e "  ./deploy.sh down     # Stop services"
    echo ""
}

cmd_stop() {
    log_info "Stopping all services..."
    echo ""

    stop_service "Backend" "$SERVER_PID_FILE"
    stop_service "Frontend" "$WEB_PID_FILE"

    # Clean up port bindings
    kill_process_on_port "$SERVER_PORT" || true
    kill_process_on_port "$WEB_PORT" || true

    echo ""
    log_success "All services stopped"
}

cmd_status() {
    echo ""
    echo -e "${C_CYAN}Service Status${C_RESET}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Backend
    local server_pid
    server_pid=$(read_pid "$SERVER_PID_FILE")
    if is_process_running "$server_pid"; then
        echo -e "${C_GREEN}✓${C_RESET} Backend:   Running (PID: $server_pid)"
        if curl -sf "http://localhost:$SERVER_PORT/health" >/dev/null 2>&1; then
            echo -e "             Health check: ${C_GREEN}OK${C_RESET}"
        else
            echo -e "             Health check: ${C_RED}FAILED${C_RESET}"
        fi
    else
        echo -e "${C_RED}✗${C_RESET} Backend:   Not running"
    fi

    echo ""

    # Frontend
    local web_pid
    web_pid=$(read_pid "$WEB_PID_FILE")
    if is_process_running "$web_pid"; then
        echo -e "${C_GREEN}✓${C_RESET} Frontend:  Running (PID: $web_pid)"
        if curl -sf "http://localhost:$WEB_PORT" >/dev/null 2>&1; then
            echo -e "             Accessible: ${C_GREEN}YES${C_RESET}"
        else
            echo -e "             Accessible: ${C_YELLOW}STARTING${C_RESET}"
        fi
    else
        echo -e "${C_RED}✗${C_RESET} Frontend:  Not running"
    fi

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # Port status
    echo ""
    echo -e "${C_CYAN}Port Status${C_RESET}"
    echo "  :$SERVER_PORT - $(lsof -i ":$SERVER_PORT" -sTCP:LISTEN -t >/dev/null 2>&1 && echo -e "${C_GREEN}IN USE${C_RESET}" || echo -e "${C_YELLOW}FREE${C_RESET}")"
    echo "  :$WEB_PORT - $(lsof -i ":$WEB_PORT" -sTCP:LISTEN -t >/dev/null 2>&1 && echo -e "${C_GREEN}IN USE${C_RESET}" || echo -e "${C_YELLOW}FREE${C_RESET}")"
    echo ""
}

cmd_logs() {
    local service=${1:-all}

    case $service in
        server|backend)
            log_info "Tailing backend logs (Ctrl+C to stop)"
            tail -f "$SERVER_LOG"
            ;;
        web|frontend)
            log_info "Tailing frontend logs (Ctrl+C to stop)"
            tail -f "$WEB_LOG"
            ;;
        all|*)
            log_info "Tailing all logs (Ctrl+C to stop)"
            tail -f "$SERVER_LOG" "$WEB_LOG"
            ;;
    esac
}

cmd_help() {
    cat << EOF

${C_CYAN}ALEX SSE Service - Local Deployment${C_RESET}

${C_YELLOW}Usage:${C_RESET}
  ./deploy.sh [command]

${C_YELLOW}Commands:${C_RESET}
  ${C_GREEN}start${C_RESET}              Start all services (default)
  ${C_GREEN}down, stop${C_RESET}         Stop all services
  ${C_GREEN}status${C_RESET}             Show service status
  ${C_GREEN}logs [service]${C_RESET}     Tail logs (all/server/web)
  ${C_GREEN}help${C_RESET}               Show this help

${C_YELLOW}Examples:${C_RESET}
  ./deploy.sh              # Start everything
  ./deploy.sh status       # Check status
  ./deploy.sh logs server  # Tail backend logs
  ./deploy.sh down         # Stop all

${C_YELLOW}Log Files:${C_RESET}
  Backend:  logs/server.log
  Frontend: logs/web.log
  Build:    logs/build.log

${C_YELLOW}Environment:${C_RESET}
  Edit .env to configure API keys and settings

EOF
}

###############################################################################
# Main
###############################################################################

main() {
    cd "$SCRIPT_DIR"

    local cmd=${1:-start}

    case $cmd in
        start|up|run)
            cmd_start
            ;;
        stop|down|kill)
            cmd_stop
            ;;
        status|ps)
            cmd_status
            ;;
        logs|log|tail)
            cmd_logs "${2:-all}"
            ;;
        help|-h|--help)
            cmd_help
            ;;
        *)
            log_error "Unknown command: $cmd"
            cmd_help
            exit 1
            ;;
    esac
}

main "$@"
