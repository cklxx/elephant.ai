# Plan: Allow Lark calendar tools to use tenant access tokens when user OAuth is unavailable

Date: 2026-02-03

## Context
- User OAuth redirect flow is blocked by `redirect_uri` validation errors.
- Calendar tools currently require a user-scoped access token and fail when OAuth is missing.
- We need a tenant access token fallback path configurable via YAML for local testing.

## Goals
1) Add a `tenant_access_token` setting under `channels.lark` in YAML config.
2) Propagate the token into Lark tool execution context.
3) Update calendar tools to use tenant access tokens when user OAuth is missing or unavailable.
4) Add tests for tenant token fallback behavior.

## Non-Goals
- Rework OAuth redirect flow or app configuration.
- Persisting or refreshing tenant tokens (caller-provided only).

## Approach
1) Config plumbing:
   - Extend Lark channel config structs (`file_config.go`, `bootstrap/config.go`, `channels/lark/config.go`).
   - Wire `TenantAccessToken` into `LarkGatewayConfig`.
   - Inject token into tool context via `shared.WithLarkTenantToken`.
2) Lark API options:
   - Add `lark.WithTenantToken` call option (wraps SDK `WithTenantAccessToken`).
3) Calendar tools:
   - Replace `requireLarkUserAccessToken` with a resolver that prefers user tokens and falls back to tenant tokens.
   - Map token kind to `larkcore.WithUserAccessToken` or `WithTenantAccessToken` for SDK calls.
4) Tests:
   - Add coverage for tenant token fallback (calendar create + query).

## Progress
- [x] Add config + context plumbing for tenant tokens.
- [x] Update calendar tool auth resolution to support tenant fallback.
- [x] Add/adjust tests for tenant token fallback.
- [x] Run `./dev.sh lint && ./dev.sh test`.
- [x] Update plan status and document outcomes.
