# Plan: Auto-only Lark tenant_access_token

Date: 2026-02-04

## Context
- Lark tenant access tokens should be fetched automatically via SDK using app_id/app_secret.
- Static tenant_access_token configuration should be removed to avoid manual setup.
- Calendar tools still require a shared tenant calendar_id when using tenant tokens.

## Goals
1) Remove tenant_access_token / tenant_token_mode config fields and static branches.
2) Keep tenant token auto-refresh via Lark SDK as the only path.
3) Preserve tenant_calendar_id requirement for tenant-scoped calendar calls.
4) Update docs/examples to reflect auto token behavior.
5) Update tests to match auto-only flow.

## Non-Goals
- Change OAuth user token flow.
- Auto-detect tenant calendar_id.

## Approach
1) Config cleanup: remove tenant_access_token and tenant_token_mode from file config, bootstrap config, and channel config.
2) Gateway cleanup: stop injecting tenant token/mode into tool context; keep tenant_calendar_id.
3) Tool auth: simplify to auto-tenant fallback only; remove static token validation.
4) Tests: remove static token cases; ensure auto mode still triggers tenant token endpoint; keep missing calendar_id errors.
5) Docs: add tenant_calendar_id note and auto token explanation.

## Progress
- [x] Add plan file.
- [x] Remove tenant token config fields and gateway wiring.
- [x] Simplify larktools auth path to auto-only.
- [x] Update tests for auto-only behavior.
- [x] Update docs/examples.
- [x] Update long-term memory timestamp.
- [ ] Run full lint + tests (Go tests passed; web lint failed: eslint not found).
- [x] Merge to main and record plan updates.
