#!/bin/bash
# refresh_claude_token.sh — Claude OAuth token refresh helper
# elephant.ai kernel utility
# Created: 2026-03-02T16:40:00+08:00
#
# Context:
#   Claude Code v2.1+ stores OAuth credentials in macOS Keychain
#   (service: "Claude Code-credentials") instead of ~/.claude/.credentials.json.
#   The credential file path is kept as a fallback for older versions.
#   accessToken (sk-ant-oat01-*) expires every ~10h after login.
#   refreshToken (sk-ant-ort01-*) MAY be used for programmatic refresh.
#
#   Alex auto-detects credentials in this order:
#     1. CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_AUTH_TOKEN env vars
#     2. ~/.claude/.credentials.json (legacy file paths)
#     3. macOS Keychain ("Claude Code-credentials")
#     4. `claude setup-token` CLI
#
# Usage:
#   ./scripts/refresh_claude_token.sh          # inspect current status
#   ./scripts/refresh_claude_token.sh --refresh # attempt token refresh
#   ./scripts/refresh_claude_token.sh --force   # force re-login via claude CLI

set -euo pipefail

CREDS_FILE="$HOME/.claude/.credentials.json"
WARN_HOURS=2   # warn if fewer than this many hours remain

# ── helpers ──────────────────────────────────────────────────────────────────
die() { echo "ERROR: $*" >&2; exit 1; }

check_jq() {
    command -v jq &>/dev/null || die "jq is required (brew install jq)"
}

# ── read & parse credentials ─────────────────────────────────────────────────
read_token_info() {
    [[ -f "$CREDS_FILE" ]] || die "credentials file not found: $CREDS_FILE"
    EXPIRES_MS=$(jq -r '.claudeAiOauth.expiresAt' "$CREDS_FILE")
    ACCESS_TOKEN=$(jq -r '.claudeAiOauth.accessToken' "$CREDS_FILE")
    REFRESH_TOKEN=$(jq -r '.claudeAiOauth.refreshToken' "$CREDS_FILE")
    SUBSCRIPTION=$(jq -r '.claudeAiOauth.subscriptionType' "$CREDS_FILE")

    # convert ms → seconds
    EXPIRES_S=$(( EXPIRES_MS / 1000 ))
    NOW_S=$(date -u +%s)
    REMAINING_S=$(( EXPIRES_S - NOW_S ))
    REMAINING_H=$(echo "scale=2; $REMAINING_S / 3600" | bc)
    EXPIRES_ISO=$(date -u -r "$EXPIRES_S" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null \
                  || date -u -d "@$EXPIRES_S" +"%Y-%m-%dT%H:%M:%SZ")  # Linux fallback
}

print_status() {
    echo "═══════════════════════════════════════════════"
    echo "  Claude OAuth Token Status"
    echo "═══════════════════════════════════════════════"
    echo "  Subscription : $SUBSCRIPTION"
    echo "  Expires UTC  : $EXPIRES_ISO"
    echo "  Now UTC      : $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    echo "  Remaining    : ${REMAINING_H}h"
    echo "  Access Token : ${ACCESS_TOKEN:0:20}...${ACCESS_TOKEN: -6}"
    echo "  Refresh Token: ${REFRESH_TOKEN:0:20}...${REFRESH_TOKEN: -6}"

    if (( REMAINING_S <= 0 )); then
        echo "  STATUS       : ❌ EXPIRED — re-login required"
    elif (( REMAINING_S <= WARN_HOURS * 3600 )); then
        echo "  STATUS       : ⚠️  EXPIRING SOON (<${WARN_HOURS}h) — consider re-login"
    else
        echo "  STATUS       : ✅ VALID"
    fi
    echo "═══════════════════════════════════════════════"
}

