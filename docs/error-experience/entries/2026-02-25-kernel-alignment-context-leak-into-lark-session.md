# Kernel alignment context leaked into normal Lark session prompt

**Date:** 2026-02-25
**Severity:** High
**Category:** Context Boundary / Prompt Assembly

## What happened

Normal Lark user sessions received kernel-only alignment payload (`GOAL.md` + kernel objective block) in the first `system` message, inflating the prompt to ~80k+ estimated tokens.

## Root cause

1. Kernel alignment context was wired into shared preparation/context assembly path.
2. Injection lacked runtime boundary gating (`unattended` vs normal interactive run).
3. Tests covered injection success but not the negative boundary (must not inject for normal Lark sessions).

## Fix / mitigation

1. Gate kernel alignment injection strictly behind `appcontext.IsUnattendedContext(ctx)`.
2. Add two regression tests:
   - non-unattended session: no injection + provider not called.
   - unattended session: injection occurs.
3. Add postmortem/checklist mechanism under `docs/postmortems/` for cross-channel leakage incidents.

## Validation

- `go test ./internal/app/agent/preparation -count=1`

## Lessons

- Any runtime-only context injection must have an explicit boundary condition and dual-path tests (positive + negative).
- Cross-layer feature wiring (DI -> preparation -> context) needs leakage checks by channel/runtime mode.

## Metadata
- id: err-2026-02-25-kernel-context-leak
- tags: [error, kernel, lark, context-boundary, regression]
- links:
  - docs/postmortems/incidents/2026-02-25-kernel-alignment-context-leak-into-lark.md
  - docs/postmortems/checklists/incident-prevention-checklist.md
