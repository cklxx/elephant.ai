# Plan: Feature Review Architecture Simplification Implementation

Date: 2026-03-04
Owner: Codex

## Goal
Implement the approved optimization blueprint on core backend:
1. Event pipeline convergence in coordinator.
2. Shared context compaction/compression planning policy.
3. Task store interface decomposition with minimal-interface adoption.

## Scope
- `internal/app/agent/coordinator/*`
- `internal/domain/agent/ports/*`
- `internal/app/context/*`
- `internal/domain/agent/react/*`
- `internal/domain/task/*`
- `internal/infra/external/bridge/*`

## Steps
- [completed] Add EventDispatcher abstraction and migrate `ExecuteTask` to single assembly point.
- [completed] Introduce shared compression plan builder in ports and migrate app/react callers.
- [completed] Split task domain store interface into focused sub-interfaces.
- [completed] Adopt narrowed store dependency in bridge resumer.
- [completed] Run tests and lint on touched packages.
- [completed] Run mandatory code review skill and address P0/P1.
- [completed] Commit in incremental commits, merge back to `main`, remove worktree.

## Verification Notes
- Targeted Go tests on touched packages: pass.
- `make check-arch`: pass.
- `./scripts/run-golangci-lint.sh run ./...`: pass.
- Web lint/test (using existing main workspace `web/node_modules`): pass.
- `make check-arch-policy`: fails with 2 pre-existing baseline violations (`internal/infra/taskadapters` -> delivery imports), reproducible on `main`.
- `make test`: fails in existing integration/e2e suites (workspace-manager availability / bridge signal termination), reproducible and not introduced by this diff.
