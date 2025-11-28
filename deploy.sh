#!/bin/bash
###############################################################################
# ALEX SSE Service - Deployment Helper
#
# Design Philosophy:
#   - Production-first stack
#   - Fast, reliable, debuggable
#   - Clear output, proper cleanup
#   - Port conflict detection
#
# Usage:
#   ./deploy.sh          # Start production stack behind nginx on :80 (default)
#   ./deploy.sh pro down # Stop production stack
#   ./deploy.sh docker   # Manage docker-compose helpers
###############################################################################

set -euo pipefail

# Configuration
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SERVER_PORT=8080
readonly WEB_PORT=3000
readonly SANDBOX_DEFAULT_URL="http://localhost:8090"
readonly SANDBOX_COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
readonly SANDBOX_SERVICE_NAME="sandbox"
readonly SANDBOX_CONTAINER_NAME="alex-sandbox"
readonly SANDBOX_CHINA_IMAGE="enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest"
readonly PID_DIR="${SCRIPT_DIR}/.pids"
readonly LOG_DIR="${SCRIPT_DIR}/logs"
readonly SERVER_PID_FILE="${PID_DIR}/server.pid"
readonly WEB_PID_FILE="${PID_DIR}/web.pid"
readonly SERVER_LOG="${LOG_DIR}/server.log"
readonly WEB_LOG="${LOG_DIR}/web.log"
readonly BIN_DIR="${SCRIPT_DIR}/.bin"
readonly DOCKER_COMPOSE_BIN="${BIN_DIR}/docker-compose"
readonly ALEX_CONFIG_PATH="${ALEX_CONFIG_PATH:-$HOME/.alex-config.json}"
source "${SCRIPT_DIR}/scripts/lib/deploy_common.sh"

COMPOSE_CORE_VARS=()
COMPOSE_MISSING_VARS=()

readonly DEFAULT_DOCKER_COMPOSE_VERSION="v2.29.7"
DOCKER_COMPOSE_VERSION="${DOCKER_COMPOSE_VERSION:-${DEFAULT_DOCKER_COMPOSE_VERSION}}"
DOCKER_COMPOSE_CMD=()

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

print_cn_mirrors() {
    log_info "Using China mirrors for deployment:"
    [[ -n "${DOCKER_REGISTRY_MIRROR:-}" ]] && log_info "  Docker mirror: ${DOCKER_REGISTRY_MIRROR}"
    [[ -n "${SANDBOX_IMAGE:-}" ]] && log_info "  Sandbox image: ${SANDBOX_IMAGE}"
    [[ -n "${NPM_CONFIG_REGISTRY:-}" ]] && log_info "  npm registry: ${NPM_CONFIG_REGISTRY}"
    [[ -n "${GOPROXY:-}" ]] && log_info "  Go proxy: ${GOPROXY}"
    [[ -n "${GOSUMDB:-}" ]] && log_info "  Go checksum DB: ${GOSUMDB}"
}

banner() {
    local line
    line="$(printf '%*s' 80 '' | tr ' ' '─')"
    echo -e "${C_CYAN}${line}${C_RESET}"
}

die() {
    log_error "$*"
    exit 1
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
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
        if curl -sf --noproxy '*' "$url" >/dev/null 2>&1; then
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

wait_for_docker_health() {
    local container_name=$1
    local max_attempts=${2:-30}

    log_info "Waiting for $container_name to be healthy..."

    for i in $(seq 1 $max_attempts); do
        local health_status
        health_status=$(docker inspect --format='{{.State.Health.Status}}' "$container_name" 2>/dev/null || echo "none")

        case "$health_status" in
            healthy)
                log_success "$container_name is healthy!"
                return 0
                ;;
            none)
                # Container has no health check, check if it's running
                if docker ps --filter "name=^${container_name}$" --format '{{.Names}}' | grep -q "^${container_name}$"; then
                    log_success "$container_name is running!"
                    return 0
                fi
                ;;
            starting)
                # Still starting, continue waiting
                ;;
            unhealthy)
                log_error "$container_name is unhealthy"
                return 1
                ;;
            *)
                # Container doesn't exist or is not running
                if [[ $i -eq $max_attempts ]]; then
                    log_error "$container_name failed to start within ${max_attempts}s"
                    return 1
                fi
                ;;
        esac

        sleep 1
    done

    log_error "$container_name failed to become healthy within ${max_attempts}s"
    return 1
}

###############################################################################
# Environment Setup
###############################################################################

generate_auth_secret() {
    if command_exists python3; then
        python3 - <<'PY'
import secrets
print(secrets.token_hex(32))
PY
        return
    fi

    if command_exists python; then
        python - <<'PY'
import secrets
print(secrets.token_hex(32))
PY
        return
    fi

    if command_exists openssl; then
        openssl rand -hex 32
        return
    fi

    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
}

