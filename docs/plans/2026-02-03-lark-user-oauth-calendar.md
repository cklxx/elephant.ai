# Plan: Lark user OAuth for calendar booking (self only)

Date: 2026-02-03

## Context
- `lark_calendar_create` currently defaults to tenant-scoped calls when `user_access_token` is not provided.
- For a user’s own calendar (including the primary calendar), tenant-scoped calls often fail with:
  - `code=191002 msg=no calendar access_role`
- Lark requires a user OAuth flow to obtain `user_access_token` + `refresh_token`:
  - Redirect user to `/open-apis/authen/v1/index` to get a login pre-auth `code`
  - Exchange `code` via `/open-apis/authen/v1/access_token`
  - Refresh via `/open-apis/authen/v1/refresh_access_token`
- Product decision: the personal assistant can **only book the sender’s own calendar**, so callers should not pass `calendar_id`.

## Goals
1) Add Lark user OAuth endpoints (start + callback) on the server.
2) Persist user tokens by sender `open_id`, with automatic refresh.
3) Make calendar tools automatically use the stored `user_access_token`, and surface a clear “authorize first” URL when missing.
4) Remove the `calendar_id` parameter from calendar tools; always book into the sender’s primary calendar.

## Approach
1) `internal/lark/oauth`:
   - Token store: file-backed by default, Postgres-backed when SessionDB is available.
   - State store for OAuth `state` (one-time use) with TTL.
   - Service:
     - Build auth URL (based on `channels.lark.base_domain`)
     - Exchange code → tokens (SDK: `authen/v1/access_token`)
     - Refresh tokens when nearing expiry (SDK: `authen/v1/refresh_access_token`)
2) `internal/server/http`:
   - `GET /api/lark/oauth/start`: create state → redirect to Lark auth page
   - `GET /api/lark/oauth/callback`: consume state + exchange code → persist tokens → show success HTML
3) Lark gateway:
   - Inject the OAuth service/provider into the execution context so tools can retrieve tokens by `open_id`.
4) Calendar tools:
   - Remove `calendar_id` from schema and required args; default to `"primary"`.
   - Before calendar calls: resolve `user_access_token` from provider and attach it via `WithUserAccessToken`.
   - If no token: return a tool error that contains the OAuth start URL.
5) TDD + validation:
   - Unit tests for token refresh, state consume, handler redirects, and calendar tool behaviour.
   - Run `./dev.sh lint && ./dev.sh test`.

## Progress
- [ ] Add plan + scaffolding.
- [ ] Implement OAuth token + state storage.
- [ ] Implement server HTTP OAuth handler + routes.
- [ ] Wire provider into Lark gateway context.
- [ ] Update calendar tools to remove `calendar_id` and require user OAuth.
- [ ] Run `./dev.sh lint && ./dev.sh test`; commit.

