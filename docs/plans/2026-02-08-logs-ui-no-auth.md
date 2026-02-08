# Plan: Make `./dev.sh logs-ui` Work Without Login

Date: 2026-02-08
Owner: Codex (for cklxx)

## Goal
Ensure the local log analyzer flow (`./dev.sh logs-ui` -> `/dev/log-analyzer` -> `/api/dev/logs/*`) works without authentication in development mode, while keeping other dev/internal endpoints protected.

## Scope
- Backend router auth boundary for dev log endpoints.
- Frontend log analyzer auth gating and API auth behavior.
- Regression tests for both open and still-protected endpoints.

## Out of Scope
- Changing auth model for non-log endpoints.
- Production auth behavior changes.

## Steps
- [x] Add/adjust failing tests first:
  - [x] Unauthenticated `GET /api/dev/logs/index` returns `200` when auth module is enabled.
  - [x] A protected dev endpoint (e.g. `GET /api/dev/memory`) still returns `401` unauthenticated.
- [x] Update router so only `/api/dev/logs*` bypass auth middleware in development mode.
- [x] Remove `RequireAuth` wrapper from log analyzer page.
- [x] Mark log analyzer API calls as `skipAuth` to avoid refresh-token flows for this page.
- [x] Run targeted tests, then full lint + tests.
- [x] Commit in incremental steps.

## Risks & Mitigations
- Risk: Accidentally broadening unauthenticated surface.
  - Mitigation: Restrict bypass only to three log endpoints and add explicit regression test for a protected endpoint.
- Risk: Frontend still triggers auth refresh indirectly.
  - Mitigation: Use `skipAuth: true` for log API client calls.

## Verification
- `go test ./internal/delivery/server/http -run 'TestRouterE2EDevLogIndexWithAuth|TestRouterE2EDevMemoryRequiresAuth'`
- `./dev.sh lint`
- `./dev.sh test`
