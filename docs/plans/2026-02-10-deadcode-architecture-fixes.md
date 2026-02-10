# 2026-02-10 Dead Code + Architecture Fixes

## Goal
- Remove confirmed dead code in Go/Web layers.
- Fix architecture issues found in systematic review:
  - Domain depending directly on infra (`internal/domain/materials`).
  - Delivery depending directly on infra tool internals (`internal/delivery/channels/lark`, `internal/delivery/server/app`).
  - App DI depending directly on delivery gateway type.
  - Delivery HTTP router constructing infra sandbox client directly.

## Constraints
- Keep behavior stable; prefer adapter/injection over broad rewrites.
- Use incremental commits and verify after each batch.

## Plan
1. Dead code cleanup
   - [x] Remove `evaluatePhase` helper and adjust tests.
   - [x] Remove `emitTypewriter` and its test.
   - [x] Remove unreferenced frontend components (`ErrorBoundary`, `A2UIRenderer`, `HomeManifestoPage`).
2. Domain/Delivery layering fixes
   - [x] Refactor `attachment_migrator` to remove direct infra imports.
   - [x] Introduce app-level adapters for delivery lark context/artifact/path helpers.
   - [x] Update lark gateway/task/model/plan handlers to use app-level adapters.
   - [x] Remove `di.Container` direct dependency on `delivery/channels/lark` type.
   - [x] Move sandbox client construction from HTTP router to bootstrap injection.
3. Validation + review
   - [x] Run formatting/lint/tests.
   - [x] Run mandatory code review workflow and fix findings.
   - [ ] Commit in incremental steps and merge back to `main`.

## Progress log
- 2026-02-10 14:10: plan created; implementation started.
- 2026-02-10 14:22: dead-code cleanup + architecture refactor completed; `make check-arch`, `make fmt`, `make test`, `web npm run lint`, `web npm run test` all passed.
- 2026-02-10 14:23: code-review checklist executed and findings addressed (no open P0/P1/P2 blockers).