append_env_var_if_missing() {
    local key=$1
    local value=$2
    if ! grep -q "^${key}=" .env 2>/dev/null; then
        printf "\n%s=%s\n" "$key" "$value" >> .env
        log_warn "Appended default ${key} to .env"
    fi
}

hydrate_env_from_config() {
    deploy_config::resolve_var OPENAI_API_KEY '.api_key // .models.basic.api_key // .models.reasoning.api_key' >/dev/null || true
    deploy_config::resolve_var OPENAI_BASE_URL '.base_url // .models.basic.base_url // .models.reasoning.base_url' >/dev/null || true
    deploy_config::resolve_var ALEX_MODEL '.llm_model // .model // .models.basic.model' >/dev/null || true
    deploy_config::resolve_var ALEX_SANDBOX_BASE_URL '.sandbox_base_url' >/dev/null || true
    deploy_config::resolve_var AUTH_JWT_SECRET '.auth.jwtSecret' >/dev/null || true
    deploy_config::resolve_var AUTH_DATABASE_URL '.auth.databaseUrl' >/dev/null || true
    deploy_config::resolve_var NEXT_PUBLIC_API_URL '.web.apiUrl' >/dev/null || true
}

ensure_api_url_default() {
    local default_value="$1"
    local mode="$2"  # local|nginx

    local source="default"
    local resolved_source
    if resolved_source=$(deploy_config::resolve_var NEXT_PUBLIC_API_URL '.web.apiUrl' "$default_value" 2>/dev/null); then
        source="$resolved_source"
    else
        export NEXT_PUBLIC_API_URL="$default_value"
    fi

    local resolved_api_url="${NEXT_PUBLIC_API_URL:-}" # Prevent nounset errors before hydration

    if [[ "$mode" == "local" ]]; then
        if [[ "$resolved_api_url" == "auto" ]]; then
            log_warn "NEXT_PUBLIC_API_URL=auto relies on nginx to proxy API traffic back to :${SERVER_PORT}"
        else
            log_success "NEXT_PUBLIC_API_URL=${resolved_api_url} (${source})"
        fi
    else
        if [[ "$resolved_api_url" != "auto" ]]; then
            log_warn "NEXT_PUBLIC_API_URL='${resolved_api_url}'. nginx already proxies all exits, so 'auto' keeps the same-origin flow."
        fi
    fi
}

ensure_auth_env_defaults() {
    local default_redirect="http://localhost:${SERVER_PORT}"

    if ! grep -q '^AUTH_JWT_SECRET=' .env 2>/dev/null; then
        local secret
        secret=$(generate_auth_secret)
        append_env_var_if_missing "AUTH_JWT_SECRET" "$secret"
    fi

    append_env_var_if_missing "AUTH_ACCESS_TOKEN_TTL_MINUTES" "15"
    append_env_var_if_missing "AUTH_REFRESH_TOKEN_TTL_DAYS" "30"
    append_env_var_if_missing "AUTH_REDIRECT_BASE_URL" "$default_redirect"
}

ensure_web_env_file() {
    local web_env_file="web/.env.development"
    local resolved_api="${NEXT_PUBLIC_API_URL:-http://localhost:${SERVER_PORT}}"

    if [[ ! -f "$web_env_file" ]]; then
        log_warn ".env.development not found, creating it"
        printf "NEXT_PUBLIC_API_URL=%s\n" "$resolved_api" > "$web_env_file"
        log_success "Created web/.env.development"
        return
    fi

    if ! grep -q '^NEXT_PUBLIC_API_URL=' "$web_env_file"; then
        printf "NEXT_PUBLIC_API_URL=%s\n" "$resolved_api" >> "$web_env_file"
        log_warn "NEXT_PUBLIC_API_URL was missing from web/.env.development, appended ${resolved_api}"
        return
    fi

    local existing
    existing=$(grep '^NEXT_PUBLIC_API_URL=' "$web_env_file" | tail -n1 | cut -d= -f2- || true)
    if [[ -n "$existing" && "$existing" != "$resolved_api" ]]; then
        log_warn "web/.env.development NEXT_PUBLIC_API_URL=${existing}, current session resolved ${resolved_api}. Update the file if you need same-origin behavior."
    fi
}

compose_reset_var_tracking() {
    COMPOSE_CORE_VARS=()
    COMPOSE_MISSING_VARS=()
}

source_root_env_if_present() {
    if [[ -f .env ]]; then
        set -a
        # shellcheck disable=SC1091
        source .env
        set +a
    fi
}

compose_record_core_var() {
    local name="$1"
    local source="$2"
    COMPOSE_CORE_VARS+=("${name}=${source}")
}

