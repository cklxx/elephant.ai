#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"
ENV_EXAMPLE_FILE="${ROOT_DIR}/.env.example"
COMPOSE_FILE="${ROOT_DIR}/deploy/docker/docker-compose.dev.yml"
MIGRATION_FILE="${ROOT_DIR}/migrations/auth/001_init.sql"
AUTH_DB_CONTAINER="alex-auth-db"
LOG_DIR="${ROOT_DIR}/logs"
LOG_FILE="${LOG_DIR}/setup_auth_db.log"
DEFAULT_DB_URL="postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable"
DEFAULT_PASSWORD_HASH='argon2id$1$65536$4$X/2c361Hs7Z7BTh06+aZaQ$FN9oVAe9UTRi7adCznuGy7sQrKYhanWBDhVG3en+HV4'

log() {
    local level=$1
    shift
    printf '[setup_auth_db] %s %s\n' "${level}" "$*"
}

log_info() { log 'INFO' "$@"; }
log_warn() { log 'WARN' "$@"; }
log_error() { log 'ERROR' "$@"; }
log_success() { log 'DONE' "$@"; }

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        log_error "Required command '$1' not found. Please install it first."
        exit 1
    fi
}

psql_available() {
    command -v psql >/dev/null 2>&1
}

ensure_env_file() {
    mkdir -p "$LOG_DIR"

    if [[ -f "$ENV_FILE" ]]; then
        return
    fi

    if [[ -f "$ENV_EXAMPLE_FILE" ]]; then
        cp "$ENV_EXAMPLE_FILE" "$ENV_FILE"
        log_info "Created .env from .env.example"
    else
        cat > "$ENV_FILE" <<EOF_ENV
OPENAI_API_KEY=
AUTH_JWT_SECRET=
AUTH_DATABASE_URL=${DEFAULT_DB_URL}
EOF_ENV
        log_warn ".env.example not found. Created minimal .env"
    fi
}

random_hex() {
    if command -v python3 >/dev/null 2>&1; then
        python3 - <<'PY'
import secrets
print(secrets.token_hex(32))
PY
        return
    fi

    if command -v python >/dev/null 2>&1; then
        python - <<'PY'
import secrets
print(secrets.token_hex(32))
PY
        return
    fi

    openssl rand -hex 32
}

ensure_env_var() {
    local key=$1
    local value=$2
    if grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then
        return
    fi
    printf '\n%s=%s\n' "$key" "$value" >> "$ENV_FILE"
    log_warn "Appended missing ${key} to .env"
}

load_env() {
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
}

ensure_required_envs() {
    if ! grep -q '^AUTH_JWT_SECRET=' "$ENV_FILE" 2>/dev/null; then
        ensure_env_var "AUTH_JWT_SECRET" "$(random_hex)"
    fi

    ensure_env_var "AUTH_DATABASE_URL" "${DEFAULT_DB_URL}"
    ensure_env_var "AUTH_DB_USER" "alex"
    ensure_env_var "AUTH_DB_PASSWORD" "alex"
    ensure_env_var "AUTH_DB_NAME" "alex_auth"
    ensure_env_var "AUTH_DB_PORT" "5432"
    ensure_env_var "AUTH_DB_IMAGE" "postgres:15"
}

compose_cmd() {
    if docker compose version >/dev/null 2>&1; then
        echo "docker compose"
        return
    fi

    if command -v docker-compose >/dev/null 2>&1; then
        echo "docker-compose"
        return
    fi

    log_error "docker compose is required to start auth-db"
    exit 1
}

ensure_docker_daemon() {
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker daemon does not appear to be running"
        log_error "Start Docker Desktop/daemon and rerun this script"
        exit 1
    fi
}

start_auth_db() {
    local cmd
    cmd=$(compose_cmd)
    log_info "Starting auth-db via docker compose"

    set +e
    # shellcheck disable=SC2086
    ${cmd} -f "$COMPOSE_FILE" up -d auth-db >/dev/null 2>"${LOG_FILE}"
    local status=$?
    set -e

    if [[ $status -ne 0 ]]; then
        log_error "Failed to start auth-db via docker compose"
        log_error "Check ${LOG_FILE} for details"
        log_error "Common causes: network restrictions preventing image pull, or Docker Hub login issues"
        log_warn "You can rerun with SKIP_LOCAL_AUTH_DB=1 to bypass automatic provisioning"
        exit $status
    fi
}

wait_for_auth_db() {
    local timeout=${1:-60}
    local elapsed=0
    log_info "Waiting for ${AUTH_DB_CONTAINER} to be healthy"

    while (( elapsed < timeout )); do
        local status
        status=$(docker inspect --format='{{.State.Health.Status}}' "$AUTH_DB_CONTAINER" 2>/dev/null || echo "")
        if [[ "$status" == "healthy" ]]; then
            log_success "auth-db is healthy"
            return
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done

    log_error "auth-db did not become healthy within ${timeout}s"
    exit 1
}

run_migrations() {
    if [[ ! -f "$MIGRATION_FILE" ]]; then
        log_error "Migration file not found: $MIGRATION_FILE"
        exit 1
    fi

    log_info "Running auth DB migrations"
    run_psql -f "$MIGRATION_FILE" >/dev/null
    log_success "Schema initialized"
}

seed_admin_user() {
    local email="${AUTH_SEED_EMAIL:-admin@example.com}"
    local display_name="${AUTH_SEED_DISPLAY_NAME:-Admin}"
    local status="${AUTH_SEED_STATUS:-active}"
    local password_hash="${AUTH_SEED_PASSWORD_HASH:-$DEFAULT_PASSWORD_HASH}"
    local points="${AUTH_SEED_POINTS_BALANCE:-0}"
    local tier="${AUTH_SEED_SUBSCRIPTION_TIER:-free}"
    local expires="${AUTH_SEED_SUBSCRIPTION_EXPIRES_AT:-}"

    local expires_sql
    if [[ -z "$expires" ]]; then
        expires_sql="NULL"
    else
        expires_sql="'${expires}'"
    fi

    log_info "Seeding auth user ${email}"
    run_psql <<SQL
INSERT INTO auth_users (
    id,
    email,
    display_name,
    status,
    password_hash,
    points_balance,
    subscription_tier,
    subscription_expires_at,
    created_at,
    updated_at
)
VALUES (
    gen_random_uuid(),
    '${email}',
    '${display_name}',
    '${status}',
    '${password_hash}',
    ${points},
    '${tier}',
    ${expires_sql},
    NOW(),
    NOW()
)
ON CONFLICT (email) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    status = EXCLUDED.status,
    password_hash = EXCLUDED.password_hash,
    points_balance = EXCLUDED.points_balance,
    subscription_tier = EXCLUDED.subscription_tier,
    subscription_expires_at = EXCLUDED.subscription_expires_at,
    updated_at = NOW();
SQL
    log_success "Admin user ensured (${email})"
}

run_psql() {
    if psql_available; then
        psql "$AUTH_DATABASE_URL" "$@"
        return
    fi

    docker exec -i \
        -e "PGPASSWORD=${AUTH_DB_PASSWORD}" \
        "$AUTH_DB_CONTAINER" \
        psql -U "$AUTH_DB_USER" -d "$AUTH_DB_NAME" "$@"
}

main() {
    require_command docker
    ensure_docker_daemon

    ensure_env_file
    ensure_required_envs
    load_env

    if ! psql_available; then
        log_warn "psql not found; using docker exec for migrations and seeding"
    fi

    start_auth_db
    wait_for_auth_db
    run_migrations
    seed_admin_user

    log_success "Local auth database is ready"
}

main "$@"