# ── attempt refresh via refreshToken (best-effort) ───────────────────────────
attempt_refresh() {
    echo "Attempting programmatic token refresh via refreshToken..."
    echo "(Note: Anthropic does not publicly document this endpoint —"
    echo " this is a best-effort attempt using the Claude.ai OAuth flow)"

    local REFRESH_URL="https://claude.ai/api/auth/refresh"
    local RESULT
    RESULT=$(curl -s -X POST "$REFRESH_URL" \
        -H "Content-Type: application/json" \
        -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}" \
        --max-time 15 2>&1) || true

    if echo "$RESULT" | jq -e '.access_token' &>/dev/null; then
        echo "✅ Refresh succeeded — updating credentials file"
        local NEW_AT NEW_RT NEW_EXP
        NEW_AT=$(echo "$RESULT" | jq -r '.access_token')
        NEW_RT=$(echo "$RESULT" | jq -r '.refresh_token // empty')
        NEW_EXP=$(echo "$RESULT" | jq -r '.expires_at // empty')

        # update in-place using jq
        local TMP_FILE
        TMP_FILE=$(mktemp)
        jq --arg at "$NEW_AT" \
           --arg rt "${NEW_RT:-$REFRESH_TOKEN}" \
           --argjson exp "${NEW_EXP:-$EXPIRES_MS}" \
           '.claudeAiOauth.accessToken = $at |
            .claudeAiOauth.refreshToken = $rt |
            .claudeAiOauth.expiresAt = $exp' \
           "$CREDS_FILE" > "$TMP_FILE"
        mv "$TMP_FILE" "$CREDS_FILE"
        chmod 600 "$CREDS_FILE"
        echo "✅ Credentials updated: $CREDS_FILE"
    else
        echo "⚠️  Programmatic refresh failed (endpoint may not be public)."
        echo "   Response: ${RESULT:0:200}"
        echo ""
        echo "→ FALLBACK: Manual re-login required (see --force flag)"
        return 1
    fi
}

# ── force re-login ────────────────────────────────────────────────────────────
force_relogin() {
    echo "Launching Claude CLI re-login..."
    echo "(This will open a browser window — complete the OAuth flow)"
    if command -v claude &>/dev/null; then
        claude auth login
    else
        echo "⚠️  'claude' CLI not found in PATH."
        echo "   Manual steps:"
        echo "   1. Install Claude CLI: npm install -g @anthropic-ai/claude-code"
        echo "   2. Run: claude auth login"
        echo "   3. Complete OAuth in browser"
        echo "   4. Token saved to: $CREDS_FILE"
        exit 1
    fi
}

# ── check and warn if kernel is at risk ───────────────────────────────────────
kernel_risk_check() {
    LLM_SEL="$HOME/.alex/llm_selection.json"
    if [[ -f "$LLM_SEL" ]]; then
        LARK_MODE=$(jq -r '.selections.lark.mode // "unknown"' "$LLM_SEL")
        LARK_PROVIDER=$(jq -r '.selections.lark.provider // "unknown"' "$LLM_SEL")
        echo "Kernel lark mode: $LARK_MODE / $LARK_PROVIDER"
        if [[ "$LARK_MODE" == "cli" && "$LARK_PROVIDER" == "anthropic" ]]; then
            echo "⚠️  Kernel is using claude-cli OAuth — token expiry will break lark inference."
            echo "   Mitigation: refresh before expiry or switch lark mode to config+API key."
        fi
    fi
}

# ── main ──────────────────────────────────────────────────────────────────────
MODE="${1:---status}"

check_jq
read_token_info
print_status
echo ""
kernel_risk_check
echo ""

case "$MODE" in
    --status)
        echo "Tip: run with --refresh to attempt auto-refresh, or --force to re-login."
        ;;
    --refresh)
        if (( REMAINING_S > WARN_HOURS * 3600 )); then
            echo "Token still has ${REMAINING_H}h — no refresh needed yet."
        else
            attempt_refresh || true
        fi
        ;;
    --force)
        force_relogin
        ;;
    --watch)
        # Monitor mode: check every 30min, warn/refresh as needed
        echo "Watch mode: checking every 30 minutes..."
        while true; do
            read_token_info
            print_status
            if (( REMAINING_S <= WARN_HOURS * 3600 && REMAINING_S > 0 )); then
                echo "⚠️  Token expiring soon — attempting refresh..."
                attempt_refresh || echo "Manual re-login needed!"
            elif (( REMAINING_S <= 0 )); then
                echo "❌ Token expired — cannot auto-renew without user interaction."
                break
            fi
            sleep 1800
        done
        ;;
    *)
        echo "Usage: $0 [--status|--refresh|--force|--watch]"
        exit 1
        ;;
esac