compose_resolve_required_var() {
    local name="$1"
    local expr="$2"
    local default_value="${3:-}"
    local resolved_source

    if resolved_source=$(deploy_config::resolve_var "$name" "$expr" "$default_value" 2>/dev/null); then
        compose_record_core_var "$name" "$resolved_source"
        return 0
    fi

    if [[ -z "${!name:-}" ]]; then
        COMPOSE_MISSING_VARS+=("$name")
        return 1
    fi

    compose_record_core_var "$name" "env"
}

compose_resolve_optional_var() {
    local name="$1"
    local expr="${2:-}"
    local default_value="${3:-}"
    deploy_config::resolve_var "$name" "$expr" "$default_value" >/dev/null || true
}

compose_validate_core_vars() {
    if [[ ${#COMPOSE_MISSING_VARS[@]} -eq 0 ]]; then
        return 0
    fi

    log_error "Missing required variables: ${COMPOSE_MISSING_VARS[*]}"
    cat <<'EOF'
Core secrets must be provided either via environment variables or ~/.alex-config.json
with structure similar to:
{
  "api_key": "sk-...",
  "auth": {
    "jwtSecret": "super-secret",
    "databaseUrl": "postgres://user:pass@host:5432/alex?sslmode=disable"
  }
}
EOF
    exit 1
}

compose_show_summary() {
    if [[ ${#COMPOSE_CORE_VARS[@]} -eq 0 ]]; then
        return
    fi

    log_info "Resolved variables:"
    for entry in "${COMPOSE_CORE_VARS[@]}"; do
        IFS='=' read -r name source <<<"$entry"
        printf "  - %s (%s)\n" "$name" "$source"
    done
    [[ -n "${OPENAI_BASE_URL:-}" ]] && printf "  - OPENAI_BASE_URL\n"
    [[ -n "${ALEX_MODEL:-}" ]] && printf "  - ALEX_MODEL\n"
    [[ -n "${ALEX_SANDBOX_BASE_URL:-}" ]] && printf "  - ALEX_SANDBOX_BASE_URL\n"
}

prepare_compose_environment() {
    compose_reset_var_tracking
    source_root_env_if_present

    compose_resolve_required_var OPENAI_API_KEY '.api_key // .models.basic.api_key // .models.reasoning.api_key' || true
    compose_resolve_optional_var AUTH_JWT_SECRET '.auth.jwtSecret'
    compose_resolve_optional_var AUTH_DATABASE_URL '.auth.databaseUrl'
    compose_resolve_optional_var NEXT_PUBLIC_API_URL '.web.apiUrl' auto

    compose_resolve_optional_var OPENAI_BASE_URL '.base_url // .models.basic.base_url // .models.reasoning.base_url'
    compose_resolve_optional_var ALEX_MODEL '.llm_model // .model // .models.basic.model'
    compose_resolve_optional_var ALEX_SANDBOX_BASE_URL '.sandbox_base_url'

    if [[ -z "${NEXT_PUBLIC_API_URL:-}" ]]; then
        export NEXT_PUBLIC_API_URL=auto
    fi

    if [[ "${NEXT_PUBLIC_API_URL}" == "auto" ]]; then
        log_info "NEXT_PUBLIC_API_URL=auto (nginx same-origin default)"
    else
        log_warn "NEXT_PUBLIC_API_URL='${NEXT_PUBLIC_API_URL}'. nginx already proxies all exits, so 'auto' keeps the same-origin flow."
    fi

    if [[ -z "${AUTH_JWT_SECRET:-}" || -z "${AUTH_DATABASE_URL:-}" ]]; then
        log_warn "Auth DB/secret not set; login flows stay disabled (set AUTH_JWT_SECRET and AUTH_DATABASE_URL to enable)."
    fi

    if [[ -z "${ALEX_SANDBOX_BASE_URL:-}" ]]; then
        export ALEX_SANDBOX_BASE_URL=http://alex-sandbox:8080
    fi

    compose_validate_core_vars
}

apply_auth_migrations() {
    local migration_file="${SCRIPT_DIR}/migrations/auth/001_init.sql"

    if [[ "${SKIP_AUTH_MIGRATIONS:-false}" == "true" ]]; then
        log_warn "Skipping auth migrations (SKIP_AUTH_MIGRATIONS=true)"
        return
    fi

    if [[ -z "${AUTH_DATABASE_URL:-}" ]]; then
        return
    fi

    if [[ ! -f "$migration_file" ]]; then
        log_warn "Migration file not found: $migration_file"
        return
    fi

    if ! command_exists psql; then
        log_warn "psql is not installed; skipping automatic auth migrations"
        return
    fi

    log_info "Applying auth migrations"
    if ! psql "$AUTH_DATABASE_URL" -f "$migration_file" >/dev/null; then
        log_warn "Failed to run auth migrations. Ensure the database is reachable and initialized."
    else
        log_success "Auth migrations applied"
    fi
}

setup_environment() {
    # Create directories
    mkdir -p "$PID_DIR" "$LOG_DIR"

    # Check prerequisites
    command -v go >/dev/null 2>&1 || die "Go not installed"
    command -v node >/dev/null 2>&1 || die "Node.js not installed"
    command -v npm >/dev/null 2>&1 || die "npm not installed"

    # Check .env
    local default_auth_secret
    if [[ ! -f .env ]]; then
        log_warn ".env not found, creating default"
        default_auth_secret=$(generate_auth_secret)
        cat > .env << EOF
OPENAI_API_KEY=
OPENAI_BASE_URL=https://openrouter.ai/api/v1
ALEX_MODEL=anthropic/claude-3.5-sonnet
ALEX_VERBOSE=false
ALEX_SANDBOX_BASE_URL=http://localhost:8090
NEXT_PUBLIC_API_URL=http://localhost:${SERVER_PORT}

# Authentication defaults for local development
AUTH_JWT_SECRET=${default_auth_secret}
AUTH_ACCESS_TOKEN_TTL_MINUTES=15
AUTH_REFRESH_TOKEN_TTL_DAYS=30
AUTH_REDIRECT_BASE_URL=http://localhost:${SERVER_PORT}

# China Mirror Configuration (uncomment to enable)
# Use pre-built China sandbox image (recommended for China - no build required!)
# SANDBOX_IMAGE=enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest
# SANDBOX_SECURITY_OPT=seccomp=unconfined

# Or use custom npm/pip mirrors for building (slower than pre-built image)
# NPM_REGISTRY=https://registry.npmmirror.com/
# PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
EOF
    fi

    ensure_auth_env_defaults

    # Source environment
    set -a
    source .env
    set +a

    hydrate_env_from_config
    ensure_api_url_default "http://localhost:${SERVER_PORT}" "local"
    ensure_web_env_file

    if [[ -z "${OPENAI_API_KEY:-}" ]]; then
        log_warn "OPENAI_API_KEY not set in .env"
    else
        log_success "API key configured: ${OPENAI_API_KEY:0:12}..."
    fi

    if [[ -z "${ALEX_SANDBOX_BASE_URL:-}" ]]; then
        ALEX_SANDBOX_BASE_URL="${SANDBOX_DEFAULT_URL}"
        export ALEX_SANDBOX_BASE_URL
        log_warn "ALEX_SANDBOX_BASE_URL not set, defaulting to ${ALEX_SANDBOX_BASE_URL}"
    else
        log_success "Sandbox URL configured: ${ALEX_SANDBOX_BASE_URL}"
    fi

    # Verify .env.development exists
}

ensure_local_auth_db() {
    if [[ "${SKIP_LOCAL_AUTH_DB:-0}" == "1" ]]; then
        log_warn "Skipping local auth database setup (SKIP_LOCAL_AUTH_DB=1)"
        return 0
    fi

    local script_path="${SCRIPT_DIR}/scripts/setup_local_auth_db.sh"
    if [[ ! -x "$script_path" ]]; then
        log_warn "Auth DB setup script not found or not executable at $script_path"
        return 0
    fi

    if ! command_exists docker; then
        log_warn "Docker not available; skipping local auth database setup"
        return 0
    fi

    if ! command_exists psql; then
        log_warn "psql not available; skipping local auth database setup"
        return 0
    fi

    log_info "Ensuring local auth database is running..."
    if "$script_path"; then
        log_success "Local auth database ready"
    else
        die "Failed to initialize local auth database"
    fi
}

ensure_docker_compose() {
    if [[ ${#DOCKER_COMPOSE_CMD[@]} -gt 0 ]]; then
        return
    fi

    if ! command_exists docker; then
        die "Docker is required for docker-compose deployments"
    fi

    if docker compose version >/dev/null 2>&1; then
        DOCKER_COMPOSE_CMD=(docker compose)
        return
    fi

    if command_exists docker-compose; then
        DOCKER_COMPOSE_CMD=("$(command -v docker-compose)")
        return
    fi

    if [[ -x "$DOCKER_COMPOSE_BIN" ]]; then
        DOCKER_COMPOSE_CMD=("$DOCKER_COMPOSE_BIN")
        return
    fi

    download_docker_compose
}

download_docker_compose() {
    local os
    local arch
    local url

    case "$(uname -s)" in
        Linux*)
            os="Linux"
            ;;
        Darwin*)
            os="Darwin"
            ;;
        *)
            die "Unsupported OS for automatic docker-compose installation"
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)
            arch="x86_64"
            ;;
        arm64|aarch64)
            arch="aarch64"
            ;;
        *)
            die "Unsupported architecture for automatic docker-compose installation"
            ;;
    esac

    url="https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-${os}-${arch}"

    log_info "Downloading docker-compose ${DOCKER_COMPOSE_VERSION}..."
    mkdir -p "$BIN_DIR"

    if command_exists curl; then
        if ! curl -fsSL "$url" -o "$DOCKER_COMPOSE_BIN"; then
            die "Failed to download docker-compose with curl from $url"
        fi
    elif command_exists wget; then
        if ! wget -q "$url" -O "$DOCKER_COMPOSE_BIN"; then
            die "Failed to download docker-compose with wget from $url"
        fi
    else
        die "Neither curl nor wget is available to download docker-compose"
    fi

    chmod +x "$DOCKER_COMPOSE_BIN"

    if ! "$DOCKER_COMPOSE_BIN" version >/dev/null 2>&1; then
        die "Downloaded docker-compose binary failed to execute"
    fi

    log_success "docker-compose installed at $DOCKER_COMPOSE_BIN"
    DOCKER_COMPOSE_CMD=("$DOCKER_COMPOSE_BIN")
}

run_docker_compose() {
    ensure_docker_compose
    "${DOCKER_COMPOSE_CMD[@]}" "$@"
}

is_local_sandbox_url() {
    local url=$1

    [[ -z "$url" ]] && return 1

    case "$url" in
        http://localhost*|https://localhost*|http://127.0.0.1*|https://127.0.0.1*|http://0.0.0.0*|https://0.0.0.0*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

start_sandbox() {
    local base_url=${ALEX_SANDBOX_BASE_URL:-$SANDBOX_DEFAULT_URL}

    if ! is_local_sandbox_url "$base_url"; then
        log_info "ALEX_SANDBOX_BASE_URL=${base_url} is not local; skipping sandbox container startup"
        return 0
    fi

    if [[ ! -f "$SANDBOX_COMPOSE_FILE" ]]; then
        die "Sandbox compose file not found at $SANDBOX_COMPOSE_FILE"
    fi

    if ! command_exists docker; then
        die "Docker is required to run the sandbox locally. Install Docker or point ALEX_SANDBOX_BASE_URL to a remote sandbox."
    fi

    log_info "Starting sandbox container via docker-compose..."

    # Check if using China sandbox image
    if [[ -n "${SANDBOX_IMAGE:-}" ]]; then
        log_info "Using pre-built sandbox image: $SANDBOX_IMAGE"
        log_success "No build required - pulling image from registry"
    else
        # Show mirror configuration if set
        if [[ -n "${NPM_REGISTRY:-}" ]] || [[ -n "${PIP_INDEX_URL:-}" ]]; then
            log_info "Building sandbox with custom mirrors:"
            [[ -n "${NPM_REGISTRY:-}" ]] && log_info "  NPM: ${NPM_REGISTRY}"
            [[ -n "${PIP_INDEX_URL:-}" ]] && log_info "  PIP: ${PIP_INDEX_URL}"
        else
            log_info "Building sandbox from Dockerfile.sandbox..."
        fi
    fi

    local existing_id
    existing_id=$(docker ps -aq --filter "name=^/${SANDBOX_CONTAINER_NAME}$" 2>/dev/null || true)

    if [[ -n "$existing_id" ]]; then
        local is_running
        is_running=$(docker inspect --format='{{.State.Running}}' "$existing_id" 2>/dev/null || echo "false")

        if [[ "$is_running" == "true" ]]; then
            log_info "Sandbox container already running (ID: ${existing_id}), reusing existing instance"
            if wait_for_docker_health "$SANDBOX_CONTAINER_NAME" 10; then
                log_success "Sandbox running at $base_url"
                return 0
            fi

            log_warn "Existing sandbox container is running but unhealthy, attempting restart"
            if docker restart "$existing_id" >/dev/null 2>&1; then
                if wait_for_docker_health "$SANDBOX_CONTAINER_NAME" 30; then
                    log_success "Sandbox running at $base_url"
                    return 0
                fi
            else
                log_warn "Failed to restart existing sandbox container, will recreate"
            fi
        else
            log_info "Sandbox container exists but is stopped, starting existing instance (${existing_id})"
            if docker start "$existing_id" >/dev/null 2>&1; then
                if wait_for_docker_health "$SANDBOX_CONTAINER_NAME" 30; then
                    log_success "Sandbox running at $base_url"
                    return 0
                fi
            else
                log_warn "Failed to start existing sandbox container, will recreate"
            fi
        fi

        log_warn "Removing unhealthy sandbox container (${existing_id}) before recreation"
        docker rm -f "$existing_id" >/dev/null 2>&1 || true
    fi

    run_docker_compose -f "$SANDBOX_COMPOSE_FILE" up -d "$SANDBOX_SERVICE_NAME"

    # Wait for container to be healthy using Docker's built-in health check
    if ! wait_for_docker_health "$SANDBOX_CONTAINER_NAME" 30; then
        log_error "Sandbox failed to become healthy"
        run_docker_compose -f "$SANDBOX_COMPOSE_FILE" logs --tail 50 "$SANDBOX_SERVICE_NAME" || true
        return 1
    fi

    log_success "Sandbox running at $base_url"
}

stop_sandbox() {
    local base_url=${ALEX_SANDBOX_BASE_URL:-$SANDBOX_DEFAULT_URL}

    if ! is_local_sandbox_url "$base_url"; then
        return 0
    fi

    if ! command_exists docker; then
        return 0
    fi

    if ! docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^${SANDBOX_CONTAINER_NAME}$"; then
        return 0
    fi

    log_info "Stopping sandbox container..."
    run_docker_compose stop "$SANDBOX_SERVICE_NAME" >/dev/null 2>&1 || true
    run_docker_compose rm -f "$SANDBOX_SERVICE_NAME" >/dev/null 2>&1 || true
    log_success "Sandbox container stopped"
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

run_go_tests() {
    log_info "Running Go tests..."

    if ! make test 2>&1 | tee "$LOG_DIR/go-test.log"; then
        log_error "Go tests failed, check logs/go-test.log"
        tail -20 "$LOG_DIR/go-test.log"
        return 1
    fi

    log_success "Go tests passed"
}

run_web_unit_tests() {
    log_info "Running web unit tests..."

    local api_url="${NEXT_PUBLIC_API_URL:-http://localhost:${SERVER_PORT}}"

    if ! NEXT_PUBLIC_API_URL="$api_url" npm --prefix web test -- --run 2>&1 | tee "$LOG_DIR/web-test.log"; then
        log_error "Web unit tests failed, check logs/web-test.log"
        tail -20 "$LOG_DIR/web-test.log"
        return 1
    fi

    log_success "Web unit tests passed"
}

ensure_playwright_browsers() {
    local cache_dir="${SCRIPT_DIR}/web/node_modules/.cache/ms-playwright"

    if [[ -d "$cache_dir" ]]; then
        return 0
    fi

    log_info "Installing Playwright browsers (first run may take a while)..."

    if ! (cd web && npx playwright install 2>&1 | tee "$LOG_DIR/playwright-install.log"); then
        log_error "Playwright browser install failed, check logs/playwright-install.log"
        tail -20 "$LOG_DIR/playwright-install.log"
        return 1
    fi

    log_success "Playwright browsers installed"
}

run_web_e2e_tests() {
    log_info "Running web end-to-end tests..."

    ensure_playwright_browsers || return 1

    local api_url="${NEXT_PUBLIC_API_URL:-http://localhost:${SERVER_PORT}}"

    if ! NEXT_PUBLIC_API_URL="$api_url" npm --prefix web run e2e 2>&1 | tee "$LOG_DIR/web-e2e.log"; then
        log_error "Web end-to-end tests failed, check logs/web-e2e.log"
        tail -20 "$LOG_DIR/web-e2e.log"
        return 1
    fi

    log_success "Web end-to-end tests passed"
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

    # Start server in background with deploy mode flag
    ALEX_SERVER_MODE=deploy ./alex-server > "$SERVER_LOG" 2>&1 &
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

    # Determine API origin even if NEXT_PUBLIC_API_URL hasn't been hydrated yet
    local api_url="${NEXT_PUBLIC_API_URL:-http://localhost:${SERVER_PORT}}"

    # Start frontend in background
    cd web
    NEXT_PUBLIC_API_URL="$api_url" PORT=$WEB_PORT npm run dev > "$WEB_LOG" 2>&1 &
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
    ensure_local_auth_db

    # Build & start
    build_backend || die "Backend build failed"
    install_frontend_deps || die "Frontend dependency installation failed"
    start_sandbox || die "Sandbox failed to start"
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

cmd_test() {
    banner

    setup_environment

    run_go_tests || die "Go tests failed"
    install_frontend_deps || die "Frontend dependency installation failed"
    run_web_unit_tests || die "Web unit tests failed"
    run_web_e2e_tests || die "Web end-to-end tests failed"

    log_success "All tests passed"
}

cmd_stop() {
    log_info "Stopping all services..."
    echo ""

    stop_service "Backend" "$SERVER_PID_FILE"
    stop_service "Frontend" "$WEB_PID_FILE"
    stop_sandbox

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
        if curl -sf --noproxy '*' "http://localhost:$SERVER_PORT/health" >/dev/null 2>&1; then
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
        if curl -sf --noproxy '*' "http://localhost:$WEB_PORT" >/dev/null 2>&1; then
            echo -e "             Accessible: ${C_GREEN}YES${C_RESET}"
        else
            echo -e "             Accessible: ${C_YELLOW}STARTING${C_RESET}"
        fi
    else
        echo -e "${C_RED}✗${C_RESET} Frontend:  Not running"
    fi

    echo ""

    # Sandbox
    local sandbox_base=${ALEX_SANDBOX_BASE_URL:-$SANDBOX_DEFAULT_URL}
    if ! is_local_sandbox_url "$sandbox_base"; then
        echo -e "${C_YELLOW}⚠${C_RESET} Sandbox:   Using external sandbox at ${sandbox_base}"
    else
        local sandbox_status
        local sandbox_container
        sandbox_container=$(
            if command_exists docker; then
                docker ps --filter "name=${SANDBOX_CONTAINER_NAME}" --format '{{.Names}}' 2>/dev/null | head -n1 || true
            fi
        )
        if [[ -n "$sandbox_container" ]]; then
            sandbox_status=$(docker ps --filter "name=${SANDBOX_CONTAINER_NAME}" --format '{{.Status}}' 2>/dev/null | head -n1 || true)
            echo -e "${C_GREEN}✓${C_RESET} Sandbox:   Running (${sandbox_status})"
            echo -e "             URL: ${sandbox_base}"
        else
            local sandbox_known
            sandbox_known=$(
                if command_exists docker; then
                    docker ps -a --filter "name=${SANDBOX_CONTAINER_NAME}" --format '{{.Status}}' 2>/dev/null | head -n1 || true
                fi
            )
            if [[ -n "$sandbox_known" ]]; then
                echo -e "${C_YELLOW}⚠${C_RESET} Sandbox:   Stopped (${sandbox_known})"
                echo -e "             URL: ${sandbox_base}"
            else
                if command_exists docker; then
                    echo -e "${C_RED}✗${C_RESET} Sandbox:   Not running"
                else
                    echo -e "${C_YELLOW}⚠${C_RESET} Sandbox:   Docker not available"
                fi
            fi
        fi
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
        sandbox)
            local base_url=${ALEX_SANDBOX_BASE_URL:-$SANDBOX_DEFAULT_URL}
            if ! is_local_sandbox_url "$base_url"; then
                log_warn "Sandbox logs are managed externally at ${base_url}"
                return 0
            fi

            if command_exists docker; then
                log_info "Tailing sandbox logs (Ctrl+C to stop)"
                run_docker_compose -f "$SANDBOX_COMPOSE_FILE" logs -f "$SANDBOX_SERVICE_NAME"
            else
                log_error "Docker is not available; cannot tail sandbox logs"
                return 1
            fi
            ;;
        all|*)
            log_info "Tailing all logs (Ctrl+C to stop)"
            tail -f "$SERVER_LOG" "$WEB_LOG"
            ;;
    esac
}

cmd_docker() {
    local action=${1:-up}
    if (($# > 0)); then
        shift
    fi

    case $action in
        up|start)
            prepare_compose_environment
            compose_show_summary
            log_info "Starting docker-compose stack behind nginx reverse proxy..."
            run_docker_compose up -d --build nginx
            log_success "Services are available via http://localhost"
            ;;
        down|stop)
            log_info "Stopping docker-compose services..."
            run_docker_compose down
            log_success "Docker services stopped"
            ;;
        logs)
            local target=${1:-}
            if [[ -n "$target" ]]; then
                log_info "Tailing logs for service: $target"
                run_docker_compose logs -f "$target"
            else
                log_info "Tailing logs for all services"
                run_docker_compose logs -f
            fi
            ;;
        ps|status)
            log_info "Listing docker-compose services"
            run_docker_compose ps
            ;;
        pull)
            log_info "Pulling docker images..."
            run_docker_compose pull
            log_success "Images updated"
            ;;
        help|-h|--help)
            cmd_docker_help
            ;;
        *)
            log_error "Unknown docker command: $action"
            cmd_docker_help
            exit 1
            ;;
    esac
}

