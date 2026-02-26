# 2026-02-26 Context Overflow Budget + Progress Cadence

## Goal
- Fix recurring `context_length_exceeded` where think-step fails before meaningful compression.
- Ensure long foreground Lark tasks send periodic, human-readable progress updates with recent tool-call summaries.

## Problem Statement
- Current preflight budget enforcement estimates only message tokens, but request payload also includes tool definitions/schemas.
- This can under-estimate true input size, so compression may not trigger before upstream rejects with context overflow.
- Slow progress summary listener currently sends a one-shot update (default 30s), not multi-stage cadence.

## Plan
- [completed] Add request-level budget partitioning (`total -> tools + message budget`) in ReAct think path.
- [completed] Keep existing compression chain but enforce against message budget after tool-cost subtraction.
- [completed] Add diagnostics log fields for budget breakdown.
- [completed] Upgrade slow summary cadence to `30s -> 60s -> 180s -> every 180s`.
- [completed] Add “recent tool calls (humanized)” section in periodic summaries.
- [completed] Add/adjust unit tests for budget partitioning and repeated summary dispatch.
- [completed] Run targeted tests, then run mandatory code review skill and fix P0/P1 if any.

## Validation Targets
- `go test ./internal/domain/agent/react -count=1`
- `go test ./internal/delivery/channels/lark -run SlowProgressSummaryListener -count=1`
- `python3 skills/code-review/run.py '{"action":"review"}'`

## Progress Log
- 2026-02-26: Completed pre-work checklist on `main` (`git diff --stat`, `git log --oneline -10`), workspace clean.
- 2026-02-26: Reviewed engineering guide and loaded latest memory/error/good-experience entries.
- 2026-02-26: Confirmed current logic gap: preflight compression budget uses only message estimate and does not include tool-schema token cost.
- 2026-02-26: Added budget split in `react` think path (`total`, `tool_tokens`, `message_limit`) and passed message budget into preflight enforcement.
- 2026-02-26: Added context budget tests for tool-token subtraction and floor behavior.
- 2026-02-26: Upgraded Lark slow summary from one-shot timer to recurring cadence (`30s -> 60s -> 180s -> 180s...`) and added humanized recent tool summary block.
- 2026-02-26: Added cadence + repeated-send + humanized-summary tests.
- 2026-02-26: Validation passed:
  - `go test ./internal/domain/agent/react -count=1`
  - `go test ./internal/delivery/channels/lark -count=1`
  - `./scripts/pre-push.sh`
