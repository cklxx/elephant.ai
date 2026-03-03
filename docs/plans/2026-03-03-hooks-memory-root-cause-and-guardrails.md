# 2026-03-03 Hooks + Memory Root Cause and Guardrails

## Context
- User reported `alex-server` memory blow-up.
- Recent logs show high `/api/hooks/claude-code` traffic, but raw request count alone cannot explain multi-GB growth.
- Local evidence shows giant event-history JSONL lines (40-80MB each) in `_server/events`.

## Goals
1. Prevent duplicate hook delivery caused by overlapping hook configs.
2. Add server-side dedupe guard in hooks bridge to suppress near-simultaneous duplicate events.
3. Bound event-history payload size at persistence boundary to avoid giant workflow snapshots.
4. Validate with focused tests and quantitative evidence.

## Plan
1. Implement hook dedupe guard in `internal/delivery/server/hooks_bridge.go` and add tests.
2. Implement history payload slimming/truncation in `internal/delivery/server/app/event_payload_sanitize.go` and add tests.
3. De-duplicate `.claude` hook config entries to avoid double forwarding.
4. Run targeted Go tests for touched packages.
5. Summarize proof: request counts vs per-event payload sizes vs observed memory behavior.

## Progress
- [x] Root cause evidence collected from live logs and event-history files.
- [x] Hook dedupe guard implemented.
- [x] History payload slimming implemented.
- [x] Hook config de-duplicated.
- [x] Tests green.
- [x] Final quantitative analysis delivered.

## Phase 2 (Direct Optimization + Verification)
### Additional Goals
1. Eliminate the remaining giant history lines caused by typed `workflow.WorkflowSnapshot` / `workflow.NodeSnapshot` payloads bypassing map-based sanitization.
2. Reduce repeated heavy persistence for context snapshot diagnostic events by capping and sanitizing message payloads.
3. Add focused regression tests that reproduce the previous blow-up pattern and verify bounded persisted payload size.

### Execution Plan
1. Extend history sanitizer to handle typed workflow/node snapshots directly, producing bounded summaries instead of full node outputs.
2. Sanitize persisted `domain.Event` payloads (especially `workflow.diagnostic.context_snapshot`) by truncating message text and dropping heavy nested fields.
3. Add/expand tests in `event_broadcaster_history_test.go` to cover:
   - typed workflow snapshot sanitization path
   - diagnostic context snapshot sanitization path
4. Run targeted package tests (`server/app`, `server`, `server/http`) to confirm behavior and prevent regressions.

### Progress
- [x] Typed workflow/node sanitizer path implemented.
- [x] Domain diagnostic context snapshot sanitizer implemented.
- [x] Regression tests added.
- [x] Targeted tests green.