cmd_pro() {
    local action=${1:-up}
    if (($# > 0)); then
        shift
    fi

    case $action in
        up|start|deploy)
            prepare_compose_environment
            compose_show_summary
            apply_auth_migrations
            log_info "Starting production stack (nginx reverse proxy on :80)..."
            run_docker_compose up -d --build nginx
            log_success "Production services are running at http://localhost"
            ;;
        down|stop)
            log_info "Stopping production stack..."
            run_docker_compose down
            log_success "Production services stopped"
            ;;
        restart)
            prepare_compose_environment
            compose_show_summary
            apply_auth_migrations
            log_info "Restarting production stack..."
            run_docker_compose down
            run_docker_compose up -d --build nginx
            log_success "Production services restarted"
            ;;
        logs)
            local target=${1:-nginx}
            log_info "Tailing production logs for service: $target (Ctrl+C to stop)"
            run_docker_compose logs -f "$target"
            ;;
        status|ps)
            log_info "Listing production services..."
            run_docker_compose ps
            ;;
        config)
            prepare_compose_environment
            compose_show_summary
            ;;
        test)
            if ! command_exists make; then
                log_error "make is required to run repository tests"
                exit 1
            fi
            prepare_compose_environment
            compose_show_summary
            log_info "Validating docker compose stack"
            run_docker_compose config >/dev/null
            log_success "docker compose config is valid"
            log_info "Running repository test suite (make test)"
            (cd "$SCRIPT_DIR" && make test)
            log_success "Repository tests passed"
            ;;
        help|-h|--help)
            cmd_pro_help
            ;;
        *)
            log_error "Unknown pro command: $action"
            cmd_pro_help
            exit 1
            ;;
    esac
}

