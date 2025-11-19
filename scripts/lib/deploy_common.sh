#!/usr/bin/env bash
# Shared helpers for deployment scripts (local + production).
# Handles ~/.alex-config.json hydration without forcing jq unless needed.

: "${ALEX_CONFIG_PATH:=${HOME}/.alex-config.json}"
: "${DEPLOY_CONFIG_WARNED_NO_JQ:=0}"

_deploy_config_log_warn() {
    if declare -F log_warn >/dev/null 2>&1; then
        log_warn "$1"
    else
        printf 'WARN: %s\n' "$1" >&2
    fi
}

deploy_config::load_value() {
    local jq_expr="$1"

    if [[ -z "$jq_expr" || ! -f "$ALEX_CONFIG_PATH" ]]; then
        return 1
    fi

    if ! command -v jq >/dev/null 2>&1; then
        if [[ "${DEPLOY_CONFIG_WARNED_NO_JQ}" -eq 0 ]]; then
            _deploy_config_log_warn "Install jq to hydrate values from $ALEX_CONFIG_PATH"
            DEPLOY_CONFIG_WARNED_NO_JQ=1
        fi
        return 1
    fi

    jq -er "$jq_expr // empty" "$ALEX_CONFIG_PATH" 2>/dev/null || return 1
}

deploy_config::resolve_var() {
    local var_name="$1"
    local jq_expr="${2:-}"
    local default_value="${3:-}"
    local current_value="${!var_name:-}"

    if [[ -n "$current_value" ]]; then
        printf 'env'
        return 0
    fi

    local resolved_value=""
    if [[ -n "$jq_expr" ]]; then
        resolved_value="$(deploy_config::load_value "$jq_expr" || true)"
    fi

    local source=""
    if [[ -n "$resolved_value" ]]; then
        source="config"
    elif [[ -n "$default_value" ]]; then
        resolved_value="$default_value"
        source="default"
    else
        return 1
    fi

    export "$var_name"="$resolved_value"
    printf '%s' "$source"
    return 0
}
