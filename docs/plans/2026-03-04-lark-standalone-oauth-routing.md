# 2026-03-04 Lark Standalone OAuth Routing Fix

## Background
- In `alex-server lark` standalone mode, OAuth links returned by Lark tools were not usable:
  - `/api/lark/oauth/start` was not routed by debug HTTP router.
  - OAuth redirect base used `server.port` while standalone HTTP entrypoint runs on `server.debug_port`.
  - Users should receive a direct Feishu authorization URL instead of a local trampoline URL.

## Goals
- Make OAuth authorization flow usable end-to-end in standalone mode.
- Ensure local callback port in `redirect_uri` matches standalone listening port.
- Return Feishu official authorization URL to users through tool results.

## Changes
1. Wire OAuth endpoints into debug router when OAuth service is present.
2. Inject `LarkOAuthHandler` into debug router in standalone bootstrap.
3. Build OAuth redirect base from `debug_port` first (fallback to `port`).
4. Generate direct authorization URL in `UserAccessToken` missing-token path.
5. Update tests to match `/open-apis/authen/v1/authorize` and debug-router OAuth exposure behavior.

## Verification
- Run targeted Go tests for:
  - `internal/infra/lark/oauth`
  - `internal/delivery/server/http`
  - `internal/delivery/server/bootstrap` (debug server coverage)

## Status
- [x] Code changes
- [x] Tests
- [ ] Commit