cmd_cn() {
    export DOCKER_REGISTRY_MIRROR="${DOCKER_REGISTRY_MIRROR:-https://registry.docker-cn.com}"
    export SANDBOX_IMAGE="${SANDBOX_IMAGE:-$SANDBOX_CHINA_IMAGE}"
    export SANDBOX_SECURITY_OPT="${SANDBOX_SECURITY_OPT:-seccomp=unconfined}"
    export NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmmirror.com/}"
    export NPM_CONFIG_REGISTRY="${NPM_CONFIG_REGISTRY:-${NPM_REGISTRY}}"
    export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
    export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"

    print_cn_mirrors

    cmd_pro "$@"
}

cmd_pro_help() {
    cat << EOF

${C_CYAN}Production Deployment (nginx reverse proxy)${C_RESET}

${C_YELLOW}Usage:${C_RESET}
  ./deploy.sh pro [command]

${C_YELLOW}Commands:${C_RESET}
  ${C_GREEN}up|start|deploy${C_RESET}   Build and start the production stack (default)
  ${C_GREEN}down|stop${C_RESET}        Stop and remove production containers
  ${C_GREEN}restart${C_RESET}          Recreate the production stack
  ${C_GREEN}logs [service]${C_RESET}   Tail logs (defaults to nginx)
  ${C_GREEN}status|ps${C_RESET}        Show running containers
  ${C_GREEN}config${C_RESET}           Print resolved environment summary
  ${C_GREEN}test${C_RESET}             Run docker compose config + make test
  ${C_GREEN}help${C_RESET}             Show this help

${C_YELLOW}Notes:${C_RESET}
  • Frontend is exposed via nginx on http://localhost (port 80)
  • API requests are proxied through nginx to the Go backend
  • Set NEXT_PUBLIC_API_URL=auto to use same-origin API access (default)

EOF
}

