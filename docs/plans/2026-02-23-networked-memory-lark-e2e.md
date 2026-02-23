# 2026-02-23 Networked Memory + Lark/Web E2E Stabilization Plan

## Goal
Deliver a network-style memory organization update (including AGENTS/CLAUDE guidance and memory reference docs), and make Lark + Web end-to-end validation green on the current branch.

## Scope
- Memory documentation/network semantics:
  - `AGENTS.md`
  - `CLAUDE.md`
  - `docs/reference/MEMORY_SYSTEM.md`
  - `docs/reference/MEMORY_INDEXING.md`
  - `docs/reference/lark-web-agent-event-flow.md`
- Lark runtime/test stability:
  - panic-safe JSON marshal fallback
  - scenario expectation sync (`/new` reset command)
- Web mock stream + E2E stabilization:
  - preserve mock input events during session bootstrap
  - subagent/mock event alignment
  - cross-browser Playwright selector hardening

## Decisions (Best-Practice Aligned)
- Prefer deterministic UI anchors (`data-testid` and visible container scoping) over brittle global text-first selectors in cross-browser E2E.
- Use panic-safe JSON fallback in infra serialization boundary to avoid whole-flow interruption from marshal panics.
- Keep Lark scenario suite as executable acceptance gate (`go run ./cmd/alex lark scenario run ...`) with JSON/MD artifacts for traceability.

## Execution Steps
- [x] Implement network-style memory doc semantics and cross-reference rules in AGENTS/CLAUDE/reference docs.
- [x] Add panic-safe JSON fallback and targeted unit tests in `internal/shared/json`.
- [x] Fix Lark reset command scenario to match `/new` behavior.
- [x] Align mock subagent events and mock run id wiring in Web conversation flow.
- [x] Stabilize Playwright tests across desktop + mobile projects using visible-panel scoped selectors.
- [x] Run Web Playwright matrix for affected E2E specs.
- [x] Run Lark scenario end-to-end suite with JSON/MD reports.
- [x] Run full repo pre-push checks (lint/build/tests).

## Validation Log
- `pnpm playwright test e2e/console-layout.spec.ts e2e/artifact-gallery.spec.ts e2e/attachment-deduplication.spec.ts e2e/image-attachment.spec.ts e2e/markdown-line-height.spec.ts e2e/markdown-table.spec.ts e2e/subagent-events.spec.ts --reporter=line`
  - Result: 66 passed, 0 failed.
- `go run ./cmd/alex lark scenario run --dir tests/scenarios/lark --json-out tmp/lark-scenarios.json --md-out tmp/lark-scenarios.md`
  - Result: total=11 passed=11 failed=0.
- `./scripts/pre-push.sh`
  - Result: all checks passed.

## Rollback
- Revert branch commits in reverse order if regression appears:
  1. Web E2E selector/test harness updates
  2. Mock stream/session event handling updates
  3. JSON fallback and Lark scenario expectation updates
  4. Memory docs network semantics updates
