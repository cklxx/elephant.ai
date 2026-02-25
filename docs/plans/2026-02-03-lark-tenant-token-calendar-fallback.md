# Plan: Auto-refresh tenant tokens + shared calendar fallback for Lark calendar tools

Date: 2026-02-03

## Context
- User OAuth redirect flow is blocked by `redirect_uri` validation errors.
- Calendar tools currently require a user-scoped access token and fail when OAuth is missing.
- We need a tenant access token fallback path configurable via YAML, with auto-refresh support.
- Tenant-scoped calls should target a shared calendar instead of the user primary calendar.

## Goals
1) Add tenant token mode + shared calendar settings under `channels.lark`.
2) Propagate tenant token mode + calendar ID into the Lark tool execution context.
3) Update calendar tools to use tenant tokens (auto-refresh or static) when user OAuth is missing.
4) Ensure tenant mode uses a configured shared calendar ID instead of resolving primary calendars.
5) Add tests for tenant token auto-refresh + shared calendar routing.

## Non-Goals
- Rework OAuth redirect flow or app configuration.
- Reworking OAuth redirect flow or app configuration.

## Approach
1) Config plumbing:
   - Extend Lark channel config structs (`file_config.go`, `bootstrap/config.go`, `channels/lark/config.go`) with:
     - `tenant_token_mode` (`auto` | `static`)
     - `tenant_calendar_id`
   - Wire `TenantTokenMode` + `TenantCalendarID` into `LarkGatewayConfig`.
   - Inject mode + calendar ID into tool context via `shared.WithLarkTenantTokenMode` and `shared.WithLarkTenantCalendarID`.
   - Only inject `tenant_access_token` when mode is `static`.
2) Lark API options:
   - Use `larkcore.WithTenantAccessToken("")` to trigger SDK auto-refresh in `auto` mode.
   - Use `larkcore.WithUserAccessToken(token)` to send static tenant tokens directly.
3) Calendar tools:
   - Replace `requireLarkUserAccessToken` with a resolver that prefers user tokens and falls back to tenant tokens.
   - When tenant mode is active, require `tenant_calendar_id` and skip primary calendar resolution.
4) Tests:
   - Add coverage for tenant auto-refresh + shared calendar routing (create + query).
   - Add coverage for missing `tenant_calendar_id` error path.

## Progress
- [x] Add config + context plumbing for tenant token mode + calendar ID.
- [x] Update calendar tool auth resolution to support auto/static tenant modes.
- [x] Add/adjust tests for tenant auto-refresh + shared calendar routing.
- [x] Run `./dev.sh lint && ./dev.sh test`.
- [x] Update plan status and document outcomes.
