#!/usr/bin/env bash
# Shared helpers for deployment scripts (local + production).
# Handles ~/.alex/config.yaml hydration without forcing extra deps unless needed.

: "${ALEX_CONFIG_PATH:=${HOME}/.alex/config.yaml}"
: "${DEPLOY_CONFIG_WARNED_NO_YQ:=0}"

_deploy_config_log_warn() {
    if declare -F log_warn >/dev/null 2>&1; then
        log_warn "$1"
    else
        printf 'WARN: %s\n' "$1" >&2
    fi
}

deploy_config::load_value() {
    local yq_expr="$1"

    if [[ -z "$yq_expr" || ! -f "$ALEX_CONFIG_PATH" ]]; then
        return 1
    fi

    if command -v yq >/dev/null 2>&1; then
        yq -er "$yq_expr // empty" "$ALEX_CONFIG_PATH" 2>/dev/null || return 1
        return 0
    fi

    if command -v python3 >/dev/null 2>&1; then
        python3 - "$ALEX_CONFIG_PATH" "$yq_expr" << 'PY' || return 1
import sys
try:
    import yaml
except Exception:
    sys.exit(1)

path = sys.argv[1]
expr = sys.argv[2].lstrip(".")
try:
    with open(path, "r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh) or {}
except Exception:
    sys.exit(1)

current = data
for part in expr.split("."):
    if isinstance(current, dict) and part in current:
        current = current[part]
    else:
        sys.exit(1)

if current is None:
    sys.exit(1)
if isinstance(current, (dict, list)):
    sys.exit(1)
print(current)
PY
        return 0
    fi

    if [[ "${DEPLOY_CONFIG_WARNED_NO_YQ}" -eq 0 ]]; then
        _deploy_config_log_warn "Install yq or python3+pyyaml to hydrate values from $ALEX_CONFIG_PATH"
        DEPLOY_CONFIG_WARNED_NO_YQ=1
    fi
    return 1
}

deploy_config::resolve_var() {
    local var_name="$1"
    local yq_expr="${2:-}"
    local default_value="${3:-}"
    local current_value="${!var_name:-}"

    if [[ -n "$current_value" ]]; then
        printf 'env'
        return 0
    fi

    local resolved_value=""
    if [[ -n "$yq_expr" ]]; then
        resolved_value="$(deploy_config::load_value "$yq_expr" || true)"
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
