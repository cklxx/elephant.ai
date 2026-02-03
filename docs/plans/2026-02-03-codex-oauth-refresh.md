# Codex OAuth Auto-Refresh

**Date:** 2026-02-03
**Status:** Complete

## Problem

The Codex/OpenAI subscription token (JWT) in `~/.codex/auth.json` expires after ~10 days.
Currently, only Antigravity has auto-refresh. Codex tokens are static — when they expire,
Lark mode and CLI mode stop working until the user manually re-authenticates.

## Solution

### Part 1: Codex OAuth Refresh in `cli_auth.go`

Mirror the Antigravity OAuth refresh pattern:

1. Extend `codexAuthFile` struct to include `refresh_token`, `id_token`, `last_refresh`
2. Parse JWT payload to extract `exp` claim (base64url decode, no library needed)
3. Check expiry with 5-minute skew (same as Antigravity)
4. If near-expiry, POST to `https://auth.openai.com/oauth/token` with:
   - `grant_type=refresh_token`
   - `refresh_token=<rt>`
   - `client_id=<extracted from JWT>`
5. Write refreshed token back to `~/.codex/auth.json`
6. Return fresh token from `loadCodexCLIAuth()`

### Part 2: Dynamic Credential Resolution

For long-running servers (Lark mode):

1. Add `CredentialRefresher` type to preparation service
2. Before each task execution, re-resolve CLI credentials for codex provider
3. Wire through coordinator options from container builder

### Part 3: Config Cleanup

Remove hardcoded `api_key` and `base_url` from `~/.alex/config.yaml` overrides.
Keep `llm_provider: codex` and `llm_model: gpt-5.2-codex`.
The system auto-resolves from CLI credentials.

## Files Modified

- `internal/config/cli_auth.go` — Codex OAuth refresh logic
- `internal/config/cli_auth_test.go` — Tests
- `internal/agent/app/preparation/service.go` — CredentialRefresher
- `internal/agent/app/coordinator/coordinator.go` — Wire option
- `internal/di/container_builder.go` — Create refresher
- `~/.alex/config.yaml` — Remove static overrides

## Design Decisions

- **No JWT library**: Just base64url-decode the payload section. We only need `exp` and `client_id`.
- **Public client**: Codex CLI uses public OAuth client (`app_EMoamEEZ73f0CkXaXp7hrann`), no client_secret needed.
- **Fallback on failure**: Like Antigravity, return expired token on refresh failure so errors surface cleanly.
- **File write pattern**: Same as Antigravity — `os.WriteFile` directly for write-back.
