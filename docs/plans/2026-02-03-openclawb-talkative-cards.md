# Plan: OpenClaw Talkativeness + Lark Card Defaults (2026-02-03)

## Goal
- Research why OpenClaw agents can feel overly talkative/proactive.
- Default Lark replies to text (no cards) except system-failure cards.
- Nudge default persona to share more useful detail without forcing cards.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Gather web evidence on OpenClaw proactive/talkative behavior and summarize findings.
2. Update Lark card defaults to errors-only; align docs/examples.
3. Adjust default persona guidance to encourage more information.
4. Add/adjust tests if needed; run full lint + tests.

## Progress
- 2026-02-03: Plan created; engineering practices reviewed.
- 2026-02-03: Added default Lark card errors-only test; updated defaults + docs; adjusted persona verbosity guidance.
- 2026-02-03: Ran `make fmt`, `make vet`, `make test`.