cmd_docker_help() {
    cat << EOF

${C_CYAN}Docker Compose Deployment${C_RESET}

${C_YELLOW}Usage:${C_RESET}
  ./deploy.sh docker [command]

${C_YELLOW}Commands:${C_RESET}
  ${C_GREEN}up|start${C_RESET}          Start reverse-proxy stack on :80 (default)
  ${C_GREEN}down|stop${C_RESET}         Stop services
  ${C_GREEN}logs [service]${C_RESET}    Tail logs (all if omitted)
  ${C_GREEN}ps|status${C_RESET}         Show running services
  ${C_GREEN}pull${C_RESET}              Pull latest images
  ${C_GREEN}help${C_RESET}              Show this help

${C_YELLOW}Notes:${C_RESET}
  • Traffic is routed through nginx so the frontend and API share the same origin
  • Set NEXT_PUBLIC_API_URL=auto (default) to allow the frontend to auto-detect the API origin

EOF
}

cmd_help() {
    cat << EOF

${C_CYAN}ALEX SSE Service - Production Deployment${C_RESET}

${C_YELLOW}Usage:${C_RESET}
  ./deploy.sh [command]

${C_YELLOW}Commands:${C_RESET}
  ${C_GREEN}pro [command]${C_RESET}      Run production stack on :80 via nginx (default)
  ${C_GREEN}docker [command]${C_RESET}   Manage docker-compose deployment
  ${C_GREEN}cn [command]${C_RESET}       Deploy using China mirrors (docker/npm/go)
  ${C_GREEN}help${C_RESET}               Show this help

${C_YELLOW}Examples:${C_RESET}
  ./deploy.sh              # Production on :80 via nginx (same-origin frontend/API)
  ./deploy.sh pro status   # Inspect running services
  ./deploy.sh cn deploy    # Production deploy with China mirrors
  ./deploy.sh pro logs web # Tail frontend logs via docker-compose

${C_YELLOW}Environment:${C_RESET}
  Edit .env to configure API keys and settings

EOF
}

###############################################################################
# Main
###############################################################################

main() {
    cd "$SCRIPT_DIR"

    hydrate_env_from_config

    local cmd=${1:-pro}
    if (($# > 0)); then
        shift
    fi

    case $cmd in
        docker)
            cmd_docker "$@"
            ;;
        pro)
            cmd_pro "${1:-up}" "${@:2}"
            ;;
        cn)
            cmd_cn "${1:-up}" "${@:2}"
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
