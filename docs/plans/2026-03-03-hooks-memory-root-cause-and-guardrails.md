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
